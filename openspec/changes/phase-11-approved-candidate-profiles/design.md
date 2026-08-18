## Context

Everything in Phases 6 through 10 was about producing derived data correctly. This phase is about the one question none of that can answer: is it right? A citation that resolves proves the model did not invent the sentence. It does not prove the sentence means what the aspect says it means, and it certainly does not prove the profile is a fair account of the person.

So a human says yes, and the application's job is to make that yes cheap to give, hard to give accidentally, and impossible to forget was given about a *particular* set of evidence.

## Goals / Non-Goals

**Goals:**
- Four states with no ambiguity about what each permits.
- An approval that is about specific evidence, and knows when that evidence changed.
- Recruiter edits that are versions, not mutations.
- A diff a person can act on: additions, removals, conflicts.
- A path to a usable profile when extraction fails entirely.

**Non-Goals:**
- No matching or ranking. Phase 16 consumes approved profiles; this phase only guarantees it cannot consume an unapproved one.
- No aspect-level approval. The unit of approval is a version, because per-aspect approval produces a profile that is partly approved, which is a state nothing downstream knows how to interpret.
- No automatic re-approval, ever, under any similarity threshold. If the evidence changed, a person looks again.
- No Role Profiles. They are Phase 12 and deliberately do *not* require approval, because per-role approval would defeat the workflow — the asymmetry is in the PRD and is intentional.

## Decisions

### Approval attaches to a version, and staleness is computed, not stored

A version is approved by stamping it approved. Whether it is *stale* is not a fourth stored state — it is the comparison between the approved version's source hash and the current source hash of the candidate's evidence.

Storing staleness would mean something has to notice the source changed and go update a row, and the thing that notices is exactly the thing that will be missing when someone adds a new way to attach an artifact in Phase 14. Computing it means a profile cannot be stale-in-fact and fresh-on-screen, because there is no row to be out of date. The screen asks the question every time it renders, which is cheap: it is one hash of the current sources.

*ponytail: recomputed per read. Cache it behind a source-version counter if a profile view ever gets slow, which it will not at one candidate at a time.*

### The gate is a method, and it is the only way through

`Readiness(candidateID)` returns whether search and matching may use this candidate, and why not when they may not. Phase 13, 15, and 16 call it; nothing reimplements the check.

The alternative is each consumer filtering on `state == "approved"`, which is four copies of a rule that has to change together when Stale arrives — and Stale is the case that matters, because a stale approved profile is *usable with a warning*, not blocked. Encoding that in one place means the warning and the permission cannot disagree.

### Reclassification never overwrites; it proposes

Reclassifying a candidate with an approved profile does not produce a new current profile. It produces a Proposed version *and* a diff against the approved one, and the approved one stays current until a person approves the new one.

This is the rule the PRD states as "recruiter-approved aspects are never silently overwritten", and the strongest form of it is the one where reclassification cannot overwrite anything because it does not write to the approved version at all. The diff is then a pure function of two versions, which is why it can be tested without a model.

### The diff is three lists, and conflict is the interesting one

Additions are aspects in the proposal with no counterpart in the approved version. Removals are the reverse. Conflicts are aspects that clearly correspond — same type, same subject — but say different things.

"Clearly correspond" is doing real work, and the honest answer is that it is a heuristic: same type and a matching normalized structured field, or same type and substantially overlapping wording. A heuristic that is wrong produces a conflict that is really an addition-plus-removal, which is a mildly annoying review and not a wrong profile. A heuristic that tried harder — an embedding similarity, say — would be wrong in ways that are harder to see, and would make the diff depend on which model is loaded.

*ponytail: type plus structured-field agreement, then wording overlap. An embedding-based pairing when a recruiter complains the diffs are noisy, not before.*

### Recruiter edits produce versions, and carry their own origin

Editing an aspect writes a new version with that aspect changed and its origin set to recruiter supplied, citing the edit. Removing writes a new version without it. Adding is Phase 10's `AddRecruiterAspect`.

Making an edit a version rather than a mutation costs a row and buys the property that every profile on screen was, at some moment, exactly what somebody looked at. It also means "who said this" is answerable for every aspect: the model, with a document citation, or the recruiter, with a record citation.

### A failed extraction does not make a candidate unusable

A candidate whose only resume is a scan produces a Failed profile. From there, `AddRecruiterAspect` builds a profile by hand, one aspect at a time, and that profile can be approved like any other.

There is no separate manual-profile mode and no different table. The manual path is the same path with every aspect recruiter supplied — which is why Phase 10 insisted recruiter aspects carry citations in their own currency rather than being allowed to have none. A hand-built approved profile is a first-class profile, visibly composed of recruiter assertions.

### Resume intake is one transaction or nothing

Dropping a resume onto a Job Search Initiative creates a Candidate, an Artifact, and the links between them, in one transaction. Cancelling creates neither.

The failure mode this avoids is the orphan: a candidate row with no evidence, or an artifact attached to nothing, either of which is invisible in the UI and permanent in the database. One transaction makes "cancel" mean cancel.

### Structured candidate data is evidence too

Classification reads the candidate's structured record — location, work rights, availability, compensation expectations — as well as their artifacts. Aspects derived from the record cite the record, not a chunk, and therefore carry recruiter supplied origin.

Ignoring the record would mean a candidate whose availability the recruiter typed produces a profile that does not know when they are available, and Phase 16 would then report it as missing information. The record is a source; it is simply a source with a different kind of citation, which Phase 10 already supports.

## Risks / Trade-offs

- **Computed staleness costs a hash per read.** Trivial now; if a future screen lists a hundred candidates' profiles it becomes a hundred hashes, and the counter-based cache is the fix.
- **The correspondence heuristic will mispair.** A conflict that should have been an addition and a removal is extra review work, and it is the failure direction that is visible rather than silent.
- **Version count grows with editing.** A recruiter who fixes ten aspects one at a time produces ten versions. Nothing prunes them, and at desktop scale nothing needs to — but a bulk-edit screen would want to batch, and does not exist yet.
- **Approval is all-or-nothing per version.** A recruiter who agrees with nineteen aspects and not the twentieth must edit or remove the twentieth before approving. That is the intended cost: a partly approved profile has no meaning downstream.
