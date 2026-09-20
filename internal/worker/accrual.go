package worker

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/DenisChesnokov/gophermart/internal/accrual"
	"github.com/DenisChesnokov/gophermart/internal/repository"
	"golang.org/x/sync/errgroup"
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

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	for _, order := range orders {
		g.Go(func() error {
			return w.processOrder(gctx, order.Number)
		})
	}

	if err := g.Wait(); err != nil {
		var rateLimitErr *accrual.RateLimitError
		if errors.As(err, &rateLimitErr) {
			log.Printf("batch stopped due to rate limit, waiting %s", rateLimitErr.RetryAfter)
			waitCtx, cancel := context.WithTimeout(ctx, rateLimitErr.RetryAfter)
			defer cancel()
			<-waitCtx.Done()
		} else {
			log.Printf("batch processing error: %v", err)
		}
	}
}

func (w *AccrualWorker) processOrder(ctx context.Context, orderNumber string) error {
	resp, err := w.client.GetOrder(ctx, orderNumber)
	if err != nil {
		return err
	}

	if resp == nil {
		return nil
	}

	switch resp.Status {
	case "REGISTERED", "PROCESSING":
		if err := w.db.UpdateOrderStatus(ctx, orderNumber, "PROCESSING", 0); err != nil {
			log.Printf("failed to update order status: %v", err)
			return err
		}
	case "INVALID":
		if err := w.db.UpdateOrderStatus(ctx, orderNumber, "INVALID", 0); err != nil {
			log.Printf("failed to update order status: %v", err)
			return err
		}
	case "PROCESSED":
		userID, _, err := w.db.GetOrderByNumber(ctx, orderNumber)
		if err != nil {
			log.Printf("failed to get order user: %v", err)
			return err
		}
		if err := w.db.ProcessOrderAccrual(ctx, orderNumber, userID, "PROCESSED", resp.Accrual); err != nil {
			log.Printf("failed to process accrual for order %s: %v", orderNumber, err)
			return err
		}
	}
	return nil
}
