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
