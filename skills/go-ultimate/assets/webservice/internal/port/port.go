// Package port is the application's bounded-context contract: the in-process
// boundary implemented by internal/app and called by inbound adapters.
//
// Keep App, Repo, other Out ports, and port-level errors in one port.go. The
// App interface lives here; its implementation is in internal/app. See
// go-ultimate/references/architecture.md § Ports.
package port

import "context"

// App is the business interface for the whole bounded context — one In port.
// Methods take ctx first, then business args; auth identity (if needed) is an
// explicit parameter, not pulled from context.
type App interface {
	// Hello returns a friendly greeting for name. Add real use-case methods here.
	Hello(ctx context.Context, name string) (string, error)
}
