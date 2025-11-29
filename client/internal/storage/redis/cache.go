package redis

import (
	"client/api"
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type OrderCache struct {
	client *Client
}

func NewOrderCache(client *Client) *OrderCache {
	return &OrderCache{client: client}
}

func orderCacheKey(orderID string) string {
	return fmt.Sprintf("order:%s", orderID)
}

func (c *OrderCache) GetOrder(ctx context.Context, orderID string) (*api.Order, error) {
	if c.client == nil || c.client.rdb == nil {
		return nil, fmt.Errorf("redis client is not initialized")
	}

	data, err := c.client.rdb.Get(ctx, orderCacheKey(orderID)).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis GET failed: %w", err)
	}
	var order api.Order
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order: %w", err)
	}
	return &order, nil
}

func (c *OrderCache) SetOrder(ctx context.Context, order *api.Order, ttl time.Duration) error {
	if c.client == nil || c.client.rdb == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	if order == nil || order.Id == "" {
		return fmt.Errorf("invalid order")
	}
	payload, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("failed to marshal order: %w", err)
	}
	return c.client.rdb.Set(ctx, orderCacheKey(order.Id), payload, ttl).Err()
}

func (c *OrderCache) Invalidate(ctx context.Context, orderID string) error {
	if c.client == nil || c.client.rdb == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return c.client.rdb.Del(ctx, orderCacheKey(orderID)).Err()
}
