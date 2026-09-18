package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/DenisChesnokov/gophermart/internal/model"
)

var ErrInvalidOrderNumber = errors.New("invalid order number")

type OrderRepo interface {
	CreateOrder(ctx context.Context, userID int64, number string) (string, error)
	GetOrdersByUserID(ctx context.Context, userID int64) ([]model.Order, error)
}

type OrderService struct {
	repo OrderRepo
}

func NewOrderService(repo OrderRepo) *OrderService {
	return &OrderService{repo: repo}
}

func (s *OrderService) UploadOrder(ctx context.Context, userID int64, number string) (string, error) {
	if !ValidateLuhn(number) {
		return "422", ErrInvalidOrderNumber
	}
	return s.repo.CreateOrder(ctx, userID, number)
}

func (s *OrderService) GetOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	return s.repo.GetOrdersByUserID(ctx, userID)
}

func ValidateLuhn(number string) bool {
	if len(number) == 0 {
		return false
	}

	sum := 0
	nDigits := len(number)
	parity := nDigits % 2

	for i := 0; i < nDigits; i++ {
		digit, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}

	return sum%10 == 0
}
