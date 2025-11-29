package external

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PaymentsClientInterface interface {
	HoldMoneyForOrder(ctx context.Context, req *PaymentHoldRequest) (*PaymentHoldResponse, error)
	UnholdMoneyForOrder(ctx context.Context, orderID string) error
	ChargeMoneyForOrder(ctx context.Context, orderID string, amount int) error
}

type ExternalClient struct {
	apiClient *ClientWithResponses
}

var _ PaymentsClientInterface = (*ExternalClient)(nil)

func NewExternalClient(baseURL string) (*ExternalClient, error) {
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	apiClient, err := NewClientWithResponses(baseURL, WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &ExternalClient{apiClient: apiClient}, nil
}

func (c *ExternalClient) HoldMoneyForOrder(ctx context.Context, req *PaymentHoldRequest) (*PaymentHoldResponse, error) {
	requestBody := PostHoldMoneyForOrderJSONRequestBody{
		UserId:  &req.UserID,
		OrderId: &req.OrderID,
		Amount:  &req.Amount,
	}

	resp, err := c.apiClient.PostHoldMoneyForOrderWithResponse(ctx, requestBody)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("hold-money-for-order unexpected status: %d", resp.StatusCode())
	}

	var responseBody struct {
		TransactionID string `json:"transaction_id"`
		Ok            bool   `json:"ok"`
	}
	if err := json.Unmarshal(resp.Body, &responseBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &PaymentHoldResponse{
		TransactionID: responseBody.TransactionID,
		Success:       responseBody.Ok,
	}, nil
}

func (c *ExternalClient) ChargeMoneyForOrder(ctx context.Context, orderID string, amount int) error {
	emptyUserID := ""
	requestBody := PostClearMoneyForOrderJSONRequestBody{
		OrderId: &orderID,
		Amount:  &amount,
		UserId:  &emptyUserID,
	}

	resp, err := c.apiClient.PostClearMoneyForOrderWithResponse(ctx, requestBody)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("clear-money-for-order unexpected status: %d", resp.StatusCode())
	}

	return nil
}

func (c *ExternalClient) UnholdMoneyForOrder(ctx context.Context, orderID string) error {
	emptyUserID := ""
	requestBody := PostUnholdMoneyForOrderJSONRequestBody{
		OrderId: &orderID,
		UserId:  &emptyUserID,
	}

	resp, err := c.apiClient.PostUnholdMoneyForOrderWithResponse(ctx, requestBody)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unhold-money-for-order unexpected status: %d", resp.StatusCode())
	}

	return nil
}

func (c *ExternalClient) GetScooterData(ctx context.Context, scooterID string) (*ScooterData, error) {
	params := GetScooterDataParams{Id: scooterID}
	resp, err := c.apiClient.GetScooterDataWithResponse(ctx, &params)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("scooter-data unexpected status: %d", resp.StatusCode())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("scooter-data: empty response")
	}

	return resp.JSON200, nil
}

func (c *ExternalClient) GetTariffZoneData(ctx context.Context, zoneID string) (*TariffZone, error) {
	params := GetTariffZoneDataParams{Id: zoneID}
	resp, err := c.apiClient.GetTariffZoneDataWithResponse(ctx, &params)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("tariff-zone unexpected status: %d", resp.StatusCode())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("tariff-zone: empty response")
	}

	return resp.JSON200, nil
}

func (c *ExternalClient) GetUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	params := GetUserProfileParams{Id: userID}
	resp, err := c.apiClient.GetUserProfileWithResponse(ctx, &params)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("user-profile unexpected status: %d", resp.StatusCode())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("user-profile: empty response")
	}

	return resp.JSON200, nil
}

func (c *ExternalClient) GetConfigs(ctx context.Context) (*DynamicConfigs, error) {
	resp, err := c.apiClient.GetConfigsWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("configs unexpected status: %d", resp.StatusCode())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("configs: empty response")
	}

	cfg := &DynamicConfigs{
		Surge:                          1.2,
		LowChargeDiscount:              0.7,
		LowChargeThresholdPercent:      28,
		IncompleteRideThresholdSeconds: 5,
	}

	m := *resp.JSON200
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
