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

// Wire builds the dependency graph and returns the Runtime.
//
// In a real service:
//   - cfg would be a Resolved business Config, validated here.
//   - startupCtx bounds wiring work (e.g. dialing the DB).
//   - real adapters and Out ports would be constructed and injected into app.
//
// For the template we instantiate a single App with no dependencies.
func Wire(ctx context.Context, application port.App) (*Runtime, error) {
	// If no app was injected (template scaffolding), build a real one.
	if application == nil {
		application = app.New()
	}
	return &Runtime{App: application}, nil
}
