package jobs

import (
	"client/api"
	"context"
	"errors"
	"testing"
	"time"
)

type mockCleanupRepository struct {
	getOldOrdersFunc   func(ctx context.Context, olderThan time.Duration) ([]*api.Order, error)
	deleteOrdersFunc   func(ctx context.Context, orderIDs []string) error
	getOldOrdersCalled bool
	deleteOrdersCalled bool
	getOldOrdersParam  time.Duration
	deleteOrdersParam  []string
}

func (m *mockCleanupRepository) CreateOrder(ctx context.Context, order *api.Order, transactionID string) error {
	return nil
}

func (m *mockCleanupRepository) GetOrderByID(ctx context.Context, orderID string) (*api.Order, error) {
	return nil, nil
}

func (m *mockCleanupRepository) GetOrderByOfferID(ctx context.Context, offerID string) (*api.Order, error) {
	return nil, nil
}

func (m *mockCleanupRepository) FinishOrder(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
	return nil
}

func (m *mockCleanupRepository) GetOldOrders(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
	m.getOldOrdersCalled = true
	m.getOldOrdersParam = olderThan
	if m.getOldOrdersFunc != nil {
		return m.getOldOrdersFunc(ctx, olderThan)
	}
	return nil, nil
}

func (m *mockCleanupRepository) DeleteOrders(ctx context.Context, orderIDs []string) error {
	m.deleteOrdersCalled = true
	m.deleteOrdersParam = orderIDs
	if m.deleteOrdersFunc != nil {
		return m.deleteOrdersFunc(ctx, orderIDs)
	}
	return nil
}

func createTestOrder(id, userID, scooterID, offerID string, status api.OrderStatus, createdAt time.Time) *api.Order {
	pricePerMinute := 10
	priceUnlock := 20
	deposit := 100
	currentAmount := 120

	return &api.Order{
		Id:             id,
		UserId:         userID,
		ScooterId:      scooterID,
		OfferId:        offerID,
		Status:         status,
		PricePerMinute: &pricePerMinute,
		PriceUnlock:    &priceUnlock,
		Deposit:        &deposit,
		CurrentAmount:  &currentAmount,
		StartTime:      createdAt,
	}
}

func TestOrderCleanupJob_Cleanup_Success(t *testing.T) {
	mockRepo := &mockCleanupRepository{}

	now := time.Now()
	oldTime := now.Add(-2 * 24 * time.Hour)

	oldOrders := []*api.Order{
		createTestOrder("order-1", "user-1", "scooter-1", "offer-1", api.FINISHED, oldTime),
		createTestOrder("order-2", "user-2", "scooter-2", "offer-2", api.FINISHED, oldTime),
		createTestOrder("order-3", "user-3", "scooter-3", "offer-3", api.CANCELLED, oldTime),
	}

	mockRepo.getOldOrdersFunc = func(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
		return oldOrders, nil
	}

	mockRepo.deleteOrdersFunc = func(ctx context.Context, orderIDs []string) error {
		return nil
	}

	job := NewOrderCleanupJob(mockRepo, 24*time.Hour, 1*time.Hour)
	job.cleanup()

	if !mockRepo.getOldOrdersCalled {
		t.Error("GetOldOrders should have been called")
	}

	if mockRepo.getOldOrdersParam != 24*time.Hour {
		t.Errorf("GetOldOrders called with wrong olderThan: got %v, want %v", mockRepo.getOldOrdersParam, 24*time.Hour)
	}

	if !mockRepo.deleteOrdersCalled {
		t.Error("DeleteOrders should have been called")
	}

	if len(mockRepo.deleteOrdersParam) != len(oldOrders) {
		t.Errorf("DeleteOrders called with wrong number of IDs: got %d, want %d", len(mockRepo.deleteOrdersParam), len(oldOrders))
	}

	expectedIDs := []string{"order-1", "order-2", "order-3"}
	for i, expectedID := range expectedIDs {
		if i >= len(mockRepo.deleteOrdersParam) {
			t.Errorf("Missing order ID in delete call: %s", expectedID)
			continue
		}
		if mockRepo.deleteOrdersParam[i] != expectedID {
			t.Errorf("DeleteOrders called with wrong ID at index %d: got %s, want %s", i, mockRepo.deleteOrdersParam[i], expectedID)
		}
	}
}

func TestOrderCleanupJob_Cleanup_NoOldOrders(t *testing.T) {
	mockRepo := &mockCleanupRepository{}

	mockRepo.getOldOrdersFunc = func(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
		return []*api.Order{}, nil
	}

	job := NewOrderCleanupJob(mockRepo, 24*time.Hour, 1*time.Hour)
	job.cleanup()

	if !mockRepo.getOldOrdersCalled {
		t.Error("GetOldOrders should have been called")
	}

	if mockRepo.deleteOrdersCalled {
		t.Error("DeleteOrders should NOT have been called when no old orders")
	}
}

func TestOrderCleanupJob_Cleanup_GetOldOrdersError(t *testing.T) {
	mockRepo := &mockCleanupRepository{}

	expectedError := errors.New("database connection failed")
	mockRepo.getOldOrdersFunc = func(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
		return nil, expectedError
	}

	job := NewOrderCleanupJob(mockRepo, 24*time.Hour, 1*time.Hour)

	job.cleanup()

	if !mockRepo.getOldOrdersCalled {
		t.Error("GetOldOrders should have been called")
	}

	if mockRepo.deleteOrdersCalled {
		t.Error("DeleteOrders should NOT have been called when GetOldOrders returns error")
	}
}

func TestOrderCleanupJob_Cleanup_DeleteOrdersError(t *testing.T) {
	mockRepo := &mockCleanupRepository{}

	now := time.Now()
	oldTime := now.Add(-2 * 24 * time.Hour)

	oldOrders := []*api.Order{
		createTestOrder("order-1", "user-1", "scooter-1", "offer-1", api.FINISHED, oldTime),
		createTestOrder("order-2", "user-2", "scooter-2", "offer-2", api.FINISHED, oldTime),
	}

	mockRepo.getOldOrdersFunc = func(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
		return oldOrders, nil
	}

	expectedError := errors.New("delete failed")
	mockRepo.deleteOrdersFunc = func(ctx context.Context, orderIDs []string) error {
		return expectedError
	}

	job := NewOrderCleanupJob(mockRepo, 24*time.Hour, 1*time.Hour)

	job.cleanup()

	if !mockRepo.getOldOrdersCalled {
		t.Error("GetOldOrders should have been called")
	}

	if !mockRepo.deleteOrdersCalled {
		t.Error("DeleteOrders should have been called")
	}

	if len(mockRepo.deleteOrdersParam) != len(oldOrders) {
		t.Errorf("DeleteOrders called with wrong number of IDs: got %d, want %d", len(mockRepo.deleteOrdersParam), len(oldOrders))
	}
}

func TestOrderCleanupJob_LogOrders(t *testing.T) {
	mockRepo := &mockCleanupRepository{}

	now := time.Now()
	oldTime := now.Add(-2 * 24 * time.Hour)

	orders := []*api.Order{
		createTestOrder("order-1", "user-1", "scooter-1", "offer-1", api.FINISHED, oldTime),
		createTestOrder("order-2", "user-2", "scooter-2", "offer-2", api.CANCELLED, oldTime),
		createTestOrder("order-3", "user-3", "scooter-3", "offer-3", api.PAYMENTFAILED, oldTime),
	}

	orderWithNil := &api.Order{
		Id:        "order-4",
		UserId:    "user-4",
		ScooterId: "scooter-4",
		OfferId:   "offer-4",
		Status:    api.ACTIVE,
		StartTime: oldTime,
	}
	orders = append(orders, orderWithNil)

	mockRepo.getOldOrdersFunc = func(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
		return orders, nil
	}

	mockRepo.deleteOrdersFunc = func(ctx context.Context, orderIDs []string) error {
		return nil
	}

	job := NewOrderCleanupJob(mockRepo, 24*time.Hour, 1*time.Hour)

	job.cleanup()

	if !mockRepo.getOldOrdersCalled {
		t.Error("GetOldOrders should have been called")
	}

	if !mockRepo.deleteOrdersCalled {
		t.Error("DeleteOrders should have been called")
	}

	if len(mockRepo.deleteOrdersParam) != len(orders) {
		t.Errorf("DeleteOrders called with wrong number of IDs: got %d, want %d", len(mockRepo.deleteOrdersParam), len(orders))
	}
}

func TestOrderCleanupJob_NewOrderCleanupJob(t *testing.T) {
	mockRepo := &mockCleanupRepository{}
	olderThan := 48 * time.Hour
	interval := 2 * time.Hour

	job := NewOrderCleanupJob(mockRepo, olderThan, interval)

	if job == nil {
		t.Fatal("NewOrderCleanupJob returned nil")
	}

	if job.repo != mockRepo {
		t.Error("Repository not set correctly")
	}

	if job.olderThan != olderThan {
		t.Errorf("olderThan not set correctly: got %v, want %v", job.olderThan, olderThan)
	}

	if job.interval != interval {
		t.Errorf("interval not set correctly: got %v, want %v", job.interval, interval)
	}

	if job.stopChan == nil {
		t.Error("stopChan not initialized")
	}

	if job.doneChan == nil {
		t.Error("doneChan not initialized")
	}
}
