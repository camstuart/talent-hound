## 1. Rules

- [x] 1.1 `deletionservice.go`: initiative deletion, owned records only
- [x] 1.2 Candidate deletion blocked by any referencing initiative, active or archived, named
- [x] 1.3 Candidate cascade: profile, aspects, candidate-only artifacts, chunks, embeddings
- [x] 1.4 Shared candidate artifacts requiring an explicit delete-globally or retain choice
- [x] 1.5 Detach as a link-only operation; global artifact deletion listing every link first
- [x] 1.6 Exa source artifacts refusing detach and individual deletion
- [x] 1.7 Role purge: sources current and historical, profile, aspects, embeddings, matches, active drafts
- [x] 1.8 Draft deletion clearing the reference on surviving copy events

## 2. Cascade and verification

- [x] 2.1 Every cascade in one transaction
- [x] 2.2 Scoped verification queries run before success is reported
- [x] 2.3 Verification tolerating intentionally shared evidence
- [x] 2.4 Purge-all-stale applying the invariant per role and reporting what could not be purged
- [x] 2.5 Repeated deletion safe and harmless to unrelated records

## 3. Preview

- [x] 3.1 A consequence preview listing the exact records and links
- [x] 3.2 Blockers named so the recruiter can act on them
- [x] 3.3 Detach and global deletion confirmed differently

## 4. Frontend

- [x] 4.1 A delete panel with the preview and the two distinct confirmations
- [x] 4.2 Backend messages surfaced verbatim

## 5. Tests

- [x] 5.1 A table-driven suite over every row of the PRD deletion table
- [x] 5.2 Candidate deletion blocked by active and archived initiatives, succeeding after both are gone
- [x] 5.3 Candidate-only and shared-artifact branches remove or retain exactly what is intended
- [x] 5.4 Detach and global deletion have distinct effects
- [x] 5.5 Exa source detach and delete refused; purge removes current and historical sources
- [x] 5.6 Purge-all-stale applies per role and reports failures without partial deletion
- [x] 5.7 Notes survive with cleared references; audit events survive as specified
- [x] 5.8 Injected failure at every cascade step rolls everything back
- [x] 5.9 Verification checks every derived kind and tolerates shared evidence
- [x] 5.10 Repeated deletion is safe
- [x] 5.11 Vitest over the consequence preview; Playwright over stale purge and candidate deletion
- [x] 5.12 Fixtures are synthetic only — no real candidate information anywhere

## 6. Exit gate

- [x] 6.1 Every deletion acceptance condition passes, with transaction-failure coverage
- [x] 6.2 `just check` passes
