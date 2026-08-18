## 1. Job lifecycle change

- [x] 1.1 Split `JobWorker` into a compute half returning a `JobCommit` closure and the runner's transaction
- [x] 1.2 Runner: compute outside any transaction, then commit the closure and the count increment inside one
- [x] 1.3 Update the demo worker and every Phase 5 test to the new signature, with no change to their assertions

## 2. Schema

- [x] 2.1 Migration 6: `extraction_state`, `extraction_error`, `extractor`, `extractor_version`, `markdown` on `artifacts`
- [x] 2.2 Triggers on the state vocabulary and the reason-code shape, for artifacts and for jobs — a CHECK on an ALTER-added column is never evaluated, and the Phase 5 GLOB pattern admitted any lowercase sentence
- [x] 2.3 Artifact listings and reads that do not need it omit `markdown`, as they already omit `bytes`

## 3. Sidecar verification

- [x] 3.1 `internal/extract`: resolve the configured sidecar path; absolute, present, a regular file
- [x] 3.2 Probe `--version` under a short timeout and compare with the pinned version
- [x] 3.3 Cache the outcome at startup; re-stat the verified path before any byte is materialized
- [x] 3.4 Verification failure yields `sidecar_missing` / `sidecar_version` and leaves the rest of the app running

## 4. Staging and sweep

- [x] 4.1 Create `<data folder>/extract/<random hex>/` at 0700 with a file named only `input<ext>`
- [x] 4.2 Remove the directory on every exit path, including panic and cancellation
- [x] 4.3 Sweep every child of the staging root at startup, touching nothing else

## 5. Extraction

- [x] 5.1 `extractservice.go`: `Extract(artifactID, initiativeID)` enqueues an `extract` job with `{"artifactId": N}`, owned by the workspace that asked
- [x] 5.2 Register the `extract` worker with the job service; one artifact per job
- [x] 5.3 Text and Markdown bypass the sidecar; extractor recorded as the native path
- [x] 5.4 PDF and DOCX run through `platform.ExtractMarkdown` with the Phase 1 limits
- [x] 5.5 Map platform errors to codes; empty output becomes `extract_empty`; stderr is discarded, never stored or logged
- [x] 5.6 Enforce the 10 MB Markdown cap as a failure, never a truncation
- [x] 5.7 Commit the result in the job's commit half; a retry resets the artifact to pending first

## 6. Fake sidecar

- [x] 6.1 `internal/fakesidecar`: version, ok, hang, flood, child, fail, empty modes
- [x] 6.2 Build it once per test run into a temporary directory

## 7. Frontend

- [x] 7.1 Artifacts panel shows extraction state and the reason code when it failed
- [x] 7.2 Extract action for pending and failed artifacts; nothing for the wrong state
- [x] 7.3 Extracted Markdown viewed as literal text in a `<pre>`, never rendered
- [x] 7.4 Backend messages surfaced verbatim, as elsewhere

## 8. Tests

- [x] 8.1 Text and Markdown extract with no process started and no file written
- [x] 8.2 Unsupported media type fails with `unsupported_type` before any process starts
- [x] 8.3 Relative, missing, and version-mismatched sidecars are rejected before a directory exists
- [x] 8.4 A sidecar removed after startup fails the extraction before the document is written
- [x] 8.5 Output flood, non-zero exit, and empty output each produce their code and leave the parent healthy; the timeout path is driven through cancellation, since the production limit is two minutes
- [x] 8.6 A spawned child does not outlive the run
- [x] 8.7 Staging path contains no candidate name, display name, or original filename
- [x] 8.8 Success, failure, and cancellation each leave no staging directory; startup sweeps what a crash left
- [x] 8.9 Extraction reason column refuses free text
- [x] 8.10 Markdown containing scripts, prompt injection, control characters, and hostile links round-trips byte for byte
- [x] 8.11 A slow item no longer blocks another writer past the busy timeout
- [x] 8.12 Vitest over the artifacts panel: state, reason code, extract action, and the plain-text view
- [x] 8.13 Playwright extracts a text artifact through the real backend and shows the result
- [x] 8.14 Fixtures are synthetic only — no real candidate information anywhere

## 9. Windows gate (written and tagged; unrun — needs the target laptop)

- [ ] 9.1 Golden PDF and DOCX through the packaged MarkItDown preserve headings, lists, tables, and Unicode
- [ ] 9.2 Job Object memory cap terminates a memory-hungry tree (Phase 1 gate) — the mapping to `extract_memory` is unit-tested off Windows
- [ ] 9.3 Another ordinary local user cannot read a staging directory
- [ ] 9.4 Native configuration inspection and a network-attempt fixture prove plugins and network features are off

## 10. Exit gate

- [x] 10.1 Supported documents become Markdown through the contract, with every off-Windows failure path proven
- [x] 10.2 `just check` passes, with the standing `just vuln` toolchain advisories unchanged
- [x] 10.3 The Windows-only proofs are written, tagged, and recorded as unrun alongside the Phase 1 evidence — and the `windowsgate` build, which could not compile at all because a Phase 1 test package imported its way into a cycle, now does
