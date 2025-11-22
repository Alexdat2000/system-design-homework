package handler

import (
	"client/api"
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
	// Parse request body
	var reqBody api.PostOrdersJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
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
