package orders

import (
	"client/api"
	"client/internal/external"
	"context"
	"errors"
	"testing"
	"time"
)

type mockOrderRepository struct {
	createOrderFunc       func(ctx context.Context, order *api.Order, transactionID string) error
	getOrderByIDFunc      func(ctx context.Context, orderID string) (*api.Order, error)
	getOrderByOfferIDFunc func(ctx context.Context, offerID string) (*api.Order, error)
}

func (m *mockOrderRepository) CreateOrder(ctx context.Context, order *api.Order, transactionID string) error {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, order, transactionID)
	}
	return nil
}

func (m *mockOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*api.Order, error) {
	if m.getOrderByIDFunc != nil {
		return m.getOrderByIDFunc(ctx, orderID)
	}
	return nil, nil
}

func (m *mockOrderRepository) FinishOrder(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
	return nil
}

func (m *mockOrderRepository) GetOrderByOfferID(ctx context.Context, offerID string) (*api.Order, error) {
	if m.getOrderByOfferIDFunc != nil {
		return m.getOrderByOfferIDFunc(ctx, offerID)
	}
	return nil, nil
}

func (m *mockOrderRepository) GetOldOrders(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
	return nil, nil
}

func (m *mockOrderRepository) DeleteOrders(ctx context.Context, orderIDs []string) error {
	return nil
}

type mockOfferRepository struct {
	getOfferFunc        func(ctx context.Context, offerID string) (*api.Offer, error)
	markOfferAsUsedFunc func(ctx context.Context, offerID string) (bool, error)
}

func (m *mockOfferRepository) GetOffer(ctx context.Context, offerID string) (*api.Offer, error) {
	if m.getOfferFunc != nil {
		return m.getOfferFunc(ctx, offerID)
	}
	return nil, nil
}

func (m *mockOfferRepository) MarkOfferAsUsed(ctx context.Context, offerID string) (bool, error) {
	if m.markOfferAsUsedFunc != nil {
		return m.markOfferAsUsedFunc(ctx, offerID)
	}
	return true, nil
}

func (m *mockOfferRepository) GetOfferByUserScooter(ctx context.Context, userID, scooterID string) (*api.Offer, error) {
	return nil, nil
}

func (m *mockOfferRepository) SaveOffer(ctx context.Context, offer *api.Offer) error {
	return nil
}

func (m *mockOfferRepository) SetOfferByUserScooter(ctx context.Context, userID, scooterID, offerID string) error {
	return nil
}

type mockPaymentsClient struct {
	holdMoneyFunc   func(ctx context.Context, req *external.PaymentHoldRequest) (*external.PaymentHoldResponse, error)
	unholdMoneyFunc func(ctx context.Context, orderID string) error
	chargeMoneyFunc func(ctx context.Context, orderID string, amount int) error
}

func (m *mockPaymentsClient) HoldMoneyForOrder(ctx context.Context, req *external.PaymentHoldRequest) (*external.PaymentHoldResponse, error) {
	if m.holdMoneyFunc != nil {
		return m.holdMoneyFunc(ctx, req)
	}
	return &external.PaymentHoldResponse{
		TransactionID: "txn-123",
		Success:       true,
	}, nil
}

func (m *mockPaymentsClient) UnholdMoneyForOrder(ctx context.Context, orderID string) error {
	if m.unholdMoneyFunc != nil {
		return m.unholdMoneyFunc(ctx, orderID)
	}
	return nil
}

func (m *mockPaymentsClient) ChargeMoneyForOrder(ctx context.Context, orderID string, amount int) error {
	if m.chargeMoneyFunc != nil {
		return m.chargeMoneyFunc(ctx, orderID, amount)
	}
	return nil
}

func TestService_CreateOrder_Success(t *testing.T) {
	now := time.Now()
	validOffer := &api.Offer{
		Id:             "offer-123",
		UserId:         "user-456",
		ScooterId:      "scooter-789",
		ZoneId:         "zone-1",
		Deposit:        100,
		PricePerMinute: 10,
		PriceUnlock:    20,
		ExpiresAt:      now.Add(5 * time.Minute),
		CreatedAt:      &now,
	}

	orderRepo := &mockOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			return nil, nil
		},
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
		createOrderFunc: func(ctx context.Context, order *api.Order, transactionID string) error {
			if order.OfferId != "offer-123" {
				t.Errorf("Unexpected offer ID: %s", order.OfferId)
			}
			if order.UserId != "user-456" {
				t.Errorf("Unexpected user ID: %s", order.UserId)
			}
			if order.Status != api.ACTIVE {
				t.Errorf("Expected status ACTIVE, got %s", order.Status)
			}
			if transactionID == "" {
				t.Error("Expected transaction ID to be set")
			}
			return nil
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			if offerID != "offer-123" {
				t.Errorf("Unexpected offer ID: %s", offerID)
			}
			return validOffer, nil
		},
		markOfferAsUsedFunc: func(ctx context.Context, offerID string) (bool, error) {
			return true, nil
		},
	}

	paymentsClient := &mockPaymentsClient{
		holdMoneyFunc: func(ctx context.Context, req *external.PaymentHoldRequest) (*external.PaymentHoldResponse, error) {
			if req.UserID != "user-456" {
				t.Errorf("Unexpected user ID: %s", req.UserID)
			}
			if req.Amount != 100 {
				t.Errorf("Unexpected amount: %d", req.Amount)
			}
			return &external.PaymentHoldResponse{
				TransactionID: "txn-123",
				Success:       true,
			}, nil
		},
	}

	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-new-123",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if order == nil {
		t.Fatal("Expected order, got nil")
	}
	if order.OfferId != "offer-123" {
		t.Errorf("Expected offer ID %s, got %s", "offer-123", order.OfferId)
	}
	if order.UserId != "user-456" {
		t.Errorf("Expected user ID %s, got %s", "user-456", order.UserId)
	}
	if order.ScooterId != "scooter-789" {
		t.Errorf("Expected scooter ID %s, got %s", "scooter-789", order.ScooterId)
	}
	if order.Status != api.ACTIVE {
		t.Errorf("Expected status ACTIVE, got %s", order.Status)
	}
	if order.Deposit == nil || *order.Deposit != 100 {
		t.Errorf("Expected deposit 100, got %v", order.Deposit)
	}
	if order.PricePerMinute == nil || *order.PricePerMinute != 10 {
		t.Errorf("Expected price per minute 10, got %v", order.PricePerMinute)
	}
	if order.PriceUnlock == nil || *order.PriceUnlock != 20 {
		t.Errorf("Expected price unlock 20, got %v", order.PriceUnlock)
	}
}

func TestService_CreateOrder_Idempotency(t *testing.T) {
	existingOrder := &api.Order{
		Id:      "order-existing",
		OfferId: "offer-123",
		UserId:  "user-456",
		Status:  api.ACTIVE,
	}

	orderRepo := &mockOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			if orderID == "order-existing" {
				return existingOrder, nil
			}
			return nil, nil
		},
	}

	offerRepo := &mockOfferRepository{}
	paymentsClient := &mockPaymentsClient{}

	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-existing",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if order == nil {
		t.Fatal("Expected order, got nil")
	}
	if order.Id != "order-existing" {
		t.Errorf("Expected existing order ID, got %s", order.Id)
	}
}

func TestService_CreateOrder_OfferNotFound(t *testing.T) {
	orderRepo := &mockOrderRepository{
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			return nil, nil
		},
	}

	paymentsClient := &mockPaymentsClient{}
	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-not-found",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err != ErrOfferNotFound {
		t.Errorf("Expected ErrOfferNotFound, got %v", err)
	}
	if order != nil {
		t.Errorf("Expected nil order, got %v", order)
	}
}

func TestService_CreateOrder_OfferExpired(t *testing.T) {
	now := time.Now()
	expiredOffer := &api.Offer{
		Id:        "offer-123",
		UserId:    "user-456",
		ExpiresAt: now.Add(-5 * time.Minute),
	}

	orderRepo := &mockOrderRepository{
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			return expiredOffer, nil
		},
	}

	paymentsClient := &mockPaymentsClient{}
	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-expired",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err != ErrOfferExpired {
		t.Errorf("Expected ErrOfferExpired, got %v", err)
	}
	if order != nil {
		t.Errorf("Expected nil order, got %v", order)
	}
}

func TestService_CreateOrder_InvalidUser(t *testing.T) {
	now := time.Now()
	offer := &api.Offer{
		Id:        "offer-123",
		UserId:    "user-456",
		ExpiresAt: now.Add(5 * time.Minute),
	}

	orderRepo := &mockOrderRepository{
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			return offer, nil
		},
	}

	paymentsClient := &mockPaymentsClient{}
	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-invalid-user",
		OfferID: "offer-123",
		UserID:  "user-999",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err != ErrInvalidUser {
		t.Errorf("Expected ErrInvalidUser, got %v", err)
	}
	if order != nil {
		t.Errorf("Expected nil order, got %v", order)
	}
}

func TestService_CreateOrder_OfferAlreadyUsed(t *testing.T) {
	now := time.Now()
	validOffer := &api.Offer{
		Id:        "offer-123",
		UserId:    "user-456",
		ExpiresAt: now.Add(5 * time.Minute),
	}

	orderRepo := &mockOrderRepository{
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			return validOffer, nil
		},
		markOfferAsUsedFunc: func(ctx context.Context, offerID string) (bool, error) {
			return false, nil
		},
	}

	paymentsClient := &mockPaymentsClient{}
	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-already-used",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err != ErrOfferAlreadyUsed {
		t.Errorf("Expected ErrOfferAlreadyUsed, got %v", err)
	}
	if order != nil {
		t.Errorf("Expected nil order, got %v", order)
	}
}

func TestService_CreateOrder_PaymentHoldFailed(t *testing.T) {
	now := time.Now()
	validOffer := &api.Offer{
		Id:        "offer-123",
		UserId:    "user-456",
		ExpiresAt: now.Add(5 * time.Minute),
		Deposit:   100,
	}

	orderRepo := &mockOrderRepository{
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			return validOffer, nil
		},
		markOfferAsUsedFunc: func(ctx context.Context, offerID string) (bool, error) {
			return true, nil
		},
	}

	paymentsClient := &mockPaymentsClient{
		holdMoneyFunc: func(ctx context.Context, req *external.PaymentHoldRequest) (*external.PaymentHoldResponse, error) {
			return &external.PaymentHoldResponse{
				Success: false,
			}, nil
		},
	}

	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-hold-failed",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if order != nil {
		t.Errorf("Expected nil order, got %v", order)
	}
}

func TestService_CreateOrder_PaymentHoldError(t *testing.T) {
	now := time.Now()
	validOffer := &api.Offer{
		Id:        "offer-123",
		UserId:    "user-456",
		ExpiresAt: now.Add(5 * time.Minute),
		Deposit:   100,
	}

	orderRepo := &mockOrderRepository{
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			return validOffer, nil
		},
		markOfferAsUsedFunc: func(ctx context.Context, offerID string) (bool, error) {
			return true, nil
		},
	}

	paymentsClient := &mockPaymentsClient{
		holdMoneyFunc: func(ctx context.Context, req *external.PaymentHoldRequest) (*external.PaymentHoldResponse, error) {
			return nil, errors.New("payment service unavailable")
		},
	}

	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-hold-error",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if order != nil {
		t.Errorf("Expected nil order, got %v", order)
	}
}

func TestService_CreateOrder_DatabaseError(t *testing.T) {
	now := time.Now()
	validOffer := &api.Offer{
		Id:        "offer-123",
		UserId:    "user-456",
		ExpiresAt: now.Add(5 * time.Minute),
		Deposit:   100,
	}

	orderRepo := &mockOrderRepository{
		getOrderByOfferIDFunc: func(ctx context.Context, offerID string) (*api.Order, error) {
			return nil, nil
		},
		createOrderFunc: func(ctx context.Context, order *api.Order, transactionID string) error {
			return errors.New("database error")
		},
	}

	offerRepo := &mockOfferRepository{
		getOfferFunc: func(ctx context.Context, offerID string) (*api.Offer, error) {
			return validOffer, nil
		},
		markOfferAsUsedFunc: func(ctx context.Context, offerID string) (bool, error) {
			return true, nil
		},
	}

	unholdCalled := false
	paymentsClient := &mockPaymentsClient{
		holdMoneyFunc: func(ctx context.Context, req *external.PaymentHoldRequest) (*external.PaymentHoldResponse, error) {
			return &external.PaymentHoldResponse{
				TransactionID: "txn-123",
				Success:       true,
			}, nil
		},
		unholdMoneyFunc: func(ctx context.Context, orderID string) error {
			unholdCalled = true
			return nil
		},
	}

	service := NewService(orderRepo, offerRepo, paymentsClient)

	req := &CreateOrderRequest{
		OrderID: "order-db-error",
		OfferID: "offer-123",
		UserID:  "user-456",
	}
	order, err := service.CreateOrder(context.Background(), req)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if order != nil {
		t.Errorf("Expected nil order, got %v", order)
	}
	if !unholdCalled {
		t.Error("Expected unhold to be called after database error")
	}
}
