package offers

import (
	"client/api"
	"context"
)

// Repository defines the interface for offer storage operations
type Repository interface {
	// GetOffer retrieves an offer by ID from Redis
	GetOffer(ctx context.Context, offerID string) (*api.Offer, error)

	// MarkOfferAsUsed marks an offer as used (atomically)
	MarkOfferAsUsed(ctx context.Context, offerID string) (bool, error)
}
