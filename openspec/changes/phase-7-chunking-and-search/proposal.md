## Why

Phase 6 turned documents into Markdown. Markdown is still one long string, and every later phase needs to point at a *part* of it: a matching assessment must cite the sentence that proves a requirement is met, and a recruiter must be able to find that sentence in the original document without taking the application's word for it. That means evidence has to be cut into pieces that are stable, addressable, and searchable.

Stable is the hard word. If chunk boundaries move between runs, every hash, citation, and cached embedding that referenced them silently points somewhere else. So the chunker is fixed and deterministic: same Markdown and same parameters, same chunks, byte for byte, forever. Embedding-similarity boundaries are explicitly P1 in the PRD.

Retrieval starts here too, with the lexical half. FTS5 was proven available in Phase 1 and smoke-tested at every startup since; this phase finally puts something in it, with the triggers that keep it in step and the rebuild path that fixes it when they have not.

## What Changes

- Add a fixed chunker: headings, list items, and paragraphs first, then sentence segmentation to a target size, with no boundary decision that depends on anything but the text and the parameters.
- Persist chunks with ordinal, character offsets, heading path, token count, content hash, and the chunker name, version, and parameters that produced them.
- Guarantee that a chunk's stored offsets select exactly its stored text in the artifact's Markdown, so a citation can be resolved against the source rather than trusted.
- Add the external-content FTS5 table over chunks, with the triggers that keep it synchronized and an explicit rebuild path.
- Add lexical search over an initiative's evidence, returning citations that resolve to an artifact and a human-readable location.
- Run chunking through the Phase 5 job lifecycle, one artifact per item, so a cancelled run keeps the artifacts it finished and discards the one it did not.
- Discard an artifact's chunks whenever its Markdown changes, so chunks from two chunker versions can never sit side by side.

## Capabilities

### New Capabilities
- `chunking`: how Markdown is cut, what a chunk records, and what determinism means here.
- `lexical-search`: the FTS5 index, its synchronization with chunks, the rebuild path, and what a query is allowed to be.
- `citations`: resolving a retrieved chunk back to an artifact and a location a person can find.

### Modified Capabilities
- `document-extraction`: extracted Markdown is now something else's input, so replacing it invalidates everything derived from it.

## Impact

- `internal/db/migrations.go`: migration 7 — the `chunks` table, the `chunks_fts` external-content index, and its three triggers.
- New `internal/chunk/` (the chunker and its golden tests); new `chunkservice.go` registering the `chunk` job kind; new `searchservice.go`; `extractservice.go` invalidating derived rows.
- `frontend/bindings/` regenerated; a search panel in the workspace that shows a snippet, its heading path, and the artifact it came from.
- Golden chunking tests over headings, nested lists, tables, long paragraphs, abbreviations, Unicode, blank sections, and exact target-size boundaries; FTS synchronization tests across insert, update, delete, rollback, and rebuild; disk-backed tests comparing results either side of a rebuild.
- No embeddings, no ranking beyond FTS5's own, no cross-artifact deduplication. Embeddings are Phase 9; fusion is Phase 15.
