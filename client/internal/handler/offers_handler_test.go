package handler

import (
	"bytes"
	"client/api"
	"client/internal/domain/offers"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockOffersService implements offers.ServiceInterface for handler tests
type mockOffersService struct {
	createOfferFunc func(ctx context.Context, req *offers.CreateOfferRequest) (*api.Offer, error)
}

func (m *mockOffersService) CreateOffer(ctx context.Context, req *offers.CreateOfferRequest) (*api.Offer, error) {
	if m.createOfferFunc != nil {
		return m.createOfferFunc(ctx, req)
	}
	return nil, nil
}

func TestOffersHandler_PostOffers_Success(t *testing.T) {
	now := time.Now()
	expected := &api.Offer{
		Id:             "offer-123",
		UserId:         "user-1",
		ScooterId:      "scooter-1",
		ZoneId:         "zone-1",
		PricePerMinute: 6,
		PriceUnlock:    0,
		Deposit:        0,
		CreatedAt:      &now,
		ExpiresAt:      now.Add(5 * time.Minute),
	}

	mockSvc := &mockOffersService{
		createOfferFunc: func(ctx context.Context, req *offers.CreateOfferRequest) (*api.Offer, error) {
			if req.UserID != "user-1" || req.ScooterID != "scooter-1" {
				t.Errorf("unexpected req: %+v", req)
			}
			return expected, nil
		},
	}

	h := NewOffersHandler(mockSvc)

	body := api.PostOffersJSONRequestBody{
		UserId:    "user-1",
		ScooterId: "scooter-1",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostOffers(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, w.Code)
	}

	var resp api.Offer
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Id != expected.Id || resp.UserId != expected.UserId || resp.ScooterId != expected.ScooterId {
		t.Errorf("unexpected offer response: %+v", resp)
	}
}

func TestOffersHandler_PostOffers_InvalidBody(t *testing.T) {
	h := NewOffersHandler(&mockOffersService{})
	req := httptest.NewRequest(http.MethodPost, "/offers", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostOffers(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOffersHandler_PostOffers_MissingUserId(t *testing.T) {
	h := NewOffersHandler(&mockOffersService{})
	body := api.PostOffersJSONRequestBody{
		ScooterId: "scooter-1",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostOffers(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOffersHandler_PostOffers_MissingScooterId(t *testing.T) {
	h := NewOffersHandler(&mockOffersService{})
	body := api.PostOffersJSONRequestBody{
		UserId: "user-1",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostOffers(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestOffersHandler_PostOffers_ScootersUnavailable(t *testing.T) {
	mockSvc := &mockOffersService{
		createOfferFunc: func(ctx context.Context, req *offers.CreateOfferRequest) (*api.Offer, error) {
			return nil, offers.ErrScootersUnavailable
		},
	}
	h := NewOffersHandler(mockSvc)

	body := api.PostOffersJSONRequestBody{UserId: "u", ScooterId: "s"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostOffers(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestOffersHandler_PostOffers_ZoneUnavailable(t *testing.T) {
	mockSvc := &mockOffersService{
		createOfferFunc: func(ctx context.Context, req *offers.CreateOfferRequest) (*api.Offer, error) {
			return nil, offers.ErrZoneUnavailable
		},
	}
	h := NewOffersHandler(mockSvc)

	body := api.PostOffersJSONRequestBody{UserId: "u", ScooterId: "s"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostOffers(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestOffersHandler_PostOffers_GenericErrorTo400(t *testing.T) {
	mockSvc := &mockOffersService{
		createOfferFunc: func(ctx context.Context, req *offers.CreateOfferRequest) (*api.Offer, error) {
			return nil, errors.New("some business error")
		},
	}
	h := NewOffersHandler(mockSvc)

	body := api.PostOffersJSONRequestBody{UserId: "u", ScooterId: "s"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/offers", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.PostOffers(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}