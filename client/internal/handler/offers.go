package handler

import (
	"client/api"
	"client/internal/domain/offers"
	"encoding/json"
	"net/http"
)

// OffersHandler handles HTTP requests for offers
type OffersHandler struct {
	offersService offers.ServiceInterface
}

// NewOffersHandler creates a new offers handler
func NewOffersHandler(offersService offers.ServiceInterface) *OffersHandler {
	return &OffersHandler{
		offersService: offersService,
	}
}

// PostOffers handles POST /offers request
// Creates (or returns existing valid) offer for user and scooter
func (h *OffersHandler) PostOffers(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var reqBody api.PostOffersJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if reqBody.UserId == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if reqBody.ScooterId == "" {
		http.Error(w, "scooter_id is required", http.StatusBadRequest)
		return
	}

	// Call service
	req := &offers.CreateOfferRequest{
		UserID:    reqBody.UserId,
		ScooterID: reqBody.ScooterId,
	}
	offer, err := h.offersService.CreateOffer(r.Context(), req)
	if err != nil {
		switch err {
		case offers.ErrScootersUnavailable:
			http.Error(w, "External scooters service unavailable", http.StatusServiceUnavailable)
			return
		case offers.ErrZoneUnavailable:
			http.Error(w, "External zone service unavailable", http.StatusServiceUnavailable)
			return
		default:
			http.Error(w, "Unable to create offer", http.StatusBadRequest)
			return
		}
	}

	// Return created offer
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(offer)
}
