## 1. Lifecycle

- [x] 1.1 `roleprofileservice.go`: automatic creation from the role's linked, extracted artifacts
- [x] 1.2 Ready, Failed, and Stale derived — Ready and Stale from the source hash, Failed from the contract
- [x] 1.3 A role with no profile is its own reported state, not an absence
- [x] 1.4 Reprofiling from current sources restores Ready

## 2. Eligibility

- [x] 2.1 `Eligibility(roleID)` returning permission and reason in one call
- [x] 2.2 Ready passes; Failed, Stale, and missing are refused with their reasons
- [x] 2.3 The same call shape as candidate readiness, so matching asks both sides alike

## 3. Editing

- [x] 3.1 Edit and remove producing versions with recruiter supplied origin
- [x] 3.2 Manual entry for a role whose listing cannot be decomposed, with priority permitted
- [x] 3.3 No path modifies the role's source artifact

## 4. Frontend

- [x] 4.1 A role profile panel in Research listing every role with its state
- [x] 4.2 Requirements with priority, structured constraints, and citations
- [x] 4.3 Failure cards with retry and manual entry; stale cards with retry
- [x] 4.4 Edits and lifecycle labels; backend messages surfaced verbatim

## 5. Tests

- [x] 5.1 Role fixtures covering every aspect type across stated and unstated attributes
- [x] 5.2 Must-have and nice-to-have only where the source supports them; ambiguous stays unspecified
- [x] 5.3 Normalized constraints reproduce the source while preserving original wording
- [x] 5.4 Failure, retry, manual completion, source change, and stale-to-Ready transitions
- [x] 5.5 Failed and Stale profiles never disappear and never enter automatic assessment
- [x] 5.6 A recruiter edit creates a version and leaves the source artifact untouched
- [x] 5.7 Vitest: failure cards, retry and manual entry, citations, edits, lifecycle labels
- [x] 5.8 Playwright: one Ready and one Failed profile, and only the Ready role is assessable
- [x] 5.9 Fixtures are synthetic only — no real candidate information anywhere

## 6. Exit gate

- [x] 6.1 Role profiling is automatic, transparent, editable, and safe to feed into matching
- [x] 6.2 `just check` passes
