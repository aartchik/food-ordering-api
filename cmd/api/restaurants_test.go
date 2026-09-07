package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-ordering-api/internal/models"
	"github.com/julienschmidt/httprouter"
)

type restaurantListFunc func(models.RestaurantListFilter) ([]*models.Restaurant, models.Metadata, error)

func (f restaurantListFunc) GetAll(input models.RestaurantListFilter) ([]*models.Restaurant, models.Metadata, error) {
	return f(input)
}

type restaurantGetFunc func(int64) (*models.Restaurant, error)

func (f restaurantGetFunc) Get(id int64) (*models.Restaurant, error) { return f(id) }

func catalogTestApp() *application {
	return &application{errorLog: log.New(io.Discard, "", 0), infoLog: log.New(io.Discard, "", 0)}
}

func catalogRequest(t *testing.T, path, target string, handler http.HandlerFunc, status int) *httptest.ResponseRecorder {
	t.Helper()
	router := httprouter.New()
	router.HandlerFunc(http.MethodGet, path, handler)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	if w.Code != status || w.Header().Get("Content-Type") != "application/json" || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("response: status=%d, headers=%v, body=%s", w.Code, w.Header(), w.Body.String())
	}
	return w
}

func TestListRestaurants(t *testing.T) {
	for _, query := range []string{"", "?q=Bakery&only_open=true&page=2&page_size=5&sort=-name"} {
		t.Run(query, func(t *testing.T) {
			called := false
			store := restaurantListFunc(func(input models.RestaurantListFilter) ([]*models.Restaurant, models.Metadata, error) {
				called = true
				if query == "" {
					if input.Page != 1 || input.PageSize != 20 || input.Sort != "id" || input.OnlyOpen {
						t.Fatalf("defaults: %+v", input)
					}
				} else if input.Query != "Bakery" || !input.OnlyOpen || input.Page != 2 || input.PageSize != 5 || input.SortColumn() != "name" || input.SortDirection() != "DESC" {
					t.Fatalf("filters: %+v", input)
				}
				return []*models.Restaurant{{ID: 7, Name: "Bakery"}}, models.Metadata{TotalRecords: 6}, nil
			})
			w := catalogRequest(t, "/v1/restaurants", "/v1/restaurants"+query, catalogTestApp().listRestaurants(store), 200)
			var body struct {
				Restaurants []*models.Restaurant `json:"restaurants"`
				Metadata    models.Metadata      `json:"metadata"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if !called || len(body.Restaurants) != 1 || body.Restaurants[0].ID != 7 || body.Metadata.TotalRecords != 6 {
				t.Fatalf("body: %+v", body)
			}
		})
	}
}

func TestListRestaurantsInvalidFilters(t *testing.T) {
	for _, query := range []string{"page=0", "page=abc", "page=999999999999999999999", "page_size=101", "page_size=-1", "sort=invalid", "sort=", "only_open=yes", "q=" + strings.Repeat("a", 121)} {
		t.Run(query, func(t *testing.T) {
			store := restaurantListFunc(func(models.RestaurantListFilter) ([]*models.Restaurant, models.Metadata, error) {
				t.Fatal("invalid input reached database")
				return nil, models.Metadata{}, nil
			})
			catalogRequest(t, "/v1/restaurants", "/v1/restaurants?"+query, catalogTestApp().listRestaurants(store), 422)
		})
	}
}

func TestListRestaurantsEmptyAndError(t *testing.T) {
	for _, failed := range []bool{false, true} {
		store := restaurantListFunc(func(models.RestaurantListFilter) ([]*models.Restaurant, models.Metadata, error) {
			if failed {
				return nil, models.Metadata{}, errors.New("private database error")
			}
			return nil, models.Metadata{}, nil
		})
		status := 200
		if failed {
			status = 500
		}
		w := catalogRequest(t, "/v1/restaurants", "/v1/restaurants", catalogTestApp().listRestaurants(store), status)
		if strings.Contains(w.Body.String(), "private database error") {
			t.Fatal("database error leaked")
		}
		if !failed && !strings.Contains(w.Body.String(), `"restaurants": []`) {
			t.Fatal(w.Body.String())
		}
	}
}

func TestShowRestaurant(t *testing.T) {
	for _, tt := range []struct {
		id     string
		err    error
		status int
	}{
		{"7", nil, 200}, {"7", models.ErrRecordNotFound, 404}, {"7", errors.New("database error"), 500},
		{"abc", nil, 404}, {"0", nil, 404}, {"-1", nil, 404}, {"999999999999999999999", nil, 404},
	} {
		t.Run(tt.id+http.StatusText(tt.status), func(t *testing.T) {
			store := restaurantGetFunc(func(id int64) (*models.Restaurant, error) {
				if tt.id != "7" || id != 7 {
					t.Fatal("invalid id reached database")
				}
				return &models.Restaurant{ID: id, Name: "Bakery"}, tt.err
			})
			w := catalogRequest(t, "/v1/restaurants/:id", "/v1/restaurants/"+tt.id, catalogTestApp().showRestaurant(store), tt.status)
			if tt.status == 200 {
				var body struct {
					Restaurant models.Restaurant `json:"restaurant"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Restaurant.ID != 7 {
					t.Fatal(body)
				}
			}
		})
	}
}

func TestCatalogRoutesRegistered(t *testing.T) {
	handler := catalogTestApp().routes()
	for _, tt := range []struct {
		target string
		status int
	}{
		{"/v1/restaurants?page=invalid", 422},
		{"/v1/restaurants/invalid", 404},
		{"/v1/restaurants/7/menu?available_only=invalid", 422},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.target, nil))
		if w.Code != tt.status {
			t.Fatalf("%s: %d", tt.target, w.Code)
		}
	}
}
