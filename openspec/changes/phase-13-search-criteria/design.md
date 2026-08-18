## Context

Two things meet in this phase that do not obviously belong together: separating intent from evidence, and refusing unlawful searches. They are here together because they are the same mechanism seen twice — criteria are the only thing that drives discovery, so criteria are the only place either rule can be enforced once.

## Goals / Non-Goals

**Goals:**
- Criteria that belong to an initiative and cannot be confused with candidate facts.
- Proposals that require a person to apply them, always.
- A deterministic block that a model cannot talk its way past and a recruiter cannot override.
- A model-driven warning that never blocks.
- A version that changes when intent changes and not when presentation does.

**Non-Goals:**
- No weighting. The PRD defers explicit weights until recruiter testing shows a need; ordering is presentation.
- No discovery. Phase 14 turns criteria into queries; this phase decides what a criterion may be.
- No legal advice. The list is provisional and the PRD says it needs specialist confirmation; the code's job is to enforce whatever list it is given, and to make replacing that list a one-place edit.
- No proxy detection beyond a warning. Deciding that "digital native" is age discrimination is a judgement, and judgements that block need a human.

## Decisions

### Blocking is deterministic, and determinism means not asking a model

`internal/criteria` holds the twelve protected categories and their terms, and a matcher that normalizes input before comparing: case folded, punctuation stripped, whitespace collapsed, and a small set of separators treated alike. "Under-35", "under 35", "Under 35s", and "UNDER-35" are one thing.

The alternative — asking `classify` whether a criterion is discriminatory — fails in the direction that matters. A model that is right 95% of the time is a model that permits an unlawful search one time in twenty, silently, with the application's apparent blessing. A fixed list is right about exactly what is on it and honest about the rest, and the rest is what the warning is for.

*ponytail: substring matching over a normalized string, no stemming, no fuzzy distance. The list is short and the variants are the ones people type. Add a stemmer when a real miss is observed, not before.*

### The block cannot be overridden; the warning always can

A criterion matching the protected list is refused, and there is no confirmation dialog that proceeds anyway. A criterion the model flags as a possible proxy is saved with a warning attached, visible wherever the criterion is.

This asymmetry is the whole design. Override on a deterministic block would make the block advisory, and an advisory block is a checkbox on a discrimination claim. Override on a model warning is mandatory in the other direction, because the model will flag "must have a driver's licence" as a possible disability proxy and sometimes that criterion is simply the job.

### Warnings are stored, not recomputed

The warning is attached to the criterion when it is created or edited, and stays. It is not recalculated on read.

Recomputing would call a model on every render, and — worse — would make a criterion's displayed state depend on which model happens to be loaded. A warning that appears and disappears as the recruiter changes their generate model is a warning nobody will trust.

### History is not preference, and the proposal path enforces it structurally

Criteria are proposed only from aspect types that describe *what a person can do or needs*: skill, responsibility, experience, qualification, seniority, work rights, employment type, work arrangement. Location and compensation aspects are excluded from proposals entirely, and so is anything whose wording came from an employment or education history.

The PRD's rule is "candidate preferences are never inferred from resume history alone". The structural version of that is: the proposer never sees the aspect types that carry history. A recruiter who *wants* a location criterion types it, which is a person deciding, which is the whole point.

*ponytail: exclusion by aspect type, which is coarse — an experience aspect can mention a city. The recruiter reviews every proposal before it applies, and that review is the backstop the coarseness leans on.*

### Applying is always a separate act

`Propose` returns candidates for criteria and writes nothing. `Apply` takes the ones the recruiter chose. Nothing in the chat, classifier, or proposal path can create or modify a criterion.

Stated as an invariant rather than a workflow: the only methods that write to `search_criteria` take an explicit recruiter action as their entire input. Phase 17's Q&A cannot reach them, and neither can a future feature written by someone who has not read this paragraph, because there is no write path that accepts model output.

### The version changes with content, not with order

Adding, removing, editing, or re-prioritizing a criterion bumps the initiative's criteria version. Reordering does not.

Assessments record the criteria version they were made under, so Phase 16 can tell a stale match from a current one. Ordering is presentation — the PRD is explicit that it is not weighting — so a version bump on reorder would invalidate every assessment because someone dragged a row, which teaches recruiters that the staleness indicator means nothing.

### Work rights are a criterion, nationality is not

"Must have Australian work rights" is lawful and necessary and must keep working. "Must be an Australian citizen" is on the blocked list under national origin.

The distinction is in the terms, not in a clever rule: the matcher blocks nationality and citizenship phrasing and does not block work-rights phrasing, and there are fixtures for both sides because this is precisely the boundary someone will accidentally break while tidying the list.

## Risks / Trade-offs

- **The list is provisional.** It is one Go slice with the categories named, and replacing it is a single edit — which is the most the code can do about a question that needs a lawyer.
- **Substring matching over-blocks.** "Agenda" contains "age" as a substring, so the matcher compares whole words rather than raw substrings, and the fixtures include the near-misses that would otherwise be caught.
- **Proxy warnings depend on a model being available.** With no classify model, criteria save with no warning rather than being refused — refusing would make an unavailable model into a block, which is the one thing blocks are not allowed to be.
- **Aspect-type exclusion is coarse.** A skill aspect could carry a location. The recruiter reviews every proposal, and nothing applies without that review.
