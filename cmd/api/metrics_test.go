package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPMetricsUseRoutePattern(t *testing.T) {
	metrics := newApplicationMetrics(nil)
	handler := metrics.observeHTTP(http.MethodGet, "/v1/orders/:id", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/orders/42", nil))

	count := testutil.ToFloat64(metrics.httpRequests.WithLabelValues(http.MethodGet, "/v1/orders/:id", "200"))
	if count != 1 {
		t.Fatalf("request count: %v", count)
	}
	if inFlight := testutil.ToFloat64(metrics.httpInFlight.WithLabelValues(http.MethodGet, "/v1/orders/:id")); inFlight != 0 {
		t.Fatalf("in-flight requests: %v", inFlight)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	metrics := newApplicationMetrics(nil)
	metrics.recordMenuCache("read", "hit")
	response := httptest.NewRecorder()

	metrics.handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status: %d", response.Code)
	}
	if got := testutil.ToFloat64(metrics.menuCacheAccess.WithLabelValues("read", "hit")); got != 1 {
		t.Fatalf("cache hit count: %v", got)
	}
}
