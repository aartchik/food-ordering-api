package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodGet, "/healthz", app.healthcheck)
	router.HandlerFunc(http.MethodGet, "/v1/restaurants", app.listRestaurants(app.models.Restaurants))
	router.HandlerFunc(http.MethodGet, "/v1/restaurants/:id", app.showRestaurant(app.models.Restaurants))
	router.HandlerFunc(http.MethodGet, "/v1/restaurants/:id/menu", app.listMenuItems(app.models.Restaurants, app.models.MenuItems))
	router.HandlerFunc(http.MethodPost, "/v1/carts", app.createCart(app.models.Carts))
	router.HandlerFunc(http.MethodGet, "/v1/carts/:id", app.showCart(app.models.Carts))
	router.HandlerFunc(http.MethodPost, "/v1/carts/:id/items", app.addCartItem(app.models.Carts))
	router.HandlerFunc(http.MethodPatch, "/v1/carts/:id/items/:item_id", app.updateCartItem(app.models.Carts))
	router.HandlerFunc(http.MethodDelete, "/v1/carts/:id/items/:item_id", app.deleteCartItem(app.models.Carts))
	router.HandlerFunc(http.MethodPost, "/v1/orders", app.createOrder(app.models.Orders))
	router.HandlerFunc(http.MethodGet, "/v1/orders/:id", app.showOrder(app.models.Orders))

	partner := alice.New(app.requirePartner)
	router.Handler(http.MethodPut, "/v1/partner/catalog", partner.Then(app.updatePartnerCatalog(app.models.Catalog)))
	router.Handler(http.MethodGet, "/v1/partner/orders", partner.Then(app.listPartnerOrders(app.models.Orders)))
	router.Handler(http.MethodGet, "/v1/partner/orders/:id", partner.Then(app.showPartnerOrder(app.models.Orders)))
	router.Handler(http.MethodPatch, "/v1/partner/orders/:id/status", partner.Then(app.updatePartnerOrderStatus(app.models.Orders)))

	standard := alice.New(app.recoverPanic, app.rateLimit, app.logRequest, app.requestID, secureHeaders)

	return standard.Then(router)
}
