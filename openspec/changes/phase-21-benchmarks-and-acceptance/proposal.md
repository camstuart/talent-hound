## Why

Everything the product claims is now implemented and tested against fixtures this repository wrote. That proves the rules hold; it does not prove the product works. The difference is measured on frozen data the tuning never saw, on the laptop it ships for.

The failure mode this phase exists to prevent is the comfortable one: running a benchmark, seeing a number below the bar, and deciding the bar was the wrong bar. So the bar is written down first, the corpus is frozen before anything is tuned against it, and every run records what it ran with — model digests, prompt and schema versions, corpus hash — so that a later "it passes now" can be checked against what actually changed.

The second failure mode is quieter: a live run that finds too few roles, reported as a pass because nothing failed. The PRD is explicit that this is source-coverage inconclusive, and inconclusive is its own outcome, recorded as itself.

## What Changes

- Freeze a benchmark corpus — five matching scenarios and twenty classifier role listings — as data, with the labels applied before any run.
- Score the classifier benchmark on its four conditions: every aspect cited, no unsupported critical constraint introduced, at least 80% of material aspects captured, and structured constraints reproduced exactly.
- Score the matching benchmark on the PRD's rule: at least three plausible in the top five, in at least four of five scenarios, after duplicates collapse and with absent slots counted as not plausible.
- Record every run with its configuration, model digests, prompt and schema versions, corpus hash, measurements, and results.
- Report a live run finding fewer than ten eligible roles as source-coverage inconclusive, never as a pass or a failure.
- Report cloud-assisted runs separately, and never as a PoC pass.
- Add the acceptance record: every functional, security, deletion, offline, recovery, classifier, and matching gate, with what is measured on the target laptop recorded as NOT RUN until it is.

## Capabilities

### New Capabilities
- `frozen-corpus`: what the corpus is, when it is frozen, and what a change to it means.
- `benchmark-record`: what every run records, and why a result without it is not evidence.
- `acceptance-gates`: the pass conditions, the inconclusive outcome, and what cloud runs cannot do.

## Impact

- New `internal/bench/` — the corpus, the two scorers, the corpus hash, and the run record. Pure, so the scoring rules are table tests rather than model runs.
- New `internal/bench/testdata/` — the frozen corpus. Entirely invented: five candidate profiles, twenty role listings, and the labels a recruiter would apply.
- New `bench_livemodel_test.go` — the runs themselves, behind the same build tag as the existing live-model gates, reusing the service wiring the tests already have.
- `justfile` — `just bench`, alongside the existing gates.
- `docs/product/POC_ACCEPTANCE.md` — the acceptance record.
