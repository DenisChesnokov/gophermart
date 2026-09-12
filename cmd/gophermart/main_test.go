package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DenisChesnokov/gophermart/internal/handler"
	"github.com/DenisChesnokov/gophermart/internal/model"
	"github.com/DenisChesnokov/gophermart/internal/repository"
	"github.com/DenisChesnokov/gophermart/internal/service"

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

func setupTestServer(t *testing.T) (http.Handler, *repository.PostgresDB, func()) {
	t.Helper()

	db, cleanup := setupTestDB(t)
	authService := service.NewAuthService(db, "test-secret")
	orderService := service.NewOrderService(db)
	balanceService := service.NewBalanceService(db)
	h := handler.New(authService, orderService, balanceService)
	r := handler.NewRouter(h, authService)

	return r, db, cleanup
}

func registerAndGetToken(t *testing.T, r http.Handler) string {
	t.Helper()

	body := `{"login":"testuser","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	for _, c := range rec.Result().Cookies() {
		if c.Name == "token" {
			return c.Value
		}
	}
	t.Fatal("token cookie not found")
	return ""
}

func TestRegister(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"login":"testuser","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected cookie to be set")
	}

	found := false
	for _, c := range cookies {
		if c.Name == "token" {
			found = true
			if c.Value == "" {
				t.Fatal("token cookie is empty")
			}
		}
	}
	if !found {
		t.Fatal("token cookie not found")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"login":"testuser","password":"password123"}`

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	req2 := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec2.Code)
	}
}

func TestLogin(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"login":"testuser","password":"password123"}`

	regReq := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	r.ServeHTTP(regRec, regReq)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", loginRec.Code)
	}
}

func TestUploadOrder(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("79927398713"))
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestUploadOrderInvalid(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("123456789"))
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestGetOrders(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("79927398713"))
	uploadReq.Header.Set("Content-Type", "text/plain")
	uploadReq.AddCookie(&http.Cookie{Name: "token", Value: token})
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, uploadReq)

	getReq := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	getReq.AddCookie(&http.Cookie{Name: "token", Value: token})
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}

	var orders []model.Order
	if err := json.NewDecoder(getRec.Body).Decode(&orders); err != nil {
		t.Fatalf("failed to decode orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].Number != "79927398713" {
		t.Fatalf("expected order number 79927398713, got %s", orders[0].Number)
	}
}

func TestGetBalance(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var balance model.Balance
	if err := json.NewDecoder(rec.Body).Decode(&balance); err != nil {
		t.Fatalf("failed to decode balance: %v", err)
	}
	if balance.Current != 0 {
		t.Fatalf("expected current=0, got %f", balance.Current)
	}
}

func TestGetBalanceUnauthorized(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestWithdraw(t *testing.T) {
	r, db, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	// Пополняем баланс
	db.AddBalance(context.Background(), 1, 1000)

	body := `{"order":"79927398713","sum":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestWithdrawInvalidOrder(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	body := `{"order":"123456789","sum":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestWithdrawInsufficientFunds(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	// Баланс = 0, пробуем списать
	body := `{"order":"79927398713","sum":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}
}

func TestGetWithdrawals(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Пока нет списаний — ожидаем 204
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestGetWithdrawalsEmpty(t *testing.T) {
	r, _, cleanup := setupTestServer(t)
	defer cleanup()

	token := registerAndGetToken(t, r)

	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}
