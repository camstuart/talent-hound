# Talent Hound

A local-first AI desktop app built with Wails v3 (Go backend) and SolidJS (TypeScript frontend).

## Prerequisites

- Go 1.24+
- [Bun](https://bun.sh) — the only JS package manager/runtime used in this repo
- [Wails v3 CLI](https://v3.wails.io): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- [just](https://github.com/casey/just) — task runner (`winget install Casey.Just` / `brew install just`)

QA tooling (Go-side; the TS-side tools are regular `frontend/` devDependencies):

```sh
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$(go env GOPATH)/bin"
```

Run `wails3 doctor` to verify your environment.

## Commands

`just` is the front door for day-to-day work (run `just` alone to list recipes):

| Command | Description |
| --- | --- |
| `just dev` | Run the app in development mode (desktop window, hot reload) |
| `just test` | All three test layers: Go → Vitest → Playwright |
| `just qa` | All linting & static analysis (see below) |
| `just check` | `qa` + `test` — the full pre-commit gate |

`just qa` runs, individually addressable as recipes:

| Recipe | Tool | What it checks |
| --- | --- | --- |
| `just lint-go` | [golangci-lint](https://golangci-lint.run) | Go meta-linter: staticcheck, govet, errcheck, revive, gocritic, gosec, unparam, misspell, gofmt/goimports and more (config: `.golangci.yml`) |
| `just sec` | [gosec](https://github.com/securego/gosec) | Go security scanner (standalone pass) |
| `just vuln` | [govulncheck](https://go.dev/blog/vuln) | Known CVEs in Go deps, call-graph aware |
| `just typecheck` | tsc | `tsc --noEmit` over `src/` + generated bindings |
| `just lint-ts` | [oxlint](https://oxc.rs) | Fast TS/JS linter, warnings are errors |
| `just dupes` | [jscpd](https://jscpd.dev) | Copy-paste detection across Go + TS (config: `.jscpd.json`) |

The underlying non-just commands still work (`wails3 dev`, `go test ./...`, `wails3 task test`, `bun run test:unit` / `test:e2e` in `frontend/`). Frontend dependencies are installed automatically by `wails3 dev`/`wails3 task` via Bun. To install manually: `cd frontend && bun install`.

## Project layout

- `main.go` — Wails application entry point, DB bootstrap and window setup
- `greetservice.go` — minimal example Go service (`GreetService.Greet`, no UI uses it)
- `initiativeservice.go` — initiative CRUD service backed by SQLite
- `internal/models/` — GORM models, one file per model (`initiative.go`, ...)
- `internal/db/` — SQLite bootstrap: pure-Go driver (`glebarez/sqlite`, no CGO), auto-migration, per-user DB path (`TALENT_HOUND_DB_PATH` overrides; E2E uses `frontend/.e2e-db/e2e.db`)
- `frontend/src/` — SolidJS UI (`App.tsx`, `main.tsx`)
- `frontend/bindings/` — TypeScript bindings generated from Go services by `wails3 generate bindings`. **Never hand-edit** — regenerated automatically on build/dev.
- `frontend/e2e/` — Playwright E2E specs
- `Taskfile.yml` / `build/Taskfile.yml` — Wails v3 Taskfile-based build system

## E2E testing strategy

Playwright drives the app through Wails v3's **server build**: a build tagged `-tags server` that runs the app as a plain HTTP server (no native GUI) with the real Go backend attached, serving the built frontend and handling bound-service calls over HTTP (`/wails/runtime`) and events over WebSocket (`/wails/events`).

`frontend/playwright.config.ts` starts this via `wails3 task run:server DEV=true` (from the repo root) as Playwright's `webServer`, and waits on `http://localhost:8080`. This means E2E tests exercise the actual Go services (e.g. `InitiativeService` writing to SQLite), not a mock — unlike the Vitest unit tests, which mock `frontend/bindings/*` to run without a Go backend.

Plain browser tabs (e.g. `wails3 dev`'s Vite dev server viewed directly in a browser) cannot complete binding calls — that scheme only works inside the native WebView2 window or the server build's HTTP transport.

## Known environment quirks

- **Node.js version warning**: `vite`/`playwright`'s shell shims resolve to the system Node.js on PATH (not Bun's runtime) on Windows. If that Node is older than what Vite 8 wants (20.19+/22.12+), you'll see a harmless startup warning; the build still succeeds. Upgrade Node if you'd rather not see it.
- **`jsdom` pinned to `26.1.0`**: `jsdom@27+` depends on `html-encoding-sniffer@6`, which `require()`s an ESM-only transitive dependency (`@exodus/bytes`) and crashes Vitest's worker pool (`ERR_REQUIRE_ESM`). Keep `frontend/package.json`'s `jsdom` pin until upstream fixes this.
