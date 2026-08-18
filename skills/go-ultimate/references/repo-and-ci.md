# Repository and CI

The files every Go repository must have, and the pipeline that enforces the rest
of this skill. Read this when initializing a repository, reviewing repository
hygiene, or setting up CI.

[engineering-policy.md § Dependencies and CI](engineering-policy.md) states the
mandatory gates; this reference is the concrete shape — the files, the workflow,
and the tool-invocation policy.

## Contents

- [Required files](#required-files)
- [Ignore baselines](#ignore-baselines)
- [Tool invocation — `go tool`](#tool-invocation-go-tool)
- [Linter configuration](#linter-configuration)
- [GitHub Actions](#github-actions)
- [Build reproducibility and version stamping](#build-reproducibility-and-version-stamping)
- [Task runner](#task-runner)
- [Containers](#containers)
- [Sources](#sources)

---

## Required files

Every Go repository, public or private, has all of these. A missing one is a
review finding, not a nice-to-have.

| File | Why |
|---|---|
| `LICENSE` | Without it the code is legally unusable by anyone else — "public on GitHub" is not a licence. Default to **MIT** unless the project says otherwise. |
| `README.md` | What it is, how to install it, one runnable example. For libraries the bar is higher — see [libraries.md § README](libraries.md). |
| `AGENTS.md` | Project-specific conventions for AI agents. This skill defers to it (SKILL.md § Project-file precedence), so it is where a project records its legitimate deviations. |
| `.gitignore` | Below. |
| `.dockerignore` | Any repo that builds an image. Without it the build context includes `.git/` and every local artifact, which is both slow and a secret-leak path. |
| `.golangci.yml` | The lint contract, versioned with the code. |
| `.github/workflows/` | Lint and test gates on every PR. |

`AGENTS.md` should be short and specific — the deviations, not a restatement of
general Go practice:

```markdown
# AGENTS

## Conventions
- Go version: as pinned in `go.mod`. Do not raise it without an ADR.
- Run `go tool golangci-lint run ./...` and `go test -race ./...` before pushing.
- Deviations from go-ultimate: <none | list them with the reason>

## Layout
- <anything a reader could not infer from the tree>
```

> **When a project already has one of these, merge — do not overwrite.** Adding
> the missing baseline patterns to an existing `.gitignore` is correct;
> replacing it wholesale deletes project-specific entries that someone added for
> a reason. The same applies to `AGENTS.md` and `.golangci.yml`.

---

## Ignore baselines

### `.gitignore`

Cover four categories. Anything less and build artifacts end up in review diffs.

```gitignore
# Binaries and build output
bin/
dist/
*.exe
*.test

# Test and coverage output
*.coverprofile
coverage.out
coverage.html

# Tool caches
.cache/
.gocache/
.gomodcache/

# Editors and IDEs
.idea/
.vscode/
*.swp
*~

# OS noise
.DS_Store
Thumbs.db

# Local configuration — never commit real values
.env
```

Note what is **not** here: `vendor/` is only ignored if the project has decided
not to vendor. If it vendors, `vendor/` is committed on purpose.

### `.dockerignore`

The build context should contain the source needed to compile and nothing else:

```dockerignore
.git/
.github/
bin/
dist/
*.md
.env
testdata/
Dockerfile
```

### `.aiignore` (optional)

Keeps AI agents out of files that waste context or should not be read — large
generated files, binary assets, vendored trees:

```gitignore
bin/
vendor/
*.generated.go
*.pb.go
*.png
*.jpg
*.pdf
*.log
```

This is a convention, not a standard: support varies by harness, so treat it as a
hint rather than an access control.

---

## Tool invocation: `go tool`

**Pin every tool in `go.mod` with the `tool` directive (Go 1.24+) and invoke it
through `go tool`.** Never the old `tools.go` blank-import hack, and never a
bare `go install` that resolves to whatever the developer happens to have.

```console
$ go get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
$ go tool golangci-lint run ./...
```

The point is that CI, every developer, and every agent run the **same version**.
An unpinned linter means a PR that is green locally and red in CI for reasons
nobody can reproduce; an unpinned generator means checked-in generated code that
churns from machine to machine ([engineering-policy.md § Code
generation](engineering-policy.md)).

> **When tool dependencies pollute the main module** — `golangci-lint` in
> particular pulls a large graph into `go.sum` — isolate them in a dedicated
> `tools/go.mod`. That keeps the library's own dependency graph honest, which
> matters for anything others import. For applications, the churn is usually
> acceptable and one module is simpler.

---

## Linter configuration

The mandatory linter set is in
[engineering-policy.md § Linting](engineering-policy.md). Two rules about the
config file itself:

- **Version the config with the code.** `.golangci.yml` at the repo root, in git.
  A lint standard that lives in someone's CI settings is not a standard.
- **Pin the `golangci-lint` version in CI.** The linter changes what it reports
  between minor versions; an unpinned linter turns an unrelated PR red.

### Strict configurations

For a new project with no legacy to grandfather, a curated strict config is a
better starting point than assembling one by hand —
[`powerman/golangci-lint-strict`](https://github.com/powerman/golangci-lint-strict)
publishes one config per exact `golangci-lint` version.

If you adopt it:

- **Fetch the config for the exact `golangci-lint` version you run.** Config
  schema changes between versions; a mismatched config fails to parse or silently
  ignores sections.
- **Do not edit the upstream file.** Keeping it byte-identical is the whole
  point: it makes lint behavior reproducible and upgrades mechanical. Put
  project overrides in a separate config that extends it, or accept a rule and
  fix the code.
- **Never overwrite an existing `.golangci.yml` without asking.** Back it up
  first; a project's exclusions usually encode decisions nobody wrote down.

This is an **opt-in upgrade**, not the skill's default. The default remains the
explicit linter list in engineering-policy — it is small enough to read, argue
with, and own.

---

## GitHub Actions

### Pin the Go version to `go.mod`, not to the workflow

```yaml
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
```

`go-version-file` means the version lives in exactly one place. A hardcoded
`go-version: '1.25'` in the workflow silently diverges from `go.mod` the first
time someone bumps one and not the other, and then CI is testing a different
language version than the code declares.

### Minimum workflow

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.event_name }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go build ./...
      - run: go vet ./...
      - run: go test -race -cover ./...
      - name: modules are tidy
        run: |
          go mod tidy
          git diff --exit-code -- go.mod go.sum

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@v7
        with:
          version: v2.12   # pinned on purpose

  vuln:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Ready-to-copy versions of these two jobs are in
[`assets/ci/`](../assets/ci/).

Details that matter:

- **`permissions: contents: read`** — the default token is over-privileged.
  Grant per-job what a job actually needs.
- **`concurrency` with `cancel-in-progress`** — a stale run on an amended PR is
  wasted minutes. Include `github.event_name` in the group so a `push` and a
  `pull_request` for the same ref do not cancel each other.
- **Pin actions to a major tag** at minimum; pin to a commit SHA for anything
  handling secrets.
- **The tidy check is `go mod tidy` + `git diff --exit-code`** (or
  `go mod tidy -diff` on Go 1.23+). Without it, `go.sum` drifts until someone's
  build breaks for an unrelated reason.

### Library repositories: test the supported range

An application tests the one version it ships. A library must test the versions
it claims to support — at minimum the `go.mod` floor and the current release:

```yaml
strategy:
  matrix:
    go: ['1.25', '1.26']
```

See [engineering-policy.md § `go.mod` version policy](engineering-policy.md) for
which floor a library should declare.

---

## Build reproducibility and version stamping

A binary in production must be traceable to a commit.

```console
$ go build -trimpath \
    -ldflags "-X main.version=$(git describe --tags --always) \
              -X main.commit=$(git rev-parse --short HEAD) \
              -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    ./cmd/myapp
```

- **`-trimpath`** removes local filesystem paths from the binary, which is what
  makes the build reproducible across machines and stops it leaking
  `/Users/you/...` into stack traces.
- **Stamp version, commit and build date** via `-ldflags -X`, and expose them
  through a `--version` flag. "Which build is running?" must be answerable from
  the running process, not inferred from a deploy log.
- On Go 1.24+, much of this is also available at runtime from
  `runtime/debug.ReadBuildInfo()` — use it for VCS revision and dependency
  versions rather than stamping what the toolchain already records.

For tagged releases, `goreleaser` is the common choice and handles the above plus
checksums, archives and SBOM generation. It is a fine default for a CLI
distributed as binaries; it is unnecessary for a library.

---

## Task runner

Standardize the entry points so that a contributor — or an agent — does not have
to reverse-engineer the pipeline from CI YAML:

```makefile
.PHONY: lint test generate build
lint:     ; go tool golangci-lint run ./...
test:     ; go test -race -cover ./...
generate: ; go generate ./...
build:    ; go build ./...
```

`make lint`, `make test`, `make generate`, `make build` (or the Taskfile
equivalents). **CI runs the same targets** — if CI and the Makefile drift, the
Makefile becomes a lie and everyone goes back to reading the YAML.

---

## Containers

Use a **multi-stage build**. The final image contains the binary and its
runtime dependencies, nothing else — no toolchain, no source, no module cache.

```dockerfile
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/app ./cmd/app

FROM gcr.io/distroless/static-nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
```

- **Copy `go.mod`/`go.sum` and run `go mod download` before copying the source**
  so the dependency layer stays cached across source changes.
- **`CGO_ENABLED=0`** for a static binary, which is what lets the runtime stage
  be `distroless/static` or `scratch`. If the build genuinely needs cgo, the
  runtime stage needs a matching libc.
- **Run as non-root.** See
  [production-readiness.md § Secrets and hardening](production-readiness.md).
- Place the `Dockerfile` where the project expects it — repo root for a single
  binary, next to the entrypoint or under `build/` for several.

---

## Sources

- [metalagman `go-oss-maintainer`](https://github.com/metalagman/agent-skills) — required-file set, ignore baselines, the merge-don't-overwrite rule, `go-version-file: go.mod`, `go tool` invocation policy.
- [metalagman `golangci-lint-strict`](https://github.com/metalagman/agent-skills) + [powerman/golangci-lint-strict](https://github.com/powerman/golangci-lint-strict) — exact-version config pinning, do-not-edit-upstream rule.
- [metalagman `go-senior-developer`](https://github.com/metalagman/agent-skills) — `-trimpath`/`-ldflags` stamping, task-runner standardization, multi-stage Docker default, tools-module isolation.
- Workflow permissions/concurrency shape, the library test matrix, and the layer-caching Dockerfile ordering are this skill's additions.
