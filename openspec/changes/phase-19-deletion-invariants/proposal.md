## Why

Deletion is the one operation with no undo, and this application holds information about people who did not choose to be in it. So "delete" has to mean delete — not "hide", not "mostly", and not "the record but not the six derived things pointing at it".

The PRD's deletion table is a set of rules that look fussy until you notice what each one prevents. Candidate deletion blocked by archived initiatives prevents an audit trail losing its subject. Shared artifacts requiring an explicit choice prevents deleting someone's résumé because it happened to be attached to two people. Exa source artifacts being read-only prevents a role's provenance dissolving one detach at a time. Notes surviving a purge with a dangling reference prevents the recruiter's own words vanishing because a listing came down.

The other half is proof. A cascade that reports success while leaving embeddings behind is worse than one that fails, because nobody looks again. So every deletion runs in one transaction and then *queries for what should be gone*, scoped to the deleted entity so that evidence deliberately shared with something else is not mistaken for a failure.

## What Changes

- Implement every row of the PRD's deletion table exactly, as one table-driven invariant suite.
- Block candidate deletion while any Active or Archived initiative references them, and say which.
- Require an explicit choice for a candidate artifact linked elsewhere: delete it globally, or retain it under its other links after being warned.
- Keep Exa source artifacts read-only: they cannot be detached or deleted independently, only purged with their role.
- Preserve recruiter notes and metadata-only audit events, clearing references that no longer resolve.
- Run every cascade in one transaction, and verify with scoped queries before reporting success.
- Preview the consequences of a deletion before it happens, listing the exact links and records affected.

## Capabilities

### New Capabilities
- `deletion-rules`: the table, row by row, as behaviour.
- `deletion-cascade`: transactional cascades, scoped verification, and what happens when a step fails.
- `deletion-preview`: what the recruiter is told before they confirm.
- `surviving-records`: notes and audit events that outlive what they referenced.

### Modified Capabilities
None. Deletion is a new operation over records that already exist.

## Impact

- New `deletionservice.go` — the rules, the previews, the cascades, and the verification.
- `frontend/src/components/DeletePanel.tsx` — the consequence preview and the confirmations, distinct for detach and global delete.
- A table-driven suite over every row of the PRD table; a fault-injection suite failing at each cascade step and asserting a full rollback; verification-query tests that tolerate intentionally shared evidence.
- No new schema. Deletion removes rows; the shapes already exist.
