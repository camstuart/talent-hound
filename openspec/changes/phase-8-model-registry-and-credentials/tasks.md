## 1. Schema

- [x] 1.1 Migration 8: `model_assignments` with role, endpoint, model, digest, params, validation, revision, created_at
- [x] 1.2 Unique index on (role, revision), which is also the lookup for a role
- [x] 1.3 CHECK on role and validation status — a table created rather than altered, so a CHECK is evaluated — plus a trigger refusing Validated with no benchmark reference

## 2. Registry

- [x] 2.1 `internal/models`: the three roles, validation statuses, and their validity rules
- [x] 2.2 `modelservice.go`: `Assign(role, model, digest, params)` appending a revision
- [x] 2.3 Refuse an unknown role, a non-local endpoint, an invalid URL, and unsupported parameters
- [x] 2.4 Re-assigning an identical configuration is a no-op, not a revision
- [x] 2.5 `Resolve(role)` with `classify` inheriting `generate` while unassigned
- [x] 2.6 `List()` for the settings view; history preserved
- [x] 2.7 `MarkValidated(role, benchmarkRef)` refusing an empty reference; new revisions start Unvalidated

## 3. Availability

- [x] 3.1 `internal/platform/ollama.go`: list installed models and pull one
- [x] 3.2 Classify a check into ready, endpoint unavailable, model missing, timeout, malformed response, or out of memory
- [x] 3.3 `Check()` returning one status per role, with the unassigned case distinct from a missing model
- [x] 3.4 `Decline(role)` remembered in memory for the process only
- [x] 3.5 `Pull(role)` as a background job with a coded failure reason

## 4. Credentials

- [x] 4.1 Native Windows Credential Manager and macOS Keychain stores, with unsupported platforms refusing and no fallback store
- [x] 4.2 A `SecretStore` seam with the platform implementation and a test-only in-memory one
- [x] 4.3 `credentialservice.go`: `Store`, `Has`, `Delete`, and no getter of any kind
- [x] 4.4 Refuse an empty secret; a missing credential is an answer, not an error

## 5. Frontend

- [x] 5.1 A settings view listing the three roles with model, revision, validation status, and availability
- [x] 5.2 Assigning a model per role; the inherited `classify` labelled as inherited
- [x] 5.3 A missing model offers a pull, and declining is visible
- [x] 5.4 Masked credential entry per provider, showing only whether one is stored
- [x] 5.5 Backend messages surfaced verbatim, as elsewhere

## 6. Tests

- [x] 6.1 Registry refuses missing roles, invalid URLs, non-local endpoints, and unsupported parameters
- [x] 6.2 Endpoint, model, digest, and parameter changes each create a revision; an identical assignment does not
- [x] 6.3 `classify` follows `generate`, keeps following when it changes, and stops once assigned
- [x] 6.4 Assignments are Unvalidated; validation without a benchmark reference is refused; a new revision resets it
- [x] 6.5 A fake OpenAI-compatible endpoint asserts the exact payload each role sends, and that a check carries no content
- [x] 6.6 Endpoint unavailable, model missing, pull declined, pull failed, timeout, malformed response, and memory error are distinct
- [x] 6.7 Credential create, replace, revoke, and missing-entry against the in-memory store; the platform store refuses off Windows
- [x] 6.8 A stored secret appears in neither the database file, the captured logs, nor any error string
- [x] 6.9 Vitest over the settings view: roles, inheritance label, pull prompt, masked entry
- [x] 6.10 Playwright: the settings view reports the roles through the real backend
- [x] 6.11 Fixtures are synthetic only — no real candidate information or real provider key anywhere

## 7. Windows gate (written and tagged; unrun — needs the target laptop)

- [ ] 7.1 Credential create, replace, retrieve, revoke, and missing-entry through the real Credential Manager, at the service level
- [ ] 7.2 A stored credential is absent from the database, the data folder, and a copied recovery folder

## 8. Exit gate

- [x] 8.1 All three local roles can be configured and checked without a credential in application data
- [x] 8.2 `just check` passes, with the standing `just vuln` toolchain advisories unchanged
