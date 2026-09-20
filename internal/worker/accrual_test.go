package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"path/filepath"

	"github.com/DenisChesnokov/gophermart/internal/accrual"
	"github.com/DenisChesnokov/gophermart/internal/model"
	"github.com/DenisChesnokov/gophermart/internal/repository"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*repository.PostgresDB, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("failed to get connection string: %v", err)
	}

	pg, err := repository.NewPostgresDB(dsn)
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("NewPostgresDB: %v", err)
	}

	rootDir, err := filepath.Abs("../..")
	if err != nil {
		pg.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("failed to get root dir: %v", err)
	}
	migrationsPath := "file://" + filepath.Join(rootDir, "migrations")

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		pg.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("failed to create migrate instance: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		pg.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("failed to run migrations: %v", err)
	}

	cleanup := func() {
		pg.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return pg, cleanup
}

func TestAccrualWorker_HappyPath(t *testing.T) {
	// 1. Mock accrual server — возвращает PROCESSED с accrual=500
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.AccrualOrder{
			Order:   "79927398713",
			Status:  "PROCESSED",
			Accrual: 500,
		})
	}))
	defer mock.Close()

	// 2. Тестовая БД
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// 3. Создать пользователя + заказ (status=NEW)
	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)
	db.CreateOrder(ctx, 1, "79927398713")

	// 4. Запустить worker на 1 итерацию
	client := accrual.NewClient(mock.URL)
	w := NewAccrualWorker(client, db, 1*time.Second)

	// Вызываем processPendingOrders напрямую (one-shot)
	w.processPendingOrders(ctx)

	// 5. Проверить: order status = PROCESSED, balance = 500
	orders, err := db.GetOrdersByUserID(ctx, 1)
	if err != nil {
		t.Fatalf("GetOrdersByUserID: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].Status != "PROCESSED" {
		t.Fatalf("expected status PROCESSED, got %s", orders[0].Status)
	}
	if orders[0].Accrual != 500 {
		t.Fatalf("expected accrual 500, got %f", orders[0].Accrual)
	}

	current, _, err := db.GetBalance(ctx, 1)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if current != 500 {
		t.Fatalf("expected balance=500, got %f", current)
	}
}

func TestAccrualWorker_Processing(t *testing.T) {
	// Mock — возвращает PROCESSING
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.AccrualOrder{
			Order:  "79927398713",
			Status: "PROCESSING",
		})
	}))
	defer mock.Close()

	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)
	db.CreateOrder(ctx, 1, "79927398713")

	client := accrual.NewClient(mock.URL)
	w := NewAccrualWorker(client, db, 1*time.Second)
	w.processPendingOrders(ctx)

	// Статус должен измениться на PROCESSING
	orders, _ := db.GetOrdersByUserID(ctx, 1)
	if orders[0].Status != "PROCESSING" {
		t.Fatalf("expected status PROCESSING, got %s", orders[0].Status)
	}

	// Баланс не должен измениться
	current, _, _ := db.GetBalance(ctx, 1)
	if current != 0 {
		t.Fatalf("expected balance=0, got %f", current)
	}
}

func TestAccrualWorker_Invalid(t *testing.T) {
	// Mock — возвращает INVALID
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.AccrualOrder{
			Order:  "79927398713",
			Status: "INVALID",
		})
	}))
	defer mock.Close()

	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)
	db.CreateOrder(ctx, 1, "79927398713")

	client := accrual.NewClient(mock.URL)
	w := NewAccrualWorker(client, db, 1*time.Second)
	w.processPendingOrders(ctx)

	orders, _ := db.GetOrdersByUserID(ctx, 1)
	if orders[0].Status != "INVALID" {
		t.Fatalf("expected status INVALID, got %s", orders[0].Status)
	}

	current, _, _ := db.GetBalance(ctx, 1)
	if current != 0 {
		t.Fatalf("expected balance=0, got %f", current)
	}
}
