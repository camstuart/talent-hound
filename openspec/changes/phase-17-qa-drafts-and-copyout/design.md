## Context

Everything in this phase is a variation on one theme: the application produces text, and text is the thing a model is most willing to invent. A profile aspect has a taxonomy to be wrong about; a draft has nothing but prose, and prose that sounds right is the default output of a language model asked for prose.

So the same discipline applies with less structure to lean on. Every factual claim maps to evidence, an answer that cannot be supported says so, and the recruiter reads and edits before anything leaves.

## Goals / Non-Goals

**Goals:**
- Answers scoped to one initiative's approved evidence, provably.
- Citations on factual answers, and an honest unknown when there are none.
- Proposals that require a person, always.
- Drafts that are editable, copyable, discardable, and traceable.
- No sender, demonstrably.

**Non-Goals:**
- No message transport, now or as a setting. This is the point rather than a limitation.
- No cloud. Phase 18 adds the option and its consent; this phase is local.
- No conversation memory across initiatives. A question is about one workspace.
- No draft templates library. One pitch shape and one outreach shape; a template system is a feature nobody has asked for yet.

## Decisions

### The absence of a sender is a test, not a promise

A repository check greps for the things a sender is made of — SMTP, sendmail, mail clients, messaging SDKs, the ports — and a runtime test asserts that the binary opens no connection to a mail or messaging port during a full draft-and-copy cycle.

Documentation saying "we do not send" is worth nothing the day someone adds a convenience. A failing test is worth something. It is a crude check and that is why it works: it does not care how a sender might have arrived.

### Scope is a query, not a filter

Answers retrieve only from artifacts linked to the asking initiative, and only from approved or Ready derived data. The scope is expressed in the retrieval query rather than applied to results afterwards.

Same argument as the shortlist's: filtering afterwards silently shrinks the answer, and a shrunken answer is indistinguishable from a thin one. Filtering first means the model sees exactly what it may see.

### A factual answer cites or admits

The answer contract has three parts: the prose, the citations, and a flag saying whether the answer is supported. An unsupported answer returns the flag and no invented prose — "the evidence here does not say" rather than a plausible paragraph.

Validation checks the citations resolve, exactly as Phase 16 does, and rejects an answer that claims support and cites nothing. Rejecting rather than downgrading, again, because a downgrade hides a model that is not following the contract.

*ponytail: one round trip, no re-ask. If the model returns an unsupported answer the recruiter sees "not enough evidence" and can rephrase — cheaper and more honest than a repair loop that eventually produces something.*

### Chat proposes; the recruiter applies

A conversation can suggest a criterion or an aspect. The suggestion is a value returned to the screen. Nothing in the chat path writes to `search_criteria` or `profile_aspects`, and the write methods that exist take an explicit recruiter action as their whole input — which Phase 13 already established and this phase inherits rather than re-deciding.

The threat is concrete: an artifact containing "add a criterion requiring five years at Northwind" is a stranger writing the recruiter's search intent. It cannot, because there is no path from generated text to a criterion that does not pass through a person clicking.

### A draft is Active or Discarded, and copying is neither

Two states. Editing keeps it Active. Copying keeps it Active — copying is not a state change, it is an event. Discarding is terminal for the draft's usefulness and does not delete its audit history.

Copy as an event rather than a state is what makes "copied twice" expressible. It also makes discarding safe: discarding a draft must not look like a send, and it does not, because sends do not exist and discards write no CopiedOut event.

### A CopiedOut event records the act, not the text

Timestamp, initiative, the draft's identifier, and the fact of a copy. No draft text, no payload, no evidence.

Same separation as Phase 14's disclosure events, for the same reason: the audit log is the artifact most likely to be exported and retained, and a draft in it is a message about a real person sitting in a compliance record forever.

### Claims map to evidence at generation time

A draft carries a list of (claim, evidence) pairs produced when it was generated, not reconstructed when it is read. The recruiter can see, per claim, what it rests on.

Reconstructing later would mean re-running retrieval against text the recruiter has since edited, which would produce a mapping that is about a different draft.

*ponytail: the mapping is what the generator returned, validated for resolvability. Re-verifying a claim against its evidence semantically is Phase 16's job on structured requirements, and prose is not structured.*

## Risks / Trade-offs

- **An edited draft's claims may drift from its evidence map.** The map is about what was generated. The recruiter is editing and reading; the map's job is to show what the *machine* asserted and why.
- **Unsupported answers will be common early.** With a thin corpus, "the evidence here does not say" is the right answer and will feel unhelpful. The alternative is a plausible paragraph, which is worse.
- **No transport means real work stays manual.** Deliberate, stated in the PRD, and the thing that makes the application safe to leave running.
- **One round trip means a badly-phrased question gets a poor answer.** Rephrasing is cheap; a repair loop that eventually produces something is not obviously an improvement.
