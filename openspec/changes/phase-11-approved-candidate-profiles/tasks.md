## 1. Schema

- [x] 1.1 Migration 11: approval columns on `profiles` — approved at, and the source hash the approval was about
- [x] 1.2 An index that finds a candidate's approved version without scanning versions

## 2. Lifecycle

- [x] 2.1 `candidateprofileservice.go`: classify from the candidate record plus linked extracted artifacts
- [x] 2.2 `Approve(version)` stamping the approval and making it the version in use
- [x] 2.3 Staleness computed from the current source hash, never stored
- [x] 2.4 Edit, add, and remove producing versions with recruiter supplied origin
- [x] 2.5 A Failed classification leaves a visible, retryable version and does not block manual construction

## 3. Gate

- [x] 3.1 `Readiness(candidateID)` returning permission, reason, and warning in one call
- [x] 3.2 Missing, Proposed, and Failed block; Approved passes; Stale passes with a warning

## 4. Diff

- [x] 4.1 Correspondence: same type plus structured agreement, else substantial wording overlap
- [x] 4.2 Additions, removals, and conflicts as three lists; unchanged aspects in none
- [x] 4.3 A pure function of two versions — deterministic, and calls no model
- [x] 4.4 Resolving a conflict produces a new version and modifies neither side

## 5. Intake

- [x] 5.1 Drop a resume onto a Job Search Initiative: one candidate, one artifact, one transaction
- [x] 5.2 Attach to an already-selected candidate without creating a duplicate
- [x] 5.3 Cancellation and mid-way failure leave nothing behind

## 6. Frontend

- [x] 6.1 A profile panel listing aspects with type, wording, origin, and citations
- [x] 6.2 Citation navigation: see the source wording an aspect came from
- [x] 6.3 Edit, remove, and add an aspect
- [x] 6.4 The diff view with additions, removals, and conflicts, and per-conflict resolution
- [x] 6.5 Approve, with the state and any staleness warning visible
- [x] 6.6 Keyboard operable throughout; backend messages surfaced verbatim

## 7. Tests

- [x] 7.1 Initial classification combines record and artifacts without losing provenance
- [x] 7.2 A resume drop creates exactly one candidate and one artifact; cancellation creates neither
- [x] 7.3 Readiness blocks missing, Proposed, and Failed; passes Approved; warns on Stale
- [x] 7.4 Approval freezes a version and its cited evidence still resolves
- [x] 7.5 Source addition, replacement, edit, detach, and deletion each produce the right stale state
- [x] 7.6 Reclassification never overwrites an approved aspect; the difference is a conflict
- [x] 7.7 A stale approved profile stays usable with a warning until reapproval
- [x] 7.8 Manual construction works when extraction failed entirely
- [x] 7.9 Vitest: diff review, citation navigation, edit and approve, conflict resolution, keyboard operation
- [x] 7.10 Playwright: drop, review, edit one aspect, approve, change a source, observe staleness
- [x] 7.11 Fixtures are synthetic only — no real candidate information anywhere

## 8. Exit gate

- [x] 8.1 A recruiter can create and maintain an approved Candidate Profile with complete provenance and human control
- [x] 8.2 `just check` passes
