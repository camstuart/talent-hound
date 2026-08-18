## Context

An optional cloud endpoint is the kind of feature that starts as an escape hatch and becomes the default runtime, one convenience at a time. The interesting design work is not making it work — it is making that drift impossible.

## Goals / Non-Goals

**Goals:**
- A deny list enforced in code, not configuration.
- Consent that cannot generalize across initiative, endpoint, or task.
- A preview that is the payload.
- Identifiers replaced before anything eligible leaves.

**Non-Goals:**
- No broad free-text PII detection. The PRD makes it P1 and calls the structured substitution the PoC scope; a half-working redactor would be worse than an honest boundary.
- No cloud embeddings, ever. Phase 9's registry already refuses a non-local embed endpoint, and this phase does not soften it.
- No provider-specific features. One OpenAI-compatible endpoint, because two is a matrix nobody has asked for.
- No default-on anything. Every task starts denied.

## Decisions

### The deny list is code, and there is no setting beside it

Raw candidate artifacts, Candidate Profile extraction, and candidate embeddings are refused by a function that takes a task and returns whether the cloud may do it. The refusal has no parameter.

A configuration flag would be a flag someone sets. A function with no way to say yes is a boundary that holds against code nobody has written yet, which is the only kind of boundary worth having — and the test asserts it by trying every task rather than by reading the list.

### Consent is a row keyed by all three, and nothing widens it

An approval is `(initiative, endpoint revision, task)`. Looking one up matches all three or finds nothing. There is no fallback to a broader approval, because the fallback *is* the generalization the PRD forbids.

An endpoint change produces a new revision, so approvals for the old one simply stop matching. Nothing sweeps or deletes them, and nothing needs to: they are approvals for a configuration that no longer exists.

*ponytail: stale rows accumulate at one per revoked configuration. On a single-user desktop that is a handful, forever, and pruning them would be work with no observable benefit.*

### The preview is the payload, again

`Preview(task, initiative)` builds the payload and returns it. `Send` takes that payload and transmits it unchanged. Same shape as Phase 14's query preview and the same reason: a preview built by one path and a request by another diverge the day someone adds a header.

Cloud chat previews every send rather than only the first, because a chat payload is different every time — the first-use preview would be about a message the recruiter has since replaced.

### Placeholders replace what is structured and known

Names, emails, phones, and addresses from the candidate record become `[name]`, `[email]`, `[phone]`, `[address]` in eligible payloads. The substitution reuses Phase 14's scrubber, which already knows the record's identifiers and the shapes.

What it does not do is find an identifier the record does not know about, spelled in a way the shapes miss. That is the P1 gap, and the honest form of it is that the recruiter previews the payload — which is the control that actually catches the case nobody anticipated.

### Every non-localhost request is audited on Phase 14's terms

The same table, the same prohibition, one more task value. There is no second audit mechanism, because a second one is a second place for content to leak into.

## Risks / Trade-offs

- **Structured substitution is narrow.** A name written differently in a document survives it. The preview is the backstop, and the PRD is explicit that broader redaction is P1.
- **Per-send chat previews are friction.** Deliberate: a chat payload is the least predictable thing this application would send, and predictability is what a one-time approval assumes.
- **Consent rows accumulate.** One per revoked configuration, on a single-user desktop, forever.
- **One endpoint only.** A recruiter with two providers configures one at a time. Two would be a matrix and a set of interactions nobody has asked for.
