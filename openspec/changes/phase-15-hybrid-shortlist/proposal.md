## Why

Assessment is the expensive stage: a local model reading a role and a candidate and reasoning about fit, per requirement, with citations. Running it against every discovered role would take minutes per search and produce a wall of results nobody reads.

So something cheap has to choose which twenty roles are worth that. This phase is that something, and its whole job is to be *cheap, deterministic, and explainable* — three properties that are easy to have individually and easy to lose together.

Cheap means lexical search and an exact cosine scan, both of which already exist. Deterministic means the same corpus produces the same twenty in the same order, every run, on every machine. Explainable means a recruiter can ask why a role is on the list and get an answer that names the criterion and the evidence rather than a number.

The subtle requirement is the one about must-have failures. A role in Sydney when the criteria say Melbourne is a role the recruiter needs to *see and reject*, not one the application quietly drops — because "no results" and "results you would have rejected" look identical on screen, and only one of them is true.

## What Changes

- Exclude only out-of-scope, deleted, and Stale roles, before retrieval rather than after.
- Run lexical and semantic searches for each approved Search Criterion and each compatible Candidate Profile aspect against role chunks and role aspects.
- Enforce the PRD's aspect compatibility map exactly, including the edges it does not have.
- Fuse the ranked lists with reciprocal-rank fusion, group by role, and return a stable top twenty.
- Keep potential structured must-have failures visible on the shortlist rather than filtering them out.
- Record, per shortlisted role, which criteria and aspects retrieved it and by which method, so the list explains itself.

## Capabilities

### New Capabilities
- `aspect-compatibility`: the map, stated once, and the fact that its inverse is derived rather than written twice.
- `shortlist-fusion`: reciprocal-rank fusion, role grouping, tie-breaking, and the stable top twenty.
- `shortlist-scope`: what is excluded before retrieval, and what is deliberately not excluded.
- `shortlist-provenance`: why each role is on the list, in terms a recruiter can act on.

### Modified Capabilities
None. This phase composes Phase 7's lexical search and Phase 9's cosine scan; neither changes.

## Impact

- New `internal/fusion/` — reciprocal-rank fusion and the compatibility map, pure functions with no database, so every edge of the map and every fusion case is a table test.
- New `shortlistservice.go` — scope filtering, per-criterion retrieval, fusion, grouping, and provenance.
- `frontend/src/components/ShortlistPanel.tsx` — the twenty roles, each with why it is there and any structured conflict it carries.
- A hand-calculated corpus for the compatibility map: every permitted edge, and every disallowed one asserted absent.
- Fusion tables covering lexical-only, semantic-only, overlapping, empty, duplicate, and tied lists.
- Timing measurements at representative corpus sizes, recorded in the gate evidence beside Phase 9's.
- No assessment. Phase 16 reads this shortlist; this phase only decides what is on it.
