# Classifier evidence rule, measured on the tuning corpus

The frozen corpus diagnoses; it does not get to set rules. This is the same
argument the retrieval constants were settled by, applied to an evidence rule:
the change below was measured on the tuning corpus — different companies,
cities, and domains, with a test refusing any overlap — before the frozen set
was allowed to score it.

Model: `qwen2.5:14b-instruct`, temperature 0, top_p 1.

Three rules were measured here, one at a time, each kept only because this
corpus said it worked.

| | baseline | wording | + derivation | + region |
| --- | --- | --- | --- | --- |
| Capture | 91% (82/90) | 96% (86/90) | 99% (89/90) | 99% (89/90) |
| Uncited | 0 | 0 | 0 | 0 |
| Unsupported | 2 | 2 | 2 | 0 |
| Misreported | 7 | 3 | 0 | 0 |

## The rule

Wording assembled entirely from the words of a chunk the aspect cites counts as
quoting, and only the wording is admitted as evidence — not the sentences it
borrowed from.

The contract already accepted wording that appears verbatim in a source as
quoting, whether or not the model labelled it a quote. What it did not accept
was wording that compresses two of the document's sentences into one. A listing
reading

> Silverleaf Consulting is hiring a backend engineer (contract) in Melbourne.
> This is a remote role, offered as contract work at AUD 900 per day.

produced a location worded "remote role in Melbourne" citing the second
sentence. The city was dropped, leaving an aspect whose wording says Melbourne
beside a location field naming no city — the same aspect stating a place in
prose and omitting it from the field the matching reads. That inconsistency is
a defect on its own terms: what the recruiter reads and what the shortlist
filters on disagree.

The rule admits no word the cited chunk does not already contain, so a place
the listing never names is still unsupported however confidently it is worded,
and a word taken from a chunk the aspect did not cite is refused. All three
cases are pinned by unit tests.

## The second rule: a constraint no aspect carries

Scoring this corpus for the first time showed `employment_type` missing on 7 of
10 listings, from wording near-identical to the frozen corpus's, which the model
reads correctly on all 20 of those. The wording rule moved it to 3, which was
not its purpose and made it look like noise. It was not.

A diagnostic run on the three that still failed gave the cause, and it was not
the one assumed. Nothing was refusing the aspect: the model folds the employment
type into a different aspect's wording and emits no aspect for it at all —

    work_arrangement {arrangement: onsite} wording="onsite, permanent"

`DeriveStructured` fills fields on aspects that exist, so nothing could reach
evidence sitting in a sentence the profile already cites. `DeriveConstraintAspects`
records a constraint a cited sentence states outright when no aspect carries it,
using the same enumeration every other value is judged by, and the new aspect
carries the citation that stated it. A sentence naming two employment types
states neither.

## The third rule: a bare place name is the city

The two remaining unsupported values were both a `region` holding a place the
source called no kind of place — "hiring a telemetry engineer in Nelson"
producing region: Nelson. The convention that a bare place name is the city is
the product's own, stated in the prompt and built into the taxonomy;
`NormalizeStructured` now applies it, and only when no city is recorded and the
evidence says neither region, state, province, county, territory nor district.
A source that says which kind of place it means has said it.

## What is still open

- **Run-to-run variance is real.** `platform-engineer-melbourne` lost its
  employment type in one frozen run and recorded it correctly in the next, at
  temperature 0 and top_p 1. A single passing run is a sample, not a property.
  Nothing here is safe to read as a constant.
- **The corpora are invented.** Both of them, as their own files say.
