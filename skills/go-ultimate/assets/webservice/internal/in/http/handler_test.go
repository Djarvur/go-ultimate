package inhttp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	inhttp "example.com/webservice/internal/in/http"
	"example.com/webservice/internal/port"
)

// fakeApp is a hand-written stub for port.App. A one-method port does not need a
// generated mock; reach for gomock when the assertions are about call counts,
// argument matchers or ordering. See go-ultimate/references/testing.md § Mocking.
type fakeApp struct {
	greeting string
	err      error
	panicMsg string
}

var _ port.App = (*fakeApp)(nil)

func (f *fakeApp) Hello(_ context.Context, _ string) (string, error) {
	if f.panicMsg != "" {
		panic(f.panicMsg)
	}
	return f.greeting, f.err
}

// Adapter tests drive the handler through httptest — no listener, no port.
// See go-ultimate/references/testing.md § HTTP adapters.
func TestNewHandler_index(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		app      *fakeApp
		wantCode int
		wantBody string
	}{
		{
			name:     "ok",
			app:      &fakeApp{greeting: "Hello, World!"},
			wantCode: http.StatusOK,
			wantBody: "Hello, World!",
		},
		{
			name:     "app error becomes 500",
			app:      &fakeApp{err: errors.New("boom")},
			wantCode: http.StatusInternalServerError,
		},
		{
			// The recovery middleware must turn a panicking handler into a 500
			// instead of taking the process down.
			name:     "panic becomes 500",
			app:      &fakeApp{panicMsg: "unexpected nil"},
			wantCode: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/?name=World", nil)
			rec := httptest.NewRecorder()

			inhttp.NewHandler(tt.app).ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestNewHandler_health(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	inhttp.NewHandler(&fakeApp{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}
