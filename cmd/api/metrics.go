package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type applicationMetrics struct {
	registry        *prometheus.Registry
	httpRequests    *prometheus.CounterVec
	httpDuration    *prometheus.HistogramVec
	httpInFlight    *prometheus.GaugeVec
	menuCacheAccess *prometheus.CounterVec
}

func newApplicationMetrics(db *sql.DB) *applicationMetrics {
	registry := prometheus.NewRegistry()
	m := &applicationMetrics{
		registry: registry,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "food_ordering_api",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "food_ordering_api",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "route"}),
		httpInFlight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "food_ordering_api",
			Name:      "http_requests_in_flight",
			Help:      "Current number of HTTP requests.",
		}, []string{"method", "route"}),
		menuCacheAccess: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "food_ordering_api",
			Name:      "menu_cache_operations_total",
			Help:      "Total number of menu cache operations.",
		}, []string{"operation", "result"}),
	}

	registry.MustRegister(m.httpRequests, m.httpDuration, m.httpInFlight, m.menuCacheAccess)
	registry.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	if db != nil {
		registerDatabaseMetrics(registry, db)
	}

	return m
}

func registerDatabaseMetrics(registry *prometheus.Registry, db *sql.DB) {
	definitions := []struct {
		name  string
		help  string
		value func(sql.DBStats) float64
	}{
		{name: "open_connections", help: "Number of open database connections.", value: func(s sql.DBStats) float64 { return float64(s.OpenConnections) }},
		{name: "connections_in_use", help: "Number of database connections currently in use.", value: func(s sql.DBStats) float64 { return float64(s.InUse) }},
		{name: "idle_connections", help: "Number of idle database connections.", value: func(s sql.DBStats) float64 { return float64(s.Idle) }},
		{name: "connection_waits_total", help: "Total number of waits for a database connection.", value: func(s sql.DBStats) float64 { return float64(s.WaitCount) }},
		{name: "connection_wait_duration_seconds_total", help: "Total time spent waiting for database connections.", value: func(s sql.DBStats) float64 { return s.WaitDuration.Seconds() }},
	}

	for _, definition := range definitions {
		definition := definition
		registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "food_ordering_api",
			Subsystem: "database",
			Name:      definition.name,
			Help:      definition.help,
		}, func() float64 {
			return definition.value(db.Stats())
		}))
	}
}

func (m *applicationMetrics) handler() http.Handler {
	if m == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *applicationMetrics) observeHTTP(method, route string, next http.Handler) http.Handler {
	if m == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		m.httpInFlight.WithLabelValues(method, route).Inc()
		defer m.httpInFlight.WithLabelValues(method, route).Dec()

		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		m.httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
		m.httpDuration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
	})
}

func (m *applicationMetrics) recordMenuCache(operation, result string) {
	if m != nil {
		m.menuCacheAccess.WithLabelValues(operation, result).Inc()
	}
}
