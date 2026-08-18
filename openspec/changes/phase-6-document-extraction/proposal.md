## Why

An artifact is bytes until something reads it. Everything the product does after this — chunking, citations, retrieval, profiles, assessments — starts from Markdown, and the two formats a recruiter actually receives, PDF and DOCX, cannot be read in Go without either a heavyweight parser or a document library with a long history of parser bugs. The PRD's answer is a bundled MarkItDown sidecar, one file per process, contained by the limits Phase 1 proved on Windows. This phase turns that proof into a feature: verified invocation, temporary data that cannot outlive the run, failures that say why without saying what, and output treated as text a stranger wrote.

It also collects a debt. Phase 5 noted that a worker holds the SQLite write lock for as long as its item runs, and marked the split as an upgrade path. Extraction is a two-minute subprocess. The upgrade path is now the road.

## What Changes

- Verify the sidecar once at startup — absolute path, present, a file, pinned version — and refuse extraction with a coded reason when it is not, before any document byte is written to disk.
- Materialize one input into a randomly named, current-user-only directory inside the data folder, run one process per file under the Phase 1 containment limits, and remove the directory afterwards whatever happened.
- Sweep abandoned extraction directories at startup, so a crash costs a restart rather than a plaintext resume on disk.
- Bypass the sidecar entirely for plain text and Markdown: no subprocess, no temporary file.
- Store extracted Markdown on the artifact with its extractor name and version, capped at 10 MB, as untrusted text that is displayed but never rendered or executed.
- Record extraction failures as short codes — timeout, memory, output cap, unsupported type, sidecar missing or mismatched — never the sidecar's own words, which quote the document.
- Run extraction through the Phase 5 job lifecycle: one artifact per job, cancellable, retryable.
- Split the job worker into a compute half and a commit half, so slow work no longer runs inside the transaction that holds SQLite's single writer.

## Capabilities

### New Capabilities
- `document-extraction`: which artifacts extract, how, what is stored, and what a failure is allowed to say.
- `extraction-isolation`: sidecar verification, containment, and the life and death of temporary plaintext.

### Modified Capabilities
- `job-lifecycle`: a worker computes outside the transaction and commits inside it, so an item's duration no longer blocks every other writer.

## Impact

- `internal/db/migrations.go`: migration 6 — extraction columns on `artifacts`.
- New `internal/extract/` (sidecar verification, staging directories, sweep); new `extractservice.go` registering the `extract` job kind; `jobservice.go` worker signature split; `main.go` wiring.
- `frontend/bindings/` regenerated; the artifacts panel gains extraction state, an extract action, and a plain-text view of the result.
- New Go tests driven by a fake sidecar that succeeds, hangs, floods stdout, spawns children, and exits non-zero; new Vitest and Playwright coverage.
- Windows-only proofs — Job Object memory limits, the packaged MarkItDown build, and the permission check that another local user cannot read a staging directory — stay behind the `windowsgate` tag with the rest of the Phase 1 evidence, unrun until the target laptop is available.
- No OCR, no batch mode, no re-extraction on a version bump. One file, one process, one result.
