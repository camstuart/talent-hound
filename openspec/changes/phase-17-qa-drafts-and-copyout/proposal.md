## Why

The recruiter now has profiles, criteria, roles, a shortlist, and assessed matches. What they do not have is the last mile: asking a question about what they are looking at, and turning an answer into something they can paste into an email.

This phase closes that loop, and it closes it *locally*. The application drafts; the recruiter sends. That division is not a limitation to work around later — it is the product's position. An application that could send outreach on a recruiter's behalf is an application that can make a mistake nobody caught, in someone's name, to a person who did not ask to hear from it. So there is no sender, and a test proves there is none.

The two model-facing rules are the same ones from earlier phases, restated where they now bite. Chat can *propose* a criterion or an aspect and can never apply one, because an interface that lets a conversation change stored intent is an interface where a stranger's document can change stored intent. And a factual answer cites evidence or says it does not know, because the alternative to "I cannot find that" is invention.

## What Changes

- Add initiative-scoped question answering over approved local context, with retrieval that cannot reach another initiative or unapproved evidence.
- Require citations for factual answers, and return an explicit unknown when the evidence does not support one.
- Let chat propose structured changes — criteria, aspects — that the recruiter applies explicitly or not at all.
- Generate editable pitch and outreach drafts whose factual claims map to evidence.
- Keep drafts in one of two states, Active or Discarded, and let them be edited and copied repeatedly.
- Record every copy as a metadata-only CopiedOut audit event carrying no draft text.
- Prove, by a repository check and a runtime test, that no outreach transport exists.

## Capabilities

### New Capabilities
- `scoped-answers`: what a question may see, what an answer must cite, and what happens when it cannot.
- `chat-proposals`: what a conversation may suggest and what it may never do.
- `outreach-drafts`: the draft lifecycle, evidence mapping, editing, and copying.
- `copy-out-audit`: what a CopiedOut event records and what it must never contain.
- `no-transport`: the absence of any sender, asserted rather than assumed.

### Modified Capabilities
None. Q&A and drafts read what earlier phases produced; nothing they read changes.

## Impact

- `internal/db/migrations.go`: migration 15 — `drafts` and `answers`, and the CopiedOut task on the existing audit table.
- New `qaservice.go` — scoped retrieval, the answer contract, citation validation, and proposals that write nothing.
- New `draftservice.go` — generation, editing, the two-state lifecycle, copy events, and the evidence map.
- `frontend/src/components/AskPanel.tsx` and `DraftsPanel.tsx` — the question box with citation navigation, the proposal list with an explicit apply, the draft editor, copy, and discard.
- A transport check: a repository scan for sender libraries and protocols, plus a runtime assertion that nothing dials a mail or messaging port.
- Injection fixtures: text inside an artifact cannot change retrieval scope, apply a proposal, reach a provider, delete anything, or cause a copy.
