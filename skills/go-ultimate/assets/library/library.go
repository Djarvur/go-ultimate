// Package library is a template for an importable Go library.
//
// Template: library. Layout: public API at the module root in package library;
// private implementation under internal/. No pkg/.
// See go-ultimate/references/libraries.md.
package library

import "fmt"

// Hello returns a friendly greeting for name.
//
// Exported identifiers carry doc comments starting with the identifier name.
// Keep the public surface minimal — every export is a maintenance promise.
func Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
