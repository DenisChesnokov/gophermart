package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	const maxRetries = 3

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second * time.Duration(attempt)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber), nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			var order model.AccrualOrder
			decErr := json.NewDecoder(resp.Body).Decode(&order)
			resp.Body.Close()
			if decErr != nil {
				return nil, decErr
			}
			return &order, nil

		case http.StatusNoContent:
			resp.Body.Close()
			return nil, nil

		case http.StatusTooManyRequests:
			resp.Body.Close()
			seconds, perr := strconv.Atoi(resp.Header.Get("Retry-After"))
			if perr != nil {
				log.Printf("failed to parse Retry-After header %q: %v, defaulting to 60s",
					resp.Header.Get("Retry-After"), perr)
				seconds = 60
			}
			return nil, &RateLimitError{RetryAfter: time.Duration(seconds) * time.Second}

		default:
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil, fmt.Errorf("accrual service returned status %d", resp.StatusCode)
			}
			lastErr = fmt.Errorf("accrual service returned status %d", resp.StatusCode)
			continue
		}
	}

	return nil, lastErr
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, retry after %s", e.RetryAfter)
}
