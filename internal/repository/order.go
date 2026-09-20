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

func (db *PostgresDB) GetPendingOrders(ctx context.Context, limit int) ([]model.Order, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT number, status, accrual, uploaded_at FROM orders
		 WHERE status IN ('NEW', 'PROCESSING')
		 ORDER BY uploaded_at ASC
		 LIMIT $1`, limit,
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

	return orders, rows.Err()
}

func (db *PostgresDB) UpdateOrderStatus(ctx context.Context, number string, status string, accrual float64) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`,
		status, accrual, number,
	)
	return err
}

func (db *PostgresDB) GetOrderByNumber(ctx context.Context, number string) (int64, string, error) {
	var userID int64
	var status string
	err := db.pool.QueryRow(ctx,
		`SELECT user_id, status FROM orders WHERE number = $1`, number,
	).Scan(&userID, &status)
	if err != nil {
		return 0, "", err
	}
	return userID, status, nil
}

func (db *PostgresDB) ProcessOrderAccrual(ctx context.Context, number string, userID int64, status string, accrual float64) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE orders SET status = $1, accrual = $2 WHERE number = $3`,
		status, accrual, number,
	)
	if err != nil {
		return err
	}

	if accrual > 0 {
		_, err = tx.Exec(ctx,
			`UPDATE balances SET current = current + $1 WHERE user_id = $2`,
			accrual, userID,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
