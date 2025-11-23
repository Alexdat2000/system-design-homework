package offers

import (
	"client/api"
	"context"
)

// Repository defines the interface for offer storage operations
type Repository interface {
	// GetOffer retrieves an offer by ID from Redis
	// Returns (nil, nil) if not found
	GetOffer(ctx context.Context, offerID string) (*api.Offer, error)

	// SaveOffer stores an offer with TTL derived from offer.ExpiresAt
	SaveOffer(ctx context.Context, offer *api.Offer) error

	// MarkOfferAsUsed marks an offer as used (atomically). Returns true if marked, false if already used.
	MarkOfferAsUsed(ctx context.Context, offerID string) (bool, error)

	// GetOfferByUserScooter returns an existing valid offer for (user_id, scooter_id) if present.
	// Implementation typically uses an index key that points to the actual offer id and validates TTL.
	GetOfferByUserScooter(ctx context.Context, userID, scooterID string) (*api.Offer, error)

	// SetOfferByUserScooter indexes offer id by (user_id, scooter_id) for fast idempotent lookup.
	// TTL should match main offer TTL.
	SetOfferByUserScooter(ctx context.Context, userID, scooterID, offerID string) error
}
