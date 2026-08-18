## Context

A benchmark is a measurement, and a measurement nobody can reproduce is an anecdote. The design problem is not scoring — the PRD already fixed the rules — it is making every run carry enough of its own context that a later result can be compared with an earlier one honestly.

## Goals / Non-Goals

**Goals:**
- Scoring rules as pure functions, so they are table tests and not model runs.
- A corpus frozen as data, hashed, and read the same way by every run.
- A record that says what it ran with, not only what it scored.
- Inconclusive as a first-class outcome.

**Non-Goals:**
- No tuning against the frozen corpus. That is the point of freezing it, and no amount of tooling enforces it — the corpus hash makes a change visible instead.
- No automatic plausibility judgement. The PRD says the recruiter rates the top five; a scorer that decided plausibility would be measuring the same model twice.
- No new statistics. Five scenarios is five scenarios, and a confidence interval over it would dress up a number the PRD deliberately kept simple.
- No cloud runs in the pass path. They are recorded, separately, and cannot pass anything.

## Decisions

### The corpus is data, and the labels come with it

`internal/bench/testdata/` holds the scenarios and listings as JSON, each with the labels the PRD says are applied before any model runs: material aspects, structured constraints, and — for matching — the recruiter's plausibility ratings.

Everything in it is invented. That is a real limitation and it is written into the corpus file itself: these labels stand in for a recruiter's, so the harness can be exercised end to end, and the acceptance run replaces them with the real corpus. What the fixtures do prove is that the scorers, the thresholds, and the record are correct before any of that arrives.

### The corpus hash is over the bytes, sorted

One SHA-256 over every corpus file, in name order, with each name length-prefixed — the same canonical-hashing rule Phase 16 uses for assessment inputs, for the same reason: a hash that depends on map iteration order is a hash that changes when nothing did.

A changed hash does not fail a run. It appears in the record, so "it passes now" and "the corpus changed" cannot be confused.

### Scoring is four conditions, reported separately

The classifier score reports citation coverage, unsupported critical constraints, material-aspect capture, and structured-constraint reproduction as four results plus one pass. A single boolean would answer "did it pass" and lose the only thing a failing run is useful for: which of the four it failed.

Capture is measured by meaning key, not string equality — the same key Phase 10 already uses to detect duplicate aspects, so "must have Go" and "Go is required" are one aspect here as they are everywhere else.

### Matching takes ratings, it does not produce them

`ScoreMatching` takes the top five per scenario with the recruiter's rating attached, collapses duplicates first, and counts an absent slot as not plausible. The recruiter's ratings are an input, because the PRD says a human decides plausibility.

### Too few roles is inconclusive, and inconclusive is not a result

A live run yielding fewer than ten eligible roles returns `OutcomeInconclusive` with the count. Not pass, not fail. The distinction exists because a thin corpus and a bad matcher produce the same low number, and only one of them is this product's fault.

### The runs sit behind the live-model build tag

The benchmark needs real models, so it lives where the existing live-model gates live: a `livemodel`-tagged test file using the same service wiring the tests already have. `just bench` runs it. Nothing about a model on a laptop belongs in the default suite, which has to pass on a machine with no models at all.

*ponytail: the benchmark reuses the test environment constructors rather than building a second service graph. They already wire everything correctly, and a second wiring is a second thing to keep in step.*

## Risks / Trade-offs

- **The frozen corpus is invented.** Stated in the corpus and in the record. It exercises the harness; it does not substitute for the recruiter's real placements, and the acceptance record says so.
- **Five scenarios is a small sample.** The PRD's choice, kept deliberately.
- **Meaning-key matching can merge two aspects a recruiter meant separately.** It merges them everywhere else in the product too, so the benchmark measures what the product does rather than something kinder.
- **Nothing enforces that tuning avoided the corpus.** Only the hash and the record make a change visible.
