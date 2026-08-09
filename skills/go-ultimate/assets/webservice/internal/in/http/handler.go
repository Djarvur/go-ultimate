// Package inhttp is the inbound HTTP adapter — a thin layer that adapts
// HTTP requests to port.App. It contains no business decisions.
//
// It may import net/http (it is an adapter, not business logic). See
// go-ultimate/references/architecture.md § Adapters and the "internal/in/*" rule.
package inhttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"example.com/webservice/internal/port"
)

// NewHandler wires the app to its HTTP routes and returns an http.Handler.
// Uses the enhanced Go 1.22+ ServeMux with method + path patterns.
//
// Middleware is applied outermost-first: recovery wraps logging wraps the mux,
// so a panic anywhere inside is still caught.
func NewHandler(app port.App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		handleIndex(w, r, app)
	})
	mux.HandleFunc("GET /health", handleHealth)
	return recovery(logging(mux))
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

func handleHealth(w http.ResponseWriter, _ *http.Request) {
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

// recovery turns a panicking handler into a 500 instead of a dead process.
// net/http recovers per connection, but it logs nothing structured and drops the
// response silently — so a server owns this explicitly.
//
// It only covers the request goroutine: a panic inside a goroutine a handler
// spawned still kills the process. Goroutines must not panic.
func recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// http.ErrAbortHandler is the documented way to abort a response;
			// re-panic so net/http handles it as intended.
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			slog.Error("panic in handler",
				"method", r.Method, "path", r.URL.Path,
				"panic", rec, "stack", string(debug.Stack()))
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
