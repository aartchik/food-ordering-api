package main

import (
	"errors"
	"net/http"

	"food-ordering-api/internal/models"
	"food-ordering-api/internal/validator"
)

type partnerCatalogUpdater interface {
	Update(string, *models.RestaurantCatalogInput) (*models.Restaurant, []*models.MenuItem, error)
}

func (app *application) updatePartnerCatalog(store partnerCatalogUpdater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input models.RestaurantCatalogInput
		if err := app.readJSON(w, r, &input); err != nil {
			app.badRequestResponse(w, r, err)
			return
		}

		v := validator.New()
		models.ValidateRestaurantCatalogInput(v, &input)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}

		restaurant, items, err := store.Update(partnerIDFromContext(r), &input)
		if err != nil {
			if errors.Is(err, models.ErrInvalidInput) {
				app.errorResponse(w, r, http.StatusUnprocessableEntity, "invalid catalog")
			} else {
				app.serverError(w, r, err)
			}
			return
		}

		response := envelope{"restaurant": restaurant, "items": items}
		if err := app.writeJSON(w, http.StatusOK, response, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}
