## Context

Two ranked lists arrive from two systems that do not share a scale. FTS5 returns a bm25 score where lower is better and the magnitude depends on corpus statistics; cosine returns a similarity in [-1, 1] whose distribution depends on the embedding model. Averaging them is meaningless, and normalizing them is meaningless in a subtler way — the normalization would depend on the corpus, so the same role would rank differently as the corpus grew.

Reciprocal-rank fusion sidesteps this by throwing the scores away and keeping only the ranks. That is the whole reason it is the right choice here, and it is also why it is boring, which is a compliment.

## Goals / Non-Goals

**Goals:**
- Identical output for identical input, on any machine, every run.
- A shortlist a recruiter can interrogate: why this role, from which criterion, by which method.
- The compatibility map enforced exactly, including its absences.
- Structured conflicts surfaced rather than filtered.

**Non-Goals:**
- No weighting between criteria. The PRD defers explicit weights, and criterion order is presentation.
- No learned ranking, no relevance feedback. Both would make the list undefendable and non-reproducible.
- No assessment. This chooses the twenty; Phase 16 judges them.
- No new retrieval. Lexical is Phase 7's, semantic is Phase 9's, and neither is modified.

## Decisions

### Fusion is reciprocal rank, and the constant is stated

For each ranked list, a role at rank *r* contributes `1 / (k + r)` with `k = 60`, and a role's fused score is the sum of its contributions across every list it appears in.

`k = 60` is the value from the original RRF paper and it is written down rather than tuned, because tuning it against this corpus would produce a number that is right for today's fixtures and unjustifiable tomorrow. Its effect is to flatten the difference between rank 1 and rank 2 relative to the difference between appearing and not appearing — which is exactly the property wanted, since a role found by three criteria is more interesting than a role found first by one.

Scores are never compared across the two systems, and never leave this calculation.

### Ties break by role identifier, and the sort is stable

Equal fused scores happen constantly — two roles found at the same rank by the same one criterion have identical scores. The order is `score descending, then role id ascending`, which makes "repeated runs return identical ordering" a property rather than a hope.

The same rule as Phase 9's semantic search, for the same reason, stated the same way.

### The compatibility map is written once and inverted in code

The PRD states the map as *role aspect → candidate aspects searched*. The shortlist searches in the other direction: it has candidate aspects and criteria, and it is looking at roles.

So the map is transcribed exactly as the PRD writes it, and the inverse is computed. Writing the inverse by hand would be a second copy that drifts the first time someone adds an edge, and the drift would be invisible — a missing edge is a role that quietly never appears.

The absences matter as much as the presences: `qualification` searches only `qualification`, and a test asserts that a qualification aspect does not retrieve a skill.

### Scope is applied before retrieval, and Stale is the only lifecycle exclusion

Out-of-scope, deleted, and Stale roles are excluded by the query that selects candidates for retrieval, not by filtering results afterwards.

Filtering afterwards is how excluded things leak: a top-20 computed over everything and then filtered returns fewer than twenty, and the missing slots are invisible. Filtering first means twenty eligible roles are twenty eligible roles.

Stale is excluded because Phase 12 already decided a stale role is not assessed. Nothing else about a role excludes it — in particular, a role that obviously fails a must-have does not.

### Must-have failures are carried, not applied

A role whose location conflicts with a must-have location criterion stays on the shortlist, with the conflict attached.

This is the PRD's rule and it is worth restating why: "no results" and "results you would have rejected" look identical on screen, and only one of them is true. A recruiter who sees zero roles concludes the market is empty; a recruiter who sees three roles marked "Sydney, and you asked for Melbourne" concludes something useful and possibly changes the criterion.

*ponytail: the conflict check is structured-value equality on the five normalizable types, computed at shortlist time and attached. Anything cleverer is Phase 16's assessment, which is the stage that is allowed to be expensive.*

### Provenance is recorded per role, not reconstructed

Each shortlisted role carries the list of (criterion or aspect, method, rank) that put it there.

Reconstructing it later would mean re-running retrieval, which would be slow and — worse — could produce a different answer if anything changed in between. Recording it at fusion time makes the explanation exactly as true as the list.

## Risks / Trade-offs

- **RRF ignores score magnitude.** A role that matched one criterion superbly ranks below one that matched three adequately. That is the intended behaviour for a *shortlist*, whose job is coverage rather than precision — Phase 16 does precision.
- **Twenty is fixed.** The PRD says twenty; nothing here makes it configurable, and a recruiter who wants more runs a narrower search.
- **The conflict check is shallow.** Structured equality only, on five types. A location conflict expressed in prose is not caught here and is caught by assessment.
- **Retrieval cost grows with criteria.** Each criterion is one lexical and one semantic query. At a realistic dozen criteria over a few hundred roles this is milliseconds; the measurement records it.
