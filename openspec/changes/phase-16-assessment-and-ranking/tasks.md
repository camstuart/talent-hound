## 1. Pure logic

- [x] 1.1 `internal/assess`: canonical serialization and the input hash over every listed input
- [x] 1.2 Structured comparison for the five types, with unknown on either side yielding unknown
- [x] 1.3 Compensation as overlapping ranges; differing currencies are unknown
- [x] 1.4 The ranking comparator, in the PRD's exact order, ending in role identifier

## 2. Schema

- [x] 2.1 Migration 14: `matches` with the input hash indexed, and `match_results` per aspect
- [x] 2.2 CHECKs keeping result states and directions inside their enumerations

## 3. Service

- [x] 3.1 `assessservice.go`: both directions, run separately
- [x] 3.2 KNN evidence selection whose score never reaches the result
- [x] 3.3 The `generate` call with a constrained schema
- [x] 3.4 Validation: uncited met, unknown states, unresolvable citations, injected instructions
- [x] 3.5 A role's assessment is stored whole or not at all
- [x] 3.6 Reuse if and only if the recomputed hash matches
- [x] 3.7 A cancellable job whose completed roles survive cancellation

## 4. Frontend

- [x] 4.1 A matches panel with both directions per role
- [x] 4.2 Per-aspect results with their evidence, gaps, and unknowns
- [x] 4.3 Progress, cancel, and staleness
- [x] 4.4 Backend messages surfaced verbatim

## 5. Tests

- [x] 5.1 Per-aspect fixtures: met with citation, not met with contrary evidence, not met without, unknown, no evidence
- [x] 5.2 Structured comparison over every type, including unknown values
- [x] 5.3 Adversarial similarity cannot change a result state
- [x] 5.4 Validation rejects uncited met, invalid states, unavailable citations, and injected instructions
- [x] 5.5 A ranking oracle tests each tie-break alone and in combination, ending in role identifier
- [x] 5.6 Every hash input invalidates when changed alone; presentation-only changes do not
- [x] 5.7 Canonical serialization agrees across repeats and across a separate process
- [x] 5.8 Batch cancellation keeps completed roles and rolls back the current one
- [x] 5.9 Reassessment reuses unchanged results and recomputes only stale ones
- [x] 5.10 Vitest: two-direction labels, gaps, unknowns, evidence navigation, progress, cancel, stale
- [x] 5.11 Playwright: run and rerun an assessment, verifying order and cache reuse
- [x] 5.12 Fixtures are synthetic only — no real candidate information anywhere

## 6. Exit gate

- [x] 6.1 Every assessed result is explainable, correctly ordered, cancellable, and invalidated by all decision-relevant changes
- [x] 6.2 `just check` passes
