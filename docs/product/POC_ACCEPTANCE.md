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

| Check | Result |
| --- | --- |
| Every action reachable by keyboard | NOT RUN |
| Focus order follows the visible order | NOT RUN |
| Source, recruiter-authored, and AI content are visibly distinct | NOT RUN |
| Every control has an accessible name | PARTIAL — asserted throughout the Playwright suite |

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
