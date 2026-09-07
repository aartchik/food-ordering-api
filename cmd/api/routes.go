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

	standard := alice.New(app.recoverPanic, app.rateLimit, app.logRequest, app.requestID, secureHeaders)

	return standard.Then(router)
}
