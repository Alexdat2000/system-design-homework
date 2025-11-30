package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"external/api"

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
	storage *Storage
}

func NewServer() (*Server, error) {
	storage := NewStorage()
	if err := storage.LoadFromJSONFiles(); err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}
	return &Server{storage: storage}, nil
}

func (s *Server) GetScooterData(w http.ResponseWriter, r *http.Request, params api.GetScooterDataParams) {
	if strings.HasPrefix(params.Id, "load-") {
		params.Id = "scooter-1"
	}
	scooter, ok := s.storage.GetScooter(params.Id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scooter)
}

func (s *Server) GetTariffZoneData(w http.ResponseWriter, r *http.Request, params api.GetTariffZoneDataParams) {
	if strings.HasPrefix(params.Id, "load-") {
		params.Id = "zone-1"
	}
	zone, ok := s.storage.GetZone(params.Id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(zone)
}

func (s *Server) GetUserProfile(w http.ResponseWriter, r *http.Request, params api.GetUserProfileParams) {
	if strings.HasPrefix(params.Id, "load-") {
		params.Id = "user-1"
	}
	user, ok := s.storage.GetUser(params.Id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (s *Server) GetConfigs(w http.ResponseWriter, r *http.Request) {
	configs := s.storage.GetConfigs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

func (s *Server) PostHoldMoneyForOrder(w http.ResponseWriter, r *http.Request) {
	var req api.PostHoldMoneyForOrderJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// MVP: always succeed, return transaction ID
	resp := map[string]any{
		"transaction_id": fmt.Sprintf("txn-hold-%s", *req.OrderId),
		"ok":             true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) PostClearMoneyForOrder(w http.ResponseWriter, r *http.Request) {
	var req api.PostClearMoneyForOrderJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// MVP: always succeed
	resp := map[string]any{
		"ok": true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) PostUnholdMoneyForOrder(w http.ResponseWriter, r *http.Request) {
	var req api.PostUnholdMoneyForOrderJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// MVP: always succeed
	resp := map[string]any{
		"ok": true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func main() {
	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
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

	port := getEnv("PORT", "8081")
	addr := ":" + port

	log.Printf("External service starting on %s", addr)
	if err := http.ListenAndServe(addr, api.HandlerFromMux(server, router)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
