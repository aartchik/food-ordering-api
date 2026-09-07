package main

import "net/http"

func (app *application) healthcheck(w http.ResponseWriter, r *http.Request) {
	data := envelope{
		"status": "available",
		"system": "food-ordering-api",
		"env":    app.config.env,
	}

	err := app.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		app.serverError(w, r, err)
	}
}
