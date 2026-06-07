package main

import (
	"context"
	"fmt"
	"log"
	"time"

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
	mallrepo "github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
	mallsvc "github.com/standardsoftware/culture_points_mall/internal/modules/mall/service"
	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/worker"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	usersrepo "github.com/standardsoftware/culture_points_mall/internal/modules/users/repository"
	usersvc "github.com/standardsoftware/culture_points_mall/internal/modules/users/service"
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
	ding := dingtalk.New(cfg.DingTalk, redisClient, mock)

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

	// 启动 TCC 兜底 sweeper（定时取消过期 freeze）
	mRepo := mallrepo.New(db)
	mallSvc := mallsvc.New(mRepo, pointsSvc, vsvc)
	sweeperCtx, sweeperCancel := context.WithCancel(context.Background())
	defer sweeperCancel()
	sweeper := &worker.FreezeSweeper{Repo: mRepo, Points: pointsSvc}
	sweeper.Start(sweeperCtx, 5*time.Second)

	toolReg := tools.NewRegistry()
	tools.RegisterBusiness(toolReg, tools.BusinessDeps{
		Activities:   actSvc,
		Points:       pointsSvc,
		Leaderboard:  lbSvc,
		Achievements: achvSvcInst,
	})
	tools.RegisterMall(toolReg, tools.MallDeps{Mall: mallSvc})
	tools.RegisterDingtalk(toolReg, tools.DingDeps{Client: ding})
	usersSvc := usersvc.New(usersrepo.New(db))
	tools.RegisterInteractive(toolReg, tools.InteractiveDeps{Users: usersSvc, Achievements: achvSvcInst})
	tools.RegisterBatch(toolReg, tools.BatchDeps{Points: pointsSvc, Users: usersSvc, Activities: actSvc})

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
