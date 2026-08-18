## 1. Protected-term matcher

- [x] 1.1 `internal/criteria`: the twelve provisional categories and their terms, in one replaceable place
- [x] 1.2 Normalization: case folding, punctuation, hyphens, and whitespace collapsed before comparison
- [x] 1.3 Whole-word matching, so "agenda" is not caught by "age"
- [x] 1.4 Work-rights phrasing permitted; nationality and citizenship phrasing blocked

## 2. Schema

- [x] 2.1 Migration 12: `search_criteria` per initiative, with a CHECK on priority and a position column
- [x] 2.2 A criteria version per initiative that changes with content and not with order

## 3. Service

- [x] 3.1 `criteriaservice.go`: add, edit, remove, reprioritize, reorder
- [x] 3.2 The deterministic block runs before anything is written, with no override
- [x] 3.3 The proxy warning comes from `classify`, is stored, and never blocks
- [x] 3.4 An unavailable classify model means no warning, never a refusal
- [x] 3.5 `Propose` writes nothing; `Apply` takes the recruiter's chosen proposals
- [x] 3.6 Proposals draw only from an approved profile, excluding history-bearing aspect types

## 4. Frontend

- [x] 4.1 A criteria panel in Context: add, edit, remove, prioritize, reorder
- [x] 4.2 Proposals listed with an explicit apply, never applied automatically
- [x] 4.3 A refusal and a warning shown distinctly, the refusal with no way past it
- [x] 4.4 Keyboard operable; backend messages surfaced verbatim

## 5. Tests

- [x] 5.1 Candidate facts and criteria stay separately editable and separately versioned
- [x] 5.2 Prior employer, school, location history, and compensation history never become proposals
- [x] 5.3 The whole provisional list is blocked across case, punctuation, and wording variants
- [x] 5.4 Near-miss words are not blocked; clearly lawful criteria produce no block
- [x] 5.5 Ambiguous proxies warn without blocking, and a missing model is not a block
- [x] 5.6 No path stores a refused criterion; no model output can write a criterion
- [x] 5.7 Reordering changes presentation only; content and priority change the version
- [x] 5.8 Work-rights criteria are available and nationality criteria are not
- [x] 5.9 Vitest and Playwright: block, warning, human apply, reorder, accessibility
- [x] 5.10 Fixtures are synthetic only — no real candidate information anywhere

## 6. Exit gate

- [x] 6.1 Only recruiter-approved lawful search intent can drive discovery and matching
- [x] 6.2 `just check` passes
