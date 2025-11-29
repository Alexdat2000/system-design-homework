package handler

import (
	"bytes"
	"client/api"
	"client/internal/domain/orders"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type mockOrdersService struct {
	createOrderFunc func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error)
	getOrderFunc    func(ctx context.Context, orderID string) (*api.Order, error)
	finishOrderFunc func(ctx context.Context, orderID string) (*api.Order, error)
}

func (m *mockOrdersService) CreateOrder(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockOrdersService) GetOrder(ctx context.Context, orderID string) (*api.Order, error) {
	if m.getOrderFunc != nil {
		return m.getOrderFunc(ctx, orderID)
	}
	return nil, nil
}

func (m *mockOrdersService) FinishOrder(ctx context.Context, orderID string) (*api.Order, error) {
	if m.finishOrderFunc != nil {
		return m.finishOrderFunc(ctx, orderID)
	}
	return nil, nil
}

func TestOrdersHandler_PostOrders_Success(t *testing.T) {
	now := time.Now()
	expectedOrder := &api.Order{
		Id:             "order-123",
		OfferId:        "offer-456",
		UserId:         "user-789",
		ScooterId:      "scooter-abc",
		Status:         api.ACTIVE,
		StartTime:      now,
		PricePerMinute: intPtr(10),
		PriceUnlock:    intPtr(20),
		Deposit:        intPtr(100),
		CurrentAmount:  intPtr(20),
	}

	mockService := &mockOrdersService{
		createOrderFunc: func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
			if req.OrderID != "order-789" || req.OfferID != "offer-456" || req.UserID != "user-789" {
				t.Errorf("Unexpected request: order_id=%s, offer_id=%s, user_id=%s", req.OrderID, req.OfferID, req.UserID)
			}
			return expectedOrder, nil
		},
	}

	handler := NewOrdersHandler(mockService)

	body := makeBody("order-789", "offer-456", "user-789")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response api.Order
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Id != expectedOrder.Id {
		t.Errorf("Expected order ID %s, got %s", expectedOrder.Id, response.Id)
	}
	if response.OfferId != expectedOrder.OfferId {
		t.Errorf("Expected offer ID %s, got %s", expectedOrder.OfferId, response.OfferId)
	}
	if response.UserId != expectedOrder.UserId {
		t.Errorf("Expected user ID %s, got %s", expectedOrder.UserId, response.UserId)
	}
}

func TestOrdersHandler_PostOrders_InvalidBody(t *testing.T) {
	mockService := &mockOrdersService{}
	handler := NewOrdersHandler(mockService)

	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_MissingOfferId(t *testing.T) {
	mockService := &mockOrdersService{}
	handler := NewOrdersHandler(mockService)

	body := makeBody("order-1", "", "user-789")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_MissingUserId(t *testing.T) {
	mockService := &mockOrdersService{}
	handler := NewOrdersHandler(mockService)

	body := makeBody("order-1", "offer-456", "")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_OfferNotFound(t *testing.T) {
	mockService := &mockOrdersService{
		createOrderFunc: func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
			return nil, orders.ErrOfferNotFound
		},
	}
	handler := NewOrdersHandler(mockService)

	body := makeBody("order-404", "offer-456", "user-789")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_OfferExpired(t *testing.T) {
	mockService := &mockOrdersService{
		createOrderFunc: func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
			return nil, orders.ErrOfferExpired
		},
	}
	handler := NewOrdersHandler(mockService)

	body := makeBody("order-expired", "offer-456", "user-789")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_OfferAlreadyUsed(t *testing.T) {
	mockService := &mockOrdersService{
		createOrderFunc: func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
			return nil, orders.ErrOfferAlreadyUsed
		},
	}
	handler := NewOrdersHandler(mockService)

	body := makeBody("order-used", "offer-456", "user-789")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_InvalidUser(t *testing.T) {
	mockService := &mockOrdersService{
		createOrderFunc: func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
			return nil, orders.ErrInvalidUser
		},
	}
	handler := NewOrdersHandler(mockService)

	body := makeBody("order-invalid", "offer-456", "user-789")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_InternalError(t *testing.T) {
	mockService := &mockOrdersService{
		createOrderFunc: func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
			return nil, errors.New("internal error")
		},
	}
	handler := NewOrdersHandler(mockService)

	body := makeBody("order-err", "offer-456", "user-789")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostOrders(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestOrdersHandler_FinishOrder_Twice(t *testing.T) {
	orderID := "order-finish-1"
	now := time.Now()

	// Используем состояние заказа для симуляции его завершения
	finished := false

	expectedOrder := &api.Order{
		Id:        orderID,
		UserId:    "user-123",
		OfferId:   "offer-xxx",
		Status:    api.FINISHED,
		StartTime: now,
	}

	mockService := &mockOrdersService{
		createOrderFunc: func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
			return &api.Order{
				Id:      orderID,
				OfferId: req.OfferID,
				UserId:  req.UserID,
				Status:  api.ACTIVE,
			}, nil
		},
		getOrderFunc: func(ctx context.Context, oid string) (*api.Order, error) {
			if oid != orderID {
				return nil, orders.ErrNoSuchOrder
			}
			st := api.ACTIVE
			if finished {
				st = api.FINISHED
			}
			return &api.Order{
				Id:        orderID,
				OfferId:   "offer-xxx",
				UserId:    "user-123",
				Status:    st,
				StartTime: now,
			}, nil
		},
		finishOrderFunc: func(ctx context.Context, oid string) (*api.Order, error) {
			if oid != orderID {
				return nil, orders.ErrNoSuchOrder
			}
			if finished {
				return nil, orders.ErrOrderNotActive
			}
			finished = true
			return expectedOrder, nil
		},
	}

	handler := NewOrdersHandler(mockService)

	createReqBody := makeBody(orderID, "offer-xxx", "user-123")
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(createReqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.PostOrders(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Expected create status %d, got %d", http.StatusCreated, w.Code)
	}

	finishReq := httptest.NewRequest(http.MethodPost, "/orders/"+orderID+"/finish", nil)
	w2 := httptest.NewRecorder()
	handler.PostOrdersOrderIdFinish(w2, finishReq, orderID)

	if w2.Code != http.StatusOK {
		t.Fatalf("Expected finish status %d, got %d", http.StatusOK, w2.Code)
	}

	w3 := httptest.NewRecorder()
	handler.PostOrdersOrderIdFinish(w3, finishReq, orderID)

	if w3.Code != http.StatusBadRequest && w3.Code != http.StatusConflict {
		t.Fatalf("Expected error status for repeated finish, got %d", w3.Code)
	}
}

func intPtr(i int) *int { return &i }

func makeBody(orderID, offerID, userID string) []byte {
	m := map[string]string{}
	if orderID != "" {
		m["order_id"] = orderID
	}
	if offerID != "" {
		m["offer_id"] = offerID
	}
	if userID != "" {
		m["user_id"] = userID
	}
	b, _ := json.Marshal(m)
	return b
}
