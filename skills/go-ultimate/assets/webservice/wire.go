// wire.go is the mandatory wiring entrypoint for the webservice template.
//
// It builds the dependency graph and returns an assembled Runtime plus any
// startable units. It starts nothing — main owns the process lifecycle.
// See go-ultimate/references/architecture.md § Wiring and
// references/wiring-and-runtime.md for the full pattern.
package main

import (
	"context"

	"example.com/webservice/internal/app"
	"example.com/webservice/internal/port"
)

// Config is the application's resolved business configuration. CLI/env grammar
// is parsed in main and mapped onto this; Wire validates it so every entrypoint
// surfaces the same field-attributed errors. Add business fields here (e.g.
// HTTPAddr, DBDSN) as the service grows.
type Config struct{}

// Unit is a startable descriptor returned by Wire and run by main's serve loop.
// It closes over Wire's ctx; main runs each in its own goroutine and cancels the
// rest when any one returns. Define concrete unit types as needed; for the
// template the slice is empty.
type Unit interface {
	Run(context.Context) error
}

// Runtime is the assembled application boundary plus any owned resources.
// Expose the boundary (port.App); keep resources private and release them in Close.
type Runtime struct {
	App port.App
	// db *sql.DB, clients, etc. — private, released in Close.
}

// Close releases every resource the wiring created. Tolerate partially-wired
// state (nil checks) so Wire can call Close on its own error path.
func (r *Runtime) Close() error {
	// close db pools, clients, etc. here.
	return nil
}

// Wire builds the dependency graph and returns the Runtime plus its startable
// units. It constructs the application boundary itself — callers do not pass an
// app in (that would invert the point of wiring).
//
//   - ctx:        the long-lived base context; startable units close over it.
//   - startupCtx: bounds wiring work only (e.g. dialing the DB); never stored.
//   - cfg:        resolved business Config, validated here.
//
// For the template we instantiate a single App with no dependencies and no units.
func Wire(ctx, startupCtx context.Context, cfg Config) (*Runtime, []Unit, error) {
	_ = ctx        // units would close over this; none in the template.
	_ = startupCtx // would bound adapter construction (DB dial, etc.).
	_ = cfg        // would be validated and threaded into adapters.

	a := app.New()
	return &Runtime{App: a}, nil, nil
}
