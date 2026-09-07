package main

import (
	"net/http"

	"food-ordering-api/internal/models"
	"food-ordering-api/internal/validator"
)

type cartItemAdder interface {
	AddItem(int64, int64, int) (*models.CartView, error)
}

type cartItemUpdater interface {
	UpdateItem(int64, int64, int) (*models.CartView, error)
}

type cartItemDeleter interface {
	DeleteItem(int64, int64) (*models.CartView, error)
}

func (app *application) addCartItem(store cartItemAdder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		var input models.CartItemInput
		if err := app.readJSON(w, r, &input); err != nil {
			app.badRequestResponse(w, r, err)
			return
		}
		v := validator.New()
		models.ValidateCartItemInput(v, &input)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		cart, err := store.AddItem(id, input.MenuItemID, input.Quantity)
		if err != nil {
			app.cartErrorResponse(w, r, err)
			return
		}
		app.writeCart(w, r, cart)
	}
}

func (app *application) updateCartItem(store cartItemUpdater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		itemID, err := readNamedIDParam(r, "item_id")
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		var input struct {
			Quantity int `json:"quantity"`
		}
		if err := app.readJSON(w, r, &input); err != nil {
			app.badRequestResponse(w, r, err)
			return
		}
		v := validator.New()
		v.Check(input.Quantity >= 1 && input.Quantity <= 99, "quantity", "must be between 1 and 99")
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}
		cart, err := store.UpdateItem(id, itemID, input.Quantity)
		if err != nil {
			app.cartErrorResponse(w, r, err)
			return
		}
		app.writeCart(w, r, cart)
	}
}

func (app *application) deleteCartItem(store cartItemDeleter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		itemID, err := readNamedIDParam(r, "item_id")
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		cart, err := store.DeleteItem(id, itemID)
		if err != nil {
			app.cartErrorResponse(w, r, err)
			return
		}
		app.writeCart(w, r, cart)
	}
}
