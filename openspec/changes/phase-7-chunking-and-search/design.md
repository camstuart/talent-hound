## Context

Everything downstream of this phase cites. A match says "met, because of this"; a draft quotes; a recruiter checks. All of that resolves through a chunk, so a chunk has to be two things at once: a retrieval unit big enough to carry meaning, and a pointer precise enough that a person can find the same words in the original document.

Those two pull in opposite directions, and the resolution is the same one the PRD already chose: fixed structural chunking, no cleverness. Cleverness here is not a feature, it is a source of drift.

## Goals / Non-Goals

**Goals:**
- Same input, same parameters, same chunks — byte for byte, hash for hash, run for run.
- A stored offset that provably selects the stored text, checked at citation time rather than assumed.
- An FTS5 index that stays in step with chunks through every write path, and a rebuild that fixes it when it has not.
- Chunking that uses the Phase 5 job lifecycle, like everything else slow.

**Non-Goals:**
- No embeddings and no semantic similarity. Phase 9 owns those; embedding-similarity chunk boundaries are P1 in the PRD and this phase must not make them look imminent.
- No ranking beyond FTS5's own ordering. Reciprocal-rank fusion is Phase 15's, and inventing half of it here would mean throwing half of it away.
- No re-chunking sweep when the chunker version changes. Chunks record the version that made them; a later phase can find the stale ones.
- No cross-artifact deduplication. Two CVs that share a paragraph are two pieces of evidence with two provenances.

## Decisions

### The chunker is structural first, sentences second, and merges only within a heading

`structural-1` reads the Markdown once and produces blocks: an ATX heading, a fenced code block, a table, a list item with its continuation lines, or a paragraph. Blank lines separate blocks and belong to none. Then it packs consecutive blocks into chunks, greedily, left to right, while two conditions hold: the running token count stays under the target, and the heading path has not changed.

Packing matters more than it looks. Without it a heading is its own chunk — three tokens, matching nothing, retrievable by nobody — and the paragraph it introduces loses the one line that says what it is about. A heading takes the path that *includes itself*, so it groups forward with the text beneath it rather than backwards with the section above.

Splitting only happens when a single block is over the maximum. Then it segments into sentences and packs those the same way. A sentence is never split: half a sentence is a citation nobody can read, and a pathological unpunctuated block producing one oversized chunk is a better failure than a chunk that starts mid-clause.

Sentence segmentation is a small deterministic scanner over `.`, `!`, `?` followed by whitespace, with a fixed list of abbreviations, single-capital initials, and decimals excluded. It is not a language model and it is not correct in general; it is correct on CVs and job ads, which is what it is for, and it is the same wrong in every run.

### A chunk's offsets are the contract, and the text is the proof

`markdown[startOffset:endOffset] == text`, always. That is why nothing is trimmed, normalized, or rewritten on its way into a chunk: the moment the stored text differs from the source slice, an offset becomes a claim rather than a fact, and every later citation inherits the doubt.

Offsets are byte offsets into the extracted Markdown, not rune offsets. The Markdown is stored as bytes and the slice that has to match is a byte slice; converting to runes would put a second encoding assumption between a citation and its source for no benefit anyone can see.

Citation resolution checks this at runtime rather than trusting it. `Cite` re-reads the artifact's Markdown, takes the slice, and compares it with both the stored text and the stored hash. If the artifact was re-extracted by a different extractor version and the chunks were somehow not discarded, the citation fails loudly instead of pointing confidently at the wrong sentence.

### Token counts are word counts, and say so

A chunk records a token count because the PRD says so and because Phase 9 will need to keep chunks under an embedding model's context limit. Nothing in this phase can compute a real token count — the tokenizer belongs to a model that has not been chosen yet.

So the count is whitespace-separated words, the parameters are recorded in the chunker parameters JSON alongside it, and the chunker version is what changes when a real tokenizer replaces it. Calling it a token count and quietly counting words without recording which is how the boundary between "approximately 200 tokens" and "over the model's limit" gets discovered in production.

### Derived rows never outlive the text they came from

Chunks are derived data with a provenance chain: bytes → Markdown → chunks. Break a link and everything below it is a lie, so re-extraction deletes the artifact's chunks in the same transaction that writes the new Markdown. Not a background cleanup, not a version comparison at read time — the same transaction, because a chunk that survives its Markdown by even a moment is a chunk something could cite.

That also settles the mixed-version question the plan raises: re-chunking is delete-then-insert for the whole artifact, never a merge, so an artifact's chunks are always the product of exactly one chunker version and one set of parameters.

### The FTS5 index is external-content, kept by triggers, and rebuildable on demand

`chunks_fts` is `content='chunks', content_rowid='id'`: the text lives once, in `chunks`. Three triggers — after insert, after delete, after update — keep the index in step, which is what makes it correct through paths that never go near the search service, including a rollback (the triggers are inside the transaction, so the index rolls back with the rows) and a bulk delete during re-extraction.

An external-content table can still drift if something writes to `chunks` outside SQLite's notice, so the rebuild path is exposed rather than hidden: `Rebuild` runs FTS5's own `'rebuild'` command, and a disk-backed test asserts the same query returns the same hits either side of it. A rebuild path that has never been run is not a rebuild path.

### A query is a bag of quoted terms, never FTS5 syntax

FTS5's `MATCH` takes an expression language with operators, prefixes, column filters, and `NEAR`. A recruiter typing `senior engineer (contract)` is not writing an expression, and handing their text to `MATCH` unmodified means an unbalanced parenthesis is a database error and a stray `NOT` silently changes what they searched for.

So the query is split on anything that is not a letter, digit, or underscore — the same rule FTS5's own tokenizer uses — and each term becomes a quoted FTS5 string, joined implicitly, so every term must appear. An empty query returns nothing rather than everything. That gives up phrase and prefix search, which nothing in the PoC asks for, and buys a search box where every possible input is a search rather than a syntax error or an injection.

It also makes the test for this readable in a way that took a moment to see. `text:Melbourne` returns *nothing* — and that empty result is the proof, because as a column filter it would have matched. Every operator-shaped query lands the same way: `NEAR`, `NOT`, and `text` are words, this document does not contain them, so nothing comes back. The assertion that the input was searched rather than obeyed is an assertion that it found nothing.

### Search is scoped to the workspace that asked

A hit is scoped to the artifacts linked to the initiative — the same set the artifacts panel shows as attached. Searching every chunk in the database would put one candidate's CV in another engagement's results, which in a recruiting tool is not a bug about relevance.

*ponytail: the scope is the initiative's own links; widen the subquery to the initiative's candidate and roles when those pipelines exist and their evidence needs finding from here.*

### Chunking is a job with one item per artifact

Extraction is one artifact per job because one document is one process. Chunking is pure computation over text already in the database, so a job carries a list of artifact identifiers and does one per item — which is what makes the cancellation guarantee real rather than vacuous: cancel a run over eight artifacts after three, and three artifacts are chunked, five are untouched, and the fourth left nothing behind.

Params stay identifiers only, as the Phase 5 rule requires: a list of artifact IDs and nothing else.

### Snippets are untrusted, like the Markdown they came from

A search result is a slice of a document a stranger wrote. It goes into a `<pre>`, exactly as Phase 6's extraction view does, for exactly the same reason: this application has no Markdown renderer, and a phase that adds one has to argue for it on its own terms.

## Risks / Trade-offs

- **Word counts are not token counts.** A chunk recorded as 200 tokens may be 260 to a real tokenizer. The chunker version and its parameters are recorded so Phase 9 can decide whether it needs a re-chunk, and the target is conservative enough that the common case has room.
- **An unpunctuated block over the maximum produces one oversized chunk.** Real CVs do not do this; a table dumped as one line might. The alternative — splitting mid-sentence — damages every citation into that chunk, which is worse than one chunk being large.
- **Search is term-AND with no phrase support.** A recruiter cannot search for an exact title. Adding phrase support means either a syntax the search box has to teach or a heuristic about quoting; neither is worth it before there is a recruiter to annoy.
- **Chunking is not automatic after extraction.** The recruiter asks for it, because chaining two job kinds together needs a rule about what happens when the second fails and the first succeeded, and this phase has no reason to invent one. Phase 9 chains embedding onto chunking and can settle it for both.
