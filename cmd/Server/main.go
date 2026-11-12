package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"xazavier/internal/config"
	"xazavier/internal/httpapi"
	"xazavier/internal/repository"
)

func main() {
	cfg := config.Load()
	if len(cfg.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters long")
	}

	var store repository.Store
	if cfg.DatabaseURL == "memory" {
		if strings.EqualFold(cfg.Env, "production") || strings.EqualFold(cfg.Env, "prod") {
			log.Fatal("DATABASE_URL='memory' is not allowed in production environment")
		}
		log.Println("DATABASE_URL set to 'memory': running with in-memory store & demo dataset")
		memStore := repository.NewMemoryStore()
		repository.SeedMemoryStore(memStore)
		store = memStore
	} else if strings.TrimSpace(cfg.DatabaseURL) == "" {
		if strings.EqualFold(cfg.Env, "production") || strings.EqualFold(cfg.Env, "prod") {
			log.Fatal("DATABASE_URL is empty in production: a valid PostgreSQL connection string is required")
		}
		log.Println("DATABASE_URL is empty: falling back to in-memory store & demo dataset (dev mode only)")
		memStore := repository.NewMemoryStore()
		repository.SeedMemoryStore(memStore)
		store = memStore
	} else {
		if cfg.AutoMigrate {
			if err := repository.AutoMigrate(context.Background(), cfg.DatabaseURL); err != nil {
				log.Fatalf("auto migrate: %v", err)
			}
		}
		pgStore, err := repository.NewPostgresStore(context.Background(), cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("connect database: %v", err)
		}
		defer pgStore.Close()
		store = pgStore
	}

	r := httpapi.NewServer(cfg, store)
	log.Printf("API server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
