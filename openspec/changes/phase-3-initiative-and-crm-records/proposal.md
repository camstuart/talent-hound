## Why

An initiative can currently be created and listed, and nothing else. It cannot be renamed, archived, reopened, or deleted, it references no records, and its workspace is a placeholder paragraph. Every later phase — artifacts, profiles, search, matching, drafts — hangs off a Candidate, a Role, or an initiative that owns them, so the shared records and the exact Job Search cardinality have to exist before any pipeline can be pointed at them. Phase 2 made the schema versioned and recoverable; this is the first phase that puts product data into it.

## What Changes

- Complete the initiative lifecycle: rename, archive, reopen, and delete, with Active ⇄ Archived as the only state transition.
- Add Candidate, Role, Company, and Contact records with exactly the structured fields named in the PRD, and no others.
- Require exactly one Candidate for a Job Search Initiative, rejecting zero or multiple at creation.
- Provide the minimal contacts-at-company count and listing.
- Keep Talent Search and Business Development as workspace shells with no pipeline behaviour.
- Add the Context, Research, Matches, and Drafts navigation skeleton with no speculative pipeline behind it.
- Add the schema migrations for all of the above; the models stop being the source of schema truth.

## Capabilities

### New Capabilities
- `initiative-lifecycle`: initiative creation, rename, archive, reopen, delete, and the Job Search candidate cardinality rule.
- `crm-records`: Candidate, Role, Company, and Contact structured records, their validation, their sharing across initiatives, and contacts-at-company.
- `initiative-workspace-shell`: the four-area workspace skeleton and the per-type shell behaviour.

### Modified Capabilities
<!-- none: this phase adds entries to the Phase 2 migration list; the `database-schema` behaviour it specifies is unchanged -->



## Impact

- `internal/db/migrations.go`: migration 2 onward — initiative status, candidates, roles, companies, contacts.
- `internal/models/`: new `candidate.go`, `role.go`, `company.go`, `contact.go`; `initiative.go` gains status and candidate reference.
- `initiativeservice.go` gains the lifecycle methods; new `recordservice.go` exposes the CRM records; both registered in `main.go`.
- `frontend/bindings/` regenerated; `frontend/src/` gains record forms and the four-area workspace shell.
- New Go table-driven tests, Vitest form/keyboard tests, and Playwright specs against the real backend.
- Deletion of Candidate, Role, Company, and Contact is deliberately **not** implemented here — the deletion invariants land in Phase 19. Initiative deletion is implemented and must never remove a shared record.
