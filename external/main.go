package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"external/api"

	"github.com/go-chi/chi/v5"
)

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
	scooter, ok := s.storage.GetScooter(params.Id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scooter)
}

func (s *Server) GetTariffZoneData(w http.ResponseWriter, r *http.Request, params api.GetTariffZoneDataParams) {
	zone, ok := s.storage.GetZone(params.Id)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(zone)
}

func (s *Server) GetUserProfile(w http.ResponseWriter, r *http.Request, params api.GetUserProfileParams) {
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
