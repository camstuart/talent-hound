## Why

Every claim the app will later make — a profile aspect, a match result, a factual sentence in a draft — has to be traceable to the bytes it came from. That means the bytes themselves, and the provenance around them, must be stored before any pipeline reads them: an artifact whose filename, source, or capture time can drift is not evidence. Phase 3 gave us the records an artifact attaches to; this phase gives us the artifacts and the link semantics that keep them shared rather than copied.

## What Changes

- Persist original bytes as a SQLite BLOB alongside immutable provenance: original filename, detected media type, byte length, SHA-256, source, and capture time.
- Keep an editable display name separate from the immutable original filename.
- Link artifacts to initiatives and records through a link table, without copying bytes.
- Retain SHA-256 for integrity while deliberately not deduplicating equal bytes.
- Enforce the 25 MB per-file input limit.
- Add the visible orphaned-artifact library for artifacts whose last link was detached.

## Capabilities

### New Capabilities
- `artifact-storage`: one immutable artifact per ingestion — bytes, provenance, the editable display name, and the size limit.
- `artifact-links`: attaching an artifact to initiatives and records, detaching one link, and the orphan library.

### Modified Capabilities
<!-- none: this phase adds a migration to the Phase 2 list and does not change the behaviour any existing spec describes -->

## Impact

- `internal/db/migrations.go`: migration 4 — `artifacts` and `artifact_links`.
- New `internal/models/artifact.go`; new `artifactservice.go` registered in `main.go`.
- `frontend/bindings/` regenerated; the Context area gains an artifact list, an upload control, display-name editing, detach, and the orphan library.
- New Go tests over bytes, boundaries, provenance, and link semantics; new Vitest and Playwright coverage.
- Extraction is **not** in scope: no sidecar, no extracted text, no extraction state columns. Those arrive with Phase 6, which needs them.
- Global artifact deletion stays disabled; detach is link-only. Phase 19 owns every deletion invariant.
