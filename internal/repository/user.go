package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrUserNotFound = errors.New("user not found")

func (db *PostgresDB) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	var id int64
	err := db.pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login, passwordHash,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, ErrUserAlreadyExists
		}
		return 0, err // другие ошибки пробрасываем
	}
	return id, nil
}

func (db *PostgresDB) GetUserByLogin(ctx context.Context, login string) (int64, string, error) {
	var id int64
	var passwordHash string
	err := db.pool.QueryRow(ctx,
		`SELECT id, password_hash FROM users WHERE login = $1`,
		login,
	).Scan(&id, &passwordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrUserNotFound
		}
		return 0, "", err // другие ошибки пробрасываем
	}
	return id, passwordHash, nil
}
