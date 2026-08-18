## 1. Scrubbing and query building

- [x] 1.1 `internal/scrub`: remove name, email, phone, street address, and stored identifiers
- [x] 1.2 Generalize employers, clients, projects, and schools
- [x] 1.3 Detect a re-added organization and, distinctly, a re-added direct identifier
- [x] 1.4 Build a query from approved aspects and criteria only

## 2. Schema

- [x] 2.1 Migration 13: `searches` with the visible query, and `disclosure_events` with no content column
- [x] 2.2 Source-observation columns: current versus historical on the role's source links
- [x] 2.3 Retrieval and closing dates on roles, for staleness

## 3. Provider

- [x] 3.1 `internal/platform/exa.go`: the client, with distinct errors for rate limit, timeout, and offline
- [x] 3.2 The empty allowlist and the permanent denylist, with no override
- [x] 3.3 Pagination, duplicates, missing fields, malformed records, and partial results

## 4. Service

- [x] 4.1 `discoveryservice.go`: `Preview` returning the exact query, `Send` transmitting it unchanged
- [x] 4.2 Cancellation writes nothing at all
- [x] 4.3 One disclosure event per request sent, carrying no content
- [x] 4.4 Identity precedence: source ID, then canonical URL, then fingerprint
- [x] 4.5 Source observations: unchanged, changed, historical, stale
- [x] 4.6 Staleness from an injected clock: thirty days and closing date
- [x] 4.7 Manual paste stored as recruiter-supplied provenance

## 5. Frontend

- [x] 5.1 A discovery panel with the exact query, editable, and confirm or cancel
- [x] 5.2 The organization warning and the stronger identifier warning, shown distinctly
- [x] 5.3 Results with provenance and lifecycle state; manual paste for thin content
- [x] 5.4 Keyboard operable; backend messages surfaced verbatim

## 6. Tests

- [x] 6.1 Query fixtures remove name, email, phone, address, and structured identifiers
- [x] 6.2 Organizations generalized by default; deliberate re-addition warns, and identifiers warn more
- [x] 6.3 The previewed query equals the sent query byte for byte
- [x] 6.4 A cancelled preview sends nothing and records nothing
- [x] 6.5 A sent search stores its visible query; its audit event stores no content
- [x] 6.6 Fake Exa covers pagination, duplicates, missing fields, malformed records, rate limits, timeout, offline, retry, partial
- [x] 6.7 Identity precedence is deterministic, including when signals disagree
- [x] 6.8 Same hash updates retrieval; changed content creates a current artifact and historicizes the old one
- [x] 6.9 Thirty-day and closing-date boundaries with a fake clock; rediscovery reactivates
- [x] 6.10 SEEK, LinkedIn, unlisted, and allowlisted-but-denied fetches all refused
- [x] 6.11 Manual paste completes thin content without claiming automated provenance
- [x] 6.12 Playwright: preview, edit, cancel, send, cache, revisit through a fake Exa server
- [x] 6.13 Fixtures are synthetic only — no real candidate information anywhere

## 7. Exit gate

- [x] 7.1 Discovery is reproducible, auditable, source-safe, and produces current evidence without browser automation
- [x] 7.2 `just check` passes
