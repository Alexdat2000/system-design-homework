package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"client/api"
	"client/internal/config"
	"client/internal/domain/offers"
	"client/internal/domain/orders"
	"client/internal/external"
	"client/internal/handler"
	"client/internal/helpers"
	"client/internal/jobs"
	"client/internal/storage/postgres"
	"client/internal/storage/redis"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	cfg := config.LoadConfig()

	db, err := postgres.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL database")

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

	orderRepo := postgres.NewOrderRepository(db)
	offerRepo := redis.NewOfferRepository(redisClient)

	extClient, err := external.NewExternalClient(cfg.ExternalServiceURL)
	if err != nil {
		log.Fatalf("Failed to create external client: %v", err)
	}

	orderCache := redis.NewOrderCache(redisClient)
	ordersService := orders.NewServiceWithCache(orderRepo, offerRepo, extClient, orderCache)
	offersService := offers.NewService(offerRepo, extClient)

	ordersHandler := handler.NewOrdersHandler(ordersService)
	offersHandler := handler.NewOffersHandler(offersService)

	cleanupJob := jobs.NewOrderCleanupJob(
		orderRepo,
		24*time.Hour,
		1*time.Hour,
	)
	cleanupJob.Start()
	defer cleanupJob.Stop()

	server := &Server{
		ordersHandler: ordersHandler,
		offersHandler: offersHandler,
	}

	router := chi.NewRouter()
	router.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Recoverer,
		helpers.RequestLoggerWithBody,
	)

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%s", cfg.Port)

	srv := &http.Server{
		Addr:         addr,
		Handler:      api.HandlerFromMux(server, router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Printf("Client service starting on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
