## Why

Phase 11 put a person in the loop for every Candidate Profile, because a candidate is a person the recruiter has a relationship with and will speak for. A role is not. A recruiter running a search sees twenty discovered listings in a sitting, and asking them to approve twenty decompositions before any matching can happen would defeat the workflow the whole product exists to provide.

So Role Profiles are created automatically and are editable — and the asymmetry with Candidate Profiles is deliberate and stated in the PRD. What replaces approval is honesty about state: a Role Profile is Ready, Failed, or Stale, only Ready enters automatic assessment, and neither Failed nor Stale ever disappears from the screen.

That last part is the real requirement. A failed decomposition that vanishes looks like a role that was never discovered; a stale one that silently keeps being assessed produces matches about a listing that has since changed. Both are ways of quietly losing information the recruiter is responsible for.

## What Changes

- Create a Role Profile automatically from the role's current source content, with no approval step.
- Give it a lifecycle of Ready, Failed, and Stale, computed the same way Phase 11 computes candidate staleness: from the source hash, not a stored flag.
- Make only Ready current versions eligible for automatic assessment, through one readiness call, as with candidates.
- Keep a Failed profile visible with retry and manual-entry actions, and keep a Stale profile visible with its warning.
- Expose extracted requirements with their priority, normalized constraints, and citations, and let the recruiter edit them.
- Make a recruiter edit a new evidence-aware version, never a change to the source artifact.

## Capabilities

### New Capabilities
- `role-profile-lifecycle`: automatic creation, Ready, Failed, Stale, and what each permits.
- `role-assessment-eligibility`: what may enter automatic assessment, and the single call that decides it.

### Modified Capabilities
None. The aspect taxonomy, validation contract, and versioning are Phase 10's and unchanged; what is new is a second subject kind using them under a different lifecycle.

## Impact

- New `roleprofileservice.go` — automatic creation, the lifecycle, the readiness call, and the edit path, reusing Phase 10's contract and Phase 11's staleness computation.
- `frontend/src/components/RoleProfilePanel.tsx` — role profiles in the Research area: requirements with priority and citations, failure cards with retry and manual entry, and lifecycle labels.
- No migration. `profiles` already carries `subject_kind`, and a role profile is a row with `subject_kind = 'role'`; the approval columns simply stay empty, which is the correct account of a record nothing approves.
- Role fixtures covering stated and unstated skills, responsibilities, experience, qualifications, seniority, location, work arrangement, work rights, employment type, and compensation; must-have and nice-to-have assigned only where the source supports them.
- Vitest over failure cards, retry and manual entry, citations, edits, and lifecycle labels; Playwright creating one Ready and one Failed profile and confirming only the Ready role is assessable.
