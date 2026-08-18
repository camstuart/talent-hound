## Context

Talent Hound today is a Wails v3 + SolidJS shell with GORM `AutoMigrate` over pure-Go SQLite (`github.com/glebarez/sqlite`, no CGO). The PRD commits to six Windows-native mechanisms that nothing in the repo exercises yet: FTS5 in that CGO-free driver, a bundled MarkItDown PyInstaller sidecar, Job Object containment of that sidecar, BitLocker/Device Encryption gating, Windows Credential Manager, and local Ollama chat/embeddings on a target Windows 11 x64 laptop.

Phase 0 supplies the fixture kit and test labels; Phase 2 replaces `AutoMigrate` with explicit migrations and adds the startup FTS5 smoke test. This phase sits between them and exists only to convert assumptions into evidence.

## Goals / Non-Goals

**Goals:**
- One executable proof per mechanism, failing loudly rather than skipping when the mechanism is missing.
- Evidence recorded in-repo: pass/fail per gate, model identity, dimensions, wall-clock, peak memory.
- Keep only code the production path will reuse (the probe functions themselves become the startup smoke test, the encryption gate, the credential accessor, the sidecar runner).
- Default `just check` stays green on any OS with no network.

**Non-Goals:**
- No platform abstraction layer, no interface-per-mechanism, no OS plugin registry — there is exactly one target OS.
- No product UI, no service wiring into `main.go`.
- No exploit sandboxing of the sidecar (PRD accepts user-permission isolation for the PoC).
- No chunking, retrieval, or profile work — those are later phases.

## Decisions

**Probes are Go tests plus the small production functions they call, in `internal/platform/`.**
Alternative: standalone `cmd/` spike binaries. Rejected — spikes get deleted and re-written; a test that runs in CI on the Windows laptop is the same work and survives. `internal/platform/` holds one file per mechanism (`fts5.go`, `sidecar.go`, `jobobject_windows.go`, `encryption_windows.go`, `credential_windows.go`, `ollama.go`) with `_windows` suffixes doing the OS gating — no build-tag matrix beyond Go's own filename rules.

**Gate suites are separated by Go build tag, not by skip-if-missing.**
`//go:build windowsgate` and `//go:build livemodel` on the proof test files. A missing mechanism must fail the gate; `t.Skip` on error would make a broken gate look green. Routine `go test ./...` never compiles them, so `just check` stays platform- and network-independent. `just gate` (Windows) and `just gate-model` run them.

**Job Object containment uses `golang.org/x/sys/windows` directly.**
Alternative: a third-party sandbox wrapper. Rejected — the needed surface is `CreateJobObject`, `SetInformationJobObject` with `JOBOBJECT_EXTENDED_LIMIT_INFORMATION` (`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` + `PROCESS_MEMORY_LIMIT`), `AssignProcessToJobObject`, and `TerminateJobObject`. Timeout is `exec.CommandContext` + explicit `TerminateJobObject`; output limit is an `io.LimitedReader` over stdout that cancels on overflow. `x/sys` is already an indirect dependency.

**Misbehaving sidecars are fake Go binaries, not real malformed documents.**
One tiny `testdata` main with four modes — hang, spawn-grandchild, allocate, flood-stdout — built by the test via `go build`. Deterministic, no Python needed, and each maps 1:1 to a containment scenario.

**Encryption detection shells out to `manage-bde -status <volume>` and falls back to a PowerShell CIM query of `Win32_EncryptableVolume.ProtectionStatus`.**
Alternative: a COM/WMI client library. Rejected — `manage-bde` is present on Windows 11 and parses easily, and where it is absent (some Device Encryption SKUs) one PowerShell `Get-CimInstance` call is the same query with no new dependency. Outcomes are a four-value enum (`encrypted`, `unencrypted`, `unavailable`, `permission-denied`) — critically, no error path may collapse into `encrypted`.

**Credential access uses the Win32 `CredWrite`/`CredRead`/`CredDelete` calls via `x/sys/windows` syscalls.**
No new dependency for three syscalls. Target name is `TalentHound:<purpose>`. The secret is `[]byte` and never formatted into an error, log line, or evidence file.

**Ollama probes speak plain HTTP against the OpenAI-compatible `/v1` paths with `net/http` + `encoding/json`.**
No SDK. Three requests (chat, chat with `response_format` JSON schema, embeddings) and a `/api/show` call for the model digest. Timings and `runtime`/Windows process memory readings are collected around the calls and written to the evidence file.

**Evidence lives at `docs/product/PHASE1_GATE_EVIDENCE.md`, written by hand from test output.**
Alternative: tests emit the file. Rejected for now — the record includes machine identity and a human decision on model selection; auto-generation is a build system for one document.

## Risks / Trade-offs

- FTS5 absent from the pure-Go driver → prototype an alternative before Phase 2 (bleve-style external index, or a CGO build) and reopen the PRD's "no CGO" constraint; this is precisely why the gate runs first.
- Job Object semantics differ under a debugger or an existing job (nested jobs) → tests assert on process liveness after termination, not on API return codes alone.
- `manage-bde` output is localized and version-dependent → parse the protection-status field defensively, and treat any parse failure as `unavailable`.
- Live-model timings vary with laptop thermal state and other load → record conditions with the numbers; these guide model selection, not acceptance thresholds (Phase 21 owns benchmarks).
- PyInstaller one-dir sidecar bloats the repo/build → the package is a build artifact, gitignored; only its version/digest pin is committed.
- Gate tests behind build tags can rot unnoticed → `just qa` compiles them with `go vet -tags windowsgate,livemodel` so they at least stay buildable.

## Migration Plan

Additive only; nothing existing changes behavior. Phase 2 starts once the evidence record shows all gates passing. Rollback is deleting `internal/platform/` — no production path depends on it yet.

## Open Questions

- Which instruct and embedding models are pinned? Answered by this phase's measurements, not before it.
- Does Device Encryption (non-BitLocker SKU) report through `manage-bde` on the actual target laptop, or is the WMI fallback mandatory?
- Exact Job Object memory and output limits — start at the PRD's 25 MB input / 10 MB extracted-Markdown envelope and record what the real sidecar needs.
