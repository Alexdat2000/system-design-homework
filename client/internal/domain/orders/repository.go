package orders

import (
	"client/api"
	"context"
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
}
