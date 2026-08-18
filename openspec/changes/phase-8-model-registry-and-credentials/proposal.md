## Why

Phases 6 and 7 got documents into a state a model could use. Nothing in the application yet knows which model, at which endpoint, with which parameters — and everything after this depends on that answer being written down rather than assumed. An embedding is only comparable with another embedding from the same model at the same revision; an assessment is only reproducible if the configuration that produced it can be named. Phase 9 keys embedding spaces on exactly this record, so it has to exist first and it has to be immutable once something has been derived from it.

The second half is the credential. Exa arrives in Phase 12 and optional cloud tasks later, and the PRD's rule is absolute: provider keys live in the Windows credential store and never enter application data, logs, diagnostics, or a copied recovery folder. That rule is easy to state and easy to break by accident — a struct field that gets serialized, an error that quotes its input, a debug log. It is worth building the boundary before there is a key to put through it.

## What Changes

- Persist the `embed`, `classify`, and `generate` assignments as append-only revisions: changing the endpoint, model, digest, or parameters writes a new revision, and re-assigning an identical configuration writes nothing.
- Default `classify` to the `generate` assignment by inheritance rather than by copying it, so it keeps following until it is explicitly assigned.
- Record model name, immutable digest when available, parameters, and a validation status that starts Unvalidated and cannot be set without a benchmark reference.
- Refuse a registry assignment pointing anywhere but the local endpoint: the cloud is a task-level override in a later phase, never a required role here.
- Verify Ollama and the assigned models, reporting unavailable, model missing, pull declined, pull failed, timeout, malformed response, and memory error as distinct states rather than one failure.
- Run a model pull as a background job, with the Phase 5 lifecycle.
- Store provider secrets only in the Windows credential store, behind an interface with no file-backed implementation, and never return one to the frontend.
- Prove by test that a stored secret appears nowhere in the database file, the logs, or any error string.

## Capabilities

### New Capabilities
- `model-registry`: the three local roles, their revisions, inheritance, validation status, and what an assignment is allowed to point at.
- `local-model-availability`: verifying Ollama and the assigned models, and the distinct states a failure can be in.
- `provider-credentials`: where a secret may live, what may be learned about it, and everywhere it must not appear.

## Impact

- `internal/db/migrations.go`: migration 8 — `model_assignments`, append-only and revisioned.
- New `modelservice.go` and `credentialservice.go`; `internal/platform/ollama.go` gains model listing and pull; new `internal/platform/credential_other.go` refusing on every platform that is not Windows.
- `frontend/bindings/` regenerated; a settings view showing the three roles, their revisions and status, the availability of each model, and a masked credential entry.
- Go tests against a fake OpenAI-compatible endpoint asserting the exact payload each role sends; a redaction test that scans the database file and the logs for a stored secret.
- The Windows-only half — the real credential round trip through Credential Manager — joins the existing gate evidence rather than closing it.
- No cloud provider is called, no consent flow, no audit events. Those belong to the phases that introduce a request worth auditing.
