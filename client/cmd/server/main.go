package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"client/api" // замените на актуальный path если ваш модуль называется иначе

	"github.com/go-chi/chi/v5"
)

type Server struct{}

func (s *Server) PostOffers(w http.ResponseWriter, r *http.Request) {
	// Заглушка: возвращаем dummy offer
	resp := api.Offer{
		Id:             "offer-id-123",
		UserId:         "user-abc",
		ScooterId:      "scooter-xyz",
		ZoneId:         "zone-test",
		Deposit:        100,
		PricePerMinute: 5,
		PriceUnlock:    10,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) PostOrders(w http.ResponseWriter, r *http.Request) {
	// Заглушка: возвращаем dummy order
	now := time.Now()
	resp := api.Order{
		Id:             "order-id-777",
		OfferId:        "offer-id-111",
		UserId:         "user-abc",
		ScooterId:      "scooter-xyz",
		StartTime:      now,
		Status:         api.ACTIVE,
		PricePerMinute: newInt(5),
		PriceUnlock:    newInt(10),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) GetOrdersOrderId(w http.ResponseWriter, r *http.Request, orderId string) {
	// Заглушка: новый dummy order
	now := time.Now()
	resp := api.Order{
		Id:             orderId,
		OfferId:        "offer-id-222",
		UserId:         "user-abc",
		ScooterId:      "scooter-xyz",
		StartTime:      now.Add(-5 * time.Minute),
		Status:         api.FINISHED,
		PricePerMinute: newInt(7),
		PriceUnlock:    newInt(11),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) PostOrdersOrderIdFinish(w http.ResponseWriter, r *http.Request, orderId string) {
	// Заглушка: возвращаем info об order, завершённый
	now := time.Now()
	resp := api.Order{
		Id:             orderId,
		OfferId:        "offer-id-tenant",
		UserId:         "user-abc",
		ScooterId:      "scooter-xyz",
		StartTime:      now.Add(-30 * time.Minute),
		FinishTime:     &now,
		Status:         api.FINISHED,
		PricePerMinute: newInt(6),
		PriceUnlock:    newInt(13),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func newInt(i int) *int {
	return &i
}

func main() {
	router := chi.NewRouter()
	server := &Server{}

	port := getEnv("PORT", "8080")
	addr := ":" + port

	println("Client service starting on", addr)
	http.ListenAndServe(addr, api.HandlerFromMux(server, router))
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
