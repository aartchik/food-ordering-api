package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"food-ordering-api/internal/models"
	"food-ordering-api/internal/validator"
)

type orderCreator interface {
	CreateFromCart(*models.CheckoutInput) (*models.OrderView, error)
}

type orderGetter interface {
	Get(int64) (*models.OrderView, error)
}

func (app *application) createOrder(store orderCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input models.CheckoutInput
		if err := app.readJSON(w, r, &input); err != nil {
			app.badRequestResponse(w, r, err)
			return
		}
		input.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		v := validator.New()
		models.ValidateCheckoutInput(v, &input)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		order, err := store.CreateFromCart(&input)
		if err != nil {
			app.orderErrorResponse(w, r, err)
			return
		}
		headers := make(http.Header)
		headers.Set("Location", fmt.Sprintf("/v1/orders/%d", order.Order.ID))
		// A replay returns the same resource and success status as the first request.
		if err := app.writeJSON(w, http.StatusCreated, order, headers); err != nil {
			app.serverError(w, r, err)
		}
	}
}

func (app *application) showOrder(store orderGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		order, err := store.Get(id)
		if err != nil {
			app.orderErrorResponse(w, r, err)
			return
		}
		if err := app.writeJSON(w, http.StatusOK, order, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}

func (app *application) orderErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, models.ErrRecordNotFound):
		app.notFoundResponse(w, r)
	case errors.Is(err, models.ErrCartIsEmpty):
		app.errorResponse(w, r, http.StatusConflict, models.ErrCartIsEmpty.Error())
	case errors.Is(err, models.ErrItemUnavailable):
		app.errorResponse(w, r, http.StatusConflict, models.ErrItemUnavailable.Error())
	case errors.Is(err, models.ErrCartChanged):
		var changed *models.CartChangedError
		if !errors.As(err, &changed) {
			app.serverError(w, r, err)
			return
		}
		app.errorResponse(w, r, http.StatusConflict, envelope{
			"message": models.ErrCartChanged.Error(),
			"items":   changed.Items,
		})
	case errors.Is(err, models.ErrMixedCart):
		app.errorResponse(w, r, http.StatusConflict, models.ErrMixedCart.Error())
	case errors.Is(err, models.ErrIdempotencyConflict):
		app.errorResponse(w, r, http.StatusConflict, models.ErrIdempotencyConflict.Error())
	case errors.Is(err, models.ErrInvalidInput):
		app.errorResponse(w, r, http.StatusUnprocessableEntity, "invalid checkout input")
	default:
		app.serverError(w, r, err)
	}
}
