package main

import (
	"bytes"
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
	"client/internal/storage/postgres"
	"client/internal/storage/redis"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.statusCode == 0 {
		lrw.statusCode = http.StatusOK
	}
	lrw.body.Write(b)
	return lrw.ResponseWriter.Write(b)
}

func RequestLoggerWithBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)

		const maxLogBody = 2000
		respBody := lrw.body.String()
		if len(respBody) > maxLogBody {
			respBody = respBody[:maxLogBody] + "...(truncated)"
		}

		log.Printf(
			"%s %s status=%d duration=%s remote=%s query=%q response=%s",
			r.Method,
			r.URL.Path,
			lrw.statusCode,
			duration,
			r.RemoteAddr,
			r.URL.RawQuery,
			respBody,
		)
	})
}

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
	extClient, err := external.NewExternalClient(cfg.ExternalServiceURL)
	if err != nil {
		log.Fatalf("Failed to create external client: %v", err)
	}

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
	router.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Recoverer,
		RequestLoggerWithBody,
	)

	// Health check endpoint
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%s", cfg.Port)

	log.Printf("Client service starting on %s", addr)
	if err := http.ListenAndServe(addr, api.HandlerFromMux(server, router)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
