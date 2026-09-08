package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type contextKey string

const requestIDContextKey = contextKey("requestID")

const partnerIDContextKey = contextKey("partnerID")

type partnerAuthenticator interface {
	PartnerID(string) (string, bool)
}

type configuredPartners map[string][sha256.Size]byte

func apiKeyHash(key string) [sha256.Size]byte {
	return sha256.Sum256([]byte(key))
}

func (p configuredPartners) PartnerID(key string) (string, bool) {
	candidate := apiKeyHash(key)
	for partnerID, expected := range p {
		if subtle.ConstantTimeCompare(candidate[:], expected[:]) == 1 {
			return partnerID, true
		}
	}
	return "", false
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(data)
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-XSS-Protection", "0")

		next.ServeHTTP(w, r)
	})
}

func (app *application) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		app.infoLog.Printf(
			"%s - %s %s %s %d %s request_id=%s",
			r.RemoteAddr,
			r.Proto,
			r.Method,
			r.URL.RequestURI(),
			recorder.status,
			time.Since(started).String(),
			recorder.Header().Get("X-Request-ID"),
		)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	if !app.config.limiter.enabled {
		return next
	}

	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu          sync.Mutex
		clients     = make(map[string]*client)
		lastCleanup = time.Now()
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		mu.Lock()
		now := time.Now()
		if now.Sub(lastCleanup) >= time.Minute {
			for ip, client := range clients {
				if now.Sub(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			lastCleanup = now
		}
		if _, found := clients[ip]; !found {
			clients[ip] = &client{
				limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
			}
		}
		clients[ip].lastSeen = now

		if !clients[ip].limiter.Allow() {
			mu.Unlock()
			app.rateLimitExceededResponse(w, r)
			return
		}
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.serverError(w, r, fmt.Errorf("%s", err))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) requirePartner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Authorization")

		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			app.authenticationRequiredResponse(w, r)
			return
		}

		key := parts[1]
		if key == "" || app.partners == nil {
			app.authenticationRequiredResponse(w, r)
			return
		}

		partnerID, ok := app.partners.PartnerID(key)
		if !ok {
			app.authenticationRequiredResponse(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), partnerIDContextKey, partnerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func partnerIDFromContext(r *http.Request) string {
	partnerID, _ := r.Context().Value(partnerIDContextKey).(string)
	return partnerID
}

func requestIDFromContext(r *http.Request) string {
	requestID, ok := r.Context().Value(requestIDContextKey).(string)
	if !ok {
		return ""
	}
	return requestID
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
