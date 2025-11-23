package external

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// GetScooterData fetches scooter charge and zone id
func (c *Client) GetScooterData(ctx context.Context, scooterID string) (*ScooterData, error) {
	q := url.Values{}
	q.Set("id", scooterID)
	resp, err := c.makeRequest(ctx, http.MethodGet, "/scooter-data?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scooter-data unexpected status: %d", resp.StatusCode)
	}
	var data ScooterData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetTariffZoneData fetches tariff zone parameters
func (c *Client) GetTariffZoneData(ctx context.Context, zoneID string) (*TariffZone, error) {
	q := url.Values{}
	q.Set("id", zoneID)
	resp, err := c.makeRequest(ctx, http.MethodGet, "/tariff-zone-data?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tariff-zone unexpected status: %d", resp.StatusCode)
	}
	var data TariffZone
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetUserProfile fetches user subscription/trust flags
func (c *Client) GetUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	q := url.Values{}
	q.Set("id", userID)
	resp, err := c.makeRequest(ctx, http.MethodGet, "/user-profile?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user-profile unexpected status: %d", resp.StatusCode)
	}
	var data UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

// GetConfigs fetches dynamic configuration
func (c *Client) GetConfigs(ctx context.Context) (*DynamicConfigs, error) {
	resp, err := c.makeRequest(ctx, http.MethodGet, "/configs", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("configs unexpected status: %d", resp.StatusCode)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	// Map to struct with defaults per ADR (fallbacks)
	cfg := &DynamicConfigs{
		Surge:                          1.0,
		LowChargeDiscount:              1.0,
		LowChargeThresholdPercent:      0,
		IncompleteRideThresholdSeconds: 0,
	}
	if v, ok := m["surge"].(float64); ok {
		cfg.Surge = v
	}
	if v, ok := m["low_charge_discount"].(float64); ok {
		cfg.LowChargeDiscount = v
	}
	if v, ok := m["low_charge_threshold_percent"].(float64); ok {
		cfg.LowChargeThresholdPercent = int(v)
	}
	if v, ok := m["incomplete_ride_threshold_seconds"].(float64); ok {
		cfg.IncompleteRideThresholdSeconds = int(v)
	}
	return cfg, nil
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
