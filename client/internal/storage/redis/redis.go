package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client wraps go-redis client
type Client struct {
	rdb *goredis.Client
}

// NewClient creates and pings Redis client using a redis:// URL
func NewClient(redisURL string) (*Client, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	rdb := goredis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Client{rdb: rdb}, nil
}

// Close closes the underlying redis client
func (c *Client) Close() error {
	return c.rdb.Close()
}
