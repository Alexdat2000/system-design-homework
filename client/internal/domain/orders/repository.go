package orders

import (
	"client/api"
	"context"
	"time"
)

type Repository interface {
	CreateOrder(ctx context.Context, order *api.Order, transactionID string) error

	GetOrderByID(ctx context.Context, orderID string) (*api.Order, error)

	GetOrderByOfferID(ctx context.Context, offerID string) (*api.Order, error)

	FinishOrder(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error

	GetOldOrders(ctx context.Context, olderThan time.Duration) ([]*api.Order, error)

	DeleteOrders(ctx context.Context, orderIDs []string) error
}
