package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-ordering-api/internal/models"
)

type partnerCatalogUpdateFunc func(string, *models.RestaurantCatalogInput) (*models.Restaurant, []*models.MenuItem, error)

func (f partnerCatalogUpdateFunc) Update(partnerID string, input *models.RestaurantCatalogInput) (*models.Restaurant, []*models.MenuItem, error) {
	return f(partnerID, input)
}

const validCatalogBody = `{"restaurant":{"name":"Bakery","description":"Fresh bread","address":"Main street","is_open":true},"items":[{"partner_item_id":"bread","name":"Bread","description":"Wheat bread","price_kopecks":15000,"is_available":true,"preparation_min":5}]}`

func TestUpdatePartnerCatalog(t *testing.T) {
	app := catalogTestApp()
	app.partners = configuredPartners{"partner": apiKeyHash("secret")}
	called := false
	store := partnerCatalogUpdateFunc(func(partnerID string, input *models.RestaurantCatalogInput) (*models.Restaurant, []*models.MenuItem, error) {
		called = true
		if partnerID != "partner" || input.Restaurant.Name != "Bakery" || len(input.Items) != 1 || input.Items[0].PartnerItemID != "bread" {
			t.Fatalf("input: %q %+v", partnerID, input)
		}
		return &models.Restaurant{ID: 1, PartnerID: partnerID, Name: input.Restaurant.Name}, []*models.MenuItem{{ID: 2, PartnerItemID: "bread"}}, nil
	})

	request := httptest.NewRequest(http.MethodPut, "/v1/partner/catalog", strings.NewReader(validCatalogBody))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	app.requirePartner(app.updatePartnerCatalog(store)).ServeHTTP(response, request)

	if response.Code != http.StatusOK || !called || !json.Valid(response.Body.Bytes()) {
		t.Fatalf("response: %d %s, called=%v", response.Code, response.Body.String(), called)
	}
	var body struct {
		Restaurant models.Restaurant  `json:"restaurant"`
		Items      []*models.MenuItem `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Restaurant.PartnerID != "partner" || len(body.Items) != 1 {
		t.Fatalf("body: %+v", body)
	}
}

func TestUpdatePartnerCatalogRejectsInvalidRequest(t *testing.T) {
	app := catalogTestApp()
	app.partners = configuredPartners{"partner": apiKeyHash("secret")}
	store := partnerCatalogUpdateFunc(func(string, *models.RestaurantCatalogInput) (*models.Restaurant, []*models.MenuItem, error) {
		t.Fatal("invalid request reached store")
		return nil, nil, nil
	})

	tests := []struct {
		name, authorization, body string
		status                    int
	}{
		{name: "missing credentials", body: validCatalogBody, status: 401},
		{name: "invalid credentials", authorization: "Bearer wrong", body: validCatalogBody, status: 401},
		{name: "malformed json", authorization: "Bearer secret", body: "{", status: 400},
		{name: "unknown field", authorization: "Bearer secret", body: `{"restaurant":{"name":"Bakery","id":1},"items":[]}`, status: 400},
		{name: "empty body", authorization: "Bearer secret", body: `{}`, status: 422},
		{name: "empty menu", authorization: "Bearer secret", body: `{"restaurant":{"name":"Bakery"},"items":[]}`, status: 422},
		{name: "duplicate items", authorization: "Bearer secret", body: strings.Replace(validCatalogBody, `]}`, `,{"partner_item_id":"bread","name":"Other","price_kopecks":100}]}`, 1), status: 422},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/v1/partner/catalog", strings.NewReader(tt.body))
			request.Header.Set("Authorization", tt.authorization)
			response := httptest.NewRecorder()
			app.requirePartner(app.updatePartnerCatalog(store)).ServeHTTP(response, request)
			if response.Code != tt.status || !json.Valid(response.Body.Bytes()) {
				t.Fatalf("response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestUpdatePartnerCatalogErrors(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
	}{{models.ErrInvalidInput, 422}, {errors.New("private database error"), 500}} {
		app := catalogTestApp()
		request := httptest.NewRequest(http.MethodPut, "/v1/partner/catalog", strings.NewReader(validCatalogBody))
		request = request.WithContext(withPartnerID(request, "partner"))
		response := httptest.NewRecorder()
		app.updatePartnerCatalog(partnerCatalogUpdateFunc(func(string, *models.RestaurantCatalogInput) (*models.Restaurant, []*models.MenuItem, error) {
			return nil, nil, tt.err
		})).ServeHTTP(response, request)
		if response.Code != tt.status || strings.Contains(response.Body.String(), "private database error") {
			t.Fatalf("response: %d %s", response.Code, response.Body.String())
		}
	}
}

func withPartnerID(r *http.Request, partnerID string) context.Context {
	return context.WithValue(r.Context(), partnerIDContextKey, partnerID)
}
