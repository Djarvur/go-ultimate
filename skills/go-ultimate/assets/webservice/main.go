// Command webservice is a minimal HTTP service using bounded-context hexagonal
// default layout.
//
// Template: webservice. Layout: bounded-context hexagonal, default variant —
// app private to its executable, internal/{port,app,in/http}. See
// go-ultimate/references/architecture.md and references/project-layouts.md § Service.
//
// main() is intentionally thin: signal handling, wiring, lifecycle, os.Exit.
// All testable logic lives in wire.go + internal/.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	inhttp "example.com/webservice/internal/in/http"
	"example.com/webservice/internal/port"
)

func main() {
	// Long-lived base context, cancelled by SIGINT/SIGTERM. main owns signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "webservice: %v\n", err)
		os.Exit(1)
	}
}

// run holds everything testable. A real service takes a resolved business Config
// as its second parameter — run(ctx, cfg) error — and passes it to Wire.
func run(ctx context.Context) error {
	var app port.App // nil here: Wire builds the real one. See wire.go.
	rt, err := Wire(ctx, app)
	if err != nil {
		return fmt.Errorf("wire: %w", err)
	}
	defer func() {
		if err := rt.Close(); err != nil {
			slog.Error("releasing runtime resources", "err", err)
		}
	}()

	addr := net.JoinHostPort("localhost", "8080")
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           inhttp.NewHandler(rt.App),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// run the server in a goroutine with a clear exit condition (ErrServerClosed).
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
