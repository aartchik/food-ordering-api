package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"food-ordering-api/internal/models"
	"github.com/julienschmidt/httprouter"
)

type partnerOrderListFunc func(string, models.PartnerOrderListFilter) ([]*models.OrderView, models.Metadata, error)

func (f partnerOrderListFunc) GetAllForPartner(partnerID string, input models.PartnerOrderListFilter) ([]*models.OrderView, models.Metadata, error) {
	return f(partnerID, input)
}

type partnerOrderGetFunc func(string, int64) (*models.OrderView, error)

func (f partnerOrderGetFunc) GetForPartner(partnerID string, id int64) (*models.OrderView, error) {
	return f(partnerID, id)
}

func partnerOrderRequest(t *testing.T, path, target string, handler http.HandlerFunc, status int) *httptest.ResponseRecorder {
	t.Helper()
	router := httprouter.New()
	router.HandlerFunc(http.MethodGet, path, handler)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request = request.WithContext(context.WithValue(request.Context(), partnerIDContextKey, "bakery"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != status || !json.Valid(response.Body.Bytes()) {
		t.Fatalf("response: %d %s", response.Code, response.Body.String())
	}
	return response
}

func TestListPartnerOrders(t *testing.T) {
	store := partnerOrderListFunc(func(partnerID string, input models.PartnerOrderListFilter) ([]*models.OrderView, models.Metadata, error) {
		if partnerID != "bakery" || input.Status != models.OrderStatusAccepted || input.Page != 2 || input.PageSize != 5 || input.Sort != "created_at" {
			t.Fatalf("input: %q %+v", partnerID, input)
		}
		return []*models.OrderView{testOrderView()}, models.Metadata{CurrentPage: 2, TotalRecords: 6}, nil
	})
	response := partnerOrderRequest(t, "/v1/partner/orders", "/v1/partner/orders?status=accepted&page=2&page_size=5&sort=created_at", catalogTestApp().listPartnerOrders(store), 200)
	var body struct {
		Orders   []*models.OrderView `json:"orders"`
		Metadata models.Metadata     `json:"metadata"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Orders) != 1 || body.Metadata.TotalRecords != 6 {
		t.Fatalf("body: %+v", body)
	}
}

func TestListPartnerOrdersDefaultsAndEmptyResult(t *testing.T) {
	store := partnerOrderListFunc(func(partnerID string, input models.PartnerOrderListFilter) ([]*models.OrderView, models.Metadata, error) {
		if partnerID != "bakery" || input.Status != "" || input.Page != 1 || input.PageSize != 20 || input.Sort != "-created_at" {
			t.Fatalf("defaults: %q %+v", partnerID, input)
		}
		return nil, models.Metadata{}, nil
	})
	response := partnerOrderRequest(t, "/v1/partner/orders", "/v1/partner/orders", catalogTestApp().listPartnerOrders(store), 200)
	if response.Body.String() == "" || !containsJSONEmptyOrders(response.Body.Bytes()) {
		t.Fatalf("body: %s", response.Body.String())
	}
}

func containsJSONEmptyOrders(data []byte) bool {
	var body struct {
		Orders []json.RawMessage `json:"orders"`
	}
	return json.Unmarshal(data, &body) == nil && body.Orders != nil && len(body.Orders) == 0
}

func TestListPartnerOrdersRejectsInvalidFilters(t *testing.T) {
	queries := []string{"status=unknown", "page=0", "page_size=101", "sort=id", "page=wrong"}
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			store := partnerOrderListFunc(func(string, models.PartnerOrderListFilter) ([]*models.OrderView, models.Metadata, error) {
				t.Fatal("invalid filters reached store")
				return nil, models.Metadata{}, nil
			})
			partnerOrderRequest(t, "/v1/partner/orders", "/v1/partner/orders?"+query, catalogTestApp().listPartnerOrders(store), 422)
		})
	}
}

func TestListPartnerOrdersDatabaseError(t *testing.T) {
	store := partnerOrderListFunc(func(string, models.PartnerOrderListFilter) ([]*models.OrderView, models.Metadata, error) {
		return nil, models.Metadata{}, errors.New("private database error")
	})
	response := partnerOrderRequest(t, "/v1/partner/orders", "/v1/partner/orders", catalogTestApp().listPartnerOrders(store), 500)
	if bytes.Contains(response.Body.Bytes(), []byte("private database error")) {
		t.Fatal("database error leaked")
	}
}

func TestShowPartnerOrder(t *testing.T) {
	tests := []struct {
		name, id string
		err      error
		status   int
	}{
		{name: "success", id: "9", status: 200},
		{name: "missing", id: "9", err: models.ErrRecordNotFound, status: 404},
		{name: "database error", id: "9", err: errors.New("private database error"), status: 500},
		{name: "invalid id", id: "wrong", status: 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := partnerOrderGetFunc(func(partnerID string, id int64) (*models.OrderView, error) {
				if tt.id != "9" || partnerID != "bakery" || id != 9 {
					t.Fatal("unexpected store call")
				}
				return testOrderView(), tt.err
			})
			partnerOrderRequest(t, "/v1/partner/orders/:id", "/v1/partner/orders/"+tt.id, catalogTestApp().showPartnerOrder(store), tt.status)
		})
	}
}
