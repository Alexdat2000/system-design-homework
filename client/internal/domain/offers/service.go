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

var (
	// критично: scooters недоступен => нельзя создать оффер
	ErrScootersUnavailable = errors.New("scooters unavailable")
	// некритично, но с fallback: zone недоступен => используем кэш, иначе ошибка
	ErrZoneUnavailable = errors.New("zone unavailable")
)

type ServiceInterface interface {
	CreateOffer(ctx context.Context, req *CreateOfferRequest) (*api.Offer, error)
}

type External interface {
	GetScooterData(ctx context.Context, scooterID string) (*external.ScooterData, error)
	GetTariffZoneData(ctx context.Context, zoneID string) (*external.TariffZone, error)
	GetUserProfile(ctx context.Context, userID string) (*external.UserProfile, error)
	GetConfigs(ctx context.Context) (*external.DynamicConfigs, error)
}

type Service struct {
	repo Repository
	ext  External

	// in-memory caches
	zoneCache    map[string]zoneCacheEntry
	zoneCacheTTL time.Duration

	// configs cache with "infinite" TTL
	configs *external.DynamicConfigs
}

type zoneCacheEntry struct {
	zone      *external.TariffZone
	expiresAt time.Time
}

func NewService(repo Repository, ext External) *Service {
	return &Service{
		repo:         repo,
		ext:          ext,
		zoneCache:    make(map[string]zoneCacheEntry),
		zoneCacheTTL: 10 * time.Minute,
	}
}

type CreateOfferRequest struct {
	UserID    string
	ScooterID string
}

// CreateOffer performs idempotent creation or returns existing valid offer
func (s *Service) CreateOffer(ctx context.Context, req *CreateOfferRequest) (*api.Offer, error) {
	if req == nil || req.UserID == "" || req.ScooterID == "" {
		return nil, fmt.Errorf("invalid request")
	}

	if existing, err := s.repo.GetOfferByUserScooter(ctx, req.UserID, req.ScooterID); err != nil {
		return nil, fmt.Errorf("failed to get existing offer: %w", err)
	} else if existing != nil {
		return existing, nil
	}

	scooter, err := s.ext.GetScooterData(ctx, req.ScooterID)
	if err != nil {
		return nil, ErrScootersUnavailable
	}
	if scooter == nil {
		return nil, fmt.Errorf("scooter not found")
	}

	zone, err := s.getZoneWithCache(ctx, scooter.ZoneId)
	if err != nil {
		return nil, err
	}

	var hasSub, trusted bool
	if profile, err := s.ext.GetUserProfile(ctx, req.UserID); err == nil && profile != nil {
		hasSub = profile.HasSubscription
		trusted = profile.Trusted
	}

	cfg := s.getConfigsCached(ctx)

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
	expiresAt := now.Add(10 * time.Minute)
	offer := &api.Offer{
		Id:             uuid.New().String(),
		UserId:         req.UserID,
		ScooterId:      req.ScooterID,
		ZoneId:         scooter.ZoneId,
		PricePerMinute: out.PricePerMinute,
		PriceUnlock:    out.PriceUnlock,
		Deposit:        out.Deposit,
		CreatedAt:      &now,
		ExpiresAt:      expiresAt,
	}

	if err := s.repo.SaveOffer(ctx, offer); err != nil {
		return nil, fmt.Errorf("failed to save offer: %w", err)
	}
	if err := s.repo.SetOfferByUserScooter(ctx, req.UserID, req.ScooterID, offer.Id); err != nil {
		return nil, fmt.Errorf("failed to index offer: %w", err)
	}

	return offer, nil
}

func (s *Service) getZoneWithCache(ctx context.Context, zoneID string) (*external.TariffZone, error) {
	if entry, ok := s.zoneCache[zoneID]; ok && time.Now().Before(entry.expiresAt) {
		return entry.zone, nil
	}

	z, err := s.ext.GetTariffZoneData(ctx, zoneID)
	if err != nil || z == nil {
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
		s.configs = &external.DynamicConfigs{
			Surge:                          1.2,
			LowChargeDiscount:              0.7,
			LowChargeThresholdPercent:      28,
			IncompleteRideThresholdSeconds: 5,
		}
		return s.configs
	}
	s.configs = cfg
	return cfg
}
