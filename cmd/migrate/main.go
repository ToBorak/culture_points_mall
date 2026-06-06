package main

import (
	"context"
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
		seeder := &migrate.Seeder{
			DB:              db,
			DefaultTenantID: cfg.Seed.DefaultTenantID,
			WelcomeBonus:    cfg.Seed.WelcomeBonus,
			DemoData:        cfg.Seed.DemoData,
		}
		if err := seeder.Run(context.Background()); err != nil {
			log.Fatalf("seed: %v", err)
		}
		bonus := seeder.WelcomeBonus
		if bonus <= 0 {
			bonus = 100000
		}
		log.Printf("seed done (each user got %d welcome points)", bonus)
	default:
		log.Fatalf("unknown action: %s", *action)
	}
}
