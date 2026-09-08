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

type cartInsertFunc func(string) (*models.CartView, error)

func (f cartInsertFunc) Insert(user string) (*models.CartView, error) { return f(user) }

type cartGetFunc func(int64) (*models.CartView, error)

func (f cartGetFunc) Get(id int64) (*models.CartView, error) { return f(id) }

type cartAddFunc func(int64, int64, int) (*models.CartView, error)

func (f cartAddFunc) AddItem(id, item int64, quantity int) (*models.CartView, error) {
	return f(id, item, quantity)
}

type cartUpdateFunc func(int64, int64, int) (*models.CartView, error)

func (f cartUpdateFunc) UpdateItem(id, item int64, quantity int) (*models.CartView, error) {
	return f(id, item, quantity)
}

type cartDeleteFunc func(int64, int64) (*models.CartView, error)

func (f cartDeleteFunc) DeleteItem(id, item int64) (*models.CartView, error) { return f(id, item) }

func testCartView() *models.CartView {
	return &models.CartView{Cart: &models.Cart{ID: 7}, Items: []*models.CartItem{}, TotalKopecks: 0}
}

func cartRequest(t *testing.T, method, path, target, body string, handler http.HandlerFunc, status int) *httptest.ResponseRecorder {
	t.Helper()
	router := httprouter.New()
	router.HandlerFunc(method, path, handler)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(method, target, strings.NewReader(body)))
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "private database error") {
		t.Fatal("database error leaked")
	}
	if status < 300 {
		var got models.CartView
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Cart == nil || got.Cart.ID != 7 || got.Items == nil {
			t.Fatalf("cart response: %+v", got)
		}
	}
	return w
}

func TestCreateCart(t *testing.T) {
	for _, failed := range []bool{false, true} {
		called := false
		store := cartInsertFunc(func(user string) (*models.CartView, error) {
			called = true
			if user != "" {
				t.Fatal("expected anonymous cart")
			}
			if failed {
				return nil, errors.New("private database error")
			}
			return testCartView(), nil
		})
		status := http.StatusCreated
		if failed {
			status = http.StatusInternalServerError
		}
		w := cartRequest(t, "POST", "/v1/carts", "/v1/carts", "", catalogTestApp().createCart(store), status)
		if !called {
			t.Fatal("store not called")
		}
		if !failed && w.Header().Get("Location") != "/v1/carts/7" {
			t.Fatal("missing cart location")
		}
	}
}

func TestShowCart(t *testing.T) {
	for _, tt := range []struct {
		id     string
		err    error
		status int
	}{
		{"7", nil, 200}, {"7", models.ErrRecordNotFound, 404}, {"7", errors.New("private database error"), 500},
		{"invalid", nil, 404}, {"0", nil, 404}, {"-1", nil, 404},
	} {
		t.Run(tt.id+http.StatusText(tt.status), func(t *testing.T) {
			store := cartGetFunc(func(id int64) (*models.CartView, error) {
				if tt.id != "7" || id != 7 {
					t.Fatal("invalid id reached store")
				}
				return testCartView(), tt.err
			})
			cartRequest(t, "GET", "/v1/carts/:id", "/v1/carts/"+tt.id, "", catalogTestApp().showCart(store), tt.status)
		})
	}
}

func TestCartItemMutations(t *testing.T) {
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		for _, tt := range []struct {
			name   string
			err    error
			status int
		}{
			{"success", nil, 200}, {"missing", fmt.Errorf("wrapped: %w", models.ErrRecordNotFound), 404},
			{"unavailable", models.ErrItemUnavailable, 409}, {"mixed", models.ErrMixedCart, 409},
			{"invalid", models.ErrInvalidInput, 422}, {"failure", errors.New("private database error"), 500},
		} {
			t.Run(method+tt.name, func(t *testing.T) {
				called := false
				mutate := func(id, item int64, quantity int) (*models.CartView, error) {
					called = true
					if id != 7 || item != 3 || (method != "DELETE" && quantity != 2) {
						t.Fatalf("arguments: %d %d %d", id, item, quantity)
					}
					return testCartView(), tt.err
				}
				app := catalogTestApp()
				path, target, body := "/v1/carts/:id/items/:item_id", "/v1/carts/7/items/3", `{"quantity":2}`
				var handler http.HandlerFunc
				switch method {
				case "POST":
					path, target, body = "/v1/carts/:id/items", "/v1/carts/7/items", `{"menu_item_id":3,"quantity":2}`
					handler = app.addCartItem(cartAddFunc(mutate))
				case "PATCH":
					handler = app.updateCartItem(cartUpdateFunc(mutate))
				case "DELETE":
					handler = app.deleteCartItem(cartDeleteFunc(func(id, item int64) (*models.CartView, error) { return mutate(id, item, 0) }))
				}
				cartRequest(t, method, path, target, body, handler, tt.status)
				if !called {
					t.Fatal("store not called")
				}
			})
		}
	}
}

func TestCartInputRejectedBeforeDatabase(t *testing.T) {
	app := catalogTestApp()
	for _, tt := range []struct {
		method, target, body string
		status               int
	}{
		{"POST", "/v1/carts/0/items", `{}`, 404},
		{"POST", "/v1/carts/7/items", `{`, 400},
		{"POST", "/v1/carts/7/items", `{"menu_item_id":3,"quantity":2,"extra":1}`, 400},
		{"POST", "/v1/carts/7/items", `{"menu_item_id":3,"quantity":2} {}`, 400},
		{"POST", "/v1/carts/7/items", `{"menu_item_id":3,"quantity":"two"}`, 400},
		{"POST", "/v1/carts/7/items", `null`, 422},
		{"POST", "/v1/carts/7/items", `{"quantity":2}`, 422},
		{"POST", "/v1/carts/7/items", `{"menu_item_id":3,"quantity":100}`, 422},
		{"PATCH", "/v1/carts/7/items/3", `{"quantity":0}`, 422},
		{"PATCH", "/v1/carts/7/items/3", `{"quantity":100}`, 422},
		{"PATCH", "/v1/carts/7/items/3", `null`, 422},
		{"PATCH", "/v1/carts/7/items/3", `{"quantity":2,"menu_item_id":4}`, 400},
		{"PATCH", "/v1/carts/7/items/invalid", `{"quantity":2}`, 404},
		{"DELETE", "/v1/carts/7/items/0", ``, 404},
		{"DELETE", "/v1/carts/invalid/items/3", ``, 404},
	} {
		t.Run(tt.method+tt.target+tt.body, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.routes().ServeHTTP(w, httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body)))
			if w.Code != tt.status || !json.Valid(w.Body.Bytes()) {
				t.Fatalf("response: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
