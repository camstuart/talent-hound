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

# ---------- Phase 1 platform gates (see openspec/changes/phase-1-windows-platform-gates) ----------

# Windows-native proofs: FTS5, sidecar, Job Object containment, encryption, credentials.
# Run on the target Windows 11 x64 laptop; needs TH_SIDECAR_EXE (see `just sidecar`).
gate:
    go test -tags windowsgate -v -count=1 ./internal/platform/

# Live local-model proofs against Ollama at http://localhost:11434.
# Override models with TH_INSTRUCT_MODELS / TH_EMBED_MODELS.
gate-model:
    go test -tags livemodel -v -count=1 ./internal/platform/

# Classifier contract against the selected local model (set TH_CLASSIFY_MODEL)
gate-model-classify:
    go test -tags livemodel -v -count=1 -run TestGate .

# The frozen classifier and matching benchmarks against the selected local
# models. Writes a record to docs/product/benchmarks/.
# Set TH_CLASSIFY_MODEL and TH_EMBED_MODEL.
bench:
    go test -tags livemodel -v -count=1 -timeout 240m -run TestBenchmark .

# The PRD's performance budgets that need no model: hybrid retrieval P95 and
# database size at the representative corpus. Runs on any machine, so the same
# command produces the development-machine number and the target-laptop one.
perf:
    go test -tags perf -v -count=1 -timeout 60m -run TestRetrievalAtTheRepresentativeCorpus .

# The Windows laptop acceptance run: every automatable gate, then the manual checklist
laptop-gates:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "== 1. the suite that runs everywhere =="
    just check
    echo
    echo "== 2. Windows-native platform proofs =="
    just gate
    echo
    echo "== 3. the pinned sidecar =="
    just sidecar
    echo
    echo "== 4. the local models answer =="
    just gate-model
    echo
    echo "== 5. the classifier contract against the selected model =="
    just gate-model-classify
    echo
    echo "== 6. the frozen benchmarks, on this machine's timings =="
    just bench
    echo
    echo "== 7. one assessment against the 60 second target =="
    just bench-assess
    echo
    echo "== 8. retrieval and database size at the representative corpus =="
    just perf
    echo
    echo "== 9. package the application and its installer =="
    wails3 task package
    echo
    echo "Everything above is automatable. What is left needs a person:"
    echo
    sed -n '/^## Installer/,$p' docs/product/PHASE20_PACKAGING_EVIDENCE.md \
      | grep -E '^\| [A-Z]' | grep -v '^| Check' \
      | sed 's/|/ /g; s/  */ /g; s/ NOT RUN.*$//; s/^ /  [ ] /'
    echo
    echo "Record each one in docs/product/PHASE20_PACKAGING_EVIDENCE.md, and the"
    echo "real-corpus run in docs/product/POC_ACCEPTANCE.md. A gate nobody wrote"
    echo "down did not happen."

# Measure one assessment against the PRD's 60 second target. Separate from the
# benchmark, which never calls the generate model.
# Set TH_CLASSIFY_MODEL and TH_GENERATE_MODEL.
bench-assess:
    go test -tags livemodel -v -count=1 -timeout 60m -run TestAssessOneMatch .

# Diagnose the matching ranking on the tuning corpus — never the frozen one.
# Set TH_CLASSIFY_MODEL and TH_EMBED_MODEL.
tune-matching:
    go test -tags livemodel -v -count=1 -timeout 120m -run TestTuneMatching .

# Score the classifier on the tuning corpus. Set TH_CLASSIFY_MODEL.
tune-classify:
    go test -tags livemodel -v -count=1 -timeout 90m -run TestTuneClassifier .

# Choose the retrieval constants on the tuning corpus — never the frozen one.
# Set TH_CLASSIFY_MODEL and TH_EMBED_MODEL.
tune:
    go test -tags livemodel -v -count=1 -timeout 120m -run TestTuneRetrieval .

# Build the pinned MarkItDown PyInstaller one-dir sidecar (Windows only)
sidecar:
    powershell -NoProfile -ExecutionPolicy Bypass -File build/sidecar/build.ps1

# Regenerate the synthetic extraction fixtures
fixtures:
    python3 internal/platform/testdata/docs/generate.py

# ---------- QA: linting & static analysis ----------

# Run every lint / static-analysis tool
qa: lint-go sec vuln typecheck lint-ts dupes vet-gates

# Keep the build-tagged gate code compiling even though routine runs skip it.
#
# On non-Windows hosts the *_windows.go files are skipped by filename, so the
# credential store, the BitLocker check, and the job-object containment are
# never compiled by a routine run — a typo in them would wait until packaging
# day. This vets every package for Windows and builds both binaries that ship.
vet-gates:
    go vet -tags windowsgate ./internal/platform/
    go vet -tags livemodel ./internal/platform/
    go vet -tags livemodel .
    GOOS=windows go vet -tags windowsgate .
    GOOS=windows go vet ./...
    GOOS=windows go build -o /dev/null .
    GOOS=windows go build -tags server -o /dev/null .

# Go meta-linter (staticcheck, govet, errcheck, revive, gocritic, gosec, gofmt/goimports, ...)
lint-go:
    golangci-lint run

# Go security scanner (standalone gosec pass)
sec:
    gosec -quiet -exclude-dir=build -exclude-dir=frontend -exclude-dir=testdata ./...

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
