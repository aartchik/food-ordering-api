package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func routes(state *serviceState, errorLog *log.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		snapshot := state.snapshot()
		status := http.StatusOK
		if !snapshot.Ready {
			status = http.StatusServiceUnavailable
		}
		writeState(w, status, snapshot, errorLog)
	})
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, _ *http.Request) {
		writeState(w, http.StatusOK, state.snapshot(), errorLog)
	})
	return mux
}

func writeState(w http.ResponseWriter, status int, state stateSnapshot, errorLog *log.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(state); err != nil {
		errorLog.Printf("write response: %v", err)
	}
}
