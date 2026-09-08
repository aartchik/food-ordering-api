package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"food-ordering-api/internal/cache"
	"food-ordering-api/internal/models"
)

type menuCacheStub struct {
	items    []*models.MenuItem
	getErr   error
	setErr   error
	getCalls int
	setCalls int
}

func (c *menuCacheStub) Get(_ context.Context, _ int64, _ bool) ([]*models.MenuItem, error) {
	c.getCalls++
	return c.items, c.getErr
}

func (c *menuCacheStub) Set(_ context.Context, _ int64, _ bool, _ []*models.MenuItem) error {
	c.setCalls++
	return c.setErr
}

type menuListFunc func(int64, bool) ([]*models.MenuItem, error)

func (f menuListFunc) GetAllForRestaurant(id int64, available bool) ([]*models.MenuItem, error) {
	return f(id, available)
}

func TestListMenuItems(t *testing.T) {
	for _, tt := range []struct {
		name, suffix           string
		available, empty       bool
		restaurantErr, menuErr error
		status                 int
	}{
		{name: "all", status: 200},
		{name: "available", suffix: "?available_only=true", available: true, status: 200},
		{name: "empty", empty: true, status: 200},
		{name: "missing restaurant", restaurantErr: models.ErrRecordNotFound, status: 404},
		{name: "restaurant failure", restaurantErr: errors.New("database error"), status: 500},
		{name: "menu failure", menuErr: errors.New("database error"), status: 500},
		{name: "invalid filter", suffix: "?available_only=yes", status: 422},
	} {
		t.Run(tt.name, func(t *testing.T) {
			menuCache := &menuCacheStub{getErr: cache.ErrMiss}
			restaurants := restaurantGetFunc(func(id int64) (*models.Restaurant, error) {
				if tt.status == 422 || id != 7 {
					t.Fatal("unexpected restaurant query")
				}
				return &models.Restaurant{ID: id}, tt.restaurantErr
			})
			menu := menuListFunc(func(id int64, available bool) ([]*models.MenuItem, error) {
				if tt.restaurantErr != nil || tt.status == 422 || id != 7 || available != tt.available {
					t.Fatal("unexpected menu query")
				}
				if tt.empty || tt.menuErr != nil {
					return nil, tt.menuErr
				}
				return []*models.MenuItem{{ID: 3, Name: "Bread", PriceKopecks: 15000}}, nil
			})
			w := catalogRequest(t, "/v1/restaurants/:id/menu", "/v1/restaurants/7/menu"+tt.suffix, catalogTestApp().listMenuItems(restaurants, menu, menuCache), tt.status)
			if tt.status == 200 {
				var body struct {
					Items []*models.MenuItem `json:"items"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Items == nil {
					t.Fatal("null items")
				}
				if tt.empty {
					if len(body.Items) != 0 {
						t.Fatal(body)
					}
				} else if len(body.Items) != 1 || body.Items[0].PriceKopecks != 15000 {
					t.Fatal(body)
				}
			}
		})
	}
}

func TestListMenuItemsUsesCache(t *testing.T) {
	cached := []*models.MenuItem{{ID: 9, RestaurantID: 7, Name: "Cached bread"}}
	menuCache := &menuCacheStub{items: cached}
	restaurants := restaurantGetFunc(func(id int64) (*models.Restaurant, error) {
		return &models.Restaurant{ID: id}, nil
	})
	menu := menuListFunc(func(int64, bool) ([]*models.MenuItem, error) {
		t.Fatal("cache hit reached database")
		return nil, nil
	})

	w := catalogRequest(t, "/v1/restaurants/:id/menu", "/v1/restaurants/7/menu", catalogTestApp().listMenuItems(restaurants, menu, menuCache), http.StatusOK)
	if !strings.Contains(w.Body.String(), "Cached bread") || menuCache.getCalls != 1 || menuCache.setCalls != 0 {
		t.Fatalf("response: %s, cache: %+v", w.Body.String(), menuCache)
	}
}

func TestListMenuItemsFallsBackWhenCacheFails(t *testing.T) {
	menuCache := &menuCacheStub{getErr: errors.New("redis unavailable"), setErr: errors.New("redis unavailable")}
	restaurants := restaurantGetFunc(func(id int64) (*models.Restaurant, error) {
		return &models.Restaurant{ID: id}, nil
	})
	menu := menuListFunc(func(int64, bool) ([]*models.MenuItem, error) {
		return []*models.MenuItem{{ID: 3, Name: "Database bread"}}, nil
	})

	w := catalogRequest(t, "/v1/restaurants/:id/menu", "/v1/restaurants/7/menu", catalogTestApp().listMenuItems(restaurants, menu, menuCache), http.StatusOK)
	if !strings.Contains(w.Body.String(), "Database bread") || menuCache.getCalls != 1 || menuCache.setCalls != 1 {
		t.Fatalf("response: %s, cache: %+v", w.Body.String(), menuCache)
	}
}
