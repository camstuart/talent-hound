## Why

Phase 10 built a contract that turns documents into cited statements. Nothing yet decides whether those statements are *true enough to act on*, and that decision is not the model's to make. A Candidate Profile is what the application will search with, match against, and quote in a message to a client — so a person has to have looked at it and said yes.

That is the whole of this phase: the recruiter is in the loop, exactly once per material change, and the application is honest about which state it is in. Proposed means nobody has checked. Approved means someone did. Stale means someone did, and then the evidence moved underneath them.

The failure this prevents is quiet drift. A profile approved in March, whose resume was replaced in June, is still on screen saying "Approved" — and every match it produces is about a document nobody approved. So a source change never silently updates an approved profile and never silently invalidates it either: it preserves what was approved, marks it Stale, and puts the difference in front of the recruiter as additions, removals, and conflicts.

## What Changes

- Create Candidate Profiles as Proposed, and block search and matching for a candidate whose initial profile is missing, Proposed, or Failed.
- Classify from both the candidate's structured record and their linked artifacts, so a fact the recruiter typed and a fact the resume states both reach the profile with their origins intact.
- Show every aspect with its citations, its origin, and — for extracted aspects — a way to see the source wording it came from.
- Let the recruiter edit an aspect's wording, priority-free structured value, add aspects, and remove aspects, each producing a new version rather than mutating one.
- Approve a version, freezing it as the one search and matching use.
- On a source change, keep the approved version, mark it Stale, and produce a proposed diff of additions, removals, and conflicts against it.
- Never silently overwrite a recruiter-approved aspect during reclassification: a conflicting reclassification is a conflict to resolve, not an update to apply.
- Allow a profile to be built entirely by hand after an extraction failure, so a scanned resume does not make a candidate unusable.
- Support dropping a resume onto a Job Search Initiative: one Candidate and one linked Artifact, or — on cancellation — neither.

## Capabilities

### New Capabilities
- `candidate-profile-lifecycle`: Proposed, Approved, Stale, and Failed; what each permits; and what moves between them.
- `profile-approval-gate`: what is blocked before an initial approval, and what unblocks it.
- `profile-diff`: how a reclassification is presented against an approved version as additions, removals, and conflicts.
- `resume-intake`: dropping a resume onto an initiative, and the all-or-nothing creation of the candidate and artifact.

### Modified Capabilities
- `profile-contract`: a profile version now carries a lifecycle state that Phase 10 only had to leave room for, and recruiter edits become a way of producing a version without a model.

## Impact

- `internal/db/migrations.go`: migration 11 — the approval columns on `profiles` (approved-at, approved-from, superseded-by) and the index that finds a candidate's approved version.
- New `candidateprofileservice.go` — classification from record plus artifacts, the lifecycle transitions, the diff, and the readiness check other phases will call.
- `frontend/src/components/CandidateProfilePanel.tsx` — the review surface: aspects with citations and origins, edit and remove, the diff view, and the approve action, all keyboard operable.
- Vitest over diff review, citation navigation, edit and approve, conflict resolution, and keyboard operation; Playwright over the whole loop — drop a resume, review, edit, approve, change a source, observe staleness.
- No matching, no search integration beyond the gate itself. Phase 16 is what actually consumes an approved profile; this phase makes sure it can only consume an approved one.
