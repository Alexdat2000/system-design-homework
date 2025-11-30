package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *goredis.Client
}

func NewClient(redisURL string) (*Client, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	// Настройка пула соединений для высокой нагрузки
	opts.PoolSize = 100              // Максимум соединений в пуле
	opts.MinIdleConns = 20           // Минимум idle соединений
	opts.PoolTimeout = 4 * time.Second
	opts.ConnMaxIdleTime = 5 * time.Minute // Максимальное время простоя соединения
	rdb := goredis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}
