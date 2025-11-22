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

// mockOrdersService is a mock implementation of orders.ServiceInterface
type mockOrdersService struct {
	createOrderFunc func(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error)
}

func (m *mockOrdersService) CreateOrder(ctx context.Context, req *orders.CreateOrderRequest) (*api.Order, error) {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, req)
	}
	return nil, nil
}

func TestOrdersHandler_PostOrders_Success(t *testing.T) {
	// Setup
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
			if req.OfferID != "offer-456" || req.UserID != "user-789" {
				t.Errorf("Unexpected request: offer_id=%s, user_id=%s", req.OfferID, req.UserID)
			}
			return expectedOrder, nil
		},
	}

	handler := NewOrdersHandler(mockService)

	// Create request
	reqBody := api.PostOrdersJSONRequestBody{
		OfferId: "offer-456",
		UserId:  "user-789",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
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

	// Create request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_MissingOfferId(t *testing.T) {
	mockService := &mockOrdersService{}
	handler := NewOrdersHandler(mockService)

	// Create request without offer_id
	reqBody := api.PostOrdersJSONRequestBody{
		UserId: "user-789",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOrdersHandler_PostOrders_MissingUserId(t *testing.T) {
	mockService := &mockOrdersService{}
	handler := NewOrdersHandler(mockService)

	// Create request without user_id
	reqBody := api.PostOrdersJSONRequestBody{
		OfferId: "offer-456",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
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

	reqBody := api.PostOrdersJSONRequestBody{
		OfferId: "offer-456",
		UserId:  "user-789",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
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

	reqBody := api.PostOrdersJSONRequestBody{
		OfferId: "offer-456",
		UserId:  "user-789",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
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

	reqBody := api.PostOrdersJSONRequestBody{
		OfferId: "offer-456",
		UserId:  "user-789",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
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

	reqBody := api.PostOrdersJSONRequestBody{
		OfferId: "offer-456",
		UserId:  "user-789",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
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

	reqBody := api.PostOrdersJSONRequestBody{
		OfferId: "offer-456",
		UserId:  "user-789",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	handler.PostOrders(w, req)

	// Assert
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// Helper function for tests
func intPtr(i int) *int {
	return &i
}
