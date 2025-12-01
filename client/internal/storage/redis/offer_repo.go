package redis

import (
	"client/api"
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type OfferRepository struct {
	client *Client
}

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

	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	ok, err := r.client.rdb.SetNX(ctx, usedKey, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis SETNX failed: %w", err)
	}

	return ok, nil
}

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
	return r.GetOffer(ctx, offerID)
}

func (r *OfferRepository) SetOfferByUserScooter(ctx context.Context, userID, scooterID, offerID string) error {
	if r.client == nil || r.client.rdb == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	offerKey := keyOffer(offerID)
	ttl, err := r.client.rdb.TTL(ctx, offerKey).Result()
	if err != nil && err != goredis.Nil {
		return fmt.Errorf("redis TTL failed: %w", err)
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	idxKey := keyIdxUserScooter(userID, scooterID)
	if err := r.client.rdb.Set(ctx, idxKey, offerID, ttl).Err(); err != nil {
		return fmt.Errorf("redis SET index failed: %w", err)
	}
	return nil
}
