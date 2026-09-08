package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"food-ordering-api/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestMenuCache(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close redis client: %v", err)
		}
	})

	ctx := context.Background()
	menuCache := NewMenuCache(client, 5*time.Minute, time.Second)
	items := []*models.MenuItem{{ID: 3, RestaurantID: 7, Name: "Bread"}}

	if _, err := menuCache.Get(ctx, 7, false); !errors.Is(err, ErrMiss) {
		t.Fatalf("empty cache error: %v", err)
	}
	if err := menuCache.Set(ctx, 7, false, items); err != nil {
		t.Fatal(err)
	}

	cached, err := menuCache.Get(ctx, 7, false)
	if err != nil || len(cached) != 1 || cached[0].Name != "Bread" {
		t.Fatalf("cached menu: %+v, error: %v", cached, err)
	}
	if ttl := server.TTL(menuKey(7, false)); ttl != 5*time.Minute {
		t.Fatalf("ttl: %s", ttl)
	}
	if _, err := menuCache.Get(ctx, 7, true); !errors.Is(err, ErrMiss) {
		t.Fatalf("available-only cache error: %v", err)
	}
}

func TestMenuCacheDeleteRemovesBothViews(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close redis client: %v", err)
		}
	})
	menuCache := NewMenuCache(client, time.Minute, time.Second)
	ctx := context.Background()

	for _, availableOnly := range []bool{false, true} {
		if err := menuCache.Set(ctx, 7, availableOnly, []*models.MenuItem{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := menuCache.Delete(ctx, 7); err != nil {
		t.Fatal(err)
	}
	for _, availableOnly := range []bool{false, true} {
		if _, err := menuCache.Get(ctx, 7, availableOnly); !errors.Is(err, ErrMiss) {
			t.Fatalf("view %v still cached: %v", availableOnly, err)
		}
	}
}

func TestMenuCacheRejectsMalformedValue(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close redis client: %v", err)
		}
	})
	menuCache := NewMenuCache(client, time.Minute, time.Second)

	server.Set(menuKey(7, false), "not-json")
	if _, err := menuCache.Get(context.Background(), 7, false); err == nil || errors.Is(err, ErrMiss) {
		t.Fatalf("malformed cache error: %v", err)
	}
}

func TestDisabledMenuCache(t *testing.T) {
	menuCache := NewMenuCache(nil, time.Minute, time.Second)
	if _, err := menuCache.Get(context.Background(), 7, false); !errors.Is(err, ErrMiss) {
		t.Fatalf("get error: %v", err)
	}
	if err := menuCache.Set(context.Background(), 7, false, nil); err != nil {
		t.Fatalf("set error: %v", err)
	}
	if err := menuCache.Delete(context.Background(), 7); err != nil {
		t.Fatalf("delete error: %v", err)
	}
}
