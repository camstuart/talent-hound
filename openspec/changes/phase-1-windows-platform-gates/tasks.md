> **Status:** groups 1–7 are authored and typecheck for Windows (`GOOS=windows go vet -tags windowsgate ./internal/platform/`); only the FTS5 gate has
> actually been executed (passes on macOS with the same CGO-free driver). Every checked box in
> groups 3–7 means "probe and gate test written", not "proved on Windows" — group 8 is the
> real exit gate and is untouched.

## 1. Gate scaffolding

- [x] 1.1 Create `internal/platform/` and add `just gate` (build tag `windowsgate`) and `just gate-model` (build tag `livemodel`) recipes
- [x] 1.2 Add `go vet -tags windowsgate,livemodel ./...` to `just qa` so tagged files stay compilable off-Windows
- [x] 1.3 Confirm `just check` passes unchanged with no network access on the dev machine

## 2. FTS5 gate

- [x] 2.1 Add `internal/platform/fts5.go` with a `CheckFTS5(db)` that creates, inserts, MATCHes, deletes, and rebuilds a virtual table
- [x] 2.2 Add the FTS5 gate test against a disk-backed temp database; assert failure (not skip) when FTS5 is absent
- [ ] 2.3 Run on Windows 11 x64 and record the result

## 3. Sidecar packaging

- [x] 3.1 Pin MarkItDown + Python versions and add the PyInstaller one-dir build task under `build/`; gitignore the output, commit the pin and digest
- [x] 3.2 Add `internal/platform/sidecar.go`: invoke by verified absolute path, one file per process, plugins and network features disabled
- [x] 3.3 Add synthetic PDF and DOCX fixtures with known text (synthetic data only)
- [x] 3.4 Gate test: both fixtures produce non-empty Markdown containing the known text
- [x] 3.5 Gate test: a corrupt input returns non-zero exit plus a diagnostic, parent stays healthy

## 4. Job Object containment

- [x] 4.1 Add `internal/platform/jobobject_windows.go`: create job, set kill-on-close and memory limits, assign process, terminate tree
- [x] 4.2 Add timeout via `exec.CommandContext` + `TerminateJobObject`, and an output-size limit over stdout
- [x] 4.3 Add four fake-sidecar test binaries (hang, spawn grandchild, allocate, flood stdout), built by the test
- [x] 4.4 Gate tests: timeout, grandchild kill, memory limit, output limit — each asserts no surviving process and a distinct retryable failure
- [x] 4.5 Gate test: parent performs a successful invocation after each containment failure

## 5. Volume encryption detection

- [x] 5.1 Add `internal/platform/encryption_windows.go` returning the four-value status; `manage-bde` first, WMI `GetProtectionStatus` fallback, defensive parsing
- [x] 5.2 Gate tests for encrypted and unencrypted volumes on the target laptop
- [x] 5.3 Gate tests for unavailable and permission-denied paths; assert neither can yield `encrypted`

## 6. Credential Manager

- [x] 6.1 Add `internal/platform/credential_windows.go` wrapping `CredWrite`/`CredRead`/`CredDelete` under target name `TalentHound:<purpose>`
- [x] 6.2 Gate test: write → read exact secret → delete → read reports not-found
- [x] 6.3 Gate test: assert the secret appears in neither the database file, the log output, nor the evidence record

## 7. Ollama capability

- [x] 7.1 Add `internal/platform/ollama.go`: chat, JSON-schema-constrained chat, embeddings against `http://localhost:11434/v1`, plus model digest lookup
- [x] 7.2 Live-model gate tests for all three calls; assert JSON validates against the schema and record embedding dimensions
- [ ] 7.3 Capture wall-clock and peak memory per call for each candidate instruct and embedding model on the target laptop

## 8. Evidence and exit gate

- [ ] 8.1 Run `just gate` and `just gate-model` on the Windows 11 x64 target laptop
- [ ] 8.2 Write `docs/product/PHASE1_GATE_EVIDENCE.md`: per-gate pass/fail, machine and OS build, timings, memory, embedding dimensions, model digests
- [ ] 8.3 Record the pinned instruct and embedding model selection with its rationale
- [ ] 8.4 For any failed gate, record the replacement implementation choice within the PRD, or raise an explicit PRD reopen before Phase 2 starts
- [ ] 8.5 Confirm `just check` still passes twice from a clean checkout with no network
