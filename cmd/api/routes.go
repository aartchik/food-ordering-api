package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.notFound(w)
	})

	router.HandlerFunc(http.MethodGet, "/healthz", app.healthcheck)

	standard := alice.New(app.recoverPanic, app.logRequest, app.requestID, secureHeaders)

	return standard.Then(router)
}
