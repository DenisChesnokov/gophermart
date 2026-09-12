package repository

import (
	"context"
	"testing"
)

func TestCreateOrder(t *testing.T) {
	db, cleanup := SetupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём пользователя для заказа
	db.CreateUser(ctx, "user1", "hash1")

	// Новый заказ — ожидаем 202
	code, err := db.CreateOrder(ctx, 1, "79927398713")
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if code != "202" {
		t.Fatalf("expected code 202, got %q", code)
	}

	// Тот же заказ от того же пользователя — ожидаем 200 + ErrOrderAlreadyExistsByUser
	code, err = db.CreateOrder(ctx, 1, "79927398713")
	if err != ErrOrderAlreadyExistsByUser {
		t.Fatalf("expected ErrOrderAlreadyExistsByUser, got %v", err)
	}
	if code != "200" {
		t.Fatalf("expected code 200, got %q", code)
	}

	// Тот же заказ от другого пользователя — ожидаем 409 + ErrOrderExistsByOtherUser
	db.CreateUser(ctx, "user2", "hash2")
	code, err = db.CreateOrder(ctx, 2, "79927398713")
	if err != ErrOrderExistsByOtherUser {
		t.Fatalf("expected ErrOrderExistsByOtherUser, got %v", err)
	}
	if code != "409" {
		t.Fatalf("expected code 409, got %q", code)
	}
}

func TestGetOrdersByUserID(t *testing.T) {
	db, cleanup := SetupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	db.CreateUser(ctx, "user1", "hash1")
	db.CreateUser(ctx, "user2", "hash2")

	db.CreateOrder(ctx, 1, "79927398713")
	db.CreateOrder(ctx, 1, "12345678903")
	db.CreateOrder(ctx, 2, "4242424242424242")

	orders, err := db.GetOrdersByUserID(ctx, 1)
	if err != nil {
		t.Fatalf("GetOrdersByUserID: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	orders, err = db.GetOrdersByUserID(ctx, 2)
	if err != nil {
		t.Fatalf("GetOrdersByUserID: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
}
