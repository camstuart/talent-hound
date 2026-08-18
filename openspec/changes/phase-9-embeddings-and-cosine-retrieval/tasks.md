## 1. Schema

- [x] 1.1 Migration 9: `embedding_spaces` with endpoint, model, digest, assignment revision, dimensions, metric, normalization
- [x] 1.2 Unique index across the whole space identity, so a duplicate space cannot exist
- [x] 1.3 `embeddings` with space, owner kind, owner id, dimensions, and the vector blob
- [x] 1.4 Unique index on (space, owner kind, owner id) — a retrieval unit has at most one vector per space

## 2. Vector arithmetic

- [x] 2.1 `internal/vector`: little-endian float32 encode and decode
- [x] 2.2 Decode refuses a length that is not a multiple of four, and a count that is not the expected dimensions
- [x] 2.3 Exact cosine in float64 accumulators, clamped to [-1, 1]
- [x] 2.4 Refuse zero-magnitude and non-finite vectors rather than scoring them

## 3. Spaces

- [x] 3.1 Resolve the current space from the embed assignment, creating it on first use with the learned dimensions
- [x] 3.2 A new assignment revision yields a new space; older vectors are retained and excluded
- [x] 3.3 Coverage: embedded and outstanding counts for an initiative in the current space

## 4. Embedding

- [x] 4.1 `embedservice.go`: an `embed` job kind, one retrieval unit per item
- [x] 4.2 Compute half calls the endpoint; commit half upserts the vector — nothing partial
- [x] 4.3 Coded failure reasons for endpoint failure, degenerate vectors, and dimension mismatch
- [x] 4.4 `chunkservice.go` deletes vectors of replaced chunks in the replacing transaction

## 5. Retrieval

- [x] 5.1 `SemanticSearch(initiative, query, limit)` embedding the query and scanning within one space
- [x] 5.2 Scoped by initiative and space in SQL before anything is scored
- [x] 5.3 Deterministic order: score descending, identifier ascending
- [x] 5.4 Results resolve to citations through the Phase 7 verification path
- [x] 5.5 No exported way to compare two arbitrary vectors

## 6. Frontend

- [x] 6.1 A semantic search control beside the lexical one, naming the space that answered
- [x] 6.2 An embed action reporting coverage and what is outstanding
- [x] 6.3 Backend messages surfaced verbatim, as elsewhere

## 7. Tests

- [x] 7.1 Round-trip over known bit patterns; wrong lengths and wrong dimensions refused
- [x] 7.2 Cosine against a hand-computed oracle: equal, orthogonal, opposite, scaled, zero, NaN, infinite
- [x] 7.3 Tied scores return a deterministic order; a repeated search is identical
- [x] 7.4 Two spaces are never merged or compared; a model change strands the old vectors
- [x] 7.5 A configuration change creates a space and makes prior derived data non-current
- [x] 7.6 A fake cloud endpoint receives zero candidate embedding calls under every configuration
- [x] 7.7 Cancellation and provider failure leave no partial vector for the item in flight
- [x] 7.8 Exact-scan timings at increasing corpus sizes, printed as evidence lines
- [x] 7.9 Vitest over the semantic search panel; Playwright over the real backend
- [x] 7.10 Fixtures are synthetic only — no real candidate information anywhere

## 8. Exit gate

- [x] 8.1 Semantic lookup is correct, deterministic, local for candidate content, and measured at representative scale
- [x] 8.2 `just check` passes in full — including `just vuln`, which the go1.26.6 toolchain pin turned green after eight phases of standing stdlib advisories
