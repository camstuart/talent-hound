## Why

The PoC's whole architecture rests on six unproven Windows-native mechanisms: FTS5 in the CGO-free SQLite build, the packaged MarkItDown sidecar, Windows Job Object containment, volume-encryption detection, Windows Credential Manager, and Ollama's OpenAI-compatible surface on the target laptop. If any one of them does not hold, the PRD's implementation choices change — so they must be proven before Phase 2 builds product flows on top of them.

## What Changes

- Add executable probes (Go tests, Windows-tagged where native) that exercise each mechanism end to end and fail loudly when the mechanism is unavailable.
- Package and invoke the pinned MarkItDown PyInstaller one-dir sidecar from the repo's build outputs; convert a synthetic PDF and DOCX to non-empty Markdown.
- Prove Job Object timeout, memory, and process-tree kill against deliberately misbehaving fake sidecars (hang, spawn child, allocate, flood stdout).
- Prove BitLocker / Device Encryption detection returns distinct results for encrypted, unencrypted, unavailable, and permission-denied volumes.
- Prove Credential Manager create/read/delete round-trip, and that the secret never reaches the database or logs.
- Prove Ollama chat, constrained-JSON chat, and embeddings; record embedding dimensions, model digest/revision, memory use, and wall-clock timings.
- Record the resulting model selection and any changed implementation choices as decisions in the repo.
- No product features, no general platform abstraction layer. Only code the production path will reuse survives this phase.

## Capabilities

### New Capabilities
- `platform-gates`: Windows-native preflight proofs and their recorded evidence — FTS5 support, sidecar extraction, subprocess containment, volume-encryption detection, credential storage, and local model capability.

### Modified Capabilities
<!-- none: no existing specs -->

## Impact

- New `internal/platform/` probe code (Windows build tags) and `internal/platform/*_test.go`.
- New packaged sidecar under `build/` plus its pin/version record; build task to produce it.
- New fake-sidecar test binaries for containment tests.
- New decision record in `docs/` capturing model selection and measured results.
- Test labels: routine suites must pass with no network and no Windows; Windows-native and live-Ollama proofs run under explicit labels.
- Blocks Phase 2 (migrations) until every gate passes on Windows 11 x64.
