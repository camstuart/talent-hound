## Context

The PRD makes an artifact one *ingestion occurrence*, not one file: two uploads of the same bytes are two artifacts with independent provenance, because filename, source, and capture time are themselves evidence. Everything downstream — chunks, citations, matches, drafts — cites an artifact ID, so that ID must mean exactly one act of ingestion, forever.

## Goals / Non-Goals

**Goals:**
- Exact bytes in, exact bytes out, for text and arbitrary binary alike.
- Provenance that no later operation can change.
- Links that attach and detach without ever touching bytes.
- A visible home for artifacts whose last link went away.

**Non-Goals:**
- No extraction, no sidecar, no chunking, no indexing — Phase 6 and Phase 7.
- No deduplication, ever. The hash is for integrity, not identity.
- No artifact versioning: a changed source is a new artifact (Phase 14's source lifecycle).
- No global deletion. Detach is link-only; Phase 19 enforces the rest.
- No filesystem storage. Bytes live in the database, which is the system of record.

## Decisions

**Bytes live in a `BLOB` column, not on disk beside the database.**
Alternative: files in the data folder with a path in the row. Rejected — two things to keep consistent, two things to back up, two things to get wrong on a half-finished write. SQLite handles blobs of this size fine, the 25 MB limit bounds the row, and "copy the data folder" stays one file. Revisit if artifacts ever get big enough that the whole-database `VACUUM INTO` snapshot hurts.

**One `artifacts` table plus a polymorphic `artifact_links(artifact_id, target_type, target_id)`.**
An artifact attaches to initiatives, candidates, roles, companies, and contacts — five targets today and more later. A column per target type would be five nullable foreign keys and a check that exactly one is set. The polymorphic pair costs a foreign key the database cannot enforce, so the service validates the target exists before linking, and a unique index on the triple stops duplicate links.
`ponytail:` polymorphic link; if the target set ever stops growing, per-type link tables would buy real foreign keys.

**SHA-256 is stored and indexed, but never unique.**
The PRD is explicit that equal bytes are not the same artifact. The hash exists so integrity can be re-checked and so a future phase can *report* "these two artifacts have identical bytes" — never so that one can silently replace the other.

**Provenance is immutable by having no way to change it.**
There is no `UpdateArtifact`: the only mutator is `Rename`, which writes `display_name` and nothing else. Immutability enforced by the absence of a code path beats immutability enforced by a comment.

**Media type is sniffed from the bytes, and the extension is not consulted.**
`http.DetectContentType` on the leading bytes, with a small extension-driven fallback only for types sniffing reports as plain text (Markdown, and text uploaded as `.txt`). When extension and content disagree, the content wins and no error is raised — a `.pdf` that is really text is stored as text, which is the truth about the bytes.

**The original filename is kept verbatim and never used as a path.**
Path-like or Unicode filenames are provenance, not instructions. Nothing in this phase touches the filesystem, so a filename of `../../etc/passwd` is a string in a column. A test asserts it round-trips unchanged rather than being sanitised, because sanitising would corrupt provenance.

**The limit is 25 MiB (26,214,400 bytes), checked before the row is built.**
Exactly at the limit is accepted; one byte over is refused with the limit named. Zero bytes is accepted: an empty file is a real ingestion with real provenance, and refusing it would lose the evidence that it was empty.

**Creating an artifact with its first link is one transaction.**
Otherwise a failed link leaves an orphan the recruiter never asked for. Attaching to nothing is also allowed — that is how the orphan library gets its first residents — but when a target is given, both rows commit together or neither does.

**An orphan is a query, not a state.**
`ListOrphans` is "artifacts with no rows in `artifact_links`". No `is_orphan` column to drift out of step with the links that define it.

## Risks / Trade-offs

- Blobs make the database file grow with every upload, and Phase 2's `VACUUM INTO` snapshot copies all of it before each migration. At PoC volumes this is seconds; it is the reason the 25 MB limit exists.
- Passing bytes across the Wails boundary means base64 in JSON: roughly a third larger on the wire and a full copy in memory on both sides. Acceptable for a local desktop app at 25 MB; a streaming upload path is the upgrade if it ever bites.
- The polymorphic link has no database-level foreign key, so a target deleted outside the service would leave a dangling link. Nothing deletes records yet, and Phase 19 — which introduces deletion — is where the cascade belongs.
- Storing bytes for artifacts that are never extracted costs space with no benefit until Phase 6. That is the point: the bytes are the evidence, extraction is derived.

## Migration Plan

Migration 4 adds `artifacts` and `artifact_links`; no existing table changes, so every current database migrates forward untouched. Rollback is Phase 2's snapshot restore.

## Open Questions

- Should the orphan library be per-workspace or global? Currently global, because an orphan belongs to no initiative by definition.
- Does a pasted-text artifact get a synthetic filename, or an empty one with the display name doing the work? Currently an empty original filename and a recruiter-supplied display name.
