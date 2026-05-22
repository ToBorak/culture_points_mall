package main

import (
	"flag"
	"log"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/migrate"
	"github.com/standardsoftware/culture_points_mall/internal/platform/storage"
)

func main() {
	action := flag.String("action", "up", "up | seed")
	configPath := flag.String("config", "./configs", "config dir")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	db, err := storage.NewMySQL(cfg.MySQL)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	r := &migrate.Runner{DB: db, Dir: "./migrations"}
	switch *action {
	case "up":
		if err := r.Up(); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
	case "seed":
		log.Println("seed not implemented yet · 将在 Phase 2 Task 2.16 实现完整 seed")
	default:
		log.Fatalf("unknown action: %s", *action)
	}
}
