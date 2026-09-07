package main

import (
	"errors"
	"net/http"
	"strings"

	"food-ordering-api/internal/models"
	"food-ordering-api/internal/validator"
)

type restaurantLister interface {
	GetAll(models.RestaurantListFilter) ([]*models.Restaurant, models.Metadata, error)
}

type restaurantGetter interface {
	Get(int64) (*models.Restaurant, error)
}

func (app *application) listRestaurants(store restaurantLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := validator.New()
		q := r.URL.Query()
		input := models.RestaurantListFilter{
			Query:    strings.TrimSpace(q.Get("q")),
			OnlyOpen: readBoolQuery(q, "only_open", false, v),
			Filters: models.Filters{
				Page:         readIntQuery(q, "page", 1, v),
				PageSize:     readIntQuery(q, "page_size", 20, v),
				Sort:         "id",
				SortSafelist: []string{"id", "-id", "name", "-name", "created_at", "-created_at"},
			},
		}
		if q.Has("sort") {
			input.Sort = q.Get("sort")
		}
		v.Check(validator.MaxChars(input.Query, 120), "q", "must not be more than 120 characters long")
		models.ValidateFilters(v, input.Filters)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		restaurants, metadata, err := store.GetAll(input)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if restaurants == nil {
			restaurants = []*models.Restaurant{}
		}
		if err := app.writeJSON(w, http.StatusOK, envelope{"restaurants": restaurants, "metadata": metadata}, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}

func (app *application) showRestaurant(store restaurantGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		restaurant, err := store.Get(id)
		if err != nil {
			if errors.Is(err, models.ErrRecordNotFound) {
				app.notFoundResponse(w, r)
			} else {
				app.serverError(w, r, err)
			}
			return
		}
		if err := app.writeJSON(w, http.StatusOK, envelope{"restaurant": restaurant}, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}
