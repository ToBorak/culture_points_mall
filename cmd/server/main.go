package main

import (
	"context"
	"fmt"
	"log"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
	"github.com/standardsoftware/culture_points_mall/internal/platform/storage"
	"github.com/standardsoftware/culture_points_mall/internal/router"

	activitiesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/activities/repository"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	achvrepo "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/repository"
	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	agenthandler "github.com/standardsoftware/culture_points_mall/internal/modules/agent/handler"
	agentrepo "github.com/standardsoftware/culture_points_mall/internal/modules/agent/repository"
	agentsvc "github.com/standardsoftware/culture_points_mall/internal/modules/agent/service"
	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/tools"
	lbsvc "github.com/standardsoftware/culture_points_mall/internal/modules/leaderboard/service"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	valuesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/values/repository"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

func main() {
	cfg, err := config.Load("./configs")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := storage.NewMySQL(cfg.MySQL)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	redisClient, err := storage.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	bus := dingtalk.NewBus()
	mock := dingtalk.NewMock(db, bus)
	var ding dingtalk.Client = mock

	llmClient, err := llm.NewFromConfig(cfg)
	if err != nil {
		log.Fatalf("llm: %v", err)
	}

	// 启动时灌入默认维度
	vsvc := valuessvc.New(valuesrepo.New(db))
	if err := vsvc.SeedDefaults(context.Background(), cfg.Seed.DefaultTenantID, "./configs/value_dimensions.yaml"); err != nil {
		log.Printf("seed values warn: %v", err)
	}

	// 装配 Agent 编排器
	pointsRepo := pointsrepo.New(db)
	pointsSvc := pointssvc.New(db, pointsRepo, vsvc, redisClient)
	lbSvc := lbsvc.New(db)
	actRepo := activitiesrepo.New(db)
	actSvc := activitiessvc.New(actRepo, vsvc)
	achvSvcInst := achvsvc.New(&achvsvc.Wrap{Inner: achvrepo.New(db)}, pointsSvc, vsvc)

	toolReg := tools.NewRegistry()
	tools.RegisterBusiness(toolReg, tools.BusinessDeps{
		Activities:   actSvc,
		Points:       pointsSvc,
		Leaderboard:  lbSvc,
		Achievements: achvSvcInst,
	})
	tools.RegisterDingtalk(toolReg, tools.DingDeps{Client: ding})

	orchestrator := agentsvc.NewOrchestrator(llmClient, toolReg, vsvc)
	aRepo := agentrepo.New(db)
	agentH := agenthandler.New(orchestrator, aRepo)

	r := router.Build(router.Deps{DB: db, Cfg: cfg, DingMock: mock, DingBus: bus, DingClient: ding, LLM: llmClient, AgentHandler: agentH, Redis: redisClient})
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
