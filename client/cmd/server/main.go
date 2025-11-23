package main

import (
	"fmt"
	"log"
	"net/http"

	"client/api"
	"client/internal/config"
	"client/internal/domain/offers"
	"client/internal/domain/orders"
	"client/internal/external"
	"client/internal/handler"
	"client/internal/storage/postgres"
	"client/internal/storage/redis"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	ordersHandler *handler.OrdersHandler
	offersHandler *handler.OffersHandler
}

func (s *Server) PostOffers(w http.ResponseWriter, r *http.Request) {
	s.offersHandler.PostOffers(w, r)
}

func (s *Server) PostOrders(w http.ResponseWriter, r *http.Request) {
	s.ordersHandler.PostOrders(w, r)
}

func (s *Server) GetOrdersOrderId(w http.ResponseWriter, r *http.Request, orderId string) {
	s.ordersHandler.GetOrdersOrderId(w, r, orderId)
}

func (s *Server) PostOrdersOrderIdFinish(w http.ResponseWriter, r *http.Request, orderId string) {
	s.ordersHandler.PostOrdersOrderIdFinish(w, r, orderId)
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

	// Initialize Redis connection
	redisClient, err := redis.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Error closing Redis client: %v", err)
		}
	}()
	log.Println("Connected to Redis")

	// Initialize repositories
	orderRepo := postgres.NewOrderRepository(db)
	offerRepo := redis.NewOfferRepository(redisClient)

	// Initialize external clients
	extClient := external.NewClient(cfg.ExternalServiceURL)

	// Initialize services
	orderCache := redis.NewOrderCache(redisClient)
	ordersService := orders.NewServiceWithCache(orderRepo, offerRepo, extClient, orderCache)
	offersService := offers.NewService(offerRepo, extClient)

	// Initialize handlers
	ordersHandler := handler.NewOrdersHandler(ordersService)
	offersHandler := handler.NewOffersHandler(offersService)

	// Create server with handlers
	server := &Server{
		ordersHandler: ordersHandler,
		offersHandler: offersHandler,
	}

	router := chi.NewRouter()
	addr := fmt.Sprintf(":%s", cfg.Port)

	log.Printf("Client service starting on %s", addr)
	if err := http.ListenAndServe(addr, api.HandlerFromMux(server, router)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
