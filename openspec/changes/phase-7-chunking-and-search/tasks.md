## 1. Schema

- [x] 1.1 Migration 7: `chunks` with artifact, ordinal, text, offsets, heading path, token count, hash, chunker name/version/params
- [x] 1.2 Unique index on (artifact, ordinal); index on hash
- [x] 1.3 `chunks_fts` as an external-content FTS5 table over `chunks`
- [x] 1.4 After-insert, after-delete, and after-update triggers keeping the index in step

## 2. Chunker

- [x] 2.1 `internal/chunk`: block scanner for headings, fenced code, tables, list items with continuations, and paragraphs
- [x] 2.2 Heading path tracking, with a heading taking the path that includes itself
- [x] 2.3 Greedy packing of consecutive blocks while under the target and within one heading path
- [x] 2.4 Sentence segmentation for a block over the maximum, with abbreviations, initials, and decimals excluded
- [x] 2.5 Token count as whitespace-separated words, with the method recorded in the parameters
- [x] 2.6 Content hash of the chunk text; ordinals contiguous from zero
- [x] 2.7 The invariant asserted in the chunker itself: `markdown[start:end] == text`

## 3. Chunking jobs

- [x] 3.1 `chunkservice.go`: `Chunk(artifactID, initiativeID)` and `ChunkAll(initiativeID)` enqueue a `chunk` job with one item per artifact
- [x] 3.2 Worker compute half chunks one artifact; commit half deletes its old chunks and inserts the new ones
- [x] 3.3 An artifact that is not extracted fails its item with a code
- [x] 3.4 `List(artifactID)` and a per-initiative count for the UI

## 4. Search and citations

- [x] 4.1 `searchservice.go`: `Search(initiativeID, query, limit)` over `chunks_fts`, scoped to the initiative's linked artifacts
- [x] 4.2 Query terms quoted as FTS5 string literals and ANDed; empty query returns nothing
- [x] 4.3 `Cite(chunkID)` resolves to artifact, heading path, ordinal, offsets, and text, verified against the Markdown
- [x] 4.4 A citation whose offsets no longer select its text fails rather than returning the wrong words
- [x] 4.5 `Rebuild()` runs the FTS5 rebuild command

## 5. Invalidation

- [x] 5.1 Recording an extraction result deletes the artifact's chunks in the same transaction
- [x] 5.2 Re-chunking is delete-then-insert for the whole artifact, never a merge

## 6. Frontend

- [x] 6.1 Search panel in the workspace: query, results with artifact name and heading path, snippet in a `<pre>`
- [x] 6.2 An action that indexes the workspace's extracted artifacts, showing the chunk count
- [x] 6.3 A result resolves to its citation and shows the location and cited text
- [x] 6.4 Backend messages surfaced verbatim, as elsewhere

## 7. Tests

- [x] 7.1 Golden chunking over headings, nested lists, tables, long paragraphs, abbreviations, Unicode, blank sections, and exact target-size boundaries
- [x] 7.2 Repeated chunking of identical input produces identical chunks and hashes
- [x] 7.3 Every stored offset selects the cited source text; every heading path resolves
- [x] 7.4 FTS insert, update, delete, rollback, and rebuild stay synchronized with chunks
- [x] 7.5 Queries with punctuation, quoting, Unicode, empty input, common terms, and injection-shaped strings
- [x] 7.6 Cancelling a chunking job keeps committed artifacts and leaves the current one with no chunks
- [x] 7.7 Re-extraction removes the artifact's chunks and their index entries
- [x] 7.8 A stale citation fails rather than returning text from the wrong offsets
- [x] 7.9 Disk-backed test comparing search results before and after a rebuild
- [x] 7.10 Search from one initiative does not return another's evidence
- [x] 7.11 Vitest over the search panel: query, results, snippet as literal text, citation view
- [x] 7.12 Playwright: paste, extract, index, search, and resolve a citation through the real backend
- [x] 7.13 Fixtures are synthetic only — no real candidate information anywhere

## 8. Exit gate

- [x] 8.1 A supported artifact can be extracted, chunked, searched, and cited end to end
- [x] 8.2 FTS integrity survives rebuild and cancellation
- [x] 8.3 `just check` passes, with the standing `just vuln` toolchain advisories unchanged
