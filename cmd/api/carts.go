package main

import (
	"errors"
	"fmt"
	"net/http"

	"food-ordering-api/internal/models"
)

type cartInserter interface {
	Insert(string) (*models.CartView, error)
}

type cartGetter interface {
	Get(int64) (*models.CartView, error)
}

func (app *application) createCart(store cartInserter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cart, err := store.Insert("")
		if err != nil {
			app.cartErrorResponse(w, r, err)
			return
		}
		headers := make(http.Header)
		headers.Set("Location", fmt.Sprintf("/v1/carts/%d", cart.Cart.ID))
		if err := app.writeJSON(w, http.StatusCreated, cart, headers); err != nil {
			app.serverError(w, r, err)
		}
	}
}

func (app *application) showCart(store cartGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		cart, err := store.Get(id)
		if err != nil {
			app.cartErrorResponse(w, r, err)
			return
		}
		app.writeCart(w, r, cart)
	}
}

func (app *application) writeCart(w http.ResponseWriter, r *http.Request, cart *models.CartView) {
	if err := app.writeJSON(w, http.StatusOK, cart, nil); err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) cartErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, models.ErrRecordNotFound):
		app.notFoundResponse(w, r)
	case errors.Is(err, models.ErrItemUnavailable):
		app.errorResponse(w, r, http.StatusConflict, models.ErrItemUnavailable.Error())
	case errors.Is(err, models.ErrMixedCart):
		app.errorResponse(w, r, http.StatusConflict, models.ErrMixedCart.Error())
	case errors.Is(err, models.ErrInvalidInput):
		app.errorResponse(w, r, http.StatusUnprocessableEntity, "invalid cart item")
	default:
		app.serverError(w, r, err)
	}
}
