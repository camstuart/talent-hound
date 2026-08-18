## 1. Fusion and the map

- [x] 1.1 `internal/fusion`: the compatibility map exactly as the PRD states it
- [x] 1.2 The inverse derived from it, never written separately
- [x] 1.3 Reciprocal-rank fusion with a stated constant, grouping by key, best rank per list
- [x] 1.4 Deterministic order: score descending, identifier ascending

## 2. Service

- [x] 2.1 `shortlistservice.go`: scope, deletion, and staleness applied before retrieval
- [x] 2.2 One lexical and one semantic query per criterion and per compatible aspect
- [x] 2.3 Structured conflicts computed and attached, never applied as a filter
- [x] 2.4 Provenance recorded per role at fusion time
- [x] 2.5 The criteria version and embedding space recorded on the shortlist

## 3. Frontend

- [x] 3.1 A shortlist panel listing the roles in order with their fused position
- [x] 3.2 Why each role is there: the criteria and aspects that matched, and the method
- [x] 3.3 Structured conflicts shown on the entry that carries them
- [x] 3.4 Backend messages surfaced verbatim

## 4. Tests

- [x] 4.1 A hand-calculated corpus verifies every compatibility edge and every absent one
- [x] 4.2 Lexical-only, semantic-only, overlapping, empty, duplicate, and tied lists fuse as expected
- [x] 4.3 Role grouping keeps many matching chunks to one slot
- [x] 4.4 Scope, deleted, and stale filters never leak an excluded role
- [x] 4.5 Location, work-rights, and arrangement conflicts stay visible
- [x] 4.6 Fewer than twenty returns all; more than twenty returns exactly twenty with stable ties
- [x] 4.7 Repeated runs return identical ordering
- [x] 4.8 Timing measurements at representative sizes, printed as evidence lines
- [x] 4.9 Vitest and Playwright expose the provenance and the conflicts
- [x] 4.10 Fixtures are synthetic only — no real candidate information anywhere

## 5. Exit gate

- [x] 5.1 The shortlist is deterministic, explainable, and fast enough to feed assessment
- [x] 5.2 `just check` passes
