## 1. Schema

- [x] 1.1 Migration 4: `artifacts` (bytes BLOB, original filename, media type, byte length, SHA-256, source, captured at, display name) and `artifact_links` (artifact, target type, target ID)
- [x] 1.2 Unique index on (artifact, target type, target ID); index on SHA-256 for integrity lookups, deliberately not unique
- [x] 1.3 No extraction columns — Phase 6 adds them with the code that writes them

## 2. Model and service

- [x] 2.1 `internal/models/artifact.go`: `Artifact` and `ArtifactLink`, with the link target types as a validated enum
- [x] 2.2 `artifactservice.go`: `Create` (bytes, filename, source, optional target), `Get`, `Bytes`, `List`, `ListOrphans`, `Rename`, `Link`, `Detach`, `Links`
- [x] 2.3 No `Update`: provenance is immutable because there is no code path that writes it
- [x] 2.4 Media type sniffed from the bytes, with an extension fallback only where sniffing reports plain text
- [x] 2.5 25 MiB limit checked before the row is built; zero bytes accepted
- [x] 2.6 Create-with-target runs artifact and first link in one transaction
- [x] 2.7 Link target existence checked against the real table for each target type
- [x] 2.8 Register the service in `main.go` and regenerate bindings

## 3. Frontend

- [x] 3.1 Artifact list for the open initiative, with upload and pasted-text ingestion
- [x] 3.2 Display-name editing in place
- [x] 3.3 Detach control, stating that it removes one link and keeps the bytes; orphans offer "attach here" to undo it
- [x] 3.4 Orphan library listing artifacts with no links
- [x] 3.5 Backend messages surfaced verbatim, as elsewhere

## 4. Tests

- [x] 4.1 Exact byte round-trip for plain text, Markdown, PDF, DOCX, and arbitrary binary (the PDF/DOCX fixtures are headers, not real documents — this phase stores bytes and never opens them)
- [x] 4.2 Two identical ingestions produce two IDs with independent provenance
- [x] 4.3 Boundaries: zero bytes, exactly 25 MB, one byte over
- [x] 4.4 MIME/extension disagreement, Unicode filenames, path-like filenames, duplicate display names, missing filename
- [x] 4.5 Renaming leaves original filename, hash, media type, source, capture time, and bytes untouched
- [x] 4.6 Detach preserves bytes and other links; last detach produces a visible orphan
- [x] 4.7 Rollback leaves neither a half-created artifact nor a dangling link
- [x] 4.8 Vitest over the artifact panel: upload, rename, detach, orphan list, error display
- [x] 4.9 Playwright uploads, views, renames, detaches, and finds an orphan through the real backend
- [x] 4.10 Fixtures are synthetic only — no real candidate information anywhere

## 5. Exit gate

- [x] 5.1 FR-03's storage and link behaviour passes; global deletion remains disabled
- [x] 5.2 `just check` passes: `qa` plus all three test layers (`just vuln` still fails on the pre-existing Go stdlib advisories that need a go1.26.6 toolchain — unchanged by this phase)
