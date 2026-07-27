# Project Layouts

Four canonical layouts, picked by project type via the
[decision tree in SKILL.md](../SKILL.md#project-type-decision-tree).

**Universal rule: no `pkg/` directory.** Library code lives at the module root;
importable apps expose a public `port/` package.

> **Copy-paste starting points:** runnable templates for each project type live
> in [`../assets/`](../assets/) — `cli-simple/`, `cli-cobra/`, `library/`,
> `webservice/`, `mcp-server/`, `game/`. See [`../assets/README.md`](../assets/README.md)
> for how to use them.

## Contents

- [1. Backend Service (Hexagonal)](#1-backend-service-hexagonal)
- [2. CLI Application](#2-cli-application)
- [3. Library / Module](#3-library-module)
- [4. Script / Tiny Tool](#4-script-tiny-tool)
- [Layout selection — when to grow](#layout-selection-when-to-grow)
- [Naming inside layouts](#naming-inside-layouts)
- [Sources](#sources)

---

## 1. Backend Service (Hexagonal)

For HTTP/gRPC/messaging servers. Full architecture in [architecture.md](architecture.md).

### Default layout (app private to its executable)

```text
service/
├── go.mod
├── go.sum
├── main.go                     Thin: flags, env, signal handling, calls run().
├── wire.go                     Mandatory. Builds the dependency graph, returns Runtime + startable units.
└── internal/
    ├── port/
    │   ├── port.go             App + Repo + other Out ports + port-level errors.
    │   └── types.go            Optional boundary DTO/types.
    ├── app/
    │   ├── app.go              Implements port.App. Business logic only.
    │   └── ...                 Split by use case / file when needed.
    ├── in/
    │   ├── http/adapter.go     Inbound HTTP adapter (thin).
    │   ├── grpc/adapter.go
    │   └── nats/adapter.go
    ├── dal/
    │   ├── adapter.go          Owned-database adapter.
    │   └── migrations/
    │       └── 00001_init.sql
    └── out/
        ├── redis/adapter.go
        └── nats/adapter.go     Outbound publisher (distinct from inbound subscriber).
```

### Importable layout (app must be importable by other binaries/modules)

Use this only when another binary, package, or repo must call the app in-process,
or in a modular monolith.

```text
monorepo/
├── go.mod
├── api/                        External wire contracts: proto, OpenAPI, event schemas.
├── apps/
│   ├── apps.go                 Composition root for the monolith; owns no business logic.
│   └── orderapp/
│       ├── wire.go             Public wiring API.
│       ├── integration_test.go Smoke tests for the whole app.
│       ├── port/               PUBLIC — the bounded-context contract.
│       │   ├── port.go         App + InprocApp + Repo + Out ports + errors.
│       │   └── types.go
│       └── internal/
│           ├── app/            PRIVATE — implements port.App.
│           ├── in/{http,grpc,nats}/
│           ├── dal/{,migrations/}
│           └── out/{redis,nats}/
├── cmd/
│   ├── order/main.go           Single-app binary.
│   └── monolith/{main.go,wire.go}
└── platform/{log,metrics,repo,serve}/   Shared infra helpers.
```

See [architecture.md](architecture.md) for the rules governing ports, adapters,
and wiring.

---

## 2. CLI Application

For command-line tools with flags/subcommands (cobra, kong, urfave/cli).

### Simple CLI (one binary, < ~10 files): flat at root

```text
cli-simple/
├── go.mod
├── main.go                     flag parsing; calls run(ctx, cfg).
├── run.go                      func run(ctx context.Context, cfg Config) error. Testable.
└── *.go                        Other commands / helpers in package main.
```

### Multi-command CLI (one binary, several subcommands): `cmd/` + `internal/`

One binary at the root; `cmd/` is a **package of subcommand constructors**, not a
directory of binaries. Matches [`../assets/cli-cobra/`](../assets/cli-cobra/).

```text
cli-multi/
├── go.mod
├── main.go                     Thin: calls cmd.Execute().
├── cmd/
│   ├── root.go                 newRootCmd(); owns global flags, adds subcommands.
│   ├── add.go                  newAddCmd()
│   └── remove.go               newRemoveCmd()
└── internal/
    ├── config/config.go
    └── store/store.go
```

Build commands with factory functions (`newRootCmd`, `newAddCmd`, …) rather than
package-level vars, so no hidden global state leaks between tests or invocations.

### Multiple binaries: `cmd/<app-name>/main.go`

Only when the repo genuinely ships more than one executable — a different rule
from the one above, and the one [engineering-policy.md](engineering-policy.md)
§ `package main` refers to.

```text
tools/
├── go.mod
├── cmd/
│   ├── importer/main.go        Binary 1.
│   └── reporter/main.go        Binary 2.
└── internal/
    └── store/store.go          Shared private code.
```

### The `run(ctx, cfg) error` pattern (mandatory)

```go
// main.go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    cfg := parseFlags()
    if err := run(ctx, cfg); err != nil {
        fmt.Fprintf(os.Stderr, "my-tool: %v\n", err)
        os.Exit(1)
    }
}

// run.go — testable, no os.Args / os.Exit here.
func run(ctx context.Context, cfg Config) error {
    // ... actual work ...
    return nil
}
```

`main()` handles only flags, env, signals, and `os.Exit`. All testable logic
lives in `run` (or below). `main()` itself must contain no logic worth testing.

---

## 3. Library / Module

For code meant to be imported by other modules.

```text
library/
├── go.mod                       go directive = latest-1
├── doc.go                       Package doc — contracts, invariants, concurrency guarantees.
├── *.go                         Public API at the module root.
├── internal/                    Private implementation.
├── example_test.go              Runnable examples (rendered on pkg.go.dev).
├── README.md                    Rationale, honest comparison, payoff-first quick start.
└── LICENSE
```

Rules:
- Public API lives at the module root in `package <name>` — **not** under `pkg/`.
- Use `internal/` for everything that is not part of the public API.
- Major versions v2+ live on a `/v2` suffix path or branch.
- See [libraries.md](libraries.md) for README vs `doc.go` split, semver, and API stability.

### `/v2` major-version layout

```text
library/v2/
├── go.mod                       module github.com/user/library/v2
├── doc.go
└── *.go
```

---

## 4. Script / Tiny Tool

For single-file or few-file utilities run with `go run`.

### Stdlib-only script (no external deps, no go.mod needed)

If the script uses only the standard library, it is a single self-contained
file — no `go.mod`, no `internal/`, no `cmd/`. This is the simplest form and
matches the skill's own bundled `scripts/goversion/main.go`:

```text
project/
└── scripts/
    └── some-tool/
        └── main.go              Single file, package main, stdlib only.
```

Run it by pointing `go run` at the file directly (works from any directory):

```
go run ./scripts/some-tool/main.go [args…]
```

### Script with external dependencies

When the script needs third-party packages, give it its own module so
dependencies resolve:

```text
project/
└── scripts/
    └── some-tool/
        ├── go.mod               module-local; holds the require directives.
        ├── go.sum
        └── main.go
```

Run with `go run ./scripts/some-tool/main.go` from inside `scripts/`, or from
the script's own directory with `go run main.go`.

### Standalone tiny tool (flat)

```text
tool/
├── go.mod
└── main.go                      Single file, package main.
```

A tiny tool may collapse the Service roles (`wire`, `port`, `app`, `out`, `dal`)
into separate files in one package — *if* it stays truly tiny. Grow into the CLI
or Service layout as soon as it stops feeling flat.

---

## Layout selection — when to grow

- **Script → CLI**: when flags/subcommands appear, or files exceed ~5.
- **CLI flat → `cmd/` + `internal/`**: when multiple binaries exist or private
  packages are needed.
- **Service Default → Importable**: only when another binary/module/repo must
  call the app in-process, or for a modular monolith.
- **Any → Library**: when the code is meant to be imported (not run), move the
  public API to the module root and everything else to `internal/`.

**Do not skip ahead to a more complex layout because future growth is
imaginable.** Move when a concrete need appears.

---

## Naming inside layouts

- Short, high-signal package names: `dom`, `app`, `dal`, `port`, `srv`, `svc`, `repo`.
- Do not repeat the directory noun in filenames:
  - ❌ `order/order.go`, `service/order_service.go`
  - ✅ `order/app.go`, `service/adapter.go`
- Generic filenames where the package gives context: `adapter.go`, `app.go`, `port.go`, `wire.go`.
- Use `New` as the constructor name when the package already explains what is built.

---

## Sources

- [powerman `go-bounded-context-hexagonal`](https://github.com/powerman/skills) — Flat / Default / Importable layouts.
- [danicat `go-project-setup`](https://github.com/danicat/skills) — template-driven scaffolding (cli-simple, cli-cobra, library, webservice, mcp-server, game).
- The `run(ctx, cfg) error` pattern is standard Go; both danicat and powerman imply it.
