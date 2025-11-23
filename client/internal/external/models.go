package external

// PaymentHoldRequest represents a request to hold money for an order
type PaymentHoldRequest struct {
	UserID  string `json:"user_id"`
	OrderID string `json:"order_id"`
	Amount  int    `json:"amount"`
}

// PaymentHoldResponse represents a response from payment hold operation
type PaymentHoldResponse struct {
	TransactionID string `json:"transaction_id"`
	Success       bool   `json:"ok"`
}

// ScooterData is returned by external scooters API
type ScooterData struct {
	ID     string `json:"id"`
	ZoneID string `json:"zone_id"`
	Charge int    `json:"charge"`
}

// TariffZone is returned by external zone API
type TariffZone struct {
	ID              string `json:"id"`
	PricePerMinute  int    `json:"price_per_minute"`
	PriceUnlock     int    `json:"price_unlock"`
	DefaultDeposit  int    `json:"default_deposit"`
}

// UserProfile is returned by external users API
type UserProfile struct {
	ID               string `json:"id"`
	HasSubscription  bool   `json:"has_subscribtion"`
	Trusted          bool   `json:"trusted"`
}

// DynamicConfigs carries dynamic configuration values
type DynamicConfigs struct {
	// Multiplicative surge factor, e.g., 1.2
	Surge float64 `json:"surge"`
	// Discount multiplier applied to per-minute price if low charge, e.g., 0.7
	LowChargeDiscount float64 `json:"low_charge_discount"`
	// Charge threshold (percent) below which discount is applied
	LowChargeThresholdPercent int `json:"low_charge_threshold_percent"`
	// Free ride threshold in seconds (period not billed at start)
	IncompleteRideThresholdSeconds int `json:"incomplete_ride_threshold_seconds"`
}
