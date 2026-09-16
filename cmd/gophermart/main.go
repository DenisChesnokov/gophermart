package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/DenisChesnokov/gophermart/internal/accrual"
	"github.com/DenisChesnokov/gophermart/internal/config"
	"github.com/DenisChesnokov/gophermart/internal/handler"
	"github.com/DenisChesnokov/gophermart/internal/repository"
	"github.com/DenisChesnokov/gophermart/internal/service"
	"github.com/DenisChesnokov/gophermart/internal/worker"
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
	orderService := service.NewOrderService(db)
	balanceService := service.NewBalanceService(db)

	h := handler.New(authService, orderService, balanceService)
	r := handler.NewRouter(h, authService)

	// Запуск accrual worker
	accrualClient := accrual.NewClient(cfg.AccrualSystemAddress)
	accrualWorker := worker.NewAccrualWorker(accrualClient, db, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go accrualWorker.Run(ctx)

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
