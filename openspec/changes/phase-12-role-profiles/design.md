## Context

This phase is mostly Phase 11 with the approval removed, and the interesting question is what fills the hole. Approval was doing two jobs: making a person responsible for the content, and marking a moment the evidence was known-good. Roles need the second without the first.

The answer is that Ready *is* the moment. A Role Profile is Ready when it was derived from the role's current source content and validated; it stops being Ready when that content moves. Nobody signs it, and nothing pretends anybody did.

## Goals / Non-Goals

**Goals:**
- Automatic creation with no approval step, and no screen that asks for one.
- Three states that are all visible, including the two that mean "do not assess this".
- One eligibility call, matching the candidate gate's shape.
- Editing that produces versions and never touches a source artifact.

**Non-Goals:**
- No approval, ever, for roles. Adding it "for safety" would reintroduce exactly the workflow cost the PRD rejected.
- No automatic retry of a failed decomposition. A failure that retries itself in the background is a failure the recruiter never sees, and the PRD requires it to stay visible.
- No matching. Phase 16 consumes Ready profiles; this phase decides which are Ready.
- No role discovery. Phase 14 finds roles; this phase profiles whatever roles exist.

## Decisions

### Ready is derived, not stamped

A role profile is Ready when its state is not failed and its source hash equals the role's current source hash. Stale is the same comparison coming out false. Failed is the contract's own outcome.

This is the Phase 11 staleness argument with the approval removed, and removing the approval makes it simpler rather than harder: there is no "approved about *what*" to remember, because the version records the sources it was made from, and that is the only thing to compare. Nothing is stored that could be out of date.

### Failed and Stale are visible, and visible is a requirement rather than a courtesy

Both are returned by the listing the Research area renders, both carry their reason, and both offer actions — retry and manual entry for Failed, retry for Stale.

The failure mode is specific: a role whose decomposition failed, if hidden, is indistinguishable from a role that was never discovered, and the recruiter's response to those is completely different. So the listing shows every role's profile state, and a role with no profile at all is its own visible state rather than an absence.

### Manual entry is the same path as candidates

A role whose listing cannot be decomposed gets aspects typed by the recruiter, with recruiter supplied origin, citing the role record. Phase 10 built that; Phase 11 proved it on candidates; nothing here is new except that the subject is a role.

Role aspects entered by hand may carry a priority, which candidate aspects may not — that asymmetry is the taxonomy's, and it is the only difference in the call.

### An edit never touches the artifact

Editing an aspect writes a new profile version. It does not rewrite the role's source content, and it cannot: artifacts are immutable from Phase 4, and the profile is derived data that may disagree with its source.

That disagreement is a feature. A recruiter who knows the listing's "5+ years" is negotiable can say so in the profile, and the citation still points at the listing saying five, which is exactly the account a person should see.

### Eligibility is one call, and it says why not

`Eligibility(roleID)` returns whether the role may be assessed automatically, and the reason when it may not. Same shape as `Readiness` for candidates, for the same reason: Phase 16 asks about both sides of a match, and two differently-shaped answers would grow two code paths that drift.

Unlike candidates, there is no "eligible with a warning" case. A stale role profile is not assessed, because unlike a candidate — whom the recruiter knows and can vouch for from memory — nobody has any independent knowledge of a listing that changed. Reprofiling a role is one cheap local call, so the correct action is to do it rather than to warn.

*ponytail: no automatic reprofile on staleness. The Research view offers the button; a background sweep is Phase 14's problem if role content starts changing under people.*

## Risks / Trade-offs

- **Nothing guarantees a Role Profile is right.** That is the deliberate trade: automatic decomposition of twenty listings, with citations, editable, versus a workflow nobody would use. The citation requirement is what makes it checkable when a match turns on it.
- **Stale roles drop out of assessment silently unless the recruiter looks.** Mitigated by the state being on every row in the Research view, and by reprofiling being one click. A notification is a Phase 20 concern.
- **Two lifecycles now exist.** Candidates have approval and a usable-stale state; roles have neither. The asymmetry is real and stated, and the single-call eligibility shape on both sides is what keeps Phase 16 from having to know why.
