# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Talent Hound is a local-first AI desktop app: Wails v3 (Go backend) + SolidJS/TypeScript frontend. **Bun** is the only JS package manager/runtime used here — never use npm/yarn/pnpm. The build system is Taskfile-based, driven through the `wails3` CLI.

## Commands

`just` is the preferred entry point (see `justfile`; `just` alone lists recipes):

| Command | Description |
| --- | --- |
| `just dev` | Run the app in development mode (desktop window, hot reload) |
| `just test` | All three test layers in sequence (Go → Vitest → Playwright) |
| `just qa` | All linting/static analysis: golangci-lint (`.golangci.yml`), gosec, govulncheck, tsc, oxlint, jscpd (`.jscpd.json`) |
| `just check` | `qa` + `test` — run before committing |
| `go test ./...` | Go unit tests |
| `go test -run TestName ./...` | Single Go test |
| `bun run test:unit` (in `frontend/`) | TypeScript unit tests (Vitest) |
| `bunx vitest run src/App.test.tsx` (in `frontend/`) | Single Vitest file |
| `bun run test:e2e` (in `frontend/`) | Playwright E2E tests |
| `bunx playwright test e2e/initiatives.spec.ts` (in `frontend/`) | Single E2E spec |
| `wails3 task build` / `wails3 task package` | Production build / package |
| `wails3 generate bindings` | Regenerate TS bindings from Go services |

Frontend deps install automatically via `wails3 dev`/`wails3 task`; manual install is `cd frontend && bun install`. The Vite dev server runs on port 9245 (`WAILS_VITE_PORT`).

## Architecture

**Go backend → TypeScript frontend flow:**

1. Go services live in the root package (e.g. `greetservice.go`) and are registered in `main.go` under `application.Options.Services`.
2. `wails3 generate bindings` (run automatically on build/dev) emits TypeScript bindings into `frontend/bindings/camstuart/talent-hound/`. **Never hand-edit `frontend/bindings/`** — it is regenerated.
3. The SolidJS UI (`frontend/src/`) imports those bindings and calls service methods as async functions; calls travel over the Wails runtime transport.

Adding a backend capability means: write the Go service method → register it in `main.go` (if a new service) → regenerate bindings → import from `frontend/bindings/...` in the UI.

**Persistence (SQLite + GORM):**

- The DB is SQLite via `github.com/glebarez/sqlite` (pure Go, **no CGO**) + GORM. Do not switch to `gorm.io/driver/sqlite` (that one needs CGO).
- GORM models live in `internal/models/`, one file per model (e.g. `initiative.go`). Register new models in `db.Open`'s `AutoMigrate` call (`internal/db/db.go`).
- The DB file lives at `os.UserConfigDir()/talent-hound/talent-hound.db`; the `TALENT_HOUND_DB_PATH` env var overrides it. Playwright sets it to `frontend/.e2e-db/e2e.db` (see `playwright.config.ts`) so E2E runs never touch real data. That database is emptied at the start of every run (`e2e/reset-db.ts`); it used to persist, and the accumulation made the suite slow and unreliable. Specs still use per-run-unique names and scope locators to their own rows, because they run in parallel against one shared backend.
- Go service tests use `db.Open(":memory:")` for an ephemeral database.

Root `Taskfile.yml` dispatches to `build/Taskfile.yml` (common tasks, including server mode) and per-platform Taskfiles in `build/<os>/`. App/product metadata is in `build/config.yml`.

**Ollama:** the app detects a running Ollama at `localhost:11434`, else launches a bundled copy (`ollama/` beside the app binary, endpoint `127.0.0.1:11435`) and kills it on exit — see `internal/platform/ollamamanage.go` and `docs/product/OLLAMA_BUNDLING.md`.

## Testing strategy (three layers)

- **Go unit tests** (`*_test.go`): plain Go tests against services directly.
- **Vitest unit tests** (`frontend/src/*.test.tsx`): jsdom + `@solidjs/testing-library`. These **mock `frontend/bindings/*`** with `vi.mock` — no Go backend runs.
- **Playwright E2E** (`frontend/e2e/`): exercises the **real Go backend** via Wails v3's server build (`-tags server` — a plain HTTP server, no native GUI). `frontend/playwright.config.ts` starts it as the `webServer` with `wails3 task run:server DEV=true` and waits on `http://localhost:8080`.

Binding calls only complete inside the native WebView2 window or the server build's HTTP transport. A plain browser tab pointed at the Vite dev server cannot complete binding calls — don't try to verify backend behavior that way.

## Known environment quirks

- **`jsdom` is pinned to `26.1.0`** in `frontend/package.json`: `jsdom@27+` pulls in an ESM-only transitive dep that crashes Vitest's worker pool (`ERR_REQUIRE_ESM`). Keep the pin until upstream fixes it.
- **Node.js version warning on Windows**: `vite`/`playwright` shims resolve to the system Node (not Bun). If it's older than Vite 8 wants (20.19+/22.12+), a harmless startup warning appears; builds still succeed.
