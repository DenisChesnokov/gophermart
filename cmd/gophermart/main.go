package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	cfg.ParseEnv()   // ENV парсится первым
	cfg.ParseFlags() // флаги перезаписывают ENV

	if cfg.DatabaseDSN == "" {
		log.Fatal("DATABASE_URI is required")
	}

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var workerDone sync.WaitGroup
	workerDone.Add(1)
	go func() {
		defer workerDone.Done()
		accrualWorker.Run(ctx)
	}()

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	log.Printf("server started on %s", cfg.ServerAddress)

	<-ctx.Done()
	log.Println("shutting down...")

	if err := <-serverErr; err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	workerDone.Wait()
}
