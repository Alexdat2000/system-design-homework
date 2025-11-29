package handler

import (
	"client/internal/domain/orders"
	"encoding/json"
	"net/http"
)

type OrdersHandler struct {
	ordersService orders.ServiceInterface
}

func NewOrdersHandler(ordersService orders.ServiceInterface) *OrdersHandler {
	return &OrdersHandler{
		ordersService: ordersService,
	}
}

func (h *OrdersHandler) PostOrders(w http.ResponseWriter, r *http.Request) {
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

	req := &orders.CreateOrderRequest{
		OrderID: reqBody.OrderId,
		OfferID: reqBody.OfferId,
		UserID:  reqBody.UserId,
	}

	order, err := h.ordersService.CreateOrder(r.Context(), req)
	if err != nil {
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
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(order); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *OrdersHandler) GetOrdersOrderId(w http.ResponseWriter, r *http.Request, orderId string) {
	if orderId == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	order, err := h.ordersService.GetOrder(r.Context(), orderId)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if order == nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(order); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *OrdersHandler) PostOrdersOrderIdFinish(w http.ResponseWriter, r *http.Request, orderId string) {
	if orderId == "" {
		http.Error(w, "order_id is required", http.StatusBadRequest)
		return
	}

	order, err := h.ordersService.FinishOrder(r.Context(), orderId)
	if err != nil {
		switch err {
		case orders.ErrNoSuchOrder:
			http.Error(w, "Order not found", http.StatusBadRequest)
			return
		case orders.ErrOrderNotActive:
			http.Error(w, "Order already finished", http.StatusConflict)
			return
		default:
			http.Error(w, "Finish error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(order); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
