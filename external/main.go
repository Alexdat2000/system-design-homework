package main

import (
	"encoding/json"
	"net/http"
	"os"

	"external/api"

	"github.com/go-chi/chi/v5"
)

type Server struct{}

func (s *Server) GetScooterData(w http.ResponseWriter, r *http.Request, params api.GetScooterDataParams) {
	resp := api.ScooterData{
		Id:     params.Id,
		ZoneId: "zone-1",
		Charge: 77,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) GetTariffZoneData(w http.ResponseWriter, r *http.Request, params api.GetTariffZoneDataParams) {
	resp := api.TariffZone{
		Id:             params.Id,
		PricePerMinute: 7,
		PriceUnlock:    50,
		DefaultDeposit: 200,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) GetUserProfile(w http.ResponseWriter, r *http.Request, params api.GetUserProfileParams) {
	resp := api.UserProfile{
		Id:              params.Id,
		HasSubscribtion: true,
		Trusted:         true,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) PostHoldMoneyForOrder(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) PostClearMoneyForOrder(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func (s *Server) GetConfigs(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"surge":                             1.2,
		"low_charge_discount":               0.7,
		"low_charge_threshold_percent":      28,
		"incomplete_ride_threshold_seconds": 5,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	router := chi.NewRouter()
	server := &Server{}

	port := getEnv("PORT", "8081")
	addr := ":" + port

	println("External service starting on", addr)
	http.ListenAndServe(addr, api.HandlerFromMux(server, router))
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
