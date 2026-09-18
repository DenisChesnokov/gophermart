package repository

import (
	"context"
	"errors"
	"testing"
)

func TestGetBalance(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)

	current, withdrawn, err := db.GetBalance(ctx, 1)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if current != 0 {
		t.Fatalf("expected current=0, got %f", current)
	}
	if withdrawn != 0 {
		t.Fatalf("expected withdrawn=0, got %f", withdrawn)
	}
}

func TestWithdraw(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)
	db.AddBalance(ctx, 1, 1000)

	err := db.Withdraw(ctx, 1, "79927398713", 500)
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}

	current, withdrawn, err := db.GetBalance(ctx, 1)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if current != 500 {
		t.Fatalf("expected current=500, got %f", current)
	}
	if withdrawn != 500 {
		t.Fatalf("expected withdrawn=500, got %f", withdrawn)
	}
}

func TestWithdrawInsufficientFunds(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)

	err := db.Withdraw(ctx, 1, "79927398713", 100)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestGetWithdrawals(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)
	db.AddBalance(ctx, 1, 1000)

	db.Withdraw(ctx, 1, "79927398713", 100)
	db.Withdraw(ctx, 1, "12345678903", 200)

	withdrawals, err := db.GetWithdrawals(ctx, 1)
	if err != nil {
		t.Fatalf("GetWithdrawals: %v", err)
	}
	if len(withdrawals) != 2 {
		t.Fatalf("expected 2 withdrawals, got %d", len(withdrawals))
	}
}

func TestGetWithdrawalsEmpty(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)

	withdrawals, err := db.GetWithdrawals(ctx, 1)
	if err != nil {
		t.Fatalf("GetWithdrawals: %v", err)
	}
	if len(withdrawals) != 0 {
		t.Fatalf("expected 0 withdrawals, got %d", len(withdrawals))
	}
}

func TestCreditAccrual(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateBalance(ctx, 1)

	err := db.CreditAccrual(ctx, 1, 500)
	if err != nil {
		t.Fatalf("CreditAccrual: %v", err)
	}

	current, withdrawn, err := db.GetBalance(ctx, 1)
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if current != 500 {
		t.Fatalf("expected current=500, got %f", current)
	}
	if withdrawn != 0 {
		t.Fatalf("expected withdrawn=0, got %f", withdrawn)
	}
}
