package orders

import (
	"client/api"
	"context"
	"time"
)

// Repository defines the interface for order storage operations
type Repository interface {
	// CreateOrder creates a new order and payment transaction in a database transaction
	// transactionID is the external transaction ID from payment service
	CreateOrder(ctx context.Context, order *api.Order, transactionID string) error

	// GetOrderByID retrieves an order by its ID
	GetOrderByID(ctx context.Context, orderID string) (*api.Order, error)

	// GetOrderByOfferID retrieves an order by offer ID (for idempotency check)
	GetOrderByOfferID(ctx context.Context, offerID string) (*api.Order, error)

	// FinishOrder finalizes order: updates finish_time, duration_seconds, total_amount, status,
	// and appends payment transactions (CLEAR and REFUND(unhold)) inside a DB transaction.
	// chargeTxID is an optional external transaction id for CLEAR (can be empty).
	// chargeSuccess/unholdSuccess control statuses of inserted payment transactions.
	FinishOrder(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error
}
