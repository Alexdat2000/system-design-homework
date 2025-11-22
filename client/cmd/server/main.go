package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"client/api"
	"client/internal/config"
	"client/internal/domain/orders"
	"client/internal/external"
	"client/internal/handler"
	"client/internal/storage/postgres"
	"client/internal/storage/redis"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	ordersHandler *handler.OrdersHandler
}

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
	s.ordersHandler.PostOrders(w, r)
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
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize PostgreSQL connection
	db, err := postgres.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL database")

	// Initialize repositories
	orderRepo := postgres.NewOrderRepository(db)
	offerRepo := redis.NewOfferRepository()
	paymentsClient := external.NewClient(cfg.ExternalServiceURL)

	// Initialize services
	ordersService := orders.NewService(orderRepo, offerRepo, paymentsClient)

	// Initialize handlers
	ordersHandler := handler.NewOrdersHandler(ordersService)

	// Create server with handlers
	server := &Server{
		ordersHandler: ordersHandler,
	}

	router := chi.NewRouter()
	addr := fmt.Sprintf(":%s", cfg.Port)

	log.Printf("Client service starting on %s", addr)
	if err := http.ListenAndServe(addr, api.HandlerFromMux(server, router)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
