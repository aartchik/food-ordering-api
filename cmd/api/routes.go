package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()
	register := func(method, path string, handler http.Handler) {
		router.Handler(method, path, app.metrics.observeHTTP(method, path, handler))
	}

	router.NotFound = app.metrics.observeHTTP("UNMATCHED", "unmatched", http.HandlerFunc(app.notFoundResponse))
	router.MethodNotAllowed = app.metrics.observeHTTP("UNMATCHED", "unmatched", http.HandlerFunc(app.methodNotAllowedResponse))

	register(http.MethodGet, "/healthz", http.HandlerFunc(app.healthcheck))
	register(http.MethodGet, "/readyz", app.readiness(app.db, app.menuCache))
	register(http.MethodGet, "/v1/restaurants", app.listRestaurants(app.models.Restaurants))
	register(http.MethodGet, "/v1/restaurants/:id", app.showRestaurant(app.models.Restaurants))
	register(http.MethodGet, "/v1/restaurants/:id/menu", app.listMenuItems(app.models.Restaurants, app.models.MenuItems, app.menuCache))
	register(http.MethodPost, "/v1/carts", app.createCart(app.models.Carts))
	register(http.MethodGet, "/v1/carts/:id", app.showCart(app.models.Carts))
	register(http.MethodPost, "/v1/carts/:id/items", app.addCartItem(app.models.Carts))
	register(http.MethodPatch, "/v1/carts/:id/items/:item_id", app.updateCartItem(app.models.Carts))
	register(http.MethodDelete, "/v1/carts/:id/items/:item_id", app.deleteCartItem(app.models.Carts))
	register(http.MethodPost, "/v1/orders", app.createOrder(app.models.Orders))
	register(http.MethodGet, "/v1/orders/:id", app.showOrder(app.models.Orders))

	partner := alice.New(app.requirePartner)
	register(http.MethodPut, "/v1/partner/catalog", partner.Then(app.updatePartnerCatalog(app.models.Catalog, app.menuCache)))
	register(http.MethodGet, "/v1/partner/orders", partner.Then(app.listPartnerOrders(app.models.Orders)))
	register(http.MethodGet, "/v1/partner/orders/:id", partner.Then(app.showPartnerOrder(app.models.Orders)))
	register(http.MethodPatch, "/v1/partner/orders/:id/status", partner.Then(app.updatePartnerOrderStatus(app.models.Orders)))
	register(http.MethodGet, "/metrics", app.metrics.handler())

	standard := alice.New(app.recoverPanic, secureHeaders, app.requestID, app.logRequest, app.rateLimit)

	return standard.Then(router)
}
