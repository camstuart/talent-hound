## 1. Taxonomy

- [x] 1.1 `internal/profile`: the eleven aspect types, three priorities, two origins, closed
- [x] 1.2 Structured field sets for location, work arrangement, work rights, employment type, compensation
- [x] 1.3 Subject kinds for candidate and role, with the candidate-carries-no-priority rule

## 2. Schema and prompt

- [x] 2.1 The versioned constrained JSON schema the classify role is held to
- [x] 2.2 The versioned classifier prompt, stating the citation, priority, and unclear-value rules
- [x] 2.3 Derived identity: a hash over schema version, prompt version, assignment revision, and source hash

## 3. Validation

- [x] 3.1 Complete pass returning every violation, never a filtered subset
- [x] 3.2 Type, priority, and origin checked against the closed lists
- [x] 3.3 Citations resolve: known chunk, and quoted text present in it
- [x] 3.4 Recruiter supplied aspects cite a record instead, and cannot cite nothing
- [x] 3.5 Structured fields checked per type; an unknown field fails rather than being dropped
- [x] 3.6 Duplicates and contradictions fail

## 4. Persistence

- [x] 4.1 Migration 10: `profiles` and `profile_aspects` with CHECKs mirroring the closed lists
- [x] 4.2 Whole proposal in one transaction, or nothing
- [x] 4.3 Versions accumulate; the current profile is the highest version

## 5. Service

- [x] 5.1 `classifyservice.go`: resolve the classify model through the registry, including inheritance
- [x] 5.2 One repair retry carrying the violations, then a coded retryable failure
- [x] 5.3 Coded failure reasons carrying no source content
- [x] 5.4 Recruiter-authored aspects added directly, with record citations

## 6. Tests

- [x] 6.1 Fixtures covering every aspect type and all three role priorities
- [x] 6.2 Unsupported type, unsupported priority, missing citation, unresolvable citation, invented structured field, duplicate, contradiction — each fails the whole proposal
- [x] 6.3 Unclear wording stays unspecified or absent and is never promoted
- [x] 6.4 Zero repair calls on a valid response, exactly one on invalid-then-valid, failure after two
- [x] 6.5 A prompt-injection corpus cannot add types, remove citations, or raise priority
- [x] 6.6 Recruiter supplied aspects stay distinct and cite their record
- [x] 6.7 Schema, prompt, model, and source changes each alter derived identity; unchanged inputs do not
- [x] 6.8 The database refuses an out-of-taxonomy type, priority, or origin
- [x] 6.9 Fixtures are synthetic only — no real candidate information anywhere

## 7. Windows gate (written and tagged; unrun — needs the target laptop)

- [ ] 7.1 The contract holds against the selected local model, not only against deterministic fakes

## 8. Exit gate

- [x] 8.1 The contract returns a fully valid proposal or a visible failure, and cannot persist partial output
- [x] 8.2 `just check` passes
