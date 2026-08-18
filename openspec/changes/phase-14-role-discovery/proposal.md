## Why

This is the phase where the application talks to the internet about a person for the first time, and every rule in it exists because of that sentence.

A query built from a candidate's profile is a disclosure. Left alone it would carry their name, their employer, their university, and the shape of their career to a third party who logs it — so the query is scrubbed of direct identifiers, organizations are generalized by default, and the recruiter sees the exact text before anything is sent. Not a summary of it: the bytes.

The second reason is reproducibility. A shortlist that came from a search nobody can reconstruct is a shortlist nobody can defend. So the visible query is stored on the search record, the disclosure audit event records that a request happened without recording what it said, and role identity resolves by a fixed precedence so the same listing found twice is one role rather than two.

The third is source respect. Exa is the source; direct fetching is deny-by-default with an empty allowlist, and SEEK, LinkedIn, authenticated pages, and anti-bot challenges are never automated. When Exa's text is not enough, the recruiter pastes it themselves, and the result is honest about not being automated provenance.

## What Changes

- Build role-listing queries from approved professional aspects and Search Criteria, never from candidate facts that were not approved.
- Scrub direct identifiers — name, email, phone, street address — and generalize exact employers, clients, projects, and schools by default.
- Show the exact query, let the recruiter edit it, and require confirmation. A cancelled preview sends nothing and records nothing.
- Warn additionally when the recruiter deliberately puts a specific organization or a direct identifier back into the query.
- Persist the visible query on the search record for reproducibility, and create one metadata-only disclosure audit event per request actually sent — never the query text, never any content.
- Resolve role identity by source ID, then canonical URL, then content fingerprint, deterministically and in that order.
- Implement source observations: an unchanged hash updates the retrieval time, changed content creates a new current source artifact and makes the previous one historical, and historical artifacts stay visible but leave current retrieval.
- Make roles go Stale at thirty days or at their closing date, and Active again on rediscovery, with a clock the tests control.
- Keep direct fetching deny-by-default with an empty allowlist, and provide manual paste as the honest fallback.

## Capabilities

### New Capabilities
- `identity-safe-queries`: what a query may contain, what is removed, what is generalized, and what warns.
- `query-preview`: the exact-bytes preview, the confirmation, and what a cancellation does not do.
- `role-discovery`: the search itself, caching with provenance, and the identity precedence.
- `source-observations`: unchanged, changed, historical, and stale, and what each does to derived data.
- `source-acquisition`: deny-by-default fetching, the empty allowlist, and the manual fallback.
- `disclosure-audit`: what a disclosure event records and, more importantly, what it must never record.

### Modified Capabilities
None. Roles, artifacts, and profiles already exist; discovery adds records beside them.

## Impact

- `internal/db/migrations.go`: migration 13 — `searches`, `disclosure_events`, and the source-observation columns on `artifact_links` and `roles`.
- New `internal/scrub/` — identifier removal and organization generalization, pure functions with no network and no model.
- New `internal/platform/exa.go` — the Exa client, and the allowlist that is empty.
- New `discoveryservice.go` — preview, send, cache, identity resolution, observations, and staleness.
- `frontend/src/components/DiscoveryPanel.tsx` — the query editor with its warnings, the confirm-or-cancel, and the results with their provenance and lifecycle state.
- A fake Exa server covering pagination, duplicates, missing fields, malformed records, rate limits, timeouts, offline, retry, and partial results; a fake clock for the thirty-day and closing-date boundaries.
- No browser automation, ever. No SEEK, no LinkedIn, no authenticated pages.
