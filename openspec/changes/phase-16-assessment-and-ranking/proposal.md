## Why

This is the stage the product exists for, and the one where being wrong is most expensive. A match result is what the recruiter acts on: it says this person meets this requirement, or does not, or that nothing in the evidence says. All three of those are claims about a person's career, and the first two are claims a recruiter may repeat to a client.

So every result carries its evidence, and the ones that cannot are refused rather than shipped hedged. A `met` with no citation is the single most dangerous output this application can produce — it reads as verified and is not — so it fails validation.

The second reason this phase is large is invalidation. An assessment is a function of a dozen inputs: two profile versions, the criteria, the evidence hashes, the model, the prompt, the parameters, the ranking rules. Any of them changing makes the result about something else. The PRD's answer is one canonical hash over all of them, recomputed and compared, and that being the *sole* caching rule — no timestamps, no heuristics, no "probably still fine".

Third: matching is two-directional, and conflating the directions loses information. "Does this role suit the candidate" and "does this candidate suit the role" have different evidence and different answers, and a recruiter needs both to have a conversation with either side.

## What Changes

- Assess role fit for the candidate — the Role Profile against the initiative's Search Criteria — separately from candidate fit for the role — the approved Candidate Profile against the Role Profile's requirements.
- Compare structured constraints deterministically, with no model involved.
- Select evidence for semantic requirements by exact-cosine KNN, and treat that selection as evidence selection only: similarity never becomes a result.
- Ask `generate` for met, not met, or unknown, requiring a citation for met, contrary evidence for not met when available, and an explicit statement when nothing was found.
- Validate every result: uncited met, unknown states, unresolvable citations, and injected instructions all fail.
- Rank by the PRD's exact order, ending in role identifier so ties are stable.
- Compute one canonical `assessment_input_hash` over every listed input, and make it the only reason a cached result is or is not reused.
- Run assessment as cancellable background jobs whose completed per-role results survive cancellation.

## Capabilities

### New Capabilities
- `assessment-directions`: the two directions, what each compares, and why they stay apart.
- `per-aspect-results`: met, not met, unknown, and the citation rules that distinguish them.
- `structured-comparison`: deterministic comparison of the five normalizable types, including unknown.
- `assessment-validity`: the canonical input hash, what it covers, and that it is the sole caching rule.
- `match-ranking`: the exact order, each tie-break, and stability.

### Modified Capabilities
None. This phase consumes Phase 11's approved profiles, Phase 12's Ready role profiles, Phase 13's criteria, and Phase 15's shortlist; none of them changes.

## Impact

- `internal/db/migrations.go`: migration 14 — `matches` and `match_results`, with the hash indexed because it is the lookup.
- New `internal/assess/` — canonical hashing, the ranking comparator, and structured comparison, all pure so the oracle tests are tables.
- New `assessservice.go` — the two directions, KNN evidence selection, the `generate` call, output validation, the job, and cache reuse.
- `frontend/src/components/MatchesPanel.tsx` — both directions per role, per-aspect results with their evidence, gaps, unknowns, progress, cancel, and staleness.
- Per-aspect fixtures for every result shape; a ranking oracle exercising each tie-break alone and combined; hash tests changing one input at a time; canonical-serialization tests across process restarts.
- An adversarial suite: similarity scores that cannot change a result, injected instructions in evidence, and citations that do not resolve.
