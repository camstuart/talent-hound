## Context

A vector is meaningless on its own. `[0.31, -0.07, …]` is a claim about a sentence only relative to the model that produced it, and two models produce two incompatible geometries that are indistinguishable as bytes. Every serious failure available in this phase is the same failure: comparing numbers that were never in the same space, and getting an answer, because cosine similarity is defined for any two float slices of equal length.

So this phase is mostly bookkeeping, and the bookkeeping is the feature. The arithmetic is eight lines.

## Goals / Non-Goals

**Goals:**
- An embedding space that is a stored identity, not a convention.
- Vectors that round-trip exactly, and refuse to be read as anything but what they are.
- Exact cosine over a corpus of realistic size, measured rather than assumed.
- A comparison across spaces that cannot be expressed, rather than one that is merely discouraged.
- Candidate content embedded locally under every configuration, proven by a cloud endpoint that receives nothing.

**Non-Goals:**
- No hybrid fusion with FTS5. Phase 15 owns the combination and its weighting; doing it here would settle a ranking question with no shortlist to judge it against.
- No approximate index, no vector extension, no dimensionality reduction. The PRD makes these conditional on a measurement this phase produces.
- No Profile Aspect embeddings yet — the storage shape admits them, but Phase 10 is what creates an aspect worth embedding.
- No re-embedding sweep. A model change makes old vectors non-current; deciding when to re-embed the corpus is a recruiter-facing choice, and there is no screen for it until there is something to lose by re-embedding.

## Decisions

### A space is identified by what changes its geometry, and the revision is the anchor

`embedding_spaces` holds endpoint, model, digest, the embed assignment revision, dimensions, metric, and normalization, with a unique index across all of them. Two vectors are comparable if and only if they name the same space row.

The assignment revision is doing the real work. Endpoint and model and digest are the visible identity, but a parameter change that alters nothing visible still alters the geometry, and Phase 8 already made "the configuration changed" into a durable, append-only number. Naming that number here means the two phases cannot disagree: a revision exists exactly when something about the embed configuration changed, and a space exists exactly per revision.

Digest is recorded but not required. Ollama does not always report one, and refusing to embed because the endpoint was terse would trade a real capability for a field. Its absence is stored honestly as an empty string, and it does not make two configurations one — the revision already separated them.

### Dimensions are learned, then enforced

Nothing declares how many dimensions a model produces; the first successful embedding does. The space is created with the dimension count that first response carried, and from then on every vector for that space must match it exactly, on write and on read.

The alternative — a configured dimension count — is a second source of truth about a fact the endpoint already knows, and the failure mode of getting it wrong is silent: a shorter vector decodes fine and compares fine and means nothing. Learning it once and enforcing it forever makes the mismatch loud at the earliest possible moment.

### The wire format is little-endian float32, and the length is the check

`math.Float32bits` and `binary.LittleEndian` — the standard library, in both directions, with no framing, no header, and no version byte. A blob of 4×N bytes is N float32s and nothing else.

Skipping the header is deliberate and safe here because the blob never travels alone: it is a column in a row that names its space, and the space carries the dimensions. A length that is not 4×N, or an N that is not the space's dimension count, is refused at the boundary rather than truncated or padded. That check is the entire integrity story, and it is exact — there is no encoding in which a corrupted length passes.

*ponytail: no compression, no quantization. A 1024-dimension vector is 4KB, a realistic corpus is a few thousand of them, and 16MB of blobs is not a problem worth a format for. Quantize when the measurements say the scan is memory-bound.*

### Cosine is written out, and the degenerate cases are decisions

```
score = dot(a, b) / (norm(a) * norm(b))
```

in float64 accumulators over float32 inputs, because summing a thousand float32s in float32 loses more than the extra byte costs.

A zero vector has no direction, so its similarity to anything is undefined, and the choice here is to refuse it at storage time rather than to return zero at query time. A model that returns an all-zero embedding has failed, and storing that failure as a legitimate vector means every future query silently scores it — always identically, always meaninglessly. NaN and infinity are refused for the same reason and more urgently: one non-finite component poisons the norm, and every score in the result set becomes NaN, which sorts unpredictably.

Similarity is clamped to [-1, 1] on the way out. Floating-point accumulation can produce 1.0000000000000002 for a vector compared with itself, and a score outside the defined range is a number that later phases will use in arithmetic that assumes the range.

### Ties break by identifier, always

Scores collide — identical text embeds identically, and two chunks of boilerplate are genuinely equally similar to anything. Sorting by score alone leaves those in whatever order the scan produced, which is stable in practice and not stable by contract, and "the shortlist changed and nothing changed" is an expensive thing to debug in Phase 15.

So the order is `score descending, then embedding id ascending`, and it is part of the requirement rather than an implementation detail.

### The scan is exact, in Go, over rows filtered first

Retrieval loads the candidate vectors for one space and one owner kind, scoped to the initiative, and scores every one. No index, no approximation, no early termination.

The filter matters more than the arithmetic. Scoping by initiative and space happens in SQL, so the scan runs over the rows that could possibly match rather than over the table; a corpus of a few thousand chunks per initiative is a few million float multiplications, which is microseconds. The measurement exists to catch the day that stops being true, and until it does, an ANN index is a correctness risk and an extra dependency bought with nothing.

*ponytail: full scan, single-threaded, vectors loaded per query. Parallelize across cores first, cache the matrix second, add an index only if the recorded numbers miss the PRD threshold.*

### A comparison across spaces cannot be expressed

`Search` does not take a vector. It takes text, embeds it with the current embed assignment, resolves that to a space, and scans within it. There is no exported function anywhere that accepts two vectors and returns a similarity without a space to agree on.

This is the difference between a rule and a guarantee. A `Cosine(a, b []float32)` on the service surface is one convenient call site away from comparing last month's vectors with this month's, and the result of that call looks entirely normal. The arithmetic still exists — in `internal/vector`, over raw slices, where it is a numerical function and the caller has already established what the numbers mean.

### Vectors do not outlive their source, and the deletion is in the same transaction

Replacing an artifact's chunks deletes the embeddings of those chunks, in the transaction that does the replacing — the same rule Phase 7 applied to chunks when Markdown changes, for the same reason. A vector whose chunk is gone is not stale data, it is a retrieval result that cannot be cited.

A model change is the other direction and is treated differently: the old vectors stay. They are complete, they were correct, and re-embedding is minutes of local compute that the recruiter has not asked for. They are simply outside the current space, so retrieval does not see them, and the settings view can say how many units the current space is missing.

### A partial vector is never written

The job's compute half embeds one retrieval unit and returns the bytes; the commit half writes them. A cancellation between the two writes nothing, and a provider failure returns a coded reason and writes nothing. There is no path in which half a vector, or a vector with the wrong space, reaches the table.

The unique index on (space, owner kind, owner id) is the second half of the same guarantee: a retry cannot produce a duplicate, because the write is an upsert into a slot that can hold exactly one vector.

### Locality is proven by absence

The embed role already cannot name a cloud endpoint — Phase 8's registry refuses it. This phase adds the proof that nothing routes around the registry: a fake cloud endpoint stands up, the full embedding path runs over candidate chunks under every configuration the test can produce, and the assertion is that the cloud endpoint recorded zero requests.

An assertion about what a call site does is an assertion about the code that exists today. An assertion that a server received nothing holds against code nobody has written yet.

## Risks / Trade-offs

- **Exact scan is O(corpus).** Deliberate, measured, and reversible; the numbers go in the gate evidence, and the PRD threshold is the trigger for anything cleverer.
- **A model change strands the corpus.** Retrieval quietly narrows to whatever has been embedded in the new space. The settings view reports the shortfall, but nothing automatically fixes it, and a recruiter who changes models mid-initiative gets worse results until a re-embed they have to ask for.
- **Dimensions are learned from the first response.** An endpoint that returns a wrong-length vector once, before any other, defines the space wrongly and every correct vector afterwards is refused. Loudly, which is the mitigation — the alternative failure is silent.
- **No fusion yet.** Semantic and lexical search are two controls with two result lists until Phase 15, which is a worse experience than the eventual one and a much better one than a weighting scheme chosen with nothing to evaluate it against.
