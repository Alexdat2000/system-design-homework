package offers

import (
	"client/api"
	"context"
)

type Repository interface {
	GetOffer(ctx context.Context, offerID string) (*api.Offer, error)

	SaveOffer(ctx context.Context, offer *api.Offer) error

	MarkOfferAsUsed(ctx context.Context, offerID string) (bool, error)

	GetOfferByUserScooter(ctx context.Context, userID, scooterID string) (*api.Offer, error)

	SetOfferByUserScooter(ctx context.Context, userID, scooterID, offerID string) error
}
