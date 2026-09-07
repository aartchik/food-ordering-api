package main

import (
	"errors"
	"net/http"

	"food-ordering-api/internal/models"
	"food-ordering-api/internal/validator"
)

type menuItemLister interface {
	GetAllForRestaurant(int64, bool) ([]*models.MenuItem, error)
}

func (app *application) listMenuItems(restaurants restaurantGetter, menu menuItemLister) http.HandlerFunc {
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
		items, err := menu.GetAllForRestaurant(id, availableOnly)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if items == nil {
			items = []*models.MenuItem{}
		}
		if err := app.writeJSON(w, http.StatusOK, envelope{"items": items}, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}
