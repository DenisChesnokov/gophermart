package repository

import (
	"context"
	"errors"
	"testing"
)

func TestCreateUser(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// Создаём пользователя
	id, err := db.CreateUser(ctx, "testuser", "hash123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	// Дубликат — ожидаем ErrUserAlreadyExists
	_, err = db.CreateUser(ctx, "testuser", "hash456")
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestGetUserByLogin(t *testing.T) {
	db, cleanup := setupTestPostgres(t)
	defer cleanup()

	ctx := context.Background()

	// Несуществующий пользователь — ожидаем ErrUserNotFound
	_, _, err := db.GetUserByLogin(ctx, "nonexistent")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	// Создаём и находим
	db.CreateUser(ctx, "alice", "hash")
	id, hash, err := db.GetUserByLogin(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByLogin: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}
	if hash != "hash" {
		t.Fatalf("expected hash 'hash', got %q", hash)
	}
}
