// Package inhttp is the inbound HTTP adapter — a thin layer that adapts
// HTTP requests to port.App. It contains no business decisions.
//
// It may import net/http (it is an adapter, not business logic). See
// go-ultimate/references/architecture.md § Adapters and the "internal/in/*" rule.
package inhttp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"example.com/webservice/internal/port"
)

// NewHandler wires the app to its HTTP routes and returns an http.Handler.
// Uses the enhanced Go 1.22+ ServeMux with method + path patterns.
func NewHandler(app port.App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		handleIndex(w, r, app)
	})
	mux.HandleFunc("GET /health", handleHealth)
	return logging(mux)
}

func handleIndex(w http.ResponseWriter, r *http.Request, app port.App) {
	name := r.URL.Query().Get("name")
	greeting, err := app.Hello(r.Context(), name)
	if err != nil {
		// Adapter concern: translate app errors to HTTP status. No business logic.
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(greeting))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// logging is a minimal middleware. Real services add request-id, trace-id,
// duration, and status via the middleware chain.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// _ ensures context is imported even when no handler yet uses r.Context()
// directly (template scaffolding only — remove once handlers consume ctx).
var _ = context.Background
