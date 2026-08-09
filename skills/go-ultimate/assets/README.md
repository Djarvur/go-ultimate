# Assets — project templates

Copy-paste starting points, one per project type from the
[decision tree in SKILL.md](../SKILL.md#project-type-decision-tree). Each template
is a minimal, runnable skeleton matching the layout the skill mandates.

## How to use

1. Pick the template that matches the project type:
   - [`cli-simple/`](cli-simple/) — flat single-file CLI (no external deps).
   - [`cli-cobra/`](cli-cobra/) — multi-command CLI using Cobra.
   - [`library/`](library/) — importable package.
   - [`webservice/`](webservice/) — HTTP service (bounded-context hexagonal, default layout).
   - [`mcp-server/`](mcp-server/) — MCP server over stdio (typed handlers).
   - [`game/`](game/) — Ebitengine game.
2. Copy the directory to your new project: `cp -R cli-simple /path/to/my-tool`.
3. Find-and-replace the module path `example.com/<name>` with your real module
   (`go mod edit -module github.com/you/my-tool`).
4. Replace the placeholder name in `main.go` / the package name as appropriate.
5. **Verify before handing off** — the scaffolded project must pass this gate:

   ```
   go mod tidy
   go vet ./...
   go build ./...
   go test -race ./...
   ```

   If any step fails, **fix it before declaring the scaffold done** — a broken
   scaffold is a skill regression, not the user's problem to clean up. Common
   post-copy fixes: `go mod tidy` (for templates with external deps like
   `mcp-server/` and `cli-cobra/`), and adjusting the import path in
   `cmd/*/main.go` after the module rename. Re-run the gate from the top until
   all four commands are green.

   Only then report success to the user, with the final tree and the green
   command output.

## Rules baked in

- `package main` is **thin**: signal handling + `os.Exit` only. All testable logic
  lives in a `run(ctx, ...) error` function. (See
  [engineering-policy.md § `package main`](../references/engineering-policy.md).)
- Graceful shutdown via `signal.NotifyContext` for `os.Interrupt`/`SIGTERM` in every
  template that owns its own run loop (`cli-simple`, `cli-cobra`, `webservice`,
  `mcp-server`). `game/` is the exception: Ebitengine owns the loop and exits via
  `Update() error`.
- No `pkg/` directory. Private code under `internal/`; multiple binaries under `cmd/`.
- `webservice/` carries a **recover middleware** on its inbound HTTP adapter: a
  panicking handler returns 500 instead of killing the process. (See
  [engineering-policy.md § `panic` and `recover`](../references/engineering-policy.md).)
- Adapter tests use `httptest` against a hand-written `port.App` stub — see
  `webservice/internal/in/http/handler_test.go` and
  [testing.md](../references/testing.md).
- Layouts follow [project-layouts.md](../references/project-layouts.md).

## Source

Adapted from [danicat `go-project-setup`](https://github.com/danicat/skills) and
aligned to this skill's rules (`run` pattern, modern idioms, no `pkg/`).
