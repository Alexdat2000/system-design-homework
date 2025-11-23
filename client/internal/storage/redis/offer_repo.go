package redis

import (
	"client/api"
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// OfferRepository implements offer storage over Redis
type OfferRepository struct {
	client *Client
}

// NewOfferRepository creates a new offer repository with Redis client
func NewOfferRepository(client *Client) *OfferRepository {
	return &OfferRepository{client: client}
}

func keyOffer(offerID string) string {
	return fmt.Sprintf("offer:%s", offerID)
}

func keyOfferUsed(offerID string) string {
	return fmt.Sprintf("offer:%s:used", offerID)
}

func keyIdxUserScooter(userID, scooterID string) string {
	return fmt.Sprintf("offer_idx:user:%s:scooter:%s", userID, scooterID)
}

// GetOffer retrieves an offer by ID from Redis
// Returns (nil, nil) if offer not found
func (r *OfferRepository) GetOffer(ctx context.Context, offerID string) (*api.Offer, error) {
	if r.client == nil || r.client.rdb == nil {
		return nil, fmt.Errorf("redis client is not initialized")
	}

	data, err := r.client.rdb.Get(ctx, keyOffer(offerID)).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis GET failed: %w", err)
	}

	var offer api.Offer
	if err := json.Unmarshal(data, &offer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal offer: %w", err)
	}

	return &offer, nil
}

// MarkOfferAsUsed tries to atomically mark the offer as used via SETNX on a separate key.
// It sets the TTL of the used-key equal to the remaining TTL of the offer key to avoid leaks.
func (r *OfferRepository) MarkOfferAsUsed(ctx context.Context, offerID string) (bool, error) {
	if r.client == nil || r.client.rdb == nil {
		return false, fmt.Errorf("redis client is not initialized")
	}

	offerKey := keyOffer(offerID)
	usedKey := keyOfferUsed(offerID)

	ttl, err := r.client.rdb.TTL(ctx, offerKey).Result()
	if err != nil && err != goredis.Nil {
		return false, fmt.Errorf("redis TTL failed: %w", err)
	}

	// If TTL is negative (no key or no expiration), fallback to 5 minutes as a safe default
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	ok, err := r.client.rdb.SetNX(ctx, usedKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis SETNX failed: %w", err)
	}

	return ok, nil
}

// SaveOffer stores offer with TTL derived from offer.ExpiresAt.
// Returns error if offer is already expired by the time of call.
func (r *OfferRepository) SaveOffer(ctx context.Context, offer *api.Offer) error {
	if r.client == nil || r.client.rdb == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	if offer == nil || offer.Id == "" {
		return fmt.Errorf("invalid offer")
	}

	now := time.Now()
	if now.After(offer.ExpiresAt) {
		return fmt.Errorf("offer already expired")
	}

	ttl := time.Until(offer.ExpiresAt)
	// Guarantee minimal TTL > 0
	if ttl <= 0 {
		ttl = time.Second
	}

	payload, err := json.Marshal(offer)
	if err != nil {
		return fmt.Errorf("failed to marshal offer: %w", err)
	}

	if err := r.client.rdb.Set(ctx, keyOffer(offer.Id), payload, ttl).Err(); err != nil {
		return fmt.Errorf("redis SET failed: %w", err)
	}

	return nil
}

// GetOfferByUserScooter returns an existing valid offer for (user_id, scooter_id) if present
func (r *OfferRepository) GetOfferByUserScooter(ctx context.Context, userID, scooterID string) (*api.Offer, error) {
	if r.client == nil || r.client.rdb == nil {
		return nil, fmt.Errorf("redis client is not initialized")
	}
	idxKey := keyIdxUserScooter(userID, scooterID)
	offerID, err := r.client.rdb.Get(ctx, idxKey).Result()
	if err != nil {
		if err == goredis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis GET index failed: %w", err)
	}
	// fetch offer itself
	return r.GetOffer(ctx, offerID)
}

// SetOfferByUserScooter indexes offer id by (user_id, scooter_id) with same TTL as offer key
func (r *OfferRepository) SetOfferByUserScooter(ctx context.Context, userID, scooterID, offerID string) error {
	if r.client == nil || r.client.rdb == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	offerKey := keyOffer(offerID)
	ttl, err := r.client.rdb.TTL(ctx, offerKey).Result()
	if err != nil && err != goredis.Nil {
		return fmt.Errorf("redis TTL failed: %w", err)
	}
	// Safety net
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	idxKey := keyIdxUserScooter(userID, scooterID)
	if err := r.client.rdb.Set(ctx, idxKey, offerID, ttl).Err(); err != nil {
		return fmt.Errorf("redis SET index failed: %w", err)
	}
	return nil
}
