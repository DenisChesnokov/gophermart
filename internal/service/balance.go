package service

import (
	"context"
	"errors"

	"github.com/DenisChesnokov/gophermart/internal/model"
	"github.com/DenisChesnokov/gophermart/internal/repository"
)

type BalanceService struct {
	repo *repository.PostgresDB
}

func NewBalanceService(repo *repository.PostgresDB) *BalanceService {
	return &BalanceService{repo: repo}
}

func (s *BalanceService) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	current, withdrawn, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return model.Balance{}, err
	}
	return model.Balance{Current: current, Withdrawn: withdrawn}, nil
}

func (s *BalanceService) Withdraw(ctx context.Context, userID int64, order string, sum float64) (string, error) {
	if !ValidateLuhn(order) {
		return "422", ErrInvalidOrderNumber
	}
	err := s.repo.Withdraw(ctx, userID, order, sum)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientFunds) {
			return "402", err
		}
		return "500", err
	}
	return "200", nil
}

func (s *BalanceService) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	return s.repo.GetWithdrawals(ctx, userID)
}
