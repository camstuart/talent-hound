## Why

Phase 7 made evidence addressable and searchable by word. Words are not enough: a role asking for "distributed systems experience" must find a CV that says "designed and operated a multi-region event pipeline", and no lexical index will ever connect those two. Matching, shortlisting, and assessment — Phases 15 through 17 — all rest on being able to compare meaning.

The PRD is equally clear about two constraints. Candidate content is embedded locally, always, under every configuration; and no vector extension is added unless exact scanning is measured and found wanting. Both point at the same shape: little-endian float32 vectors in ordinary SQLite columns, and a cosine scan written in Go.

The subtle risk is not performance, it is meaning. A vector is a number only relative to the model that produced it. Two vectors from two models, or from the same model at two endpoint configurations, are not comparable — and nothing about their bytes says so. So the identity of the embedding space is stored beside every vector and enforced at every comparison, rather than remembered by a convention nobody will honour in Phase 16.

## What Changes

- Add embedding spaces as first-class rows: endpoint, model, digest, the model assignment revision that produced them, dimensions, metric, and normalization. A space is identified by that combination and nothing else.
- Store vectors as little-endian float32 blobs alongside the space that gives them meaning, with the byte length checked against the space's dimensions on every read and write.
- Embed retrieval units of distinct kinds — source chunks now, Profile Aspects when Phase 10 introduces them — under one storage shape, so retrieval does not learn a new table per kind.
- Implement exact cosine similarity in Go, with defined answers for equal, orthogonal, opposite, and zero vectors, and refusal for non-finite ones.
- Prohibit comparison across embedding spaces structurally: a query vector carries its space, and the scan filters on it before it computes anything.
- Make a change to the embed role's assignment produce a new space, leaving vectors from the old one intact but outside current retrieval.
- Run embedding through the Phase 5 job lifecycle, one retrieval unit per item, so a cancelled or failed run never leaves a partial vector for the item it was working on.
- Record exact-scan timings at increasing corpus sizes, so the decision not to add a vector extension is a measurement rather than an assumption.

## Capabilities

### New Capabilities
- `embedding-spaces`: what makes two vectors comparable, how a space is identified, and what happens to derived data when the model changes.
- `vector-storage`: the serialization format, what it refuses, and what a retrieval unit is.
- `semantic-retrieval`: exact cosine scanning, its determinism, its scoping, and its locality guarantee.

### Modified Capabilities
None. The obligation that replacing chunks must not strand vectors belongs to `vector-storage`, because it is a rule about vectors; and the embed assignment revision becoming a space's anchor changes what Phase 8's append-only rule is load-bearing for, not what it requires.

## Impact

- `internal/db/migrations.go`: migration 9 — the `embedding_spaces` and `embeddings` tables, their unique indexes, and the foreign-key-free deletion rule that keeps derived vectors from outliving their source.
- New `internal/vector/` (serialization and cosine, with a numerical oracle test); new `embedservice.go` registering the `embed` job kind; `chunkservice.go` discarding vectors when it replaces chunks.
- `frontend/bindings/` regenerated; a semantic-search control in the workspace beside the lexical one, showing which space answered.
- Round-trip tests over known bit patterns, wrong byte lengths, and wrong dimensions; cosine tests against hand-computed values including the degenerate cases; determinism tests over tied scores; a fake cloud endpoint asserting it receives nothing; timing measurements at increasing corpus sizes recorded in the gate evidence.
- No hybrid fusion, no re-ranking, no approximate index. Fusion with FTS5 is Phase 15; an ANN index is only ever added if the recorded measurements miss the PRD threshold.
