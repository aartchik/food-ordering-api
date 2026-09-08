package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfiguredPartners(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		key        string
		partnerID  string
		configured bool
		wantError  bool
	}{
		{name: "empty"},
		{name: "single", value: "bakery=secret", key: "secret", partnerID: "bakery", configured: true},
		{name: "multiple", value: "bakery=secret, coffee = another ", key: "another", partnerID: "coffee", configured: true},
		{name: "wrong key", value: "bakery=secret", key: "wrong"},
		{name: "missing separator", value: "secret", wantError: true},
		{name: "missing id", value: "=secret", wantError: true},
		{name: "missing key", value: "bakery=", wantError: true},
		{name: "duplicate id", value: "bakery=first,bakery=second", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partners, err := newConfiguredPartners(tt.value)
			if (err != nil) != tt.wantError {
				t.Fatalf("error: %v", err)
			}
			if tt.wantError {
				return
			}
			partnerID, ok := partners.PartnerID(tt.key)
			if ok != tt.configured || partnerID != tt.partnerID {
				t.Fatalf("got (%q, %v), want (%q, %v)", partnerID, ok, tt.partnerID, tt.configured)
			}
		})
	}
}

func TestRequirePartner(t *testing.T) {
	app := catalogTestApp()
	app.partners = configuredPartners{"bakery": apiKeyHash("secret")}

	tests := []struct {
		name          string
		authorization string
		status        int
	}{
		{name: "valid", authorization: "Bearer secret", status: http.StatusNoContent},
		{name: "case insensitive scheme", authorization: "bearer secret", status: http.StatusNoContent},
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong scheme", authorization: "Basic secret", status: http.StatusUnauthorized},
		{name: "missing key", authorization: "Bearer", status: http.StatusUnauthorized},
		{name: "extra value", authorization: "Bearer secret extra", status: http.StatusUnauthorized},
		{name: "wrong key", authorization: "Bearer wrong", status: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if got := partnerIDFromContext(r); got != "bakery" {
					t.Fatalf("partner id: %q", got)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/v1/partner/orders", nil)
			request.Header.Set("Authorization", tt.authorization)
			response := httptest.NewRecorder()
			app.requirePartner(next).ServeHTTP(response, request)

			if response.Code != tt.status {
				t.Fatalf("status: %d, want %d", response.Code, tt.status)
			}
			if called != (tt.status == http.StatusNoContent) {
				t.Fatalf("next called: %v", called)
			}
			if response.Header().Get("Vary") != "Authorization" {
				t.Fatalf("vary header: %q", response.Header().Get("Vary"))
			}
			if tt.status == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != `Bearer realm="partner"` {
				t.Fatalf("authenticate header: %q", response.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestRequirePartnerWithoutConfiguration(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/partner/orders", nil)
	request.Header.Set("Authorization", "Bearer secret")
	catalogTestApp().requirePartner(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler called")
	})).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", response.Code)
	}
}
