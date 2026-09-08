package main

import (
	"context"
	"errors"
	"net/http"

	"food-ordering-api/internal/cache"
	"food-ordering-api/internal/models"
	"food-ordering-api/internal/validator"
)

type menuItemLister interface {
	GetAllForRestaurant(int64, bool) ([]*models.MenuItem, error)
}

type menuCacheReaderWriter interface {
	Get(context.Context, int64, bool) ([]*models.MenuItem, error)
	Set(context.Context, int64, bool, []*models.MenuItem) error
}

func (app *application) listMenuItems(restaurants restaurantGetter, menu menuItemLister, menuCache menuCacheReaderWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		v := validator.New()
		availableOnly := readBoolQuery(r.URL.Query(), "available_only", false, v)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		if _, err := restaurants.Get(id); err != nil {
			if errors.Is(err, models.ErrRecordNotFound) {
				app.notFoundResponse(w, r)
			} else {
				app.serverError(w, r, err)
			}
			return
		}
		if items, err := menuCache.Get(r.Context(), id, availableOnly); err == nil {
			app.metrics.recordMenuCache("read", "hit")
			if err := app.writeJSON(w, http.StatusOK, envelope{"items": items}, nil); err != nil {
				app.serverError(w, r, err)
			}
			return
		} else if errors.Is(err, cache.ErrMiss) {
			app.metrics.recordMenuCache("read", "miss")
		} else {
			app.metrics.recordMenuCache("read", "error")
			app.errorLog.Printf("read menu cache: %v", err)
		}
		items, err := menu.GetAllForRestaurant(id, availableOnly)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if items == nil {
			items = []*models.MenuItem{}
		}
		if err := menuCache.Set(r.Context(), id, availableOnly, items); err != nil {
			app.metrics.recordMenuCache("write", "error")
			app.errorLog.Printf("write menu cache: %v", err)
		} else {
			app.metrics.recordMenuCache("write", "success")
		}
		if err := app.writeJSON(w, http.StatusOK, envelope{"items": items}, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}
