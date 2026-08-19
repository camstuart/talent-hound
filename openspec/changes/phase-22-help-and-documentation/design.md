## Context

Help is used when the application is not working. That is the whole design constraint: every other feature may assume a model, a data folder, and an indexed corpus, and help may assume none of them.

## Goals / Non-Goals

**Goals:**
- Answers available on first launch, offline, with no model installed.
- Every topic reachable without knowing what to search for.
- A tutorial that follows the product's actual flagship loop.
- An AI answer that is a bonus, cited, and refusable.

**Non-Goals:**
- No hosted documentation, no telemetry on what was searched, no "was this helpful" call home. The application has no outbound transport and help is not the place to grow one.
- No embeddings for help. The corpus is a few dozen sections shipped with the binary; a term index answers in microseconds and needs no model, and a cosine scan would need the embed model help exists to explain.
- No editable help. Recruiter-authored notes belong to records, not to the manual.

## Decisions

### The corpus is Markdown in the binary

Articles live in `internal/help/content/` and are embedded. They are split into sections at their headings, and a section is the retrieval unit — the same choice Phase 21 arrived at for role aspects, for the same reason: a whole article matches everything, a section answers one question.

### Ranking is BM25 over a term index, built at startup

A few dozen sections is a small enough corpus that the index is built once, in memory, in a millisecond. BM25 because it is the same family as the FTS5 ranking the product already uses for evidence, so a section that mentions a term twice does not beat one that is about it.

*ponytail: no stemming, no synonyms. "Deleting" and "delete" are handled by matching prefixes of at least four characters, which costs one line and covers English morphology well enough for a manual.*

### The answer is optional and cited, or it is not given

When a generate model is assigned, the question and the retrieved sections go to it under the same contract Phase 17 uses for Q&A: answer only from what you were given, cite the sections used, and say plainly when they do not cover it. When no model is assigned, the search results are the answer, and the interface says why there is no written answer rather than showing an empty box.

An answer that cannot cite a section is not shown. Help that invents an instruction is worse than help that says "these three sections are the closest I have".

### Nothing leaves the machine, including the question

The query is matched in memory. When a model answers, it is the local model. There is no help endpoint, no analytics, and no "was this useful" — the transport check that guards the rest of the product covers this by construction.

## Risks / Trade-offs

- **A term index misses paraphrase.** "How do I stop it emailing candidates" will not match "the application cannot send messages" on terms alone. The topic index is the mitigation: everything is reachable without searching, and the tutorial covers the flagship path in order.
- **Shipped content goes stale.** The articles describe behaviour the tests pin — deletion rules, the cloud boundary, what is never sent — so a change that contradicts them breaks a test rather than the manual quietly lying.
- **No feedback signal.** Deliberate: there is nowhere for it to go that would not be telemetry.
