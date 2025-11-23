package offers

import (
	"client/api"
	"client/internal/external"
	"context"
	"errors"
	"testing"
	"time"
)

// --- Mocks ---

type mockRepo struct {
	getOfferByUserScooterFunc func(ctx context.Context, userID, scooterID string) (*api.Offer, error)
	saveOfferFunc             func(ctx context.Context, offer *api.Offer) error
	setOfferByUserScooterFunc func(ctx context.Context, userID, scooterID, offerID string) error
}

func (m *mockRepo) GetOffer(ctx context.Context, offerID string) (*api.Offer, error) { return nil, nil }
func (m *mockRepo) SaveOffer(ctx context.Context, offer *api.Offer) error {
	if m.saveOfferFunc != nil {
		return m.saveOfferFunc(ctx, offer)
	}
	return nil
}
func (m *mockRepo) MarkOfferAsUsed(ctx context.Context, offerID string) (bool, error) { return false, nil }
func (m *mockRepo) GetOfferByUserScooter(ctx context.Context, userID, scooterID string) (*api.Offer, error) {
	if m.getOfferByUserScooterFunc != nil {
		return m.getOfferByUserScooterFunc(ctx, userID, scooterID)
	}
	return nil, nil
}
func (m *mockRepo) SetOfferByUserScooter(ctx context.Context, userID, scooterID, offerID string) error {
	if m.setOfferByUserScooterFunc != nil {
		return m.setOfferByUserScooterFunc(ctx, userID, scooterID, offerID)
	}
	return nil
}

type mockExternal struct {
	scooterFunc func(ctx context.Context, scooterID string) (*external.ScooterData, error)
	zoneFunc    func(ctx context.Context, zoneID string) (*external.TariffZone, error)
	userFunc    func(ctx context.Context, userID string) (*external.UserProfile, error)
	cfgFunc     func(ctx context.Context) (*external.DynamicConfigs, error)
}

func (m *mockExternal) GetScooterData(ctx context.Context, scooterID string) (*external.ScooterData, error) {
	if m.scooterFunc != nil {
		return m.scooterFunc(ctx, scooterID)
	}
	return &external.ScooterData{ID: scooterID, ZoneID: "zone-1", Charge: 80}, nil
}
func (m *mockExternal) GetTariffZoneData(ctx context.Context, zoneID string) (*external.TariffZone, error) {
	if m.zoneFunc != nil {
		return m.zoneFunc(ctx, zoneID)
	}
	return &external.TariffZone{ID: zoneID, PricePerMinute: 10, PriceUnlock: 20, DefaultDeposit: 100}, nil
}
func (m *mockExternal) GetUserProfile(ctx context.Context, userID string) (*external.UserProfile, error) {
	if m.userFunc != nil {
		return m.userFunc(ctx, userID)
	}
	return &external.UserProfile{ID: userID, HasSubscription: false, Trusted: false}, nil
}
func (m *mockExternal) GetConfigs(ctx context.Context) (*external.DynamicConfigs, error) {
	if m.cfgFunc != nil {
		return m.cfgFunc(ctx)
	}
	return &external.DynamicConfigs{
		Surge:                          1.0,
		LowChargeDiscount:              1.0,
		LowChargeThresholdPercent:      0,
		IncompleteRideThresholdSeconds: 0,
	}, nil
}

// --- Tests ---

func TestCreateOffer_Idempotent_ReturnsExisting(t *testing.T) {
	repo := &mockRepo{
		getOfferByUserScooterFunc: func(ctx context.Context, userID, scooterID string) (*api.Offer, error) {
			now := time.Now()
			return &api.Offer{
				Id:             "offer-existing",
				UserId:         userID,
				ScooterId:      scooterID,
				ZoneId:         "zone-1",
				PricePerMinute: 10,
				PriceUnlock:    20,
				Deposit:        100,
				ExpiresAt:      now.Add(4 * time.Minute),
				CreatedAt:      &now,
			}, nil
		},
	}
	ext := &mockExternal{}
	svc := NewService(repo, ext)

	req := &CreateOfferRequest{UserID: "user-1", ScooterID: "scooter-1"}
	offer, err := svc.CreateOffer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offer.Id != "offer-existing" {
		t.Errorf("expected existing offer, got %s", offer.Id)
	}
}

func TestCreateOffer_ScootersUnavailable_IsCritical(t *testing.T) {
	repo := &mockRepo{}
	ext := &mockExternal{
		scooterFunc: func(ctx context.Context, scooterID string) (*external.ScooterData, error) {
			return nil, errors.New("scooters down")
		},
	}
	svc := NewService(repo, ext)

	req := &CreateOfferRequest{UserID: "user-1", ScooterID: "scooter-1"}
	_, err := svc.CreateOffer(context.Background(), req)
	if err != ErrScootersUnavailable {
		t.Fatalf("expected ErrScootersUnavailable, got %v", err)
	}
}

func TestCreateOffer_ZoneFallbackFromCache(t *testing.T) {
	capturedSaved := false
	repo := &mockRepo{
		saveOfferFunc: func(ctx context.Context, offer *api.Offer) error {
			capturedSaved = true
			return nil
		},
		setOfferByUserScooterFunc: func(ctx context.Context, userID, scooterID, offerID string) error {
			return nil
		},
	}
	ext := &mockExternal{
		scooterFunc: func(ctx context.Context, scooterID string) (*external.ScooterData, error) {
			return &external.ScooterData{ID: scooterID, ZoneID: "zone-777", Charge: 50}, nil
		},
		zoneFunc: func(ctx context.Context, zoneID string) (*external.TariffZone, error) {
			return nil, errors.New("zone unavailable")
		},
		userFunc: func(ctx context.Context, userID string) (*external.UserProfile, error) {
			return &external.UserProfile{ID: userID, HasSubscription: false, Trusted: false}, nil
		},
		cfgFunc: func(ctx context.Context) (*external.DynamicConfigs, error) {
			return &external.DynamicConfigs{Surge: 1.0, LowChargeDiscount: 1.0, LowChargeThresholdPercent: 0}, nil
		},
	}
	svc := NewService(repo, ext)

	// Прогреем кэш зон
	svc.zoneCache["zone-777"] = zoneCacheEntry{
		zone:      &external.TariffZone{ID: "zone-777", PricePerMinute: 9, PriceUnlock: 1, DefaultDeposit: 2},
		expiresAt: time.Now().Add(9 * time.Minute),
	}

	req := &CreateOfferRequest{UserID: "user-2", ScooterID: "scooter-X"}
	offer, err := svc.CreateOffer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offer.ZoneId != "zone-777" {
		t.Errorf("expected zone from cache, got %s", offer.ZoneId)
	}
	if !capturedSaved {
		t.Errorf("expected SaveOffer to be called")
	}
}

func TestCreateOffer_Pricing_SurgeAndLowCharge_Discounts_SubscriptionTrusted(t *testing.T) {
	captured := &api.Offer{}
	repo := &mockRepo{
		saveOfferFunc: func(ctx context.Context, offer *api.Offer) error {
			*captured = *offer
			return nil
		},
		setOfferByUserScooterFunc: func(ctx context.Context, userID, scooterID, offerID string) error {
			return nil
		},
	}
	ext := &mockExternal{
		scooterFunc: func(ctx context.Context, scooterID string) (*external.ScooterData, error) {
			// Низкий заряд для применения скидки
			return &external.ScooterData{ID: scooterID, ZoneID: "zone-A", Charge: 10}, nil
		},
		zoneFunc: func(ctx context.Context, zoneID string) (*external.TariffZone, error) {
			return &external.TariffZone{ID: zoneID, PricePerMinute: 10, PriceUnlock: 20, DefaultDeposit: 200}, nil
		},
		userFunc: func(ctx context.Context, userID string) (*external.UserProfile, error) {
			return &external.UserProfile{ID: userID, HasSubscription: true, Trusted: true}, nil
		},
		cfgFunc: func(ctx context.Context) (*external.DynamicConfigs, error) {
			return &external.DynamicConfigs{
				Surge:                        1.2,
				LowChargeDiscount:            0.5,
				LowChargeThresholdPercent:    30,
				IncompleteRideThresholdSeconds: 0,
			}, nil
		},
	}

	svc := NewService(repo, ext)

	req := &CreateOfferRequest{UserID: "user-3", ScooterID: "scooter-3"}
	offer, err := svc.CreateOffer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Проверяем цены: 10 * 1.2 = 12; при низком заряде 12 * 0.5 = 6
	if captured.PricePerMinute != 6 || offer.PricePerMinute != 6 {
		t.Errorf("expected price_per_minute 6, got saved=%d offer=%d", captured.PricePerMinute, offer.PricePerMinute)
	}
	// Подписка => unlock 0
	if captured.PriceUnlock != 0 || offer.PriceUnlock != 0 {
		t.Errorf("expected price_unlock 0, got saved=%d offer=%d", captured.PriceUnlock, offer.PriceUnlock)
	}
	// Trusted => deposit 0
	if captured.Deposit != 0 || offer.Deposit != 0 {
		t.Errorf("expected deposit 0, got saved=%d offer=%d", captured.Deposit, offer.Deposit)
	}
	// TTL/ExpiresAt должен быть в будущем
	if !offer.ExpiresAt.After(time.Now()) {
		t.Errorf("expected ExpiresAt in future")
	}
}

func TestCreateOffer_SetsIndex_IdempotencyKey(t *testing.T) {
	indexCalled := false
	repo := &mockRepo{
		saveOfferFunc: func(ctx context.Context, offer *api.Offer) error { return nil },
		setOfferByUserScooterFunc: func(ctx context.Context, userID, scooterID, offerID string) error {
			indexCalled = true
			return nil
		},
	}
	ext := &mockExternal{}
	svc := NewService(repo, ext)

	req := &CreateOfferRequest{UserID: "user-4", ScooterID: "scooter-4"}
	_, err := svc.CreateOffer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !indexCalled {
		t.Errorf("expected SetOfferByUserScooter to be called")
	}
}