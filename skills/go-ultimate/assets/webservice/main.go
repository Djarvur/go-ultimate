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
)

func main() {
	// Long-lived base context, cancelled by SIGINT/SIGTERM. main owns signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, Config{}); err != nil {
		fmt.Fprintf(os.Stderr, "webservice: %v\n", err)
		os.Exit(1)
	}
}

// run holds everything testable. It takes the resolved business Config and
// threads it into Wire; cfg is parsed from flags/env in main in a real service.
func run(ctx context.Context, cfg Config) error {
	// startupCtx bounds wiring work (e.g. dialing the DB); it is never stored.
	// For the template there is no wiring work, so it mirrors ctx.
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rt, units, err := Wire(ctx, startupCtx, cfg)
	if err != nil {
		return fmt.Errorf("wire: %w", err)
	}
	defer func() {
		if err := rt.Close(); err != nil {
			slog.Error("releasing runtime resources", "err", err)
		}
	}()
	_ = units // run in main's serve loop; empty in the template.

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
