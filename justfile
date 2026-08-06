# justfile — task runner for Talent Hound
# On Windows, recipes run under cmd.exe; keep them to simple single commands.
set windows-shell := ["cmd.exe", "/c"]

# List available recipes
default:
    @just --list

# Run the app in development mode (desktop window, hot reload)
dev:
    wails3 dev

# ---------- Tests ----------

# Run all test layers: Go, Vitest unit (mocked bindings), Playwright E2E (real backend)
test: test-go test-unit test-e2e

# Go unit tests
test-go:
    go test ./...

# TypeScript unit tests (Vitest, bindings mocked)
[working-directory: 'frontend']
test-unit:
    bun run test:unit

# Playwright E2E tests (drives the -tags server build with the real Go backend)
[working-directory: 'frontend']
test-e2e:
    bun run test:e2e

# ---------- QA: linting & static analysis ----------

# Run every lint / static-analysis tool
qa: lint-go sec vuln typecheck lint-ts dupes

# Go meta-linter (staticcheck, govet, errcheck, revive, gocritic, gosec, gofmt/goimports, ...)
lint-go:
    golangci-lint run

# Go security scanner (standalone gosec pass)
sec:
    gosec -quiet -exclude-dir=build -exclude-dir=frontend ./...

# Known-vulnerability scan of Go dependencies (call-graph aware)
vuln:
    govulncheck ./...

# TypeScript type-checking (tsc --noEmit over src + generated bindings)
[working-directory: 'frontend']
typecheck:
    bun run typecheck

# TS/JS linter (oxlint)
[working-directory: 'frontend']
lint-ts:
    bun run lint

# Copy-paste / duplicate-code detection across Go + TS (jscpd)
[working-directory: 'frontend']
dupes:
    bun run dupes

# QA plus all tests — the full pre-commit gate
check: qa test
