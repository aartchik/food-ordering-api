package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func middlewareTestApp(infoOutput, errorOutput io.Writer) *application {
	return &application{
		infoLog:  log.New(infoOutput, "", 0),
		errorLog: log.New(errorOutput, "", 0),
	}
}

func TestSecureHeaders(t *testing.T) {
	handler := secureHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"Content-Security-Policy": "default-src 'self'",
		"Referrer-Policy":         "origin-when-cross-origin",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "deny",
		"X-XSS-Protection":        "0",
	}
	for header, value := range want {
		if got := response.Header().Get(header); got != value {
			t.Errorf("%s: got %q, want %q", header, got, value)
		}
	}
}

func TestRequestID(t *testing.T) {
	app := middlewareTestApp(io.Discard, io.Discard)
	for _, tt := range []struct {
		name     string
		provided string
	}{
		{name: "generated"},
		{name: "propagated", provided: "client-request-id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var contextID string
			handler := app.requestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				contextID = requestIDFromContext(r)
			}))
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("X-Request-ID", tt.provided)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			responseID := response.Header().Get("X-Request-ID")
			if responseID == "" || contextID != responseID {
				t.Fatalf("response id %q, context id %q", responseID, contextID)
			}
			if tt.provided != "" && responseID != tt.provided {
				t.Fatalf("got %q, want %q", responseID, tt.provided)
			}
		})
	}
}

func TestLogRequestIncludesStatusAndRequestID(t *testing.T) {
	var output bytes.Buffer
	app := middlewareTestApp(&output, io.Discard)
	handler := app.logRequest(app.requestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})))
	request := httptest.NewRequest(http.MethodPost, "/orders?source=test", nil)
	request.Header.Set("X-Request-ID", "request-42")

	handler.ServeHTTP(httptest.NewRecorder(), request)

	line := output.String()
	for _, part := range []string{"POST", "/orders?source=test", "201", "request_id=request-42"} {
		if !strings.Contains(line, part) {
			t.Fatalf("log %q does not contain %q", line, part)
		}
	}
}

func TestRecoverPanic(t *testing.T) {
	var errorOutput bytes.Buffer
	app := middlewareTestApp(io.Discard, &errorOutput)
	handler := app.recoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("unexpected failure")
	}))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError || response.Header().Get("Connection") != "close" {
		t.Fatalf("response: %d, headers: %v", response.Code, response.Header())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body.Error, "unexpected failure") || !strings.Contains(errorOutput.String(), "unexpected failure") {
		t.Fatalf("body: %q, log: %q", body.Error, errorOutput.String())
	}
}

func TestRateLimit(t *testing.T) {
	app := middlewareTestApp(io.Discard, io.Discard)
	app.config.limiter.enabled = true
	app.config.limiter.rps = 0.0001
	app.config.limiter.burst = 1
	calls := 0
	handler := app.rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	request := func(remoteAddr string) int {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	if status := request("192.0.2.1:1000"); status != http.StatusNoContent {
		t.Fatalf("first status: %d", status)
	}
	if status := request("192.0.2.1:2000"); status != http.StatusTooManyRequests {
		t.Fatalf("limited status: %d", status)
	}
	if status := request("192.0.2.2:1000"); status != http.StatusNoContent {
		t.Fatalf("independent client status: %d", status)
	}
	if calls != 2 {
		t.Fatalf("handler calls: %d", calls)
	}
}

func TestDisabledRateLimit(t *testing.T) {
	app := middlewareTestApp(io.Discard, io.Discard)
	app.config.limiter.enabled = false
	calls := 0
	handler := app.rateLimit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))

	for range 2 {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}
	if calls != 2 {
		t.Fatalf("handler calls: %d", calls)
	}
}

func TestRateLimitResponseUsesStandardMiddleware(t *testing.T) {
	var output bytes.Buffer
	app := middlewareTestApp(&output, io.Discard)
	app.config.limiter.enabled = true
	app.config.limiter.rps = 0.0001
	app.config.limiter.burst = 1
	handler := app.routes()

	for attempt := range 2 {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		request.RemoteAddr = "192.0.2.1:1000"
		request.Header.Set("X-Request-ID", "limited-request")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if attempt == 1 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("status: %d", response.Code)
			}
			if response.Header().Get("X-Request-ID") != "limited-request" {
				t.Fatalf("request id: %q", response.Header().Get("X-Request-ID"))
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("security headers: %v", response.Header())
			}
		}
	}
	if !strings.Contains(output.String(), "429") || !strings.Contains(output.String(), "request_id=limited-request") {
		t.Fatalf("access log: %q", output.String())
	}
}
