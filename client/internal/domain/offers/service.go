package offers

import (
	"client/api"
	"client/internal/domain/pricing"
	"client/internal/external"
	"context"
	"errors"
	"fmt"
	"sync"
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
	zoneCacheMu  sync.RWMutex // protects zoneCache
	zoneCacheTTL time.Duration

	// configs cache with periodic updates
	configs     *external.DynamicConfigs
	configsMu   sync.RWMutex // protects configs
	stopCleanup chan struct{}
	cleanupDone sync.WaitGroup
}

type zoneCacheEntry struct {
	zone      *external.TariffZone
	expiresAt time.Time
}

func NewService(repo Repository, ext External) *Service {
	s := &Service{
		repo:         repo,
		ext:          ext,
		zoneCache:    make(map[string]zoneCacheEntry),
		zoneCacheTTL: 10 * time.Minute,
		stopCleanup:  make(chan struct{}),
	}
	// Initialize configs with default values
	s.configs = s.getDefaultConfigs()
	// Start periodic config updates
	s.cleanupDone.Add(1)
	go s.periodicConfigUpdate()
	return s
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
	// Try to read from cache with read lock
	s.zoneCacheMu.RLock()
	entry, ok := s.zoneCache[zoneID]
	now := time.Now()
	cached := ok && now.Before(entry.expiresAt)
	s.zoneCacheMu.RUnlock()

	if cached {
		return entry.zone, nil
	}

	// Cache miss or expired, fetch from external service
	z, err := s.ext.GetTariffZoneData(ctx, zoneID)
	if err != nil || z == nil {
		// External service failed, try cache again with read lock
		s.zoneCacheMu.RLock()
		entry, ok := s.zoneCache[zoneID]
		fallbackCached := ok && now.Before(entry.expiresAt)
		s.zoneCacheMu.RUnlock()

		if fallbackCached {
			return entry.zone, nil
		}
		return nil, ErrZoneUnavailable
	}

	// Update cache with write lock
	s.zoneCacheMu.Lock()
	s.zoneCache[zoneID] = zoneCacheEntry{
		zone:      z,
		expiresAt: time.Now().Add(s.zoneCacheTTL),
	}
	s.zoneCacheMu.Unlock()

	return z, nil
}

// getDefaultConfigs returns default configuration values
func (s *Service) getDefaultConfigs() *external.DynamicConfigs {
	return &external.DynamicConfigs{
		Surge:                          1.2,
		LowChargeDiscount:              0.7,
		LowChargeThresholdPercent:      28,
		IncompleteRideThresholdSeconds: 5,
	}
}

// getConfigsCached returns current configs (thread-safe read)
// On first call, attempts to fetch configs from external service synchronously if still using defaults
func (s *Service) getConfigsCached(ctx context.Context) *external.DynamicConfigs {
	s.configsMu.RLock()
	cfg := s.configs
	isDefault := cfg != nil && cfg.IncompleteRideThresholdSeconds == 5
	s.configsMu.RUnlock()
	
	// If still using default configs, try to update synchronously (for tests and first use)
	if isDefault {
		cfg, err := s.ext.GetConfigs(ctx)
		if err == nil && cfg != nil {
			s.configsMu.Lock()
			s.configs = cfg
			s.configsMu.Unlock()
			return cfg
		}
	}
	
	return cfg
}

// updateConfigs attempts to fetch and update configs from external service.
// If the service is unavailable, old values are preserved.
func (s *Service) updateConfigs() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cfg, err := s.ext.GetConfigs(ctx)
	if err != nil || cfg == nil {
		// Service unavailable, keep old values
		return
	}

	// Update configs with write lock
	s.configsMu.Lock()
	s.configs = cfg
	s.configsMu.Unlock()
}

// periodicConfigUpdate runs periodic updates of configs from external service.
// Updates every 5 seconds. If service is unavailable, old values are preserved.
func (s *Service) periodicConfigUpdate() {
	defer s.cleanupDone.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial update attempt
	s.updateConfigs()

	for {
		select {
		case <-ticker.C:
			s.updateConfigs()
		case <-s.stopCleanup:
			return
		}
	}
}

// Stop stops the periodic config update goroutine.
// Should be called when the service is being shut down.
func (s *Service) Stop() {
	close(s.stopCleanup)
	s.cleanupDone.Wait()
}
