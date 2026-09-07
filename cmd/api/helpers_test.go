package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadJSON(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		valid bool
	}{
		{"valid", `{"name":"Anna"}`, true},
		{"unknown field", `{"extra":true}`, false},
		{"wrong type", `{"name":42}`, false},
		{"malformed", `{"name":`, false},
		{"empty", "", false},
		{"multiple values", `{"name":"Anna"} {}`, false},
		{"oversized", `{"name":"` + strings.Repeat("a", 1_048_576) + `"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{}
			var input struct {
				Name string `json:"name"`
			}
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			err := app.readJSON(httptest.NewRecorder(), r, &input)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v, error=%v", tt.valid, err)
			}
			if tt.valid && input.Name != "Anna" {
				t.Fatalf("decoded name: %q", input.Name)
			}
		})
	}
}

func TestJSONErrorResponses(t *testing.T) {
	app := &application{}
	for _, status := range []int{http.StatusBadRequest, http.StatusUnprocessableEntity} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		if status == http.StatusBadRequest {
			app.badRequestResponse(w, r, errors.New("invalid JSON"))
		} else {
			app.failedValidationResponse(w, r, map[string]string{"name": "must be provided"})
		}
		if w.Code != status || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected response: %d, %v", w.Code, w.Header())
		}
		var body struct {
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if status == http.StatusBadRequest {
			var message string
			if err := json.Unmarshal(body.Error, &message); err != nil || message != "invalid JSON" {
				t.Fatalf("bad request message: %q, %v", message, err)
			}
		} else {
			var fields map[string]string
			if err := json.Unmarshal(body.Error, &fields); err != nil || fields["name"] != "must be provided" {
				t.Fatalf("validation errors: %v, %v", fields, err)
			}
		}
	}
}

func TestClientError(t *testing.T) {
	w := httptest.NewRecorder()
	(&application{}).clientError(w, http.StatusBadRequest)
	if w.Code != http.StatusBadRequest || w.Body.String() != "Bad Request\n" {
		t.Fatalf("unexpected response: %d, %q", w.Code, w.Body.String())
	}
}
