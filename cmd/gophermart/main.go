package main

import (
	"log"
	"net/http"

	"github.com/DenisChesnokov/gophermart/internal/config"
	"github.com/DenisChesnokov/gophermart/internal/handler"
	"github.com/DenisChesnokov/gophermart/internal/repository"
	"github.com/DenisChesnokov/gophermart/internal/service"
)

func main() {
	cfg := config.New()
	cfg.ParseFlags() // флаги парсятся первыми
	cfg.ParseEnv()   // ENV перезаписывает флаги

	log.Printf("starting server on %s", cfg.ServerAddress)

	db, err := repository.NewPostgresDB(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := repository.RunMigrations(cfg.DatabaseDSN); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Printf("migrations complete")

	jwtSecret := service.GenerateJWTSecret()

	authService := service.NewAuthService(db, jwtSecret)
	h := handler.New(authService)
	r := handler.NewRouter(h, authService)

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
