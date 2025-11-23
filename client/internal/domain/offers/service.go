package offers

import (
	"client/api"
	"client/internal/domain/pricing"
	"client/internal/external"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Public errors for handler mapping (ADR: деградации/503)
var (
	// критично: scooters недоступен => нельзя создать оффер
	ErrScootersUnavailable = errors.New("scooters unavailable")
	// некритично, но с fallback: zone недоступен => используем кэш, иначе ошибка
	ErrZoneUnavailable = errors.New("zone unavailable")
)

// ServiceInterface defines offers service operations
type ServiceInterface interface {
	CreateOffer(ctx context.Context, req *CreateOfferRequest) (*api.Offer, error)
}

// External defines dependency on external services needed for offers
type External interface {
	GetScooterData(ctx context.Context, scooterID string) (*external.ScooterData, error)
	GetTariffZoneData(ctx context.Context, zoneID string) (*external.TariffZone, error)
	GetUserProfile(ctx context.Context, userID string) (*external.UserProfile, error)
	GetConfigs(ctx context.Context) (*external.DynamicConfigs, error)
}

// Service implements business logic for offers: idempotent creation, pricing and Redis storage
type Service struct {
	repo Repository
	ext  External

	// in-memory caches
	zoneCache    map[string]zoneCacheEntry
	zoneCacheTTL time.Duration

	// configs cache with "infinite" TTL (обновим при успешном запросе)
	configs *external.DynamicConfigs
}

type zoneCacheEntry struct {
	zone      *external.TariffZone
	expiresAt time.Time
}

// NewService constructs offers service
func NewService(repo Repository, ext External) *Service {
	return &Service{
		repo:         repo,
		ext:          ext,
		zoneCache:    make(map[string]zoneCacheEntry),
		zoneCacheTTL: 10 * time.Minute, // ADR: кэш зон 10 минут
	}
}

// CreateOfferRequest input
type CreateOfferRequest struct {
	UserID    string
	ScooterID string
}

// CreateOffer performs idempotent creation or returns existing valid offer
func (s *Service) CreateOffer(ctx context.Context, req *CreateOfferRequest) (*api.Offer, error) {
	if req == nil || req.UserID == "" || req.ScooterID == "" {
		return nil, fmt.Errorf("invalid request")
	}

	// Idempotent: check existing (user_id, scooter_id) offer
	if existing, err := s.repo.GetOfferByUserScooter(ctx, req.UserID, req.ScooterID); err != nil {
		return nil, fmt.Errorf("failed to get existing offer: %w", err)
	} else if existing != nil {
		// Redis TTL гарантирует актуальность
		return existing, nil
	}

	// 1) Scooters (critical)
	scooter, err := s.ext.GetScooterData(ctx, req.ScooterID)
	if err != nil {
		return nil, ErrScootersUnavailable
	}
	if scooter == nil {
		return nil, fmt.Errorf("scooter not found")
	}

	// 2) Zone with cache/fallback (non-critical with 10m cache)
	zone, err := s.getZoneWithCache(ctx, scooter.ZoneID)
	if err != nil {
		return nil, err
	}

	// 3) Users (non-critical): fallback to "no privileges" on error
	var hasSub, trusted bool
	if profile, err := s.ext.GetUserProfile(ctx, req.UserID); err == nil && profile != nil {
		hasSub = profile.HasSubscription
		trusted = profile.Trusted
	}

	// 4) Configs (non-critical): cache forever; defaults on first failure
	cfg := s.getConfigsCached(ctx)

	// 5) Pricing calculation
	out := pricing.Calculate(pricing.Inputs{
		ZonePricePerMinute:        zone.PricePerMinute,
		ZonePriceUnlock:           zone.PriceUnlock,
		ZoneDefaultDeposit:        zone.DefaultDeposit,
		Surge:                     cfg.Surge,
		LowChargeDiscount:         cfg.LowChargeDiscount,
		LowChargeThresholdPercent: cfg.LowChargeThresholdPercent,
		ScooterChargePercent:      scooter.Charge,
		HasSubscription:           hasSub,
		Trusted:                   trusted,
	})

	now := time.Now()
	expiresAt := now.Add(5 * time.Minute) // ADR: TTL 5 минут
	offer := &api.Offer{
		Id:             uuid.New().String(),
		UserId:         req.UserID,
		ScooterId:      req.ScooterID,
		ZoneId:         scooter.ZoneID,
		PricePerMinute: out.PricePerMinute,
		PriceUnlock:    out.PriceUnlock,
		Deposit:        out.Deposit,
		CreatedAt:      &now,
		ExpiresAt:      expiresAt,
	}

	// 6) Save to Redis and index for idempotency
	if err := s.repo.SaveOffer(ctx, offer); err != nil {
		return nil, fmt.Errorf("failed to save offer: %w", err)
	}
	if err := s.repo.SetOfferByUserScooter(ctx, req.UserID, req.ScooterID, offer.Id); err != nil {
		return nil, fmt.Errorf("failed to index offer: %w", err)
	}

	return offer, nil
}

func (s *Service) getZoneWithCache(ctx context.Context, zoneID string) (*external.TariffZone, error) {
	// hit
	if entry, ok := s.zoneCache[zoneID]; ok && time.Now().Before(entry.expiresAt) {
		return entry.zone, nil
	}

	// miss -> fetch
	z, err := s.ext.GetTariffZoneData(ctx, zoneID)
	if err != nil || z == nil {
		// fallback if we still have cached
		if entry, ok := s.zoneCache[zoneID]; ok && time.Now().Before(entry.expiresAt) {
			return entry.zone, nil
		}
		return nil, ErrZoneUnavailable
	}

	s.zoneCache[zoneID] = zoneCacheEntry{
		zone:      z,
		expiresAt: time.Now().Add(s.zoneCacheTTL),
	}
	return z, nil
}

func (s *Service) getConfigsCached(ctx context.Context) *external.DynamicConfigs {
	if s.configs != nil {
		return s.configs
	}
	cfg, err := s.ext.GetConfigs(ctx)
	if err != nil || cfg == nil {
		// Defaults per ADR if service unavailable on start
		s.configs = &external.DynamicConfigs{
			Surge:                          1.0,
			LowChargeDiscount:              1.0,
			LowChargeThresholdPercent:      0,
			IncompleteRideThresholdSeconds: 0,
		}
		return s.configs
	}
	s.configs = cfg
	return cfg
}
