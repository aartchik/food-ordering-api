package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"food-ordering-api/internal/models"
)

func TestKitchenClientWorkflow(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Accept") != "application/json" {
			t.Fatalf("headers: %v", r.Header)
		}
		switch {
		case requests == 1:
			if r.Method != http.MethodPut || r.URL.Path != "/v1/partner/catalog" || r.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("catalog request: %s %s", r.Method, r.URL.String())
			}
			var input models.RestaurantCatalogInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Restaurant.Name != "Demo Bakery" || len(input.Items) != 3 {
				t.Fatalf("catalog: %+v, %v", input, err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"restaurant":{"id":1},"items":[]}`))
		case requests >= 2 && requests <= 6:
			if r.Method != http.MethodGet || r.URL.Path != "/v1/partner/orders" || r.URL.Query().Get("page_size") != "100" || r.URL.Query().Get("sort") != "created_at" || r.URL.Query().Get("status") == "" {
				t.Fatalf("orders request: %s %s", r.Method, r.URL.RequestURI())
			}
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Query().Get("status") == models.OrderStatusPendingPartner {
				_, _ = w.Write([]byte(`{"orders":[{"order":{"id":9,"status":"pending_partner","version":1},"items":[]}],"metadata":{}}`))
			} else {
				_, _ = w.Write([]byte(`{"orders":[],"metadata":{}}`))
			}
		case requests == 7:
			if r.Method != http.MethodPatch || r.URL.Path != "/v1/partner/orders/9/status" {
				t.Fatalf("status request: %s %s", r.Method, r.URL.String())
			}
			var input models.OrderStatusUpdateInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Status != models.OrderStatusAccepted || input.Version != 1 {
				t.Fatalf("status: %+v, %v", input, err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"order":{"id":9},"items":[]}`))
		default:
			t.Fatalf("unexpected request %d", requests)
		}
	}))
	defer func() { server.Close() }()

	client := &kitchenClient{baseURL: server.URL, apiKey: "secret", httpClient: server.Client()}
	if err := client.syncCatalog(context.Background(), demoCatalog()); err != nil {
		t.Fatal(err)
	}
	orders, err := client.listOrders(context.Background())
	if err != nil || len(orders) != 1 || orders[0].Order.ID != 9 {
		t.Fatalf("orders: %+v, %v", orders, err)
	}
	if err := client.updateOrderStatus(context.Background(), 9, &models.OrderStatusUpdateInput{Status: models.OrderStatusAccepted, Version: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestKitchenClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "conflict", http.StatusConflict)
	}))
	defer func() { server.Close() }()
	client := &kitchenClient{baseURL: server.URL, apiKey: "secret", httpClient: server.Client()}
	_, err := client.listOrders(context.Background())
	apiErr, ok := err.(*apiError)
	if !ok || apiErr.Status != http.StatusConflict || apiErr.Body != "conflict" || apiErr.Error() == "" {
		t.Fatalf("error: %v", err)
	}
}

func TestKitchenClientInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len("invalid")))
		_, _ = w.Write([]byte("invalid"))
	}))
	defer func() { server.Close() }()
	client := &kitchenClient{baseURL: server.URL, apiKey: "secret", httpClient: server.Client()}
	if _, err := client.listOrders(context.Background()); err == nil {
		t.Fatal("expected decode error")
	}
}
