package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type dependencyPingStub struct {
	err    error
	called int
}

func (p *dependencyPingStub) PingContext(context.Context) error {
	p.called++
	return p.err
}

func TestHealthcheckDoesNotProbeDependencies(t *testing.T) {
	app := middlewareTestApp(io.Discard, io.Discard)
	app.config.env = "test"
	response := httptest.NewRecorder()

	app.healthcheck(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status: %d", response.Code)
	}
	var body struct {
		Status string `json:"status"`
		Env    string `json:"env"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "available" || body.Env != "test" {
		t.Fatalf("body: %+v", body)
	}
}

func TestReadiness(t *testing.T) {
	dependencyError := errors.New("dependency unavailable")
	tests := []struct {
		name             string
		redisEnabled     bool
		databaseErr      error
		redisErr         error
		wantStatus       int
		wantServiceState string
		wantPostgres     string
		wantRedis        string
		wantRedisCalls   int
	}{
		{name: "available", redisEnabled: true, wantStatus: 200, wantServiceState: "available", wantPostgres: "available", wantRedis: "available", wantRedisCalls: 1},
		{name: "redis degraded", redisEnabled: true, redisErr: dependencyError, wantStatus: 200, wantServiceState: "degraded", wantPostgres: "available", wantRedis: "unavailable", wantRedisCalls: 1},
		{name: "postgres unavailable", redisEnabled: true, databaseErr: dependencyError, wantStatus: 503, wantServiceState: "unavailable", wantPostgres: "unavailable", wantRedis: "available", wantRedisCalls: 1},
		{name: "all unavailable", redisEnabled: true, databaseErr: dependencyError, redisErr: dependencyError, wantStatus: 503, wantServiceState: "unavailable", wantPostgres: "unavailable", wantRedis: "unavailable", wantRedisCalls: 1},
		{name: "redis disabled", wantStatus: 200, wantServiceState: "available", wantPostgres: "available", wantRedis: "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := middlewareTestApp(io.Discard, io.Discard)
			app.config.redis.enabled = tt.redisEnabled
			database := &dependencyPingStub{err: tt.databaseErr}
			redis := &dependencyPingStub{err: tt.redisErr}
			response := httptest.NewRecorder()

			app.readiness(database, redis).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if response.Code != tt.wantStatus || database.called != 1 || redis.called != tt.wantRedisCalls {
				t.Fatalf("status: %d, database calls: %d, redis calls: %d", response.Code, database.called, redis.called)
			}
			var body struct {
				Status string            `json:"status"`
				Checks map[string]string `json:"checks"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Status != tt.wantServiceState || body.Checks["postgres"] != tt.wantPostgres || body.Checks["redis"] != tt.wantRedis {
				t.Fatalf("body: %+v", body)
			}
			if bodyText := response.Body.String(); strings.Contains(bodyText, dependencyError.Error()) {
				t.Fatalf("dependency error leaked: %s", bodyText)
			}
		})
	}
}
