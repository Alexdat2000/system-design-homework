package external

type PaymentHoldRequest struct {
	UserID  string `json:"user_id"`
	OrderID string `json:"order_id"`
	Amount  int    `json:"amount"`
}

type PaymentHoldResponse struct {
	TransactionID string `json:"transaction_id"`
	Success       bool   `json:"ok"`
}

type DynamicConfigs struct {
	Surge                          float64 `json:"surge"`
	LowChargeDiscount              float64 `json:"low_charge_discount"`
	LowChargeThresholdPercent      int     `json:"low_charge_threshold_percent"`
	IncompleteRideThresholdSeconds int     `json:"incomplete_ride_threshold_seconds"`
}
