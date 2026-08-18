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
| 6 | Held-out matching benchmark | NOT RUN — needs live models; harness and scoring PASS |
| 7 | Held-out classifier benchmark | NOT RUN — needs live models; harness and scoring PASS |
| 8 | Real-data mode refused on an unencrypted volume | PARTIAL — enforced and tested; BitLocker itself is a gate |
| 9 | Recover a copied data folder without corruption or partial migration | PASS off-laptop — second-machine run is a gate |

## Benchmarks

| Benchmark | Bar | Result |
| --- | --- | --- |
| Classifier: every aspect cited | all | NOT RUN |
| Classifier: no unsupported critical constraint | none | NOT RUN |
| Classifier: material-aspect capture | ≥ 80% | NOT RUN |
| Classifier: structured constraints reproduced | exact | NOT RUN |
| Matching: three plausible in the top five | ≥ 4 of 5 scenarios | NOT RUN |
| Live acceptance: eligible roles found | ≥ 10, else inconclusive | NOT RUN |
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
