package main

// goversion extracts the `go` directive from a project's go.mod and prints it.
// This is a one-purpose tool: its output format is EXACTLY the same as the inline
// shell command from the original JetBrains use-modern-go SKILL.md:
//
//	grep -rh "^go " --include="go.mod" . 2>/dev/null | cut -d' ' -f2 \
//	  | sort | uniq -c | sort -nr | head -1 | xargs | cut -d' ' -f2 \
//	  | grep . || echo unknown
//
// The output is the raw version string (e.g. "1.24.3") followed by a newline.
// When no go.mod or no go directive is found, it prints the current Go runtime
// version instead of "unknown".
//
// Usage:
//
//	go run ./scripts/goversion/main.go [path]
//
// Path defaults to '.' (current directory).
//
// This tool uses only the Go standard library — no external dependencies.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := run(context.Background(), os.Stdout, dir); err != nil {
		os.Exit(1)
	}
}

// --- public API used by tests (external package) ---

// ParseGoDirectiveForTest exposes parseGoDirective to the external test package.
func ParseGoDirectiveForTest(data string) (string, error) { return parseGoDirective(data) }

// RunForTest exposes run to the external test package.
func RunForTest(ctx context.Context, w io.Writer, dir string) error { return run(ctx, w, dir) }

// ErrNoGoModForTest exposes errNoGoMod to the external test package.
var ErrNoGoModForTest = errNoGoMod

// ErrNoGoDirectiveForTest exposes errNoGoDirective to the external test package.
var ErrNoGoDirectiveForTest = errNoGoDirective

// --- internal logic ---

// run reads go.mod under dir, parses the `go` directive, and writes the version
// string to w. On any failure (no go.mod, no directive, read error, parse error),
// it falls back to the Go runtime version — mirroring the original script's
// "|| echo unknown" but with the real version.
func run(ctx context.Context, w io.Writer, dir string) error {
	ver, err := detectVersion(dir)
	if err != nil {
		ver = runtimeVersion()
	}
	fmt.Fprintln(w, ver)
	return nil
}

// detectVersion resolves go.mod in dir, reads it, and returns the go directive
// version string. It returns an error if go.mod does not exist, cannot be read,
// or has no go directive — the caller is expected to fall back.
func detectVersion(dir string) (string, error) {
	gomodPath, err := findGoMod(dir)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(gomodPath)
	if err != nil {
		return "", err
	}
	return parseGoDirective(string(data))
}

// runtimeVersion returns the Go version the goversion binary was compiled with,
// in the same format as a go.mod go directive (e.g. "1.24.3").
func runtimeVersion() string {
	v := runtime.Version() // "go1.24.3", "go1.24", or "devel +abcdef..."
	v = strings.TrimPrefix(v, "go")
	return v
}

// findGoMod resolves go.mod in or below dir. It only checks the one directory;
// it does not walk upward. Returns an error if go.mod is missing or inaccessible.
func findGoMod(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", dir, err)
	}
	gomodPath := filepath.Join(abs, "go.mod")
	info, err := os.Stat(gomodPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errNoGoMod
		}
		return "", fmt.Errorf("stat %s: %w", gomodPath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file", gomodPath)
	}
	return gomodPath, nil
}

// parseGoDirective extracts the `go` version string from go.mod contents by
// scanning line by line. This uses only stdlib (no golang.org/x/mod dependency).
// Returns the bare version string (e.g. "1.24.3"), not including the "go" prefix.
//
// It skips lines that are comments or inside block directives (require, exclude,
// etc.) to avoid false positives, but the simplest reliable rule is: find the
// first non-comment line that starts with "go ".
func parseGoDirective(data string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	inBlock := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Track block nesting for multi-line directives.
		// Block openers look like "require (" or "exclude (" or just "(".
		// Block closers are ")".
		if inBlock == 0 && strings.HasSuffix(line, "(") &&
			!strings.HasPrefix(line, "go ") {
			inBlock++
			continue
		}
		if line == ")" {
			inBlock--
			continue
		}
		if inBlock > 0 || strings.HasPrefix(line, "(") {
			continue
		}

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
			continue
		}

		// Look for "go <version>".
		if strings.HasPrefix(line, "go ") {
			v := strings.TrimSpace(line[3:])
			if v == "" {
				continue
			}
			// The version is everything on the line after "go ".
			// It should be something like "1.24.3" or "1.24".
			// Validate it looks like a version number (digit dot digit...).
			if !isVersion(v) {
				continue
			}
			return v, nil
		}
	}
	return "", errNoGoDirective
}

// isVersion checks whether s looks like a Go version string (e.g. "1.24.3").
func isVersion(s string) bool {
	if s == "" {
		return false
	}
	// Must start with a digit.
	if s[0] < '0' || s[0] > '9' {
		return false
	}
	// Must contain at least one dot (major.minor).
	if !strings.Contains(s, ".") {
		return false
	}
	return true
}

// --- errors (private sentinels) ---

var errNoGoMod = errorsNew("no go.mod found")

var errNoGoDirective = errorsNew("go.mod has no go directive")

// errorsNew creates a simple sentinel error without importing the errors package.
func errorsNew(msg string) error { return &simpleError{msg: msg} }

type simpleError struct{ msg string }

func (e *simpleError) Error() string { return e.msg }

func (e *simpleError) Is(target error) bool {
	if t, ok := target.(*simpleError); ok {
		return e.msg == t.msg
	}
	return e.Error() == target.Error()
}
