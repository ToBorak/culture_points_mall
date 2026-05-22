package main

import (
	"context"
	"fmt"
	"log"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/storage"
	"github.com/standardsoftware/culture_points_mall/internal/router"

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
	bus := dingtalk.NewBus()
	mock := dingtalk.NewMock(db, bus)

	// 启动时灌入默认维度
	vsvc := valuessvc.New(valuesrepo.New(db))
	if err := vsvc.SeedDefaults(context.Background(), cfg.Seed.DefaultTenantID, "./configs/value_dimensions.yaml"); err != nil {
		log.Printf("seed values warn: %v", err)
	}

	r := router.Build(router.Deps{DB: db, Cfg: cfg, DingMock: mock, DingBus: bus})
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
