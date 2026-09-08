package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthAndState(t *testing.T) {
	state := &serviceState{}
	handler := routes(state, log.New(io.Discard, "", 0))
	for _, tt := range []struct {
		path   string
		status int
	}{
		{"/healthz", http.StatusServiceUnavailable},
		{"/state", http.StatusOK},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if response.Code != tt.status || !json.Valid(response.Body.Bytes()) {
			t.Fatalf("%s: %d %s", tt.path, response.Code, response.Body.String())
		}
	}
	state.mu.Lock()
	state.ready = true
	state.lastCatalogSync = time.Now()
	state.mu.Unlock()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("ready health: %d", response.Code)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := config{apiURL: "http://api:8080", apiKey: "secret", pollInterval: time.Second}
	if err := validateConfig(valid); err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []config{
		{apiKey: "secret", pollInterval: time.Second},
		{apiURL: "://invalid", apiKey: "secret", pollInterval: time.Second},
		{apiURL: "http://api:8080", pollInterval: time.Second},
		{apiURL: "http://api:8080", apiKey: "secret"},
	} {
		if err := validateConfig(cfg); err == nil {
			t.Fatalf("expected error for %+v", cfg)
		}
	}
}
