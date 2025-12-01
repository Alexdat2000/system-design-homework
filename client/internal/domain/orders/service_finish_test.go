package orders

import (
	"client/api"
	"client/internal/external"
	"context"
	"errors"
	"testing"
	"time"
)

type mockFinishOrderRepository struct {
	getOrderByIDFunc    func(ctx context.Context, orderID string) (*api.Order, error)
	createOrderFunc     func(ctx context.Context, order *api.Order, transactionID string) error
	getOrderByOfferFunc func(ctx context.Context, offerID string) (*api.Order, error)
	finishOrderFunc     func(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error
}

func (m *mockFinishOrderRepository) CreateOrder(ctx context.Context, order *api.Order, transactionID string) error {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, order, transactionID)
	}
	return nil
}
func (m *mockFinishOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*api.Order, error) {
	if m.getOrderByIDFunc != nil {
		return m.getOrderByIDFunc(ctx, orderID)
	}
	return nil, nil
}
func (m *mockFinishOrderRepository) GetOrderByOfferID(ctx context.Context, offerID string) (*api.Order, error) {
	if m.getOrderByOfferFunc != nil {
		return m.getOrderByOfferFunc(ctx, offerID)
	}
	return nil, nil
}
func (m *mockFinishOrderRepository) FinishOrder(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
	if m.finishOrderFunc != nil {
		return m.finishOrderFunc(ctx, orderID, finishTime, durationSeconds, totalAmount, finalStatus, chargeSuccess, unholdSuccess, chargeTxID)
	}
	return nil
}

func (m *mockFinishOrderRepository) GetOldOrders(ctx context.Context, olderThan time.Duration) ([]*api.Order, error) {
	return nil, nil
}

func (m *mockFinishOrderRepository) DeleteOrders(ctx context.Context, orderIDs []string) error {
	return nil
}

type mockPayments struct {
	chargeErr error
	unholdErr error

	chargeFunc func(ctx context.Context, orderID string, amount int) error
	chargeCalled bool
	unholdCalled bool
}

func (m *mockPayments) HoldMoneyForOrder(ctx context.Context, req *external.PaymentHoldRequest) (*external.PaymentHoldResponse, error) {
	return &external.PaymentHoldResponse{
		TransactionID: "txn-hold",
		Success:       true,
	}, nil
}
func (m *mockPayments) ChargeMoneyForOrder(ctx context.Context, orderID string, amount int) error {
	m.chargeCalled = true
	if m.chargeFunc != nil {
		return m.chargeFunc(ctx, orderID, amount)
	}
	return m.chargeErr
}
func (m *mockPayments) UnholdMoneyForOrder(ctx context.Context, orderID string) error {
	m.unholdCalled = true
	return m.unholdErr
}

type recordCache struct {
	getFunc        func(ctx context.Context, orderID string) (*api.Order, error)
	setFunc        func(ctx context.Context, order *api.Order, ttl time.Duration) error
	invalidateFunc func(ctx context.Context, orderID string) error

	setCalled        bool
	setTTL           time.Duration
	invalidateCalled bool
}

func (c *recordCache) GetOrder(ctx context.Context, orderID string) (*api.Order, error) {
	if c.getFunc != nil {
		return c.getFunc(ctx, orderID)
	}
	return nil, nil
}
func (c *recordCache) SetOrder(ctx context.Context, order *api.Order, ttl time.Duration) error {
	c.setCalled = true
	c.setTTL = ttl
	if c.setFunc != nil {
		return c.setFunc(ctx, order, ttl)
	}
	return nil
}
func (c *recordCache) Invalidate(ctx context.Context, orderID string) error {
	c.invalidateCalled = true
	if c.invalidateFunc != nil {
		return c.invalidateFunc(ctx, orderID)
	}
	return nil
}

func iptr(i int) *int { return &i }

func TestFinishOrder_Success_ChargeAndUnhold_OK(t *testing.T) {
	now := time.Now()
	active := &api.Order{
		Id:             "order-1",
		UserId:         "user-1",
		ScooterId:      "scooter-1",
		StartTime:      now.Add(-120 * time.Second),
		Status:         api.ACTIVE,
		PricePerMinute: iptr(10),
		PriceUnlock:    iptr(20),
		Deposit:        iptr(100),
	}

	calls := 0
	updated := &api.Order{
		Id:             active.Id,
		UserId:         active.UserId,
		ScooterId:      active.ScooterId,
		StartTime:      active.StartTime,
		FinishTime:     &now,
		Status:         api.FINISHED,
		PricePerMinute: active.PricePerMinute,
		PriceUnlock:    active.PriceUnlock,
		Deposit:        active.Deposit,
		CurrentAmount:  iptr(40), // 20 unlock + ceil(120/60)*10 = 40
	}

	var captured struct {
		duration int
		total    int
		status   api.OrderStatus
		charged  bool
		unheld   bool
	}

	repo := &mockFinishOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			calls++
			if calls == 1 {
				return active, nil
			}
			return updated, nil
		},
		finishOrderFunc: func(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
			captured.duration = durationSeconds
			captured.total = totalAmount
			captured.status = finalStatus
			captured.charged = chargeSuccess
			captured.unheld = unholdSuccess
			return nil
		},
	}

	pay := &mockPayments{
		chargeErr: nil,
		unholdErr: nil,
	}

	cache := &recordCache{}
	svc := NewServiceWithCache(repo, nil, pay, cache, nil)

	res, err := svc.FinishOrder(context.Background(), active.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected order, got nil")
	}
	if res.Status != api.FINISHED {
		t.Errorf("expected status FINISHED, got %s", res.Status)
	}
	if captured.status != api.FINISHED {
		t.Errorf("expected repo final status FINISHED, got %s", captured.status)
	}
	if captured.total != 40 {
		t.Errorf("expected total 40, got %d", captured.total)
	}
	if captured.duration < 120 {
		t.Errorf("expected duration >= 120, got %d", captured.duration)
	}
	if !captured.charged || !captured.unheld {
		t.Errorf("expected charged & unheld true, got charged=%v unheld=%v", captured.charged, captured.unheld)
	}
	if !pay.chargeCalled || !pay.unholdCalled {
		t.Errorf("payments not called as expected, charge=%v unhold=%v", pay.chargeCalled, pay.unholdCalled)
	}
	if !cache.invalidateCalled || !cache.setCalled {
		t.Errorf("cache invalidate/set not called: inv=%v set=%v", cache.invalidateCalled, cache.setCalled)
	}
	if cache.setTTL <= 0 {
		t.Errorf("expected SetOrder with positive TTL, got %v", cache.setTTL)
	}
}

func TestFinishOrder_ChargeFails_PaymentFailedStatus(t *testing.T) {
	now := time.Now()
	active := &api.Order{
		Id:             "order-2",
		UserId:         "user-2",
		ScooterId:      "scooter-2",
		StartTime:      now.Add(-90 * time.Second),
		Status:         api.ACTIVE,
		PricePerMinute: iptr(10),
		PriceUnlock:    iptr(0),
	}

	calls := 0
	updated := &api.Order{
		Id:        active.Id,
		UserId:    active.UserId,
		ScooterId: active.ScooterId,
		StartTime: active.StartTime,
		Status:    api.PAYMENTFAILED,
	}

	var captured struct {
		status  api.OrderStatus
		charged bool
		unheld  bool
	}

	repo := &mockFinishOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			calls++
			if calls == 1 {
				return active, nil
			}
			return updated, nil
		},
		finishOrderFunc: func(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
			captured.status = finalStatus
			captured.charged = chargeSuccess
			captured.unheld = unholdSuccess
			return nil
		},
	}
	pay := &mockPayments{
		chargeErr: errors.New("charge failed"),
		unholdErr: nil,
	}
	cache := &recordCache{}
	svc := NewServiceWithCache(repo, nil, pay, cache, nil)

	res, err := svc.FinishOrder(context.Background(), active.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected order, got nil")
	}
	if res.Status != api.PAYMENTFAILED {
		t.Errorf("expected PAYMENT_FAILED, got %s", res.Status)
	}
	if captured.status != api.PAYMENTFAILED {
		t.Errorf("expected repo final status PAYMENT_FAILED, got %s", captured.status)
	}
	if captured.charged {
		t.Errorf("expected charged=false when charge fails")
	}
	if captured.unheld {
		t.Errorf("expected unheld=false when charge fails")
	}
	// Unhold shouldn't be called if charge failed
	if pay.unholdCalled {
		t.Errorf("expected unhold not called on charge failure")
	}
	if !cache.invalidateCalled || !cache.setCalled {
		t.Errorf("cache calls: invalidate=%v set=%v", cache.invalidateCalled, cache.setCalled)
	}
}

func TestFinishOrder_Idempotent_WhenAlreadyFinished(t *testing.T) {
	now := time.Now()
	already := &api.Order{
		Id:        "order-3",
		UserId:    "user-3",
		ScooterId: "scooter-3",
		StartTime: now.Add(-10 * time.Minute),
		Status:    api.FINISHED,
	}

	repo := &mockFinishOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			return already, nil
		},
		finishOrderFunc: func(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
			t.Errorf("FinishOrder should not be called for non-ACTIVE orders")
			return nil
		},
	}
	pay := &mockPayments{}
	cache := &recordCache{}
	svc := NewServiceWithCache(repo, nil, pay, cache, nil)

	res, err := svc.FinishOrder(context.Background(), already.Id)
	if err == nil {
		t.Fatalf("expected ErrOrderNotActive, got nil")
	}
	if err != ErrOrderNotActive {
		t.Fatalf("expected ErrOrderNotActive, got %v", err)
	}
	if res == nil || res.Status != api.FINISHED {
		t.Fatalf("expected FINISHED order returned, got %+v", res)
	}
	if pay.chargeCalled || pay.unholdCalled {
		t.Errorf("payments should not be called for idempotent finish; charge=%v unhold=%v", pay.chargeCalled, pay.unholdCalled)
	}
	if cache.invalidateCalled || cache.setCalled {
		t.Errorf("cache should not be modified for idempotent finish; inv=%v set=%v", cache.invalidateCalled, cache.setCalled)
	}
}

type mockConfigsProvider struct {
	getConfigsFunc func(ctx context.Context) (*external.DynamicConfigs, error)
}

func (m *mockConfigsProvider) GetConfigs(ctx context.Context) (*external.DynamicConfigs, error) {
	if m.getConfigsFunc != nil {
		return m.getConfigsFunc(ctx)
	}
	return &external.DynamicConfigs{
		IncompleteRideThresholdSeconds: 5,
	}, nil
}

func TestFinishOrder_IncompleteRide_NoCharge(t *testing.T) {
	startTime := time.Now().Add(-3 * time.Second) // 3 seconds ago
	active := &api.Order{
		Id:        "order-incomplete",
		UserId:    "user-1",
		Status:    api.ACTIVE,
		StartTime: startTime,
		PricePerMinute: func() *int { v := 10; return &v }(),
		PriceUnlock:    func() *int { v := 20; return &v }(),
	}

	calls := 0
	repo := &mockFinishOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			calls++
			if calls == 1 {
				return active, nil
			}
			// Return finished order on second call
			finished := *active
			finished.Status = api.FINISHED
			now := time.Now()
			finished.FinishTime = &now
			duration := 3
			finished.DurationSeconds = &duration
			return &finished, nil
		},
		finishOrderFunc: func(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
			if totalAmount != 0 {
				t.Errorf("expected totalAmount=0 for incomplete ride, got %d", totalAmount)
			}
			if !chargeSuccess {
				t.Errorf("expected chargeSuccess=true even for 0 amount")
			}
			return nil
		},
	}

	chargeAmount := -1
	pay := &mockPayments{
		chargeFunc: func(ctx context.Context, orderID string, amount int) error {
			chargeAmount = amount
			return nil
		},
		unholdErr: nil,
	}

	configsProvider := &mockConfigsProvider{
		getConfigsFunc: func(ctx context.Context) (*external.DynamicConfigs, error) {
			return &external.DynamicConfigs{
				IncompleteRideThresholdSeconds: 5,
			}, nil
		},
	}

	svc := NewServiceWithCache(repo, nil, pay, nil, configsProvider)

	res, err := svc.FinishOrder(context.Background(), active.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected order, got nil")
	}
	if chargeAmount != 0 {
		t.Errorf("expected charge amount=0 for incomplete ride (<5s), got %d", chargeAmount)
	}
	if res.Status != api.FINISHED {
		t.Errorf("expected status FINISHED, got %s", res.Status)
	}
}

func TestFinishOrder_CompleteRide_ChargeApplied(t *testing.T) {
	startTime := time.Now().Add(-10 * time.Second) // 10 seconds ago
	active := &api.Order{
		Id:        "order-complete",
		UserId:    "user-1",
		Status:    api.ACTIVE,
		StartTime: startTime,
		PricePerMinute: func() *int { v := 10; return &v }(),
		PriceUnlock:    func() *int { v := 20; return &v }(),
	}

	calls := 0
	repo := &mockFinishOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			calls++
			if calls == 1 {
				return active, nil
			}
			// Return finished order on second call
			finished := *active
			finished.Status = api.FINISHED
			now := time.Now()
			finished.FinishTime = &now
			duration := 10
			finished.DurationSeconds = &duration
			return &finished, nil
		},
		finishOrderFunc: func(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
			// 10 seconds = 1 minute (ceil), so total should be 20 (unlock) + 10 (1 min * 10) = 30
			expectedTotal := 30
			if totalAmount != expectedTotal {
				t.Errorf("expected totalAmount=%d for complete ride, got %d", expectedTotal, totalAmount)
			}
			return nil
		},
	}

	chargeAmount := -1
	pay := &mockPayments{
		chargeFunc: func(ctx context.Context, orderID string, amount int) error {
			chargeAmount = amount
			return nil
		},
		unholdErr: nil,
	}

	configsProvider := &mockConfigsProvider{
		getConfigsFunc: func(ctx context.Context) (*external.DynamicConfigs, error) {
			return &external.DynamicConfigs{
				IncompleteRideThresholdSeconds: 5,
			}, nil
		},
	}

	svc := NewServiceWithCache(repo, nil, pay, nil, configsProvider)

	res, err := svc.FinishOrder(context.Background(), active.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected order, got nil")
	}
	if chargeAmount != 30 {
		t.Errorf("expected charge amount=30 for complete ride (>=5s), got %d", chargeAmount)
	}
	if res.Status != api.FINISHED {
		t.Errorf("expected status FINISHED, got %s", res.Status)
	}
}

func TestFinishOrder_IncompleteRide_DefaultThreshold(t *testing.T) {
	startTime := time.Now().Add(-3 * time.Second) // 3 seconds ago
	active := &api.Order{
		Id:        "order-incomplete-default",
		UserId:    "user-1",
		Status:    api.ACTIVE,
		StartTime: startTime,
		PricePerMinute: func() *int { v := 10; return &v }(),
		PriceUnlock:    func() *int { v := 20; return &v }(),
	}

	repo := &mockFinishOrderRepository{
		getOrderByIDFunc: func(ctx context.Context, orderID string) (*api.Order, error) {
			if orderID == "order-incomplete-default" {
				return active, nil
			}
			return nil, nil
		},
		finishOrderFunc: func(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
			if totalAmount != 0 {
				t.Errorf("expected totalAmount=0 for incomplete ride, got %d", totalAmount)
			}
			return nil
		},
	}

	chargeAmount := -1
	pay := &mockPayments{
		chargeFunc: func(ctx context.Context, orderID string, amount int) error {
			chargeAmount = amount
			return nil
		},
		unholdErr: nil,
	}

	// Service without configsProvider - should use default threshold of 5
	svc := NewServiceWithCache(repo, nil, pay, nil, nil)

	res, err := svc.FinishOrder(context.Background(), active.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected order, got nil")
	}
	if chargeAmount != 0 {
		t.Errorf("expected charge amount=0 for incomplete ride with default threshold, got %d", chargeAmount)
	}
}
