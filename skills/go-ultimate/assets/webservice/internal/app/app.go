// Package app implements port.App. This is the only place business logic lives.
//
// Adapters must stay thin; this package must not import net/http, os, or any
// transport SDK. See go-ultimate/references/architecture.md § Adapter discipline.
package app

import (
	"context"
	"fmt"

	"example.com/webservice/internal/port"
)

// New constructs the App. Real wiring goes in wire.go (parent dir).
func New() port.App {
	return &App{}
}

// App implements port.App. Keep the struct unexported; expose only the interface.
type App struct {
	// Owned resources (DB pool, clients) would live here as private fields.
}

var _ port.App = (*App)(nil)

// Hello implements port.App.
func (a *App) Hello(ctx context.Context, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if name == "" {
		name = "World"
	}
	return fmt.Sprintf("Hello, %s!", name), nil
}
