package redis

import (
	"client/api"
	"context"
	"time"
)

// OfferRepository is a stub implementation of offers.Repository
type OfferRepository struct {
	// In real implementation, this would contain Redis client
}

// NewOfferRepository creates a new offer repository
func NewOfferRepository() *OfferRepository {
	return &OfferRepository{}
}

// GetOffer retrieves an offer by ID from Redis (stub implementation)
func (r *OfferRepository) GetOffer(ctx context.Context, offerID string) (*api.Offer, error) {
	// Stub: return a dummy offer for testing
	// In real implementation, this would query Redis
	now := time.Now()
	return &api.Offer{
		Id:             offerID,
		UserId:         "user-123",
		ScooterId:      "scooter-456",
		ZoneId:         "zone-1",
		Deposit:        100,
		PricePerMinute: 10,
		PriceUnlock:    20,
		ExpiresAt:      now.Add(5 * time.Minute),
		CreatedAt:      &now,
	}, nil
}

// MarkOfferAsUsed marks an offer as used atomically (stub implementation)
func (r *OfferRepository) MarkOfferAsUsed(ctx context.Context, offerID string) (bool, error) {
	// Stub: always return true (successfully marked as used)
	// In real implementation, this would use Redis SETNX or similar atomic operation
	return true, nil
}
