package main

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"food-ordering-api/internal/validator"
	"github.com/julienschmidt/httprouter"
)

func readIDParam(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(httprouter.ParamsFromContext(r.Context()).ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

func readIntQuery(q url.Values, key string, fallback int, v *validator.Validator) int {
	if !q.Has(key) {
		return fallback
	}
	value, err := strconv.Atoi(q.Get(key))
	if err != nil {
		v.AddError(key, "must be an integer")
		return fallback
	}
	return value
}

func readBoolQuery(q url.Values, key string, fallback bool, v *validator.Validator) bool {
	if !q.Has(key) {
		return fallback
	}
	switch q.Get(key) {
	case "true":
		return true
	case "false":
		return false
	default:
		v.AddError(key, "must be true or false")
		return fallback
	}
}
