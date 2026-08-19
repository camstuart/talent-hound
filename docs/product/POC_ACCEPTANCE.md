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
| 6 | Held-out matching benchmark | NOT RUN on the target laptop — run end to end on a development machine, FAIL for the models available there |
| 7 | Held-out classifier benchmark | NOT RUN on the target laptop — run end to end on a development machine, FAIL for the models available there |
| 8 | Real-data mode refused on an unencrypted volume | PARTIAL — enforced and tested; BitLocker itself is a gate |
| 9 | Recover a copied data folder without corruption or partial migration | PASS off-laptop — second-machine run is a gate |

## Benchmarks

These rows are the acceptance environment: the Windows laptop, the pinned
models, and the recruiter's frozen corpus. A development-machine run is
recorded further down and cannot fill them in.

| Benchmark | Bar | Result |
| --- | --- | --- |
| Classifier: every aspect cited | all | NOT RUN on the target laptop — met on the development run |
| Classifier: no unsupported critical constraint | none | NOT RUN on the target laptop — met on the development run |
| Classifier: material-aspect capture | ≥ 80% | NOT RUN on the target laptop — 22–44% on the development run's five productive listings |
| Classifier: structured constraints reproduced | exact | NOT RUN on the target laptop — failed on every listing of the development run |
| Matching: three plausible in the top five | ≥ 4 of 5 scenarios | NOT RUN on the target laptop — 0 of 5 on the development run, upstream of the matcher |
| Live acceptance: eligible roles found | ≥ 10, else inconclusive | NOT RUN — 20 in scope on the development run |
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

Each row is one full run of both benchmarks against the synthetic corpus on the
development machine. The point of keeping them all is that a number only means
something beside what changed before it.

| Run | Capture | Uncited | Introduced | Constraints wrong | Scenarios at 3+ | What changed before it |
| --- | --- | --- | --- | --- | --- | --- |
| 3 | 38% | 0 | 0 | 100 | 0 | first run against the 7B |
| 6 | 36% | 0 | 0 | 100 | 0 | null fields dropped, real resumes |
| 7 | 36% | 0 | 0 | 100 | 0 | undefined fields dropped |
| 8 | 83% | 3 | 4 | 99 | 1 | quote edges trimmed, descriptive types grouped |
| 9 | 64% | 3 | 1 | 100 | 0 | nothing — sampling variance |
| 10 | 68% | 0 | 0 | 100 | 0 | temperature 0 |
| 12 | 97% | 0 | 13 | 89 | 0 | permitted values named in the prompt |
| 13 | 98% | 0 | 0 | 100 | 0 | worked examples, six-minute timeout |
| 14 | 84% | 0 | 3 | 78 | 2 | structured fields declared and required |
| 15 | 87% | 0 | 58 | 71 | 2 | mistakes named; introduced values counted everywhere |
| 16 | 87% | 0 | 12 | 71 | 2 | values checked against their citation |
| 17 | 88% | 0 | 11 | 32 | 2 | values derived from evidence that states them |
| 18 | 76% | 0 | 8 | 28 | 2 | a checklist of constraint types — **withdrawn**: it traded four constraints for twelve points of capture |

Run 18 is the reason the prompt is at version 5 and not 6. Asking the model to
check the source once more for each constraint type worked, and cost more than
it bought: the model spent its attention on the checklist and stopped recording
the skills. Measured, then reverted. Run 17 is the product as it stands.

Run 9 is the reason the rest are trustworthy: it scored nineteen points below
run 8 with no code between them. Everything before it was sampled at
temperature 0.8, so no earlier number could be attributed to the change that
preceded it.

### Frozen benchmark run, development machine

`just bench`, roughly forty minutes per run, against the synthetic corpus in
this repository. Best configuration: run 17, record
`docs/product/benchmarks/benchmark-2026-08-19T02-34-58Z.json`.

Models: classify `qwen2.5:7b-instruct`, embed `nomic-embed-text`. Generation
takes no part in either benchmark.

**Outcome: FAIL**, on two of the six conditions.

| Condition | Bar | Measured | |
| --- | --- | --- | --- |
| Every extracted aspect cited | all | 0 uncited across 20 listings | PASS |
| Material-aspect capture | ≥ 80% | 88% | PASS |
| Eligible roles in scope | ≥ 10 | 20 | a result, not source-coverage inconclusive |
| No unsupported value introduced | none | 11 | **FAIL** |
| Structured constraints reproduced | all | 68 of 100 | **FAIL** |
| Matching: three plausible in the top five | ≥ 4 of 5 | 2 of 5 | **FAIL** |

What the remaining failures are, precisely, because "the model isn't good
enough" was the wrong conclusion eleven times before it was the right one:

- **The 32 wrong constraints are mostly aspects never emitted.** Eleven
  locations and nine employment types simply do not appear in the answer for
  listings that state them. Six work-rights values carry the sponsorship but
  drop the country. Asking the model to check for each type recovered four of
  them and cost twelve points of capture, so it was withdrawn.
- **The 11 introduced values are inference the model will not stop making**: a
  country read off a city, a period read off a salary, a `days_onsite` of zero.
  Each one is now dropped from what is stored — the product records nothing
  unsupported — but the benchmark counts what the model produced, which is the
  honest thing for it to count.
- **The matching half reaches 2 of 5.** Every scenario ranks and every one puts
  plausible roles on the list; three of them put two rather than three. With
  five scenarios, one scenario is twenty points, and the bar is four of five.

**Model selection consequence.****Model selection consequence.** Three models were tried on this machine and
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

Not yet decided, and the decision is not mine to make. What can be said is what
was measured.

**The product side of Phase 21 is finished.** Fourteen defects were found and
fixed, every one of them invisible to the unit suite and every one live for as
long as the product had existed. Four of them were the single question "can a
profile carry a structured value at all", and the answer had been no since
Phase 10. One of them — the shortlist ANDing a sentence — meant the flagship
loop only ever worked when search criteria happened to be present. Another
meant the product never checked that a normalized value was supported by the
evidence cited for it, which is a rule the PRD states outright.

**The benchmarks do not pass**, on this machine, with the models that fit on
it. Capture and citation discipline clear their bars; structured-constraint
reproduction and the matching benchmark do not. The remaining failures are the
model omitting aspects a listing plainly states, and inferring values no source
gives — after schema, prompt, examples, vocabulary, evidence checking, and
deterministic normalization have each been corrected.

**What that supports, and what it does not.** It supports the conclusion that
`qwen2.5:7b-instruct` is not Validated for the classify role, which is the
label the PRD reserves for a model that has passed these benchmarks. It does
not support any conclusion about the target laptop, a larger model, or the
recruiter's real corpus, none of which have been run.

Three things would move it, in the order I would try them:

1. **A larger local model.** Everything measured here says the ceiling is the
   model's recall and its willingness to infer, not the product's handling.
   A 14B or 32B on hardware that can hold it is the obvious next measurement,
   and it needs no code change — `just bench` takes the model as an argument.
2. **The target laptop, with the recruiter's frozen corpus.** That is the
   environment the PRD names, and every row above marked NOT RUN belongs to it.
3. **A second, focused normalization call.** If a larger model still omits
   constraint aspects, asking one short question per constraint type would
   almost certainly recover them. It costs latency on a CPU-only machine, which
   is why it was not done speculatively.

A provisional performance miss is recorded as measured, never reclassified.
This is that record.
