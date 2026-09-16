package accrual

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DenisChesnokov/gophermart/internal/model"
)

func TestGetOrder_OK(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.AccrualOrder{
			Order:   "79927398713",
			Status:  "PROCESSED",
			Accrual: 500,
		})
	}))
	defer mock.Close()

	client := NewClient(mock.URL)
	order, err := client.GetOrder(context.Background(), "79927398713")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if order == nil {
		t.Fatal("expected order, got nil")
	}
	if order.Status != "PROCESSED" {
		t.Fatalf("expected status PROCESSED, got %s", order.Status)
	}
	if order.Accrual != 500 {
		t.Fatalf("expected accrual 500, got %f", order.Accrual)
	}
}

func TestGetOrder_Processing(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.AccrualOrder{
			Order:  "79927398713",
			Status: "PROCESSING",
		})
	}))
	defer mock.Close()

	client := NewClient(mock.URL)
	order, err := client.GetOrder(context.Background(), "79927398713")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if order.Status != "PROCESSING" {
		t.Fatalf("expected status PROCESSING, got %s", order.Status)
	}
}

func TestGetOrder_Invalid(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.AccrualOrder{
			Order:  "79927398713",
			Status: "INVALID",
		})
	}))
	defer mock.Close()

	client := NewClient(mock.URL)
	order, err := client.GetOrder(context.Background(), "79927398713")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if order.Status != "INVALID" {
		t.Fatalf("expected status INVALID, got %s", order.Status)
	}
}

func TestGetOrder_NotRegistered(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mock.Close()

	client := NewClient(mock.URL)
	order, err := client.GetOrder(context.Background(), "79927398713")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if order != nil {
		t.Fatalf("expected nil, got %v", order)
	}
}

func TestGetOrder_RateLimit(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer mock.Close()

	client := NewClient(mock.URL)
	_, err := client.GetOrder(context.Background(), "79927398713")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected RateLimitError, got %T", err)
	}
	if rateLimitErr.RetryAfter.Seconds() != 30 {
		t.Fatalf("expected RetryAfter=30s, got %s", rateLimitErr.RetryAfter)
	}
}
