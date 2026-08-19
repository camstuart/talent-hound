## 1. Corpus

- [x] 1.1 `internal/bench/`: corpus types and loader, refusing an unlabelled corpus
- [x] 1.2 Five matching scenarios, with the recruiter's plausibility ratings
- [x] 1.3 Twenty classifier role listings, with material aspects and structured constraints
- [x] 1.4 Corpus hash over sorted, length-prefixed file bytes
- [x] 1.5 The corpus states that its content is synthetic

## 2. Scoring

- [x] 2.1 Classifier score: citations, unsupported critical constraints, capture, structured constraints
- [x] 2.2 Capture measured by meaning, or by containment of the labelled wording within the same type
- [x] 2.3 Matching score: duplicates collapsed, absent slots not plausible, three-of-five in four-of-five
- [x] 2.4 Fewer than ten eligible roles reported as inconclusive
- [x] 2.5 A cloud-assisted run cannot be a PoC pass
- [x] 2.6 A scenario that produced nothing carries the reason into the record

## 3. Record

- [x] 3.1 Run record: configuration, model digests, prompt and schema versions, corpus hash, measurements, results
- [x] 3.2 Each classifier condition reported separately
- [x] 3.3 A missing model assignment stated rather than omitted
- [x] 3.4 Record written as JSON and as a readable summary

## 4. Runs

- [x] 4.1 `bench_livemodel_test.go` behind the live-model build tag, reusing the test wiring
- [x] 4.2 `just bench`
- [x] 4.3 Eighteen runs recorded with their models, corpus hash, and what changed before each; target-laptop timings still NOT RUN

## 5. Tests

- [x] 5.1 Scoring tables over every pass and fail condition, including the boundaries
- [x] 5.2 Corpus hash stable across processes and sensitive to one byte
- [x] 5.3 Loader refuses an unlabelled corpus
- [x] 5.4 Inconclusive is neither pass nor fail
- [x] 5.5 Fixtures are synthetic only — no real candidate information anywhere

## 6. Acceptance record

- [x] 6.1 `docs/product/POC_ACCEPTANCE.md`: every functional, security, deletion, offline, recovery, classifier, and matching gate
- [x] 6.2 (recorded NOT RUN) Target-laptop measurements listed with their conditions, recorded NOT RUN until run
- [x] 6.3 Accessibility walkthrough, as five Playwright tests: keyboard operation, and source versus recruiter versus AI content
- [x] 6.4 (installer smoke recorded NOT RUN) Final `just check`, installer smoke, security scan, vulnerability scan, redacted-log inspection

## 7. Exit gate

- [x] 7.1 Every gate that can run off the target laptop passes, except the two benchmarks, which are run and recorded as FAIL for the models this machine can hold
- [x] 7.2 `just check` passes
