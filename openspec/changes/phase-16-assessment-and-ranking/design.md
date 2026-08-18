## Context

Everything before this produced evidence. This produces conclusions, and a conclusion is a different kind of object: it can be repeated to a client, it can be wrong in a way that costs someone an interview, and it cannot be checked by looking at it — only by looking at what it cites.

That last property is the design constraint. The application cannot tell whether a `met` is correct. It can tell whether it is *cited*, and whether the citation resolves, and that is the whole of what validation can honestly do.

## Goals / Non-Goals

**Goals:**
- Two directions, kept apart, each with its own evidence.
- Every result traceable to something a person can read.
- Structured facts compared by code, not by a model.
- One hash that decides reuse, covering everything that could change the answer.
- Ranking that is a total order, stated once, tested tie-break by tie-break.

**Non-Goals:**
- No score. The PRD ranks by counts of met and unmet, not by a number, and a number would invite comparison across matches that means nothing.
- No automatic re-assessment. Staleness is visible; recomputing is the recruiter's call, because it costs minutes of local compute.
- No hiding. A match with unmet must-haves sorts down and stays visible.
- No cloud. Phase 18 adds the option; this phase runs entirely on the local `generate` role.

## Decisions

### Similarity selects evidence and never becomes a result

KNN retrieves the candidate aspects and chunks most likely to bear on a requirement. The model then reads them and says met, not met, or unknown. The similarity score is not carried into that decision, is not shown as a confidence, and is not part of the stored result.

This is the difference between a system that finds evidence and one that guesses. A cosine of 0.91 between "led a platform team" and "managed a data team" is a reason to *look*, not a finding — and a design that let the score decide would produce confident results whose confidence came from an embedding model that has never seen a résumé.

The adversarial test is the honest form of this: evidence engineered to score extremely high, whose text does not support the requirement, must not produce `met`.

### A `met` without a citation is refused, not downgraded

Validation rejects a `met` that cites nothing, or that cites something that does not resolve. It does not silently turn it into `unknown`.

Downgrading would be the friendly option and it is wrong, because it hides a model that is not following the contract. The whole result set is refused, the way Phase 10 refuses a whole proposal, and the recruiter sees a failure they can retry rather than a quietly weakened answer.

`not met` cites contrary evidence *when available*, and says so explicitly when it is not — because "the résumé says the opposite" and "the résumé is silent" are different things to tell a candidate.

### Structured constraints never reach the model

Location, work arrangement, work rights, employment type, and compensation are compared by code: normalized value against normalized value, with `unknown` on either side producing `unknown` rather than a guess.

A model asked whether "Melbourne" satisfies "Melbourne, VIC" will usually say yes and occasionally say no, and the occasional no is unexplainable. Code that compares two normalized values is boring, right every time, and produces the same answer on every machine.

### The hash is canonical, and canonical means the serialization is defined

Every input the PRD lists goes into one hash: both profile versions and states, the criteria version, the content hashes of the evidence, the structured-comparison and ranking-rule versions, the endpoint revision, the model digest, the prompt and schema versions, the generation parameters, and the role's staleness.

Serialization is explicit and ordered — sorted keys, fixed field order, length-prefixed strings — because Go's map iteration is randomized and a hash that depended on it would change between runs of the same binary. The test that matters runs the hash twice in one process and once in a subprocess, and asserts all three agree.

Presentation-only changes are absent by construction: criterion *order* is not an input, because Phase 13 already decided ordering is not weighting.

### The hash is the only caching rule

There is no timestamp, no TTL, no "assessed recently". A stored result is reused when its recomputed hash matches and not otherwise.

Every other rule is a rule that will eventually be wrong in a way nobody notices. The hash is wrong only if an input is missing from it, which is a bug with a test: each input is changed one at a time and must invalidate.

### Ranking is a comparator, written once

The six-step order from the PRD is one function. Each step is tested alone, and combinations are tested together, ending with role identifier so the order is total.

A comparator rather than a score because the steps are lexicographic — no number of met nice-to-haves compensates for an unmet must-have — and expressing lexicographic order as a weighted sum requires weights that are lies.

### Cancellation keeps what finished

Each role's assessment is independently valid under its own hash, so a cancelled batch keeps the roles it completed. The role in flight leaves nothing.

This is Phase 5's compute/commit split applied to something expensive enough that it matters: cancelling after eight of twenty roles should keep eight results, not discard them.

*ponytail: one role per job item, no intra-role parallelism. The local model is the bottleneck and it serializes anyway.*

## Risks / Trade-offs

- **The model can still be wrong within the contract.** A cited `met` whose citation is real but misread is not detectable here. The citation is what makes it checkable by a person, which is the honest ceiling.
- **Assessment is slow.** Twenty roles times a dozen requirements times two directions is a lot of local inference. The job is cancellable and results are cached by hash; Phase 21 measures it.
- **The hash is conservative.** Changing a generation parameter invalidates everything, including results that would not have differed. Correct direction: a stale-looking recompute costs compute, a wrongly-reused result costs trust.
- **Unspecified requirements are assessed but do not rank.** They appear with their result and count for neither must-have nor nice-to-have, exactly as the PRD says, which will occasionally surprise someone reading the order.
