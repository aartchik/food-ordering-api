//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"food-ordering-api/internal/cache"
	"food-ordering-api/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func TestOrderWorkflow(t *testing.T) {
	db := workflowDatabase(t)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		if err := redisClient.Close(); err != nil {
			t.Errorf("close redis client: %v", err)
		}
	})

	app := &application{
		config:    config{env: "test"},
		db:        db,
		errorLog:  log.New(io.Discard, "", 0),
		infoLog:   log.New(io.Discard, "", 0),
		models:    models.NewModels(db),
		partners:  configuredPartners{"bakery": apiKeyHash("partner-secret")},
		menuCache: cache.NewMenuCache(redisClient, 5*time.Minute, time.Second),
	}
	app.config.redis.enabled = true
	handler := app.routes()
	partnerHeaders := map[string]string{"Authorization": "Bearer partner-secret"}

	catalog := models.RestaurantCatalogInput{
		Restaurant: models.RestaurantInput{Name: "Bakery", Address: "Main street", IsOpen: true},
		Items: []*models.MenuItemInput{
			{PartnerItemID: "bread", Name: "Bread", PriceKopecks: 15000, IsAvailable: true, PreparationMin: 5},
		},
	}
	catalogResponse := workflowRequest(t, handler, http.MethodPut, "/v1/partner/catalog", catalog, partnerHeaders, http.StatusOK)
	var synchronized struct {
		Restaurant *models.Restaurant `json:"restaurant"`
		Items      []*models.MenuItem `json:"items"`
	}
	decodeWorkflowResponse(t, catalogResponse, &synchronized)
	if synchronized.Restaurant == nil || len(synchronized.Items) != 1 {
		t.Fatalf("catalog response: %+v", synchronized)
	}

	menuPath := fmt.Sprintf("/v1/restaurants/%d/menu?available_only=true", synchronized.Restaurant.ID)
	menuResponse := workflowRequest(t, handler, http.MethodGet, menuPath, nil, nil, http.StatusOK)
	var menu struct {
		Items []*models.MenuItem `json:"items"`
	}
	decodeWorkflowResponse(t, menuResponse, &menu)
	if len(menu.Items) != 1 || menu.Items[0].ID != synchronized.Items[0].ID {
		t.Fatalf("menu response: %+v", menu)
	}
	if len(redisServer.Keys()) != 1 {
		t.Fatalf("menu was not cached: %v", redisServer.Keys())
	}

	cartResponse := workflowRequest(t, handler, http.MethodPost, "/v1/carts", nil, nil, http.StatusCreated)
	var cart models.CartView
	decodeWorkflowResponse(t, cartResponse, &cart)
	if cart.Cart == nil {
		t.Fatal("created cart is missing")
	}
	cartPath := fmt.Sprintf("/v1/carts/%d/items", cart.Cart.ID)
	cartResponse = workflowRequest(t, handler, http.MethodPost, cartPath, models.CartItemInput{MenuItemID: menu.Items[0].ID, Quantity: 2}, nil, http.StatusOK)
	decodeWorkflowResponse(t, cartResponse, &cart)
	if len(cart.Items) != 1 || cart.TotalKopecks != 30000 {
		t.Fatalf("cart response: %+v", cart)
	}

	checkout := models.CheckoutInput{
		CartID: cart.Cart.ID,
		Customer: models.Customer{
			Name:    "Anna",
			Phone:   "+79990000000",
			Address: "Delivery street",
		},
	}
	checkoutHeaders := map[string]string{"Idempotency-Key": "workflow-order-1"}
	orderResponse := workflowRequest(t, handler, http.MethodPost, "/v1/orders", checkout, checkoutHeaders, http.StatusCreated)
	var order models.OrderView
	decodeWorkflowResponse(t, orderResponse, &order)
	if order.Order == nil || order.Order.Status != models.OrderStatusPendingPartner || order.Order.TotalKopecks != 30000 {
		t.Fatalf("order response: %+v", order)
	}

	replayResponse := workflowRequest(t, handler, http.MethodPost, "/v1/orders", checkout, checkoutHeaders, http.StatusCreated)
	var replay models.OrderView
	decodeWorkflowResponse(t, replayResponse, &replay)
	if replay.Order == nil || replay.Order.ID != order.Order.ID {
		t.Fatalf("idempotent replay: %+v", replay)
	}

	listPath := "/v1/partner/orders?status=pending_partner"
	listResponse := workflowRequest(t, handler, http.MethodGet, listPath, nil, partnerHeaders, http.StatusOK)
	var listed struct {
		Orders []*models.OrderView `json:"orders"`
	}
	decodeWorkflowResponse(t, listResponse, &listed)
	if len(listed.Orders) != 1 || listed.Orders[0].Order.ID != order.Order.ID {
		t.Fatalf("partner orders: %+v", listed.Orders)
	}

	statuses := []string{
		models.OrderStatusAccepted,
		models.OrderStatusCooking,
		models.OrderStatusReady,
		models.OrderStatusDelivering,
		models.OrderStatusDelivered,
	}
	version := int32(1)
	for _, status := range statuses {
		input := models.OrderStatusUpdateInput{Status: status, Version: version}
		if status == models.OrderStatusAccepted {
			input.PartnerOrderID = "bakery-order-1"
		}
		path := fmt.Sprintf("/v1/partner/orders/%d/status", order.Order.ID)
		response := workflowRequest(t, handler, http.MethodPatch, path, input, partnerHeaders, http.StatusOK)
		decodeWorkflowResponse(t, response, &order)
		version++
		if order.Order.Status != status || order.Order.Version != version {
			t.Fatalf("status %s response: %+v", status, order.Order)
		}
	}

	orderPath := fmt.Sprintf("/v1/orders/%d", order.Order.ID)
	finalResponse := workflowRequest(t, handler, http.MethodGet, orderPath, nil, nil, http.StatusOK)
	var final models.OrderView
	decodeWorkflowResponse(t, finalResponse, &final)
	if final.Order.Status != models.OrderStatusDelivered || final.Order.PartnerOrderID != "bakery-order-1" || len(final.Items) != 1 {
		t.Fatalf("final order: %+v", final)
	}
}

func workflowRequest(t *testing.T, handler http.Handler, method, target string, input any, headers map[string]string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, target, body)
	request.RemoteAddr = "192.0.2.1:1000"
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s: got %d, want %d: %s", method, target, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func decodeWorkflowResponse(t *testing.T, response *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}

func workflowDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL must point to a PostgreSQL test database")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("workflow_%x", rand.Text())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema)); err != nil {
		cancel()
		if closeErr := admin.Close(); closeErr != nil {
			t.Errorf("close admin database: %v", closeErr)
		}
		t.Fatal(err)
	}
	cancel()

	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close workflow database: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+pq.QuoteIdentifier(schema)+" CASCADE"); err != nil {
			t.Errorf("drop workflow schema: %v", err)
		}
		if err := admin.Close(); err != nil {
			t.Errorf("close admin database: %v", err)
		}
	})

	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	for _, path := range paths {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = db.ExecContext(ctx, string(migration))
		cancel()
		if err != nil {
			t.Fatalf("migration %s: %v", path, err)
		}
	}
	return db
}
