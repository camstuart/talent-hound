## Why

Everything from here on consumes profiles. Candidate Profiles are Phase 11, Role Profiles are Phase 12, criteria are Phase 13, matching is Phase 16 — and all four are the same shape: a versioned set of typed, citable statements derived from a source by the `classify` role. Building that shape four times would produce four subtly different validators, and the one place a subtle difference is fatal is the rule that every claim must be traceable to evidence.

So the contract comes first, alone, with nothing depending on it yet. That is the only moment it can be got right cheaply.

The contract has one hard property: **it either returns a fully valid profile proposal or a visible failure**. There is no third outcome. A classifier that persists the aspects it got right and drops the ones it got wrong produces a profile that looks complete and is silently missing the requirement the recruiter cared about, which is worse than no profile at all. The PRD's rules — every extracted aspect cites its source, unclear values stay absent, priority is never invented, unsupported aspects fail validation, one repair retry and then a retryable failure — are all instances of that single property.

The other reason to build it now is that the model is a stranger. A source document is written by someone the application does not trust, and a language model reading that document will follow instructions it finds there. The contract is where that is contained: constrained output, a closed taxonomy, and citations that must resolve against the source the model was given.

## What Changes

- Add the aspect taxonomy as a closed enumeration: skill, responsibility, experience, qualification, seniority, location, work arrangement, work rights, employment type, compensation, other. A type outside it is a validation failure, not a passthrough.
- Persist profiles as versioned rows carrying the schema version, the prompt version, and the `classify` assignment revision that produced them, so a change to any of the three changes the derived profile's identity.
- Persist aspects with their type, source wording, normalized structured value, priority, origin, and citations.
- Validate the model's output completely before anything is written: type, priority, origin, citation resolution against the actual source, structured field names, duplicates, and contradictions.
- Apply exactly one repair retry on invalid output, then fail visibly and retryably.
- Keep role priority at unspecified unless the source supports must-have or nice-to-have, and never promote an unclear value.
- Distinguish extracted aspects from Recruiter supplied ones, and let a Recruiter supplied aspect cite its recruiter-authored record rather than a document.
- Contain prompt injection: instructions found inside a source cannot add an aspect type, skip a citation, or change the priority rules.

## Capabilities

### New Capabilities
- `profile-contract`: what a profile is, what versions it carries, and what makes two profiles the same derived record.
- `aspect-taxonomy`: the closed type list, priorities, origins, structured values, and what each type may normalize.
- `classifier-validation`: the complete validation pass, the single repair retry, and the all-or-nothing persistence rule.

### Modified Capabilities
None. The `classify` assignment revision becoming part of a derived profile's identity changes what Phase 8's append-only rule is load-bearing for — exactly as the embed revision did in Phase 9 — not what that rule requires.

## Impact

- `internal/db/migrations.go`: migration 10 — `profiles` and `profile_aspects`, with the checks that keep type, priority, and origin inside their enumerations at the database level as well as in Go.
- New `internal/profile/` — the taxonomy, the versioned JSON schema, the versioned prompt, and the validator, with no database or service dependency so the rules can be tested as pure functions.
- New `classifyservice.go` — the call, the repair retry, and the transaction that writes a whole profile or none of it.
- `frontend/bindings/` regenerated; no UI yet beyond what Phase 11 will need, because there is no profile-bearing subject until then.
- Fixtures covering every aspect type, all three priorities, and each failure mode: unsupported type, unsupported priority, missing citation, citation that does not resolve, invented structured field, duplicate, and contradiction.
- A prompt-injection corpus, asserting that a source instructing the model to invent aspects or skip citations produces either a valid profile without them or a visible failure.
- The live-model contract test is written and tagged, joining the Windows gate; the deterministic fake covers everything that does not need a model.
