## 1. Boundary

- [x] 1.1 `internal/cloud`: the eligible tasks and the permanent deny list, with no override
- [x] 1.2 Placeholder substitution reusing the Phase 14 scrubber
- [x] 1.3 An allow/deny answer for every task, as a pure function

## 2. Schema

- [x] 2.1 Migration 16: `cloud_consents` keyed by initiative, endpoint revision, and task

## 3. Service

- [x] 3.1 `cloudservice.go`: the endpoint, its revision, and per-task approval
- [x] 3.2 Consent lookup matching all three or nothing
- [x] 3.3 An endpoint change producing a new revision, invalidating prior approvals
- [x] 3.4 Revocation taking effect before the next request
- [x] 3.5 `Preview` building the payload; `Send` transmitting it unchanged
- [x] 3.6 Chat previewing every send
- [x] 3.7 One disclosure event per non-localhost request, carrying no payload

## 4. Frontend

- [x] 4.1 A cloud panel: endpoint, per-task approvals, payload previews, revocation
- [x] 4.2 Denied tasks shown as permanently denied, not merely unapproved
- [x] 4.3 Backend messages surfaced verbatim

## 5. Tests

- [x] 5.1 A complete allow/deny matrix over every task
- [x] 5.2 No configuration sends raw candidate artifacts or candidate embeddings
- [x] 5.3 Consent for one initiative, revision, or task does not authorize another
- [x] 5.4 Endpoint change and revocation take effect before the next request
- [x] 5.5 First-use preview matches the sent payload; chat previews every send
- [x] 5.6 Names, emails, phones, and addresses become placeholders
- [x] 5.7 Cancelled previews, denied tasks, offline, timeout, provider error, and credential removal send nothing unexpected
- [x] 5.8 Audit events carry the required metadata and no content of any kind
- [x] 5.9 Playwright: approve, reuse, revoke, endpoint-change reset, and denial
- [x] 5.10 Fixtures are synthetic only — no real candidate information anywhere

## 6. Exit gate

- [x] 6.1 Cloud use is explicit, narrowly scoped, locally audited, and incapable of becoming the default runtime
- [x] 6.2 `just check` passes
