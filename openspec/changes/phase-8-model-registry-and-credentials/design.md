## Context

This phase writes down two things the application has so far been able to avoid: which model produced a result, and where a secret is allowed to live.

Both are records that other phases will depend on being immutable. An embedding space is identified by an endpoint revision and a model digest; if that record can be edited in place, every vector derived from it silently changes meaning. A credential is the opposite problem — not a record that must not change, but a value that must not be recorded at all, anywhere except one place the operating system owns.

## Goals / Non-Goals

**Goals:**
- Three local roles, always present, always nameable, with a revision that changes exactly when the configuration does.
- `classify` that follows `generate` without a second copy of the same configuration.
- Model availability reported as distinct states, because the recruiter's next action differs for each.
- A credential boundary with no way to get a secret back out and no fallback store.

**Non-Goals:**
- No cloud provider calls, consent binding, or audit events. Phase 12 introduces the first request worth auditing and can settle consent with a concrete payload in front of it.
- No benchmarks. Phase 21 owns them; this phase only ensures nothing can claim Validated without one.
- No embeddings. Phase 9 consumes this registry; nothing here anticipates its schema beyond recording what it will key on.
- No model reliability detection. The PRD is explicit: schema errors, memory failures, and timeouts are reported directly, and the application does not claim to judge a model at runtime.

## Decisions

### An assignment is an append-only revision, and the revision is its identity

`model_assignments` holds one row per configuration a role has ever had: role, endpoint, model, digest, parameters, validation status, revision. Assigning writes a new row with the next revision; the current assignment for a role is its highest revision.

Editing in place was the alternative and it is the one that quietly corrupts Phase 9. An embedding space is identified by "endpoint configuration revision, model digest, dimensions, and metric" — an identifier that points at a mutable row is not an identifier. Append-only also makes the revision rule trivial to state and to test: revision N+1 exists if and only if something about the configuration differed from revision N.

Which is the second half of the rule: **re-assigning an identical configuration writes nothing**. Without that, the settings screen's save button becomes a revision generator, and every save invalidates derived data that did not need invalidating.

### `classify` has no row until it has an opinion

The PRD says `classify` defaults to the local `generate` model, and the plan says without duplicating configuration. Copying `generate`'s row into a `classify` row would satisfy the first and break the second: the copy stops following the moment `generate` changes, and nothing on screen would say so.

So the absence of a `classify` row *is* the default. Resolving `classify` returns the `generate` assignment along with the fact that it was inherited. Assign `classify` explicitly and it gets a row and stops following; there is no un-assign in this phase, because "go back to following" is a fourth state to reason about and nothing needs it yet.

### The registry is local-only, by rule

Every registry role must point at the local Ollama endpoint. Not "should", not "by default" — an assignment naming a non-local endpoint is refused.

The PRD's boundary is that raw candidate artifacts, Candidate Profile extraction, and embeddings are local-only, and that the cloud endpoint is a task-level override, never a replacement for the required local configuration. A registry that could hold a cloud endpoint would make "local-only" a property of every call site rather than of one record — and the call sites arrive over the next ten phases, written by someone who has forgotten this paragraph.

### Validated is a claim, so nothing here can make it

Every assignment starts Unvalidated. The only way to Validated is `MarkValidated(role, benchmarkRef)` with a non-empty reference to a benchmark record, and no benchmark records exist until Phase 21.

That leaves a method nothing calls, which is usually a smell. It is not one here: the alternative is a status field that defaults to Unvalidated and has no way to change, which reads as unfinished rather than as deliberate, and invites the next person to flip it in an update statement. The gate is the point.

### Failure states are distinct because the next action is

Ollama unavailable, model missing, pull declined, pull failed, timeout, malformed response, and memory error are seven codes, not one "model unavailable". They are distinguished because what the recruiter should do differs every time: start Ollama, pull the model, pull it after all, retry, wait, report a bug, close something.

`pull_declined` is the odd one, because it is a fact about the person rather than the system. It lives in memory for the life of the process, with no column: a decline that does not survive a restart is exactly right, since the next launch should ask again. `pull_failed` is stored the same way and for the same reason — a failed download is worth remembering until the next attempt, not until the end of time.

*ponytail: in-memory decline, per process. A column when a recruiter complains about being asked twice in one week rather than twice in one session.*

Writing the test for these found that the classification only half existed. It lived inside the client's shared `post`, and the availability check does not use `post` — it lists models over `GET`, which had its own two-line error handling that reported every non-200 as a malformed response. So an endpoint that was up and out of memory read as an endpoint talking nonsense, which is the exact collapse the seven states exist to prevent, in the one call the states are computed from. The classification now belongs to the response rather than to the method that happened to fetch it.

### A pull is a job

Pulling a model is minutes of network transfer, which is the Phase 5 lifecycle's whole purpose: progress, cancellation, retry, and honest state after a crash. It registers a `pull` worker like everything else slow, and the compute/commit split from Phase 6 keeps the transfer out of the write lock.

### The secret crosses the boundary once, inwards

`CredentialService.Store(provider, secret)` takes a secret. Nothing returns one. The frontend can ask `Has(provider)` and get a boolean, and `Delete(provider)` to revoke; there is no `Get`, and the Go code that will eventually need the value reads it from the store directly at the moment of the request.

This is the whole design. A getter on a service bound to the frontend is a getter reachable from the frontend, and from there the value is one console log, one error message, or one crash report from being written down somewhere it can be read.

The service also refuses to store an empty secret and trims nothing else — a key with a trailing space is a key the provider will reject, and silently repairing it hides the recruiter's paste error.

### Each supported operating system uses its native credential store

Windows uses Credential Manager and macOS uses Keychain. `internal/platform/credential_other.go` returns a distinct "unsupported on this platform" error everywhere else. No implementation writes a file, an environment variable, or an encrypted blob.

A fallback would be the reasonable-looking change that breaks the PRD gate: credentials live only in the operating system credential store. A development convenience that stores them elsewhere would make that condition fail quietly.

To keep the service's own rules testable everywhere, the store is one small interface with two implementations: the platform one, and an in-memory one that exists only in tests. Nothing file-backed ships.

### The redaction test scans the artifacts, not the code

Asserting that no code path logs a secret is asserting about code that has not been written yet. So the test stores a distinctive secret through the service, exercises every operation, and then reads the database file's bytes, the captured log output, and the text of every error returned, searching for that value.

It is a crude test and that is why it works: it does not care how the secret might have escaped.

## Risks / Trade-offs

- **The credential round trip needs a native gate run.** Create, replace, retrieve, revoke, and missing-entry run against Windows Credential Manager or macOS Keychain under the `credentialgate` tag. Everything about the service's own behaviour is also proven against the in-memory store.
- **Availability checks need a running Ollama.** The state machine is tested against a fake OpenAI-compatible endpoint that returns each failure on demand; the real Ollama proofs remain behind the `livemodel` tag, where Phase 1 put them.
- **Append-only assignments grow.** One row per configuration change, on a single-user desktop app, forever — which is a few dozen rows in a realistic lifetime and a table nobody ever needs to prune.
- **No un-assign for `classify`.** Once explicitly assigned it stops following `generate` permanently. Restoring inheritance means adding a state; the phase that wants it can pay for it.
