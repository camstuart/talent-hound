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

Capture asks whether the substance the recruiter labelled is present, not whether the model chose the same words for it. A label counts when an extracted aspect of the same type either means the same thing by Phase 10's duplicate rule — the same meaning key, so "must have Go" and "Go is required" are one aspect here as they are everywhere else — or contains the labelled wording.

The meaning key alone was the first rule, and the first live run showed why it is wrong: a label reading "Go" never equals an extracted aspect reading "Must have strong Go and production SQLite experience", though a recruiter labelling that listing would obviously count it as captured. Containment runs one way only. The label is the terser statement, and finding it inside a fuller one is the case this rule exists for; a one-word extraction does not capture a detailed requirement.

### A scenario that produced nothing says why

An empty top five because the candidate profile could not be built is not the matcher failing, and a record showing both as "0 plausible" sends the reader after the wrong thing. The reason travels with the scenario, verbatim, into the record.

### Matching takes ratings, it does not produce them

`ScoreMatching` takes the top five per scenario with the recruiter's rating attached, collapses duplicates first, and counts an absent slot as not plausible. The recruiter's ratings are an input, because the PRD says a human decides plausibility.

### Too few roles is inconclusive, and inconclusive is not a result

A live run yielding fewer than ten eligible roles returns `OutcomeInconclusive` with the count. Not pass, not fail. The distinction exists because a thin corpus and a bad matcher produce the same low number, and only one of them is this product's fault.

### The runs sit behind the live-model build tag

The benchmark needs real models, so it lives where the existing live-model gates live: a `livemodel`-tagged test file using the same service wiring the tests already have. `just bench` runs it. Nothing about a model on a laptop belongs in the default suite, which has to pass on a machine with no models at all.

*ponytail: the benchmark reuses the test environment constructors rather than building a second service graph. They already wire everything correctly, and a second wiring is a second thing to keep in step.*

### What the benchmark may not be used to tune

The frozen corpus diagnoses; it does not get to set constants. Measured against
it, the fusion looked like it was flattening: `perQueryDepth` is thirty against
twenty eligible roles, so every role matching a query at all enters every list,
and reciprocal rank fusion at K=60 separates rank thirty from rank one by less
than a third.

Rather than change a constant against the held-out set, a separate tuning
corpus was built — different companies, cities, and domains, with a test
refusing any overlap — and the constants swept on it. The result was flat: K at
5, 10, 20 and 60 scored identically, and depth mattered only below ten. **The
shipped constants are already the best available, and the hypothesis was
wrong.**

That is the argument for tuning where you are allowed to: on the frozen corpus
the change would have looked like an improvement, been indistinguishable from
one, and been a coincidence.

A defect is different from a constant. ORing the word "the" into every query
was a defect, and fixing it was not tuning.

**Re-run on a corrected instrument, the conclusion held.** Real decomposed role
profiles, one workspace, K from 1 to 60, depth from 3 to 30: all thirty
configurations score identically. The constants were right and the reasoning
that defended them was lucky — see below for what the instrument could not see.

**That sweep's original conclusion did not transfer.** It was run when the semantic
half asked for a page of *aspects* and collapsed them to roles, and when the
benchmark's roles carried a single aspect holding the title — so `perQueryDepth`
was thirty aspects against twenty of them, and every role entered every list.
Depth now counts roles, and the roles carry their real decomposed profiles. The
constants are unchanged and nothing here proposes changing them; what changed is
that the evidence for them was gathered under different retrieval semantics, and
a future sweep has to be run again rather than cited.

## Risks / Trade-offs

- **The frozen corpus is invented.** Stated in the corpus and in the record. It exercises the harness; it does not substitute for the recruiter's real placements, and the acceptance record says so.
- **Five scenarios is a small sample.** The PRD's choice, kept deliberately.
- **Containment can count a label as captured inside an aspect that is mostly about something else.** The alternative — the meaning key alone — scored a correctly decomposed listing at zero, which is the worse error: it makes a working model look broken.
- **Nothing enforces that tuning avoided the corpus.** Only the hash and the record make a change visible.
