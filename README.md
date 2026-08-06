# Talent Hound

A local-first AI desktop app built with Wails v3 (Go backend) and SolidJS (TypeScript frontend).

## Prerequisites

- Go 1.24+
- [Bun](https://bun.sh) — the only JS package manager/runtime used in this repo
- [Wails v3 CLI](https://v3.wails.io): `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

Run `wails3 doctor` to verify your environment.

## Commands

| Command | Description |
| --- | --- |
| `wails3 dev` | Run the app in development mode (desktop window, hot reload) |
| `go test ./...` | Run Go unit tests |
| `bun run test:unit` (in `frontend/`) | Run TypeScript unit tests (Vitest) |
| `bun run test:e2e` (in `frontend/`) | Run Playwright E2E tests |
| `wails3 task test` | Run all three test layers in sequence |

Frontend dependencies are installed automatically by `wails3 dev`/`wails3 task` via Bun. To install manually: `cd frontend && bun install`.

## Project layout

- `main.go` — Wails application entry point and window setup
- `greetservice.go` — example Go service (`GreetService.Greet`) bound to the frontend
- `frontend/src/` — SolidJS UI (`App.tsx`, `main.tsx`)
- `frontend/bindings/` — TypeScript bindings generated from Go services by `wails3 generate bindings`. **Never hand-edit** — regenerated automatically on build/dev.
- `frontend/e2e/` — Playwright E2E specs
- `Taskfile.yml` / `build/Taskfile.yml` — Wails v3 Taskfile-based build system

## E2E testing strategy

Playwright drives the app through Wails v3's **server build**: a build tagged `-tags server` that runs the app as a plain HTTP server (no native GUI) with the real Go backend attached, serving the built frontend and handling bound-service calls over HTTP (`/wails/runtime`) and events over WebSocket (`/wails/events`).

`frontend/playwright.config.ts` starts this via `wails3 task run:server DEV=true` (from the repo root) as Playwright's `webServer`, and waits on `http://localhost:8080`. This means E2E tests exercise the actual Go `GreetService`, not a mock — unlike the Vitest unit tests, which mock `frontend/bindings/*` to run without a Go backend.

Plain browser tabs (e.g. `wails3 dev`'s Vite dev server viewed directly in a browser) cannot complete binding calls — that scheme only works inside the native WebView2 window or the server build's HTTP transport.

## Known environment quirks

- **Node.js version warning**: `vite`/`playwright`'s shell shims resolve to the system Node.js on PATH (not Bun's runtime) on Windows. If that Node is older than what Vite 8 wants (20.19+/22.12+), you'll see a harmless startup warning; the build still succeeds. Upgrade Node if you'd rather not see it.
- **`jsdom` pinned to `26.1.0`**: `jsdom@27+` depends on `html-encoding-sniffer@6`, which `require()`s an ESM-only transitive dependency (`@exodus/bytes`) and crashes Vitest's worker pool (`ERR_REQUIRE_ESM`). Keep `frontend/package.json`'s `jsdom` pin until upstream fixes this.
