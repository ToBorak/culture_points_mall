package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/mcp"
	"github.com/standardsoftware/culture_points_mall/internal/platform/storage"

	"github.com/standardsoftware/culture_points_mall/internal/modules/agent/tools"

	activitiesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/activities/repository"
	activitiessvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	achvrepo "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/repository"
	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	lbsvc "github.com/standardsoftware/culture_points_mall/internal/modules/leaderboard/service"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	valuesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/values/repository"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

func main() {
	cfg, err := config.Load("./configs")
	if err != nil {
		log.Fatal(err)
	}
	db, err := storage.NewMySQL(cfg.MySQL)
	if err != nil {
		log.Fatal(err)
	}
	bus := dingtalk.NewBus()
	mockDing := dingtalk.NewMock(db, bus)

	valuesS := valuessvc.New(valuesrepo.New(db))
	pointsS := pointssvc.New(db, pointsrepo.New(db), valuesS, nil)
	lbS := lbsvc.New(db)
	actS := activitiessvc.New(activitiesrepo.New(db), valuesS)
	achvWrap := &achvsvc.Wrap{Inner: achvrepo.New(db)}
	achvS := achvsvc.New(achvWrap, pointsS, valuesS)

	reg := tools.NewRegistry()
	tools.RegisterBusiness(reg, tools.BusinessDeps{Activities: actS, Points: pointsS, Leaderboard: lbS, Achievements: achvS})
	tools.RegisterDingtalk(reg, tools.DingDeps{Client: mockDing})

	r := gin.Default()
	mcp.NewServer(reg, simpleAuth(cfg)).Register(r)

	addr := fmt.Sprintf(":%d", cfg.MCP.Port)
	log.Printf("mcp server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func simpleAuth(cfg *config.Config) func(string) (int64, bool) {
	return func(token string) (int64, bool) {
		if token == "" {
			return 0, false
		}
		return cfg.Seed.DefaultTenantID, true
	}
}
