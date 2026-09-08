package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-ordering-api/internal/models"
	"github.com/julienschmidt/httprouter"
)

type orderCreateFunc func(*models.CheckoutInput) (*models.OrderView, error)

func (f orderCreateFunc) CreateFromCart(input *models.CheckoutInput) (*models.OrderView, error) {
	return f(input)
}

type orderGetFunc func(int64) (*models.OrderView, error)

func (f orderGetFunc) Get(id int64) (*models.OrderView, error) { return f(id) }

const validCheckoutBody = `{"cart_id":7,"customer":{"name":"Anna","phone":"+79990000000","address":"Main street","comment":"Call me"}}`

func testOrderView() *models.OrderView {
	return &models.OrderView{
		Order: &models.Order{ID: 9, CartID: 7, Status: models.OrderStatusPendingPartner, TotalKopecks: 30000},
		Items: []*models.OrderItem{{ID: 1, OrderID: 9, Name: "Bread", Quantity: 2, PriceKopecks: 15000}},
	}
}

func orderRequest(t *testing.T, method, path, target, body string, handler http.HandlerFunc, status int) *httptest.ResponseRecorder {
	return orderRequestWithKey(t, method, path, target, body, "checkout-1", handler, status)
}

func orderRequestWithKey(t *testing.T, method, path, target, body, idempotencyKey string, handler http.HandlerFunc, status int) *httptest.ResponseRecorder {
	t.Helper()
	router := httprouter.New()
	router.HandlerFunc(method, path, handler)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if idempotencyKey != "" {
		r.Header.Set("Idempotency-Key", idempotencyKey)
	}
	router.ServeHTTP(w, r)
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "private database error") {
		t.Fatal("database error leaked")
	}
	return w
}

func TestCreateOrder(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
	}{
		{"success", nil, 201}, {"missing cart", fmt.Errorf("wrapped: %w", models.ErrRecordNotFound), 404},
		{"empty cart", models.ErrCartIsEmpty, 409}, {"unavailable item", models.ErrItemUnavailable, 409},
		{"mixed cart", models.ErrMixedCart, 409}, {"invalid", models.ErrInvalidInput, 422},
		{"idempotency conflict", models.ErrIdempotencyConflict, 409},
		{"changed cart", &models.CartChangedError{Items: []models.CartItemChange{{MenuItemID: 3, ChangedFields: []string{"price"}, CartPriceKopecks: 100, CurrentPriceKopecks: 200, IsAvailable: true}}}, 409},
		{"failure", errors.New("private database error"), 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			store := orderCreateFunc(func(input *models.CheckoutInput) (*models.OrderView, error) {
				called = true
				if input.CartID != 7 || input.IdempotencyKey != "checkout-1" || input.Customer.Name != "Anna" || input.Customer.Phone != "+79990000000" || input.Customer.Address != "Main street" || input.Customer.Comment != "Call me" {
					t.Fatalf("input: %+v", input)
				}
				return testOrderView(), tt.err
			})
			w := orderRequest(t, "POST", "/v1/orders", "/v1/orders", validCheckoutBody, catalogTestApp().createOrder(store), tt.status)
			if !called {
				t.Fatal("store not called")
			}
			if tt.status == 201 {
				if w.Header().Get("Location") != "/v1/orders/9" {
					t.Fatal("missing location")
				}
				assertOrderResponse(t, w)
			}
		})
	}
}

func TestCheckoutChangedCartResponse(t *testing.T) {
	store := orderCreateFunc(func(*models.CheckoutInput) (*models.OrderView, error) {
		return nil, &models.CartChangedError{Items: []models.CartItemChange{{
			MenuItemID:          3,
			ChangedFields:       []string{"name", "price"},
			CartName:            "Bread",
			CurrentName:         "Fresh bread",
			CartPriceKopecks:    100,
			CurrentPriceKopecks: 120,
			IsAvailable:         true,
		}}}
	})

	w := orderRequest(t, http.MethodPost, "/v1/orders", "/v1/orders", validCheckoutBody, catalogTestApp().createOrder(store), http.StatusConflict)
	var body struct {
		Error struct {
			Message string                  `json:"message"`
			Items   []models.CartItemChange `json:"items"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Message != models.ErrCartChanged.Error() || len(body.Error.Items) != 1 || body.Error.Items[0].CurrentPriceKopecks != 120 {
		t.Fatalf("response: %+v", body)
	}
}

func assertOrderResponse(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var got models.OrderView
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Order == nil || got.Order.ID != 9 || got.Order.Status != models.OrderStatusPendingPartner || got.Order.TotalKopecks != 30000 || len(got.Items) != 1 || got.Items[0].Quantity != 2 {
		t.Fatalf("order response: %+v", got)
	}
}

func TestCheckoutInvalidBody(t *testing.T) {
	for _, tt := range []struct {
		body   string
		status int
	}{
		{"", 400}, {"{", 400}, {validCheckoutBody + " {}", 400},
		{`{"unknown":1}`, 400}, {`{"cart_id":"seven"}`, 400},
		{`null`, 422}, {`{}`, 422},
		{strings.Replace(validCheckoutBody, `"cart_id":7`, `"cart_id":0`, 1), 422},
		{strings.Replace(validCheckoutBody, `"name":"Anna"`, `"name":" "`, 1), 422},
		{strings.Replace(validCheckoutBody, `"phone":"+79990000000"`, `"phone":""`, 1), 422},
		{strings.Replace(validCheckoutBody, `"address":"Main street"`, `"address":""`, 1), 422},
	} {
		t.Run(tt.body, func(t *testing.T) {
			store := orderCreateFunc(func(*models.CheckoutInput) (*models.OrderView, error) {
				t.Fatal("invalid input reached store")
				return nil, nil
			})
			orderRequest(t, "POST", "/v1/orders", "/v1/orders", tt.body, catalogTestApp().createOrder(store), tt.status)
		})
	}
}

func TestCheckoutIdempotencyKeyHeader(t *testing.T) {
	store := orderCreateFunc(func(*models.CheckoutInput) (*models.OrderView, error) {
		t.Fatal("invalid idempotency key reached store")
		return nil, nil
	})
	handler := catalogTestApp().createOrder(store)

	orderRequestWithKey(t, http.MethodPost, "/v1/orders", "/v1/orders", validCheckoutBody, "", handler, http.StatusUnprocessableEntity)
	orderRequestWithKey(t, http.MethodPost, "/v1/orders", "/v1/orders", validCheckoutBody, strings.Repeat("a", 121), handler, http.StatusUnprocessableEntity)
	orderRequestWithKey(t, http.MethodPost, "/v1/orders", "/v1/orders", validCheckoutBody, "   ", handler, http.StatusUnprocessableEntity)
}

func TestShowOrder(t *testing.T) {
	for _, tt := range []struct {
		id     string
		err    error
		status int
	}{
		{"9", nil, 200}, {"9", models.ErrRecordNotFound, 404}, {"9", errors.New("private database error"), 500},
		{"0", nil, 404}, {"-1", nil, 404}, {"abc", nil, 404}, {"999999999999999999999", nil, 404},
	} {
		t.Run(tt.id+http.StatusText(tt.status), func(t *testing.T) {
			store := orderGetFunc(func(id int64) (*models.OrderView, error) {
				if tt.id != "9" || id != 9 {
					t.Fatal("invalid id reached store")
				}
				return testOrderView(), tt.err
			})
			w := orderRequest(t, "GET", "/v1/orders/:id", "/v1/orders/"+tt.id, "", catalogTestApp().showOrder(store), tt.status)
			if tt.status == 200 {
				assertOrderResponse(t, w)
			}
		})
	}
}

func TestOrderRoutes(t *testing.T) {
	for _, tt := range []struct {
		method, target, body string
		status               int
	}{
		{"POST", "/v1/orders", `{}`, 422}, {"GET", "/v1/orders/invalid", "", 404}, {"DELETE", "/v1/orders/9", "", 405},
	} {
		w := httptest.NewRecorder()
		catalogTestApp().routes().ServeHTTP(w, httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body)))
		if w.Code != tt.status {
			t.Fatalf("%s %s: %d", tt.method, tt.target, w.Code)
		}
	}
}

func TestCheckoutReplayResponse(t *testing.T) {
	var inputs []models.CheckoutInput
	store := orderCreateFunc(func(input *models.CheckoutInput) (*models.OrderView, error) {
		inputs = append(inputs, *input)
		return testOrderView(), nil
	})
	handler := catalogTestApp().createOrder(store)
	first := orderRequest(t, "POST", "/v1/orders", "/v1/orders", validCheckoutBody, handler, 201)
	second := orderRequest(t, "POST", "/v1/orders", "/v1/orders", validCheckoutBody, handler, 201)
	if len(inputs) != 2 || inputs[0] != inputs[1] {
		t.Fatalf("replay input changed: %+v", inputs)
	}
	if first.Body.String() != second.Body.String() || second.Header().Get("Location") != first.Header().Get("Location") {
		t.Fatal("replay response changed")
	}
}
