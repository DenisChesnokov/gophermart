package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func SetupTestPostgres(t *testing.T) (*PostgresDB, func()) {
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

	pg, err := NewPostgresDB(dsn)
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("NewPostgresDB: %v", err)
	}

	// Вычисляем путь к миграциям относительно файла теста
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
