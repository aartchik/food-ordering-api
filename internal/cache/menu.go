package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"food-ordering-api/internal/models"

	"github.com/redis/go-redis/v9"
)

var ErrMiss = errors.New("cache miss")

var ErrUnavailable = errors.New("cache unavailable")

type MenuCache struct {
	client  *redis.Client
	ttl     time.Duration
	timeout time.Duration
}

func NewMenuCache(client *redis.Client, ttl, timeout time.Duration) *MenuCache {
	return &MenuCache{client: client, ttl: ttl, timeout: timeout}
}

func (c *MenuCache) PingContext(ctx context.Context) error {
	if c == nil || c.client == nil {
		return ErrUnavailable
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.Ping(ctx).Err()
}

func (c *MenuCache) Get(ctx context.Context, restaurantID int64, availableOnly bool) ([]*models.MenuItem, error) {
	if c == nil || c.client == nil {
		return nil, ErrMiss
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	value, err := c.client.Get(ctx, menuKey(restaurantID, availableOnly)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrMiss
		}
		return nil, err
	}

	var items []*models.MenuItem
	if err := json.Unmarshal(value, &items); err != nil {
		return nil, fmt.Errorf("decode cached menu: %w", err)
	}
	if items == nil {
		items = []*models.MenuItem{}
	}

	return items, nil
}

func (c *MenuCache) Set(ctx context.Context, restaurantID int64, availableOnly bool, items []*models.MenuItem) error {
	if c == nil || c.client == nil {
		return nil
	}

	value, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("encode menu for cache: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.Set(ctx, menuKey(restaurantID, availableOnly), value, c.ttl).Err()
}

func (c *MenuCache) Delete(ctx context.Context, restaurantID int64) error {
	if c == nil || c.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.Del(ctx, menuKey(restaurantID, false), menuKey(restaurantID, true)).Err()
}

func menuKey(restaurantID int64, availableOnly bool) string {
	view := "all"
	if availableOnly {
		view = "available"
	}
	return fmt.Sprintf("menu:v1:%d:%s", restaurantID, view)
}
