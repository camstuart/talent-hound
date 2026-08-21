# Talent Hound — PoC acceptance record

**Status:** IN PROGRESS. Everything that can be proved off the target laptop is
proved and passes in `just check`. Everything that needs the Windows 11 x64
laptop, live local models, or the recruiter's real frozen corpus reads NOT RUN
until it has been run there.

A gate reads PASS only when it was actually run in the environment it names. A
green suite on a development machine is not evidence about the target laptop and
is not recorded as if it were.

## How to produce the outstanding rows

```
just check                     # the whole suite, anywhere
just gate                      # Windows-native platform proofs (TH_SIDECAR_EXE)
just gate-model                # live local-model proofs against Ollama
just gate-model-classify       # the classifier contract against the selected model
just bench                     # the frozen classifier and matching benchmarks
wails3 task package            # the Windows application and installer
```

`just bench` writes a record to `docs/product/benchmarks/`. Every record carries
the model digests, prompt and schema versions, corpus hash, and whether the
corpus was the synthetic one in this repository or the recruiter's.

## Functional gates

| # | Gate | Where it is proved | Result |
| --- | --- | --- | --- |
| FR-01 | Initiatives: lifecycle, tabs, archive, reopen | Go, Vitest, Playwright | PASS |
| FR-02 | CRM records: validation, persistence, shared references | Go, Vitest, Playwright | PASS |
| FR-03 | Artifacts: bytes, provenance, links, immutability | Go, Playwright | PASS |
| FR-04 | Extraction, chunking, FTS, embeddings, cosine retrieval | Go, Playwright | PASS |
| FR-05 | Candidate and Role Profiles, aspects, approval, staleness | Go, Vitest, Playwright | PASS |
| FR-06 | Q&A: scope, citations, unknown answers, injection | Go, Playwright | PASS |
| FR-07 | Role discovery: previews, identifiers, staleness | Go, Vitest, Playwright | PASS |
| FR-08 | Jobs, criteria, shortlist, assessment, ranking | Go, Vitest, Playwright | PASS |
| FR-09 | Drafts, copy-out, audit, no transport | Go, Vitest, Playwright | PASS |
| FR-10 | Model registry and cloud allow/deny | Go, Vitest, Playwright | PASS |
| FR-11 | Credential lifecycle and secret absence | Go (Windows store: gate) | PARTIAL — the Windows credential store is a gate |
| FR-12 | Deletion invariants, rollback, verification | Go, Vitest, Playwright | PASS |
| FR-13 | Encryption gate, first run, migrations, recovery, diagnostics | Go, Vitest, Playwright | PARTIAL — BitLocker and installer are gates |

## Additional PRD gates

| # | Gate | Result |
| --- | --- | --- |
| 1 | Work offline: CRM, artifacts, profiles, retrieval, Q&A, generation | NOT RUN — needs the laptop with models installed |
| 2 | No message-sending capability | PASS — repository-wide sender scan, plus no service method |
| 3 | Purge a stale role and verify its derived content is gone | PASS — Go and Playwright |
| 4 | Delete a candidate after its initiatives, resolving shared artifacts | PASS — Go and Playwright |
| 5 | Every non-local request had its preview or approval | PASS — audit and consent tests; confirm again on the acceptance run |
| 6 | Held-out matching benchmark | **PASS on the development machine** (5 of 5 scenarios against real role profiles, 14B) — NOT RUN on the target laptop |
| 7 | Held-out classifier benchmark | **PASS on the development machine** (all four conditions, 14B) — NOT RUN on the target laptop |
| 8 | Real-data mode refused on an unencrypted volume | PARTIAL — enforced and tested; BitLocker itself is a gate |
| 9 | Recover a copied data folder without corruption or partial migration | PASS off-laptop — second-machine run is a gate |

## Benchmarks

These rows are the acceptance environment: the Windows laptop, the pinned
models, and the recruiter's frozen corpus. A development-machine run is
recorded further down and cannot fill them in.

| Benchmark | Bar | Result |
| --- | --- | --- |
| Classifier: every aspect cited | all | PASS on the development machine; NOT RUN on the target laptop |
| Classifier: no unsupported critical constraint | none | PASS on the development machine; NOT RUN on the target laptop |
| Classifier: material-aspect capture | ≥ 80% | 99% on the development machine; NOT RUN on the target laptop |
| Classifier: structured constraints reproduced | all | **100 of 100** on the development machine; NOT RUN on the target laptop |
| Matching: three plausible in the top five | ≥ 4 of 5 scenarios | **5 of 5** on the development machine against real role profiles; NOT RUN on the target laptop |
| Live acceptance: eligible roles found | ≥ 10, else inconclusive | 20 in scope |
| Live acceptance: Ready profiles and assessments | ≥ 10 | NOT RUN |
| Live acceptance: usable evidence-backed draft | ≥ 1 | NOT RUN |

### What the passing run does not settle

Two runs of the frozen corpus differed on `platform-engineer-melbourne`: one
lost its employment type, the next recorded it correctly, at temperature 0 and
top_p 1. A single passing run is a sample, not a property of the product, and
the acceptance run on the laptop should be read the same way.

### Timings, against the PRD's provisional targets

The PRD is explicit that these are not gates: "Functional quality, security,
offline behavior, deletion, and recovery are acceptance gates. Timing and volume
targets are provisional measurements on the target laptop." Decomposition is
marked "indicative only" on top of that. So no row below fails an acceptance
gate, and none of them is taken in the acceptance environment either.

They are recorded because a provisional measurement still has to be measured,
and because of what they imply about a machine this is not.

| Measurement | Target | Measured | Met |
| --- | --- | --- | --- |
| One role profile, mean | 30 s | 263.81 s | NO |
| One role profile, slowest | 30 s | 311.62 s | NO |
| Twenty role profiles, total | 600 s | 5276.11 s | NO |
| One candidate profile, mean | 180 s | 284.73 s | NO |
| One resume ingested, chunked and indexed | 60 s | 0.01 s | yes — but already extracted; a real PDF pays the sidecar |
| Every aspect of twenty roles embedded | none set | 0.02 s | — twenty title aspects, not twenty decomposed profiles |
| Hybrid retrieval, slowest | 2 s | 0.02 s | yes — but over 20 roles, where the PRD sets the target at approximately 1,000 |
| Cold start: open a database and migrate | 5 s | 0.01 s | yes — excludes the WebView, the larger half on Windows |
| One match assessed | 60 s | 37.82 s | **yes** — 9 results, one generate call each, `just bench-assess` |

Three role-profile figures moved between runs (197 s, then 264 s) with nothing
changed but the run. Treat the magnitude, not the digits.

Decomposition is two 14B passes per document, and nothing else in the product
is close to model cost: all retrieval, fusion and scoring across the whole run
came to hundredths of a second.

### The platform these figures are not from

The PRD's platform is "Windows 11 x64, **CPU-only inference**, 16 GB RAM". The
figures above are from an Apple M3 with 24 GB of unified memory, running a 9 GB
model on the GPU. Those are not the same machine, and the difference runs the
wrong way: CPU-only inference of a 14B model on 16 GB is slower than this by a
margin nobody should estimate from here, and 9 GB of weights on a 16 GB machine
leaves little for anything else.

That makes the model a platform question rather than a timing one, and it is
answerable before the laptop arrives. `qwen2.5:7b-instruct` is 4.7 GB against
the 14B's 9.0 GB, which is the difference between comfortable and marginal on a
16 GB machine. The benchmark exists to answer exactly this — its own harness
records that a failing run "is a model-selection decision, not a bug in the
scorer" — so it was scored against the same frozen corpus, the same hash, and
the same four conditions.

### Model selection: 14B against 7B on the frozen corpus

| | `qwen2.5:14b-instruct` | `qwen2.5:7b-instruct` |
| --- | --- | --- |
| Weights | 9.0 GB | 4.7 GB |
| Every aspect cited | PASS | PASS |
| No unsupported constraint | PASS (0) | **FAIL (2)** |
| Material-aspect capture | 99% | 96% |
| Structured constraints | **100 of 100** | **FAIL (10 misreported)** |
| Matching, three plausible in the top five | 4 of 5 | 4 of 5 |
| Outcome | **pass** | **fail** |
| One role profile, mean | 197.18 s | 173.83 s |

The smaller model fails two of the four classifier conditions, and it is
**12% faster**, not twice as fast. That second number is the surprising one and
it changes the conclusion: on this machine, decomposition time is not dominated
by model size. Halving the weights bought almost nothing, so trading accuracy
for a smaller model buys almost nothing either.

The caveat that keeps this honest: on CPU-only inference the ratio should favour
the smaller model more than it does here, because CPU inference is bound by
memory bandwidth in a way an M3's GPU is not. So the speed column may look
different on the laptop. The accuracy column will not — it is the same corpus,
the same hash, and the same scoring, and accuracy is an acceptance gate where
timing is a provisional measurement.

**Decision: ship the 14B.** It is the only one of the two that passes, and the
faster one is barely faster. If 9 GB of weights proves unworkable on a 16 GB
CPU-only laptop, that is a finding for the acceptance run and a PRD reopen
request — not something to pre-empt here by shipping a model that fails the
gate.

### The matching benchmark, corrected — and now failing

The matching phase used to build each of its twenty roles with a hand-built
profile of one aspect holding the role title. The listing was chunked and
searchable, so the lexical half read the real document, but the aspect-level KNN
the PRD calls for ranked titles, and the structured constraints that drive
eligibility were not in those profiles at all. Every "4 of 5" recorded before
2026-08-21 was that.

It now decomposes all twenty listings with the live model and shortlists five
candidates against those profiles — one workspace rather than one per scenario,
which is twenty classifications instead of a hundred.

Measured properly, it scores **3 of 5, under the PRD's bar of 4**.

| Scenario | Plausible in the top five |
| --- | --- |
| go-platform-melbourne | 5 |
| data-python-melbourne | 4 |
| sre-kubernetes-brisbane | 3 |
| frontend-accessibility-sydney | 2 |
| embedded-c-perth | 1 |

Real profiles made the ranking worse, which is the useful part.
`frontend-accessibility-sydney` ranked `frontend-engineer-sydney` **first** when
every role was a title and **fifth** once the roles carried their real
requirements, behind a solutions architect and two platform roles. Structured
types are already excluded from the similarity half, so this is not constraint
noise; something about ranking over many aspects rather than one is diluting the
signal.

**This is an open PRD gap, not a harness artifact.**

One mechanical defect was found and fixed on the way, and it is not the cause.
The similarity half asked for a page of the closest *aspects* and then collapsed
them to roles, so twenty roles holding nine aspects each gave a hundred and
eighty aspects, the closest thirty were whichever listings wrote the most, and a
role whose best aspect ranked thirty-first was invisible however well it
matched. `SearchRoles` collapses to each role's closest aspect before
truncating, and a test pins it — one prolific listing can no longer fill the
page.

Measured on the tuning corpus, enlarged to twenty listings for the purpose, the
fix changes nothing: 2 of 3 scenarios before and after, with identical
per-scenario counts. The role predicted to be rescued by it was already ranked
second without it. So the defect is real, the fix is correct, and **the cause of
the matching failure is still unknown**.

What is ruled out: aspect-page truncation, structured-constraint noise (those
types never enter the similarity half), the harness, and the retrieval
constants.

**The provenance says the similarity half is nearly inert.** Every role enters
every semantic list, because the depth exceeds the corpus, and reciprocal rank
fusion at K=60 scores rank one at 1/61 and rank twenty at 1/80 — a spread of
under a third, which is less than one lexical hit is worth. On the tuning corpus
`geospatial-analyst-napier` is the second closest role by embedding and finishes
twentieth of twenty, on zero lexical hits. Scores track lexical list count
exactly: 5 lists 0.211, 3 lists 0.180, 2 lists 0.163, 1 list 0.149, none 0.124.

**And sharpening it changes nothing.** The constant sweep was re-run on a
corrected instrument — real decomposed role profiles rather than title stubs,
one workspace rather than one per scenario, K from 1 to 60 and depth from 3 to
30. All thirty configurations score 2 of 3, with eight or nine plausible. At K=1
the similarity half is fully dominant and the outcome does not move. The shipped
constants stand, now on evidence that can see the thing it was accused of.

**The labels are the reason, and they cannot be satisfied by evidence.**
`embedded-c-perth` is nine years of embedded C, CAN bus drivers and safety
interlocks. Its four plausible labels are the embedded role, a Go distributed
systems role, a Playwright QA role, and a SOAP integration role: three of the
four share no evidence at all with the candidate.
`frontend-accessibility-sydney` is the same, with a Swift and Kotlin mobile role
and a cloud architecture role among its four. Reaching three plausible in a top
five requires ranking documents by career adjacency a recruiter feels and the
listings never state.

This is what the design note meant by an invented corpus not substituting for
real placements, and it is also why the old title-stub harness scored 4 of 5:
loose title similarity surfaced adjacent-sounding roles, so it passed by
measuring something closer to intuition than evidence.

**The failure is recorded as a failure and the labels are not being changed.**
Editing held-out labels so the product passes is the one move this corpus split
exists to prevent. The gate stands at 3 of 5, and it is not decidable on a
synthetic corpus: the PRD's acceptance run replaces these with the recruiter's
real placements, where a plausibility judgement comes with a person who made it.

One caveat on the tuning corpus as an instrument: its plausibility labels are
invented, and for `search-relevance-invercargill` they call a geospatial analyst
and a broadcast streaming engineer plausible for a search-and-metadata
candidate. A ranking cannot be expected to reproduce a judgement no evidence in
the listing supports, so a scenario can be unwinnable by construction. The
frozen corpus has the same property until the acceptance run replaces it with
real placements.

The corpus currently in the repository is synthetic and says so. It exists to
prove the harness, the thresholds, and the record are correct. The acceptance
run replaces it with the recruiter's five frozen past-placement scenarios and
twenty role listings, labelled before any model runs against them.

Cloud-assisted runs are recorded separately and cannot pass any of the above.

## Failed gates and consequences

_For each failure: the gate, what failed, and either the replacement
implementation choice within the PRD or the explicit PRD reopen request._

### Held-out matching benchmark — resolved, and the reopen request withdrawn

**It passes: 5 of 5.** The entry below is kept in full because it is the record
of how a wrong diagnosis nearly became a request to change the product's locked
design, and because the measurements in it are still true.

The cause was neither the labels nor the constants. The candidate's city never
reached retrieval: structured types were excluded from the query set on a
justification that only holds for embeddings. Letting places reach the full-text
half — where "Perth" matches "Perth" exactly — took `embedded-c-perth` from one
plausible to three, and every other scenario improved with it.

**No label was edited, no constant was tuned, and the corpus hash is unchanged
from the first run of the session.**

---

### The failing record, kept

**The gate.** At least four of five scenarios put three plausible roles in the
top five.

**What failed.** Three of five, measured for the first time against role
profiles the model actually decomposed. Every earlier pass was measured against
roles whose entire profile was their job title.

**Replacement within the PRD: not yet found, and not yet ruled out.**
Everything the specified design offers *at the level of constants* was tried and
measured. Collapsing aspect KNN to roles before truncating
is a real defect fixed, and changed no scenario. The retrieval constants were
swept again on a corrected instrument — real profiles, K from 1 to 60, depth
from 3 to 30 — and all thirty configurations score identically; at K=1 the
similarity half is fully dominant and nothing moves. The provenance shows the
ranking is decided by lexical hits, and the roles it misses are missed because
no evidence connects them to the candidate.

**Why — an earlier answer here was wrong, and is retracted.** This entry
previously said the labels ask for a judgement the documents do not contain,
reasoning from three hand-picked skill lists. Measured across every rating in
both corpora, that is false: roles the recruiter called plausible share
consistently more wording with the candidate than roles they did not.

| Scenario | plausible mean overlap | other | ranked by overlap alone, plausible in top five | the product |
| --- | --- | --- | --- | --- |
| go-platform-melbourne | 8.9 | 6.3 | 5 | 5 |
| data-python-melbourne | 4.8 | 3.9 | 2 | 4 |
| frontend-accessibility-sydney | 5.8 | 4.0 | 2 | 2 |
| sre-kubernetes-brisbane | 6.0 | 3.9 | 2 | 3 |
| embedded-c-perth | 7.2 | 3.6 | **3** | **1** |

On `embedded-c-perth` — the scenario used to argue the labels were
unsatisfiable — plausible roles carry twice the overlap of the rest, and
ranking by raw word overlap alone reaches the bar. The shipped ranking scores
one. The product is beaten by the crudest possible baseline on that scenario,
which is a defect, not a corpus problem.

Overall the product still ranks better than that baseline — three scenarios to
two — and the two disagree about which scenarios they win. That is what makes
this worth pursuing rather than closing.

**The reopen request, withdrawn.** It rested on there being no replacement
within the PRD. There was one.

1. **The acceptance run decides it.** The PRD already says the synthetic corpus
   is replaced with the recruiter's five frozen past placements, labelled by the
   recruiter before any model runs. If those labels mark roles that share
   evidence with the candidate, evidence-based retrieval can reach the bar and
   this failure says nothing about the product. **This is the expected
   resolution and it needs no PRD change** — only the real corpus.

2. **If the real labels look like these, the design is what is wrong.** If a
   recruiter's genuine placements also cross domains the documents never
   connect — an embedded engineer placed into a distributed systems role because
   the recruiter knows the person — then ranking by document evidence cannot
   reproduce recruiter judgement, and matching needs a reasoning step that
   assesses fit rather than retrieves similarity. That contradicts the PRD's
   locked decision on deterministic ranking, so it is a PRD change and not a
   tuning exercise.

**What is still open: the cause is not known.** Five explanations have been
proposed and each was refuted by measurement — aspect-page truncation (fixed,
changed nothing), structured-constraint noise (those types never enter the
similarity half), fusion flattening (thirty configurations score identically),
unsatisfiable labels (retracted above), and candidate query granularity. The
last is worth recording because the correlation runs backwards: on the tuning
corpus the scenario whose queries are four rambling sentences scores four
plausible and passes, and the scenario with eight clean atomic queries scores
two and fails.

What remains unexplained is narrow and specific: the shipped ranking scores one
plausible on `embedded-c-perth` where ranking by raw word overlap scores three.

**A second defect was found, and this one is on the path.** The candidate's
profile records the place — `location "Perth, onsite, permanent, around AUD
160,000"` — and the shortlist searched with neither half of it. Structured types
were excluded from the query set entirely, justified by a comment that is only
about embeddings: "Melbourne" and "Sydney" are close in an embedding space and
opposite in fact. True, and a good reason to keep a place out of the similarity
half. Not a reason to keep it out of FTS5, where "Perth" matches "Perth" exactly
and cannot be confused with Sydney. One correct argument applied one scope too
wide.

So `embedded-c-perth` lives in Perth and works at Redgum Mining Tech;
`staff-engineer-perth` is a role at Redgum Mining Tech in Perth whose
nice-to-have is mining domain experience. Four facts both documents state, the
recruiter calls it plausible, and the word Perth reached no query at all.

Places are now lexical-only queries, taken from the normalized value rather than
the aspect wording, so the search is `Perth` and not `Perth, onsite, permanent,
around AUD 160,000`. Two tests pin it: the city becomes a lexical-only query and
the whole wording never does, and a shared city cannot outrank the work — a
listing naming Perth three times about laminated dough still loses to the right
role in Hobart.

Measured on the tuning corpus it changes nothing, and the reason is structural:
every tuning candidate's city holds exactly one role and it already ranks first,
where the frozen corpus has three roles in Perth alone. The instrument cannot
exercise this fix, which is why the harm case is a unit test rather than a
score.

**A third defect was found and is recorded unfixed.**
Seniority aspects become search queries, so the shortlist searches for `"six
years"` and `"seven years"` with the words ORed. Almost every listing says
"years", so those queries retrieve nearly the whole corpus and contribute a
near-uniform list to the fusion. It is the same shape as ORing "the" into every
query, which this project already treats as a defect rather than a tuning knob.
Whether removing it moves any score is unmeasured; it is wrong regardless.

## Development-machine live-model runs

Not the acceptance environment. This is a macOS development machine, so nothing
here can turn a target-laptop row above from NOT RUN into PASS. What it does
prove is that the live paths run end to end against real models, and it produces
the model-selection evidence Phase 1 asks for.

Machine: macOS on Apple silicon, Ollama at `http://localhost:11434`.

| Run | Model | Result |
| --- | --- | --- |
| `just gate-model` chat | `qwen2.5:3b-instruct` | PASS — 9.2 s cold, 2.2 GB resident |
| `just gate-model` constrained JSON | `qwen2.5:3b-instruct` | PASS — 0.70 s, schema honoured |
| `just gate-model` constrained JSON | `gemma4:12b-mlx` | **FAIL** — returns prose, ignores the schema |
| `just gate-model` embeddings | `nomic-embed-text` | PASS — 768 dimensions, 0.29 s |
| `just gate-model-classify` contract | `qwen2.5:3b-instruct` | **FAIL** — duplicate aspects, and citations quoting wording absent from the chunk |
| `just gate-model-classify` injection | `qwen2.5:3b-instruct` | PASS — the injected instruction was refused |
| `just gate-model` chat | `qwen3:4b` | PASS — 4.4 s |
| `just gate-model` constrained JSON | `qwen3:4b` | PASS on output, **4 m 28 s** — over the 3-minute classify budget |
| One decomposition-sized call | `qwen3:4b` | **4 m 21 s with reasoning, 4 m 35 s with `think:false`** — suppressing the reasoning field does not make it faster, the model writes the same volume into the content instead |

### What benchmarking found

Every one of these was live for as long as the product has existed, and none
could be reached by the unit suite — they only appear when a real model meets
labels written independently of it.

| Defect | Effect | Fix |
| --- | --- | --- |
| `structured` declared as an object with no properties | Under strict decoding that admits nothing: **no profile could carry a structured value**, ever | SchemaVersion 2 |
| `structured` optional | Strict decoding takes an optional object as nullable, and the model answered `null` every time | Required, SchemaVersion 3 |
| `structured` described only as "an object" | Made required, the model invented a shape: `{"location": {"city": "Melbourne"}}` | Fields declared with types and enumerations |
| The prompt called structured values optional | A model that follows instructions omits them | PromptVersion 2 |
| The prompt never named the permitted values | Beside a rule saying never guess, a careful model omits the field. Worth 29 capture points when fixed | PromptVersion 3 |
| A `null` structured field failed validation | `"basis": null` for a salary quoted without one **invalidated the whole profile** | Nulls dropped at parse |
| A field outside the taxonomy failed validation | `"state": "VIC"` discarded nine good aspects to punish a tenth for its vocabulary | Undefined fields dropped |
| A quote tidied at its edge failed validation | The model cuts mid-sentence and adds a full stop; that cost whole listings their extraction | Boundary punctuation trimmed |
| A citation naming the wrong chunk failed validation | Nine citations in one resume pointed at the wrong supplied chunk. The evidence was there | Pointer repaired, never the quote |
| **A structured value was never checked against its own citation** | The contract required an aspect to cite its evidence and never asked the same of the value beside it: cite "we do not sponsor", record `status: citizen`. **42 of 58 introduced values** | Unsupported values dropped |
| The shortlist ANDed every word of every query | A profile aspect is a sentence, so **every profile-driven shortlist returned empty** unless criteria happened to be present | Aspect queries use SearchAny |
| No temperature was ever set | Every constrained call sampled at 0.8, so the same resume produced different profiles — and profile identity claims a profile is a function of its sources | Temperature 0, fixed seed |
| Three minutes for a decomposition | Generous against a fixture, tight against a real resume: two of five scenarios were lost to the clock | Six minutes |
| Embedding evicts the classify model | The next classify call pays a full reload inside its own timeout | Known; the benchmark profiles the candidate first |

Two of my own benchmark errors are recorded with them, because a benchmark that
is wrong in the product's favour is worse than no benchmark: capture demanded
identical wording and identical aspect types, and the corpus labels used field
names the product does not have while asserting facts the listings never state.

### What the runs measured, in order

Each row is one full run of both benchmarks against the frozen corpus on the
development machine. A number only means something beside what changed before
it.

| Run | Model | Capture | Uncited | Introduced | Constraints wrong | Scenarios at 3+ | What changed before it |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 3 | 7B | 38% | 0 | 0 | 100 | 0 | first run against the 7B |
| 8 | 7B | 83% | 3 | 4 | 99 | 1 | quote edges trimmed, descriptive types grouped |
| 9 | 7B | 64% | 3 | 1 | 100 | 0 | nothing — sampling variance |
| 10 | 7B | 68% | 0 | 0 | 100 | 0 | temperature 0 |
| 12 | 7B | 97% | 0 | 13 | 89 | 0 | permitted values named in the prompt |
| 14 | 7B | 84% | 0 | 3 | 78 | 2 | structured fields declared and required |
| 17 | 7B | 88% | 0 | 11 | 32 | 2 | values derived from evidence that states them |
| 18 | 7B | 76% | 0 | 8 | 28 | 2 | a constraint checklist — **withdrawn** |
| 20 | 7B | 96% | 0 | 13 | 14 | 3 | every role rated for every scenario |
| 22 | 14B | 99% | 0 | 23 | 3 | 3 | the 14B model |
| 23 | 14B | 99% | 0 | 2 | 4 | **4** | **aspect-level retrieval**, evidence from citations only |
| 26 | 14B | 99% | 0 | 0 | 3 | 4 | place phrasing, remote arrangement, sentence scope |
| 28 | 14B | 99% | 0 | 0 | **1** | 4 | wording verbatim in a source counts as evidence |
| 29 | 14B | 99% | 0 | 8 | 1 | 4 | a dedicated normalization call — **withdrawn**: it recovered nothing and answered requires_sponsorship on eight listings that refuse it |
| 30 | 14B | 99% | 0 | **0** | **1** | **4** | that value refused when its only evidence is its own negation |
| 31 | 14B | 99% | 0 | 0 | 2 | 4 | constraint fields merged rather than whole aspects — a **regression**: a merged field lost the evidence that justified it |
| 32 | 14B | 99% | 0 | **0** | **1** | **4** | a merged field brings its citation with it |

Runs 31 and 32 are one change and its repair. Merging a field into the
surviving aspect judged it against that aspect's citations, which I argued was
"stricter, not looser, so it is safe" — stricter was true and safe was not: a
country the second pass had cited straight from the work-rights sentence was
dropped for lacking evidence the first pass never quoted. A field arrives with
its evidence, and it now travels with it.

The merge rule is better for it. The gate is where it was.

Run 29 is the third measured change withdrawn after measuring, with the
constraint checklist of run 18 and the fusion constants that were never
touched. It earned its cost anyway: the evidence check had been accepting
requires_sponsorship because the word "sponsor" appears in "we do not sponsor",
and no model had happened to emit that value before. Left alone, the product
would have recorded that a candidate requires sponsorship from a listing
stating the opposite, and the assessment step would have compared against it.

Run 9 is why the rest are trustworthy: nineteen points below run 8 with no code
between them. Everything before temperature 0 was sampled, so no earlier number
could be attributed to the change that preceded it.

Run 20 is the other correction worth naming. Matching sat at two of five for
eight runs and the corpus was the reason: each scenario rated six or seven of
twenty roles and the rest counted as not plausible by default, while one rated
two plausible in all — where three of the top five cannot be reached however
good the ranking is.

### Frozen benchmark run, development machine

`just bench`, about ninety minutes per run with the 14B and the two-pass
classifier. Best configuration: run 32, record
`docs/product/benchmarks/benchmark-2026-08-20T05-10-55Z.json`.

Models: classify `qwen2.5:14b-instruct`, embed `nomic-embed-text`.

**Outcome: FAIL**, on one of six conditions.

| Condition | Bar | Measured | |
| --- | --- | --- | --- |
| Every extracted aspect cited | all | 0 uncited across 20 listings | PASS |
| Material-aspect capture | ≥ 80% | 99% | PASS |
| No unsupported value introduced | none | 0 | PASS |
| Eligible roles in scope | ≥ 10 | 20 | a result, not source-coverage inconclusive |
| Matching: three plausible in the top five | ≥ 4 of 5 | 4 of 5 | **PASS** |
| Structured constraints reproduced | all | 99 of 100 | **FAIL** |

**The one remaining error, in full.** On `backend-contract-melbourne` the model
recorded a location of `{remote_ok: true}`, worded "remote role in Melbourne",
citing "…is hiring a backend engineer (contract) in Melbourne. This is a remote
role…". The city is in its own evidence and it did not write it down. On the
same run, the same model recorded `{city: Melbourne, remote_ok: true}` for a
near-identical listing.

Deriving it would mean knowing that "Melbourne" is a city while "Remote" and
"New Zealand" in the same grammatical position are not, which needs a
gazetteer. That is the inference this phase spent nine commits removing from
the product, and it is not being added back to move a number.

Both model passes omit the city on that listing — the decomposition and the
constraints pass alike — so there is nothing for the merge to carry, and
nothing in the pipeline can supply what neither recorded.

Five approaches were tried against this one error. Scoping evidence to the
cited sentence worked and is kept. Counting wording that appears verbatim in a
source worked and is kept — together they took the count from three to one.
Asking the model again about the single phrase, with nothing else in the
prompt, recovered nothing and was withdrawn. Merging constraint fields rather
than whole aspects regressed the count and was repaired into a better rule that
did not move it. A larger model was pulled and is not viable on this class of
machine.

What is left needs either a gazetteer or a looser evidence rule. A pattern
reading "in X" as a city takes "in Remote (Australia)" as one too, and the
special cases that would fix that are a gazetteer assembled by reading the
held-out corpus. Neither was added.

### A 32B was tried and is not viable on this class of machine

`qwen2.5:32b-instruct`, 19 GB, pulled and run against the same corpus. The
first listing exceeded the six-minute classify budget, and a one-line prompt
sent straight to the endpoint — "list three requirements from" one sentence —
did not answer inside a fifteen-minute request timeout. The run was stopped and
the model removed.

That is a finding about the hardware, not the model: the PoC's target is a
16 GB CPU-only laptop, so a 32B was never going to be viable there either. It
rules out "use a bigger model" as the answer to the one remaining constraint,
on this class of machine.

**Model selection consequence.****Model selection consequence.****Model selection consequence.****Model selection consequence.** Three models were tried on this machine and
none is a candidate for the pinned classify role:

- `gemma4:12b-mlx` ignores JSON schemas and returns prose, so it cannot serve
  any role this product uses a model for.
- `qwen2.5:3b-instruct` holds the schema but not the contract: it invents
  citations and leaves structured values empty, which is exactly what the
  contract exists to catch.
- `qwen3:4b` holds the schema, but one decomposition takes about four and a
  half minutes against a three-minute budget. It is a reasoning model, and
  `think:false` does not recover the time — it stops emitting the reasoning
  field and writes the same volume into the answer.

A reasoning model is the wrong shape for this work regardless of quality: the
product makes one constrained call per listing on a CPU-only laptop, and
tokens spent deliberating are latency with no place to go.

The PoC's recommended `qwen2.5:7b-instruct` is untested here. It is 4.7 GB and
this volume has 5.1 GB free, which is too thin a margin to pull it safely — an
earlier attempt on this machine filled the disk. Testing it needs either more
free space or an explicit decision to accept that margin.

## Target-laptop measurements

Provisional. A miss is recorded as measured, with an explicit go/no-go decision,
and is never restated as a pass.

| Measurement | Provisional target | Measured | Conditions | Decision |
| --- | --- | --- | --- | --- |
| Cold start to usable window | | NOT RUN | | |
| Indexing one 20-page document | | NOT RUN | | |
| Profile decomposition, one resume | | NOT RUN | | |
| Retrieval P95 | | NOT RUN | | |
| One assessment | | NOT RUN | | |
| End to end over twenty roles | | NOT RUN | | |
| Database size after the acceptance corpus | | NOT RUN | | |
| Overnight corpus indexing | | NOT RUN | | |

## Accessibility walkthrough

Run as `frontend/e2e/accessibility.spec.ts`, so it is re-run by `just check`
rather than being a walkthrough someone did once.

| Check | Result |
| --- | --- |
| Every action reachable by keyboard | PASS — tabbing reaches interactive controls, and every stop resolves to a name by the rules a reader uses |
| A whole flow completable by keyboard alone | PASS — new initiative, from opening the modal to creating it |
| Focus is visible wherever it lands | PASS |
| Every control has an accessible name | PASS — no anonymous button, input, select, or textarea in a workspace |
| Source, recruiter-authored, and AI content are visibly distinct | PASS — three treatments, each labelled in words so none depends on colour |

What this does not cover: a screen-reader session with a real assistive
technology, and colour-contrast ratios. Both need a person and a device, and
neither is claimed here.

## Final checks

| Check | Result |
| --- | --- |
| `just check` | PASS |
| Windows installer smoke test | NOT RUN — see PHASE20_PACKAGING_EVIDENCE.md |
| Security scan (gosec) | PASS |
| Dependency vulnerability scan (govulncheck) | PASS |
| Redacted-log inspection | PASS off-laptop — diagnostics carry no content by construction |

## Go / no-go

Not decided here, and not mine to decide. What can be said is what was measured.

**Five of the six acceptance conditions pass on the development machine**, with
a 14B local model against the synthetic frozen corpus: every aspect cited, 99%
material-aspect capture, no unsupported value introduced, twenty eligible roles
in scope, and the matching benchmark at four scenarios of five.

**One fails**: structured constraints are reproduced 99 times in 100. The single
error is the model omitting a city that appears in its own cited evidence, on a
listing where it recorded the same field correctly elsewhere in the same run.

**Nineteen product defects were found and fixed getting there**, every one of
them invisible to the unit suite and live for as long as the product had
existed. Four were the single question "can a profile carry a structured value
at all", and the answer had been no since Phase 10. One meant every
profile-driven shortlist returned empty unless search criteria happened to be
present — the flagship loop only ever worked by accident. One meant similarity
retrieval ran over source chunks rather than Profile Aspects, which is what the
PRD specifies and what no phase spec had pinned. One meant the product never
checked that a structured value was supported by the evidence cited for it, a
rule the PRD states outright.

**Six errors in the benchmark itself are recorded beside them**, because a
measuring instrument this young is at least as likely to be wrong as the thing
it measures. Every one of mine made the product look worse than it was, and one
— a scenario rated with two plausible roles in twenty — made a condition
unreachable by construction and was reported as a product failure for eight
runs before I checked.

**What this supports.** That `qwen2.5:14b-instruct` is one omission short of
Validated for the classify role on this hardware, and that `qwen2.5:7b-instruct`
is not close. That the retrieval, citation, and evidence rules hold under a real
model on a held-out set.

**What it does not.** Anything about the Windows 11 laptop, or about the
recruiter's real frozen corpus. Both are named by the PRD as the acceptance
environment and neither has been run. Every row above marked NOT RUN belongs to
them, and each carries the command that produces it.

**What would move the last condition**, in the order I would try it: the real
corpus on the target laptop, since one omission in a hundred is within the range
a different corpus resolves either way; then a larger model; then a second
focused pass for locations specifically. What would *not* move it honestly is a
gazetteer or a looser evidence rule, and neither was added.

A provisional miss is recorded as measured, never reclassified. This is that
record.
