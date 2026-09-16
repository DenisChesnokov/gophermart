package worker

import (
	"context"
	"log"
	"time"

	"github.com/DenisChesnokov/gophermart/internal/accrual"
	"github.com/DenisChesnokov/gophermart/internal/repository"
)

type AccrualWorker struct {
	client  *accrual.Client
	db      *repository.PostgresDB
	pollInt time.Duration
	limit   int
}

func NewAccrualWorker(client *accrual.Client, db *repository.PostgresDB, pollInterval time.Duration) *AccrualWorker {
	return &AccrualWorker{
		client:  client,
		db:      db,
		pollInt: pollInterval,
		limit:   10,
	}
}

func (w *AccrualWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInt)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("accrual worker stopped")
			return
		case <-ticker.C:
			w.processPendingOrders(ctx)
		}
	}
}

func (w *AccrualWorker) processPendingOrders(ctx context.Context) {
	orders, err := w.db.GetPendingOrders(ctx, w.limit)
	if err != nil {
		log.Printf("failed to get pending orders: %v", err)
		return
	}

	for _, order := range orders {
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.processOrder(ctx, order.Number)
	}
}

func (w *AccrualWorker) processOrder(ctx context.Context, orderNumber string) {
	resp, err := w.client.GetOrder(ctx, orderNumber)
	if err != nil {
		if rateLimitErr, ok := err.(*accrual.RateLimitError); ok {
			log.Printf("rate limit hit, waiting %s", rateLimitErr.RetryAfter)
			time.Sleep(rateLimitErr.RetryAfter)
			return
		}
		log.Printf("failed to get order %s: %v", orderNumber, err)
		return
	}

	if resp == nil {
		return
	}

	switch resp.Status {
	case "REGISTERED", "PROCESSING":
		w.db.UpdateOrderStatus(ctx, orderNumber, "PROCESSING", 0)

	case "INVALID":
		w.db.UpdateOrderStatus(ctx, orderNumber, "INVALID", 0)

	case "PROCESSED":
		userID, _, err := w.db.GetOrderByNumber(ctx, orderNumber)
		if err != nil {
			log.Printf("failed to get order user: %v", err)
			return
		}
		w.db.UpdateOrderStatus(ctx, orderNumber, "PROCESSED", resp.Accrual)
		if resp.Accrual > 0 {
			w.db.CreditAccrual(ctx, userID, resp.Accrual)
		}
	}
}
