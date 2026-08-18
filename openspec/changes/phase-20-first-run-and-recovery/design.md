## Context

Setup, recovery, and diagnostics are the three places where this application talks to someone who is not already inside it: a laptop that has never run it, a folder copied off a dying machine, and a report shown to whoever is helping. All three fail in the same direction — appearing to succeed — and all three are used exactly when the recruiter has the least ability to tell.

## Goals / Non-Goals

**Goals:**
- A wizard whose position is derived from what is true, not from a stored cursor.
- An encryption gate that runs at every startup and never resolves an unknown volume in favour of proceeding.
- A demo scope enforced at the write boundary, not in the UI.
- A diagnostic report built only from state the application already holds.
- Recovery that refuses without touching the recruiter's only copy.

**Non-Goals:**
- No automatic updates. The PRD says a developer-supplied installer, and an updater is a second distribution channel to secure.
- No backup feature. Copy-the-folder is the documented PoC answer; P1 owns backup.
- No log content. Not redacted content — no content, so there is nothing to redact incorrectly.
- No telemetry, not even disabled-by-default. A reporter with a flag is a reporter.
- No cross-platform encryption gate. Windows is the supported laptop; elsewhere the gate reports unavailable and real-data mode stays blocked.

## Decisions

### The wizard's position is computed, not stored

`setup.State()` runs the checks and returns the first unsatisfied step. Nothing writes "you are on step 4".

A stored cursor is wrong the moment reality moves underneath it — Ollama uninstalled after step 4, the data folder moved to a USB stick, a model deleted to free space. Recomputing means cancelling at any step is simply not finishing it, resuming is re-entering, and there is no resume bug to have. It is the same computed-not-stored rule Phases 11 and 12 apply to profile staleness.

*ponytail: every entry re-runs the checks, including the two that shell out. On a wizard the recruiter sees once, a few hundred milliseconds is not worth a cache with an invalidation rule.*

### What is remembered is one pointer and two facts

The data folder path, the acknowledgement, and the chosen scope live in `setup.json` in the user config directory — deliberately outside the data folder, because it is the pointer *to* that folder and a pointer inside the thing it points at cannot be followed.

Everything else — encryption, sidecar, Ollama, models — is checked live. Those are facts about the machine, and a remembered fact about a machine is a fact about the machine as it was.

### The encryption gate is a startup gate, and unknown is not encrypted

`platform.VolumeEncryption` already refuses to return `encrypted` on any error path. This phase calls it at every startup, not only during setup, and the result decides whether real-data mode is available at all.

Unencrypted, unavailable, and permission-denied are three different messages to the recruiter and the same answer to the application. A gate that treats "I could not check" as "probably fine" is not a gate.

### Demo scope is refused at the write boundary

In demo scope, creating an artifact and creating a candidate return an error from the service. Not a hidden button — the check is where the write is, because the UI is not the only caller and a future one will not remember.

Demo is deliberately almost useless: an empty workspace to see the shape of the product. The PRD is explicit that it is not an acceptance environment, and making it comfortable would make it a place real data ends up.

### The diagnostic report is assembled from facts, never from data

It contains the application version, the schema version, the resolved folder paths, the encryption status, sidecar and Ollama availability and versions, the configured model roles, row counts by kind, and the last N job outcomes as codes.

It never contains a name, a query, a payload, a draft, a filename, or a secret. That is achieved by not reading those tables for content at all rather than by scrubbing them afterwards — a redactor that misses one field produces a report that looks safe. Every string that does reach the report passes through `scrub.Text` as a second line, and control characters are stripped so the report is safe to paste anywhere.

*ponytail: counts and codes only. A report that cannot explain a specific failure is a real limitation, and the honest fix is better failure codes, not more content in the report.*

### Recovery opens nothing it cannot verify

Opening a copied folder is the existing `db.Open` path: integrity check, version check, snapshot before migration, restore and refuse on migration failure. This phase adds the check that runs *before* any of it — that the folder exists, is writable, and holds a database file — and reports each failure as its own reason.

A read-only folder fails before the integrity check rather than during a migration, because failing during a migration is how you find out your only copy was on a read-only volume.

### Delete-all names the resolved path

The confirmation shows `filepath.Clean` of the actual folder about to be removed, and the service compares the confirmation string against that same resolved path. Confirming "yes" to a folder described in words is how the wrong folder gets deleted; tests run only against temporary directories.

## Risks / Trade-offs

- **The Windows-only checks cannot run here.** The encryption gate, credential store, installer, and packaged sidecar are recorded as gate evidence with an explicit NOT RUN, in the same file Phase 1 established. A green suite on macOS is not evidence about Windows and is not recorded as if it were.
- **Re-running the checks on every wizard entry costs a moment.** Correctness over a cache nobody would notice.
- **Diagnostics may be too thin to explain a failure.** Accepted: the alternative leaks by design.
- **Demo mode is nearly empty.** Deliberate — see above.
