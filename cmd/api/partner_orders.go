package main

import (
	"errors"
	"net/http"

	"food-ordering-api/internal/models"
	"food-ordering-api/internal/validator"
)

type partnerOrderLister interface {
	GetAllForPartner(string, models.PartnerOrderListFilter) ([]*models.OrderView, models.Metadata, error)
}

type partnerOrderGetter interface {
	GetForPartner(string, int64) (*models.OrderView, error)
}

type partnerOrderStatusUpdater interface {
	UpdateStatusForPartner(string, int64, *models.OrderStatusUpdateInput) (*models.OrderView, error)
}

func (app *application) listPartnerOrders(store partnerOrderLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v := validator.New()
		q := r.URL.Query()
		input := models.PartnerOrderListFilter{
			Status: q.Get("status"),
			Filters: models.Filters{
				Page:         readIntQuery(q, "page", 1, v),
				PageSize:     readIntQuery(q, "page_size", 20, v),
				Sort:         "-created_at",
				SortSafelist: []string{"created_at", "-created_at", "updated_at", "-updated_at", "status", "-status"},
			},
		}
		if q.Has("sort") {
			input.Sort = q.Get("sort")
		}
		if input.Status != "" {
			models.ValidateOrderStatus(v, input.Status)
		}
		models.ValidateFilters(v, input.Filters)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}

		orders, metadata, err := store.GetAllForPartner(partnerIDFromContext(r), input)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if orders == nil {
			orders = []*models.OrderView{}
		}
		if err := app.writeJSON(w, http.StatusOK, envelope{"orders": orders, "metadata": metadata}, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}

func (app *application) showPartnerOrder(store partnerOrderGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		order, err := store.GetForPartner(partnerIDFromContext(r), id)
		if err != nil {
			if errors.Is(err, models.ErrRecordNotFound) {
				app.notFoundResponse(w, r)
			} else {
				app.serverError(w, r, err)
			}
			return
		}
		if err := app.writeJSON(w, http.StatusOK, order, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}

func (app *application) updatePartnerOrderStatus(store partnerOrderStatusUpdater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := readIDParam(r)
		if err != nil {
			app.notFoundResponse(w, r)
			return
		}
		var input models.OrderStatusUpdateInput
		if err := app.readJSON(w, r, &input); err != nil {
			app.badRequestResponse(w, r, err)
			return
		}
		v := validator.New()
		models.ValidateOrderStatusUpdateInput(v, &input)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}

		order, err := store.UpdateStatusForPartner(partnerIDFromContext(r), id, &input)
		if err != nil {
			switch {
			case errors.Is(err, models.ErrRecordNotFound):
				app.notFoundResponse(w, r)
			case errors.Is(err, models.ErrEditConflict):
				app.errorResponse(w, r, http.StatusConflict, "the order was updated by another request")
			case errors.Is(err, models.ErrInvalidTransition):
				app.errorResponse(w, r, http.StatusConflict, models.ErrInvalidTransition.Error())
			case errors.Is(err, models.ErrInvalidInput):
				app.errorResponse(w, r, http.StatusUnprocessableEntity, "invalid status update")
			default:
				app.serverError(w, r, err)
			}
			return
		}
		if err := app.writeJSON(w, http.StatusOK, order, nil); err != nil {
			app.serverError(w, r, err)
		}
	}
}
