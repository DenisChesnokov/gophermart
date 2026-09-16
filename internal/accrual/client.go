package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DenisChesnokov/gophermart/internal/model"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "http://" + baseURL
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *Client) GetOrder(ctx context.Context, orderNumber string) (*model.AccrualOrder, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var order model.AccrualOrder
		if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
			return nil, err
		}
		return &order, nil

	case http.StatusNoContent:
		return nil, nil

	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		seconds, err := strconv.Atoi(retryAfter)
		if err != nil {
			seconds = 60
		}
		return nil, &RateLimitError{RetryAfter: time.Duration(seconds) * time.Second}

	default:
		return nil, fmt.Errorf("accrual service returned status %d", resp.StatusCode)
	}
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, retry after %s", e.RetryAfter)
}
