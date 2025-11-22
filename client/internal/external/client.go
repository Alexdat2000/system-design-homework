package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PaymentsClientInterface defines the interface for payment operations
type PaymentsClientInterface interface {
	HoldMoneyForOrder(ctx context.Context, req *PaymentHoldRequest) (*PaymentHoldResponse, error)
	UnholdMoneyForOrder(ctx context.Context, orderID string) error
	ChargeMoneyForOrder(ctx context.Context, orderID string, amount int) error
}

// Client handles communication with external services
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Ensure Client implements PaymentsClientInterface
var _ PaymentsClientInterface = (*Client)(nil)

// NewClient creates a new external API client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// HoldMoneyForOrder holds (freezes) money on user's card for an order
// This is a stub implementation that always succeeds
func (c *Client) HoldMoneyForOrder(ctx context.Context, req *PaymentHoldRequest) (*PaymentHoldResponse, error) {
	// Stub: always return success
	// In real implementation, this would make HTTP POST to payments service
	return &PaymentHoldResponse{
		TransactionID: fmt.Sprintf("txn-hold-%s", req.OrderID),
		Success:       true,
	}, nil
}

// ChargeMoneyForOrder charges money from user's card for an order
func (c *Client) ChargeMoneyForOrder(ctx context.Context, orderID string, amount int) error {
	// Stub: always return success
	// In real implementation, this would make HTTP POST to payments service
	return nil
}

// UnholdMoneyForOrder releases (unfreezes) money on user's card
func (c *Client) UnholdMoneyForOrder(ctx context.Context, orderID string) error {
	// Stub: always return success
	// In real implementation, this would make HTTP POST to payments service
	return nil
}

// makeRequest is a helper for making HTTP requests (for future use)
func (c *Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}
