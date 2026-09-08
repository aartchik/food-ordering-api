package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestOpenAPIContainsAllRoutes(t *testing.T) {
	data, err := os.ReadFile("../../openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("invalid OpenAPI JSON: %v", err)
	}
	if !strings.HasPrefix(document.OpenAPI, "3.1.") {
		t.Fatalf("OpenAPI version: %q", document.OpenAPI)
	}

	want := []string{
		"GET /healthz",
		"GET /v1/carts/{id}",
		"GET /v1/orders/{id}",
		"GET /v1/partner/orders",
		"GET /v1/partner/orders/{id}",
		"GET /v1/restaurants",
		"GET /v1/restaurants/{id}",
		"GET /v1/restaurants/{id}/menu",
		"PATCH /v1/carts/{id}/items/{item_id}",
		"PATCH /v1/partner/orders/{id}/status",
		"POST /v1/carts",
		"POST /v1/carts/{id}/items",
		"POST /v1/orders",
		"PUT /v1/partner/catalog",
		"DELETE /v1/carts/{id}/items/{item_id}",
	}
	got := make([]string, 0, len(want))
	for path, operations := range document.Paths {
		for method := range operations {
			got = append(got, strings.ToUpper(method)+" "+path)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("documented routes:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestPartnerOpenAPIRoutesRequireAuthentication(t *testing.T) {
	data, err := os.ReadFile("../../openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]struct {
			Security []map[string][]string `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	for path, operations := range document.Paths {
		if !strings.HasPrefix(path, "/v1/partner/") {
			continue
		}
		for method, operation := range operations {
			if len(operation.Security) != 1 {
				t.Errorf("%s %s has no security requirement", method, path)
				continue
			}
			if _, ok := operation.Security[0]["partnerBearer"]; !ok {
				t.Errorf("%s %s does not use partnerBearer", method, path)
			}
		}
	}
}
