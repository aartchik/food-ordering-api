package main

import (
	"context"
	"net/http"
	"time"
)

type dependencyPinger interface {
	PingContext(context.Context) error
}

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

func (app *application) readiness(database, redis dependencyPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()

		status := "available"
		httpStatus := http.StatusOK
		checks := map[string]string{}

		if err := database.PingContext(ctx); err != nil {
			checks["postgres"] = "unavailable"
			status = "unavailable"
			httpStatus = http.StatusServiceUnavailable
			app.errorLog.Printf("readiness postgres check: %v", err)
		} else {
			checks["postgres"] = "available"
		}

		if !app.config.redis.enabled {
			checks["redis"] = "disabled"
		} else if err := redis.PingContext(ctx); err != nil {
			checks["redis"] = "unavailable"
			if status == "available" {
				status = "degraded"
			}
			app.errorLog.Printf("readiness redis check: %v", err)
		} else {
			checks["redis"] = "available"
		}

		data := envelope{
			"status": status,
			"system": "food-ordering-api",
			"checks": checks,
		}
		if err := app.writeJSON(w, httpStatus, data, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}
