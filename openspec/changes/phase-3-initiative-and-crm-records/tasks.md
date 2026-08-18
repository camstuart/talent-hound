## 1. Schema

- [x] 1.1 Migration 2: `candidates`, `companies`, `contacts`, `roles` with exactly the PRD's fields, foreign keys, and the indexes the listing queries need
- [x] 1.2 Migration 3: add `status` to `initiatives` (default `active`, `CHECK` in the two known states) and nullable `candidate_id`
- [x] 1.3 Job Search cardinality enforced in the service, not a table-level `CHECK` — see design.md; the column shape is the "at most one" backstop
- [x] 1.4 Pre-Phase-3 Job Search rows keep a null candidate and are readable; only new writes are constrained
- [x] 1.5 Confirm the migrations still snapshot, apply atomically, and restore on failure using the Phase 2 runner — no runner changes (the live E2E database went v1 → v3 behind `snapshots/pre-v1.db` with its rows intact)

## 2. Models

- [x] 2.1 `internal/models/candidate.go`, `role.go`, `company.go`, `contact.go` matching the migrations exactly
- [x] 2.2 Extend `initiative.go` with lifecycle state and the candidate reference
- [x] 2.3 Shared value types in `values.go`: `Date`, `StringList`, `Compensation`, `CompensationPeriod`, plus `RoleOrigin`/`RoleLifecycle`/`InitiativeStatus` with `Valid()` methods in the style of `InitiativeType`

## 3. Validation

- [x] 3.1 Trim-then-require for required fields; preserve Unicode unchanged
- [x] 3.2 Date parsing and the closing-after-published rule (calendar days are YYYY-MM-DD text, never timestamps)
- [x] 3.3 Absolute http/https URL validation with no silent rewriting
- [x] 3.4 Compensation bounds: non-negative, min ≤ max, known currency, known period, partial statement allowed
- [x] 3.5 Optional contact details validated only when present
- [x] 3.6 Email and phone lists stored as JSON arrays, normalised and round-tripped

## 4. Services

- [x] 4.1 `InitiativeService`: `Rename`, `Archive`, `Reopen`, `Delete`, `Get`, and `List(includeArchived)`
- [x] 4.2 `InitiativeService.Create` enforces exactly one candidate for Job Search and none for the other types (`candidateIDs` is a slice so "more than one" is a rejectable request)
- [x] 4.3 `Delete` removes only the initiative and its owned rows; shared records untouched
- [x] 4.4 New `recordservice.go` with create, read, update, and list for all four record types
- [x] 4.5 `RecordService.ContactsAtCompany` returning company, count, and listing; unknown company is an error, empty is not
- [x] 4.6 Register `RecordService` in `main.go` and regenerate bindings

## 5. Frontend

- [x] 5.1 Four-area tab strip inside the initiative panel, each panel naming what will live there
- [x] 5.2 Talent Search and Business Development shells stating the pipeline is outside PoC scope
- [x] 5.3 Rename, archive, and reopen in the workspace; archived state visible in the sidebar and tab, with a "show archived" filter
- [x] 5.4 One `RecordForm` driven by a field list, used for all four record types: keyboard-operable, per-field errors
- [x] 5.5 Job Search creation flow that creates or selects the one candidate
- [x] 5.6 Backend validation messages surfaced verbatim, attached to the field their text names
- [x] 5.7 No delete control in the UI: deletion confirmation must list every affected link, which is Phase 19's work. `InitiativeService.Delete` exists and is tested.

## 6. Tests

- [x] 6.1 Table-driven Go tests over every valid and invalid initiative type and every lifecycle transition, including redundant ones
- [x] 6.2 Job Search creation rejects zero and multiple candidates, and a missing candidate reference
- [x] 6.3 Structured-field validation tests: missing required values, Unicode, whitespace, dates, URLs, compensation boundaries, optional contact details
- [x] 6.4 Shared candidate and role referenced from multiple initiatives with one row and no copying
- [x] 6.5 Contacts-at-company returns only the selected company's contacts; empty result; unknown company errors
- [x] 6.6 Archive and reopen preserve every reference; initiative deletion leaves all shared records
- [x] 6.7 Vitest: form validation, keyboard operation, error display, lifecycle labels
- [x] 6.8 Playwright: create each initiative type, persist records across a reload, archive and reopen, rename, confirm the four-area shell, per-field backend errors
- [x] 6.9 Fixtures are synthetic only — no real candidate information in source control, logs, or test output

## 7. Exit gate

- [x] 7.1 FR-01 and FR-02 work end to end against the real backend, deletion rules excluded as deferred to Phase 19
- [x] 7.2 `just check` passes: `qa` plus all three test layers (`just vuln` still fails on the pre-existing Go stdlib advisories that need a go1.26.6 toolchain — unchanged by this phase)
