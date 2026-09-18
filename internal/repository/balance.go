package repository

import (
	"context"
	"errors"

	"github.com/DenisChesnokov/gophermart/internal/model"
	"github.com/jackc/pgx/v5"
)

var ErrInsufficientFunds = errors.New("insufficient funds")

func (db *PostgresDB) GetBalance(ctx context.Context, userID int64) (float64, float64, error) {
	var current, withdrawn float64
	err := db.pool.QueryRow(ctx,
		`SELECT current, withdrawn FROM balances WHERE user_id = $1`, userID,
	).Scan(&current, &withdrawn)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return current, withdrawn, nil
}

func (db *PostgresDB) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Проверяем баланс
	var current float64
	err = tx.QueryRow(ctx,
		`SELECT current FROM balances WHERE user_id = $1 FOR UPDATE`, userID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientFunds
		}
		return err
	}

	if current < sum {
		return ErrInsufficientFunds
	}

	// Обновляем баланс
	_, err = tx.Exec(ctx,
		`UPDATE balances SET current = current - $1, withdrawn = withdrawn + $1 WHERE user_id = $2`, sum, userID,
	)
	if err != nil {
		return err
	}

	// Записываем withdrawal
	_, err = tx.Exec(ctx,
		`INSERT INTO withdrawals (user_id, "order", sum) VALUES ($1, $2, $3)`, userID, order, sum,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (db *PostgresDB) CreateBalance(ctx context.Context, userID int64) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO balances (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID,
	)
	return err
}

func (db *PostgresDB) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT "order", sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var withdrawals []model.Withdrawal
	for rows.Next() {
		var w model.Withdrawal
		if err := rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return withdrawals, nil
}

func (db *PostgresDB) AddBalance(ctx context.Context, userID int64, amount float64) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO balances (user_id, current) VALUES ($1, $2)
		 ON CONFLICT (user_id) DO UPDATE SET current = balances.current + $2`,
		userID, amount,
	)
	return err
}

func (db *PostgresDB) CreditAccrual(ctx context.Context, userID int64, amount float64) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE balances SET current = current + $1 WHERE user_id = $2`,
		amount, userID,
	)
	return err
}
