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

	"example.com/webservice/internal/port"
	"example.com/webservice/internal/in/http"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "webservice: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Long-lived base context, cancelled by SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// In a real service cfg would come from flags/env and be Resolve()d here.
	app := port.App(nil) // wire builds the real app; this is a template
	rt, err := Wire(ctx, app)
	if err != nil {
		return fmt.Errorf("wire: %w", err)
	}
	defer rt.Close()

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
