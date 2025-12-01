package handler

import (
	"client/api"
	"client/internal/domain/offers"
	"encoding/json"
	"net/http"
)

type OffersHandler struct {
	offersService offers.ServiceInterface
}

func NewOffersHandler(offersService offers.ServiceInterface) *OffersHandler {
	return &OffersHandler{
		offersService: offersService,
	}
}

func (h *OffersHandler) PostOffers(w http.ResponseWriter, r *http.Request) {
	var reqBody api.PostOffersJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if reqBody.UserId == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if reqBody.ScooterId == "" {
		http.Error(w, "scooter_id is required", http.StatusBadRequest)
		return
	}

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
			http.Error(w, "External zone service unavailable for long period of time", http.StatusServiceUnavailable)
			return
		default:
			http.Error(w, "Unable to create offer", http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(offer)
}
