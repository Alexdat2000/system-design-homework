package handler

import (
	"client/internal/domain/orders"
	"encoding/json"
	"net/http"
)

// OrdersHandler handles HTTP requests for orders
type OrdersHandler struct {
	ordersService orders.ServiceInterface
}

// NewOrdersHandler creates a new orders handler
func NewOrdersHandler(ordersService orders.ServiceInterface) *OrdersHandler {
	return &OrdersHandler{
		ordersService: ordersService,
	}
}

// PostOrders handles POST /orders request
// Creates an order from an offer
func (h *OrdersHandler) PostOrders(w http.ResponseWriter, r *http.Request) {
	// Parse request body (ADR: expect client-provided order_id for idempotency)
	type postOrdersRequest struct {
		OrderId string `json:"order_id"`
		OfferId string `json:"offer_id"`
		UserId  string `json:"user_id"`
	}
	var reqBody postOrdersRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if reqBody.OrderId == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}
	if reqBody.OfferId == "" {
		http.Error(w, "offer_id is required", http.StatusBadRequest)
		return
	}
	if reqBody.UserId == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Create order request
	req := &orders.CreateOrderRequest{
		OrderID: reqBody.OrderId,
		OfferID: reqBody.OfferId,
		UserID:  reqBody.UserId,
	}

	// Call service to create order
	order, err := h.ordersService.CreateOrder(r.Context(), req)
	if err != nil {
		// Handle different error types
		switch err {
		case orders.ErrOfferNotFound:
			http.Error(w, "Offer not found", http.StatusBadRequest)
			return
		case orders.ErrOfferExpired:
			http.Error(w, "Offer expired", http.StatusBadRequest)
			return
		case orders.ErrOfferAlreadyUsed:
			http.Error(w, "Offer already used", http.StatusBadRequest)
			return
		case orders.ErrInvalidUser:
			http.Error(w, "Invalid user", http.StatusBadRequest)
			return
		default:
			// Internal server error
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Return created order
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(order); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}


// Added GET /orders/{order_id} delegating to service cache-first lookup
func (h *OrdersHandler) GetOrdersOrderId(w http.ResponseWriter, r *http.Request, orderId string) {
	// Validate input
	if orderId == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	// Call service
	order, err := h.ordersService.GetOrder(r.Context(), orderId)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if order == nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	// Return order
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(order); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// PostOrdersOrderIdFinish handles POST /orders/{order_id}/finish
// Finalizes an order. Idempotent: if already finished, returns 409.
func (h *OrdersHandler) PostOrdersOrderIdFinish(w http.ResponseWriter, r *http.Request, orderId string) {
	// Validate input
	if orderId == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	// Call service
	order, err := h.ordersService.FinishOrder(r.Context(), orderId)
	if err != nil {
		switch err {
		case orders.ErrNoSuchOrder:
			// Spec for finish defines 400/409/503; используем 400 для "не найден/нельзя завершить"
			http.Error(w, "Order not found", http.StatusBadRequest)
			return
		case orders.ErrOrderNotActive:
			http.Error(w, "Order already finished", http.StatusConflict)
			return
		default:
			// 400 как общий бизнес-ошибочный кейс (например, платёжная ошибка)
			http.Error(w, "Finish error", http.StatusBadRequest)
			return
		}
	}

	// Return updated order
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(order); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
