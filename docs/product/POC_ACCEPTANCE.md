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

### Frozen benchmark run, development machine

`just bench`, 37 minutes, against the synthetic corpus in this repository.
Record: `docs/product/benchmarks/benchmark-2026-08-18T04-43-17Z.json` and its
`.txt` summary. Corpus hash `f3280238af1715db…`.

Models: classify `qwen2.5:3b-instruct`, embed `nomic-embed-text`. Generation
takes no part in either benchmark.

**Outcome: FAIL.**

| Condition | Bar | Measured |
| --- | --- | --- |
| Every extracted aspect cited | all | met — no uncited aspect on any listing that produced one |
| No unsupported critical constraint | none | met — nothing invented |
| Material-aspect capture | ≥ 80% | **22–44% on the five listings that produced aspects; 0% on the other fifteen, which produced nothing at all** |
| Structured constraints reproduced | exact | **not met on any listing — location, work rights, employment type, and compensation come back empty** |
| Matching: three plausible in the top five | ≥ 4 of 5 scenarios | **0 of 5 — no scenario produced a candidate profile, so no shortlist was ranked** |
| Eligible roles in scope | ≥ 10 | 20 — the run is a result, not source-coverage inconclusive |

What this says about the product versus the model: the citation rule held
everywhere it was tested — the model never produced an aspect the validator
accepted without evidence — and the twenty roles were all in scope, so
retrieval had something to rank. What failed is decomposition: fifteen of
twenty listings produced nothing the contract would accept, and no resume
produced a candidate profile at all. That is the 3B model, and it is the same
finding as the contract gate above.

An earlier record from the same day,
`benchmark-2026-08-18T04-03-54Z`, is kept and **superseded**: its capture rule
demanded identical wording and so scored correctly decomposed listings at zero.
It is retained because deleting a wrong measurement is how a corrected one
stops being checkable.

**Model selection consequence.** `gemma4:12b-mlx` cannot serve any role that
needs a schema, which is every role this product uses a model for.
`qwen2.5:3b-instruct` holds the schema but not the contract: it invents
citations, which is the failure the contract exists to catch. Neither is a
candidate for the pinned classify model. The PoC's recommended
`qwen2.5:7b-instruct` was not testable here — the pull needs about 5 GB free and
this machine had less.

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

Not yet decided. The decision is made after the outstanding rows are run on the
target laptop against the recruiter's frozen corpus, and it is recorded here
with the evidence that supported it — including any provisional performance miss
accepted deliberately.
