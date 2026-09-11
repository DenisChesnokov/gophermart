package repository

import (
	"context"
	"errors"

	"github.com/DenisChesnokov/gophermart/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrOrderAlreadyExistsByUser = errors.New("order already exists by this user")
	ErrOrderExistsByOtherUser   = errors.New("order exists by other user")
	ErrOrderNotFound            = errors.New("order not found")
)

func (db *PostgresDB) CreateOrder(ctx context.Context, userID int64, number string) (string, error) {
	var existingUserID int64
	err := db.pool.QueryRow(ctx,
		`SELECT user_id FROM orders WHERE number = $1`, number,
	).Scan(&existingUserID)

	if err == nil {
		if existingUserID == userID {
			return "200", ErrOrderAlreadyExistsByUser
		}
		return "409", ErrOrderExistsByOtherUser
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return "500", err
	}

	_, err = db.pool.Exec(ctx,
		`INSERT INTO orders (user_id, number, status) VALUES ($1, $2, 'NEW')`,
		userID, number,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "409", ErrOrderExistsByOtherUser
		}
		return "500", err
	}

	return "202", nil
}

func (db *PostgresDB) GetOrdersByUserID(ctx context.Context, userID int64) ([]model.Order, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}
