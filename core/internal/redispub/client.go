package redispub

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	redis *redis.Client
}

func New(addr string) (*Client, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{redis: client}, nil
}

func (c *Client) Publish(ctx context.Context, channel string, payload []byte) error {
	if err := c.redis.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("publish %s: %w", channel, err)
	}

	return nil
}
