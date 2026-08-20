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
| 6 | Held-out matching benchmark | **PASS on the development machine** (4 of 5 scenarios, 14B) — NOT RUN on the target laptop |
| 7 | Held-out classifier benchmark | FAIL on the development machine by one constraint of a hundred — NOT RUN on the target laptop |
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
| Classifier: structured constraints reproduced | all | **99 of 100** on the development machine — the one failing condition |
| Matching: three plausible in the top five | ≥ 4 of 5 scenarios | **4 of 5** on the development machine; NOT RUN on the target laptop |
| Live acceptance: eligible roles found | ≥ 10, else inconclusive | 20 in scope |
| Live acceptance: Ready profiles and assessments | ≥ 10 | NOT RUN |
| Live acceptance: usable evidence-backed draft | ≥ 1 | NOT RUN |

The corpus currently in the repository is synthetic and says so. It exists to
prove the harness, the thresholds, and the record are correct. The acceptance
run replaces it with the recruiter's five frozen past-placement scenarios and
twenty role listings, labelled before any model runs against them.

Cloud-assisted runs are recorded separately and cannot pass any of the above.

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
classifier. Best configuration: run 30, record
`docs/product/benchmarks/benchmark-2026-08-20T00-15-23Z.json`.

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

Three approaches were tried against this one error. Scoping evidence to the
cited sentence worked and is kept. Counting wording that appears verbatim in a
source worked and is kept — together they took the count from three to one.
Asking the model again about the single phrase, with nothing else in the
prompt, recovered nothing and was withdrawn. What is left needs either a
gazetteer or a looser evidence rule, and neither was added.

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
