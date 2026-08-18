## Why

Everything so far describes what *is*: this candidate has these skills, this role wants these things. Search Criteria describe what the recruiter is *looking for*, and that is a different kind of statement — it belongs to an initiative, not to a person, and it is the recruiter's intent rather than any document's content.

Keeping the two apart is the point. A candidate's resume says they worked at Northwind and studied at Melbourne; it does not say they want to work at a company like Northwind or near Melbourne, and treating history as preference is how a search quietly narrows to "more of the same" without anyone deciding that. So criteria are proposed from approved evidence and only ever *applied* by a person.

The second reason this phase exists alone is the legal boundary. Discovery and matching are driven by criteria, so criteria are the one place to stop an unlawful search — and stopping it has to be deterministic. A model asked "is this discriminatory?" is a model that will sometimes say no. The blocklist does not ask; it refuses, on a fixed list, across the wording variants a person actually types. The model's job is the softer one it is suited to: noticing that "recent graduate" might be a proxy for age, and saying so without blocking, because a model that hard-blocks on its own judgement will eventually block something lawful and the recruiter will have no recourse.

## What Changes

- Add ordered Search Criteria per initiative, each must-have or nice-to-have, editable independently of any profile.
- Propose criteria from a candidate's approved professional aspects, requiring an explicit recruiter action to apply any of them.
- Never derive a criterion from employment history, education history, location history, or compensation history — those are facts about the past, not preferences for the future.
- Block explicit protected criteria deterministically across the whole provisional list, tolerant of case, punctuation, spacing, and straightforward wording variants.
- Use the local `classify` role only to warn about ambiguous potential proxies, never to block.
- Version the criteria set so an assessment can tell whether it was made under the intent still in force, and make reordering presentation-only.

## Capabilities

### New Capabilities
- `search-criteria`: what a criterion is, where it lives, how it is versioned, and what ordering does and does not mean.
- `criteria-proposal`: how criteria are proposed from approved evidence, and what may never become one.
- `prohibited-criteria`: the deterministic block, the advisory warning, and the line between them.

### Modified Capabilities
None. Criteria are a new record alongside profiles rather than a change to them — which is the separation this phase is about.

## Impact

- `internal/db/migrations.go`: migration 12 — `search_criteria` and `criteria_versions`, with the check that keeps priority inside its two values.
- New `internal/criteria/` — the protected-term matcher and the normalizer it runs over, as pure functions testable without a database or a model.
- New `criteriaservice.go` — the criteria list, versioning, proposal, application, and the block-and-warn path.
- `frontend/src/components/CriteriaPanel.tsx` — criteria in Context: add, edit, reorder, prioritize, plus the proposal list with an explicit apply, the deterministic refusal, and the advisory warning shown distinctly from it.
- Fixtures covering the whole provisional protected list across case, punctuation, and wording variants; ambiguous proxies that warn without blocking; and clearly lawful criteria that do neither.
- Work-rights criteria stay available: "must have Australian work rights" is a lawful requirement and must not be confused with nationality.
