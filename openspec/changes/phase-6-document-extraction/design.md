## Context

Phase 1 already proved the hard part on Windows: a process under a Job Object with a wall-clock timeout, a whole-tree memory cap, an output cap, and kill-on-close. `platform.ExtractMarkdown` exists and works. What is missing is everything around it — deciding whether to call it at all, giving it a file that carries no identity, verifying it is the binary we shipped, keeping its output at arm's length, and making sure that a crash mid-extraction does not leave a recruiter's PDF sitting unencrypted in a temporary directory forever.

The riskiest thing here is not the parser. It is the plaintext copy.

## Goals / Non-Goals

**Goals:**
- One file, one process, one result, with a temporary copy that dies with the run.
- A verified sidecar, checked before document bytes are written anywhere.
- Failures that are actionable to the recruiter and silent about content.
- Extraction that uses the Phase 5 lifecycle rather than inventing a second one.

**Non-Goals:**
- No OCR. A scanned PDF is a clear retryable failure, per the PRD.
- No batch extraction, and no contract shaped to accommodate one later.
- No re-extraction sweep when the extractor version changes. The artifact records which version produced its Markdown; deciding what to do about a stale one is a later phase's problem.
- No sandbox. This is failure isolation. The sidecar runs with the recruiter's permissions and the PRD accepts that for the PoC.

## Decisions

### A worker computes first and commits second

Phase 5 handed every worker the transaction that would also record its item as complete, and noted the ceiling: SQLite has one writer, so an item that takes a minute is a minute in which nothing else in the application can write. A demo worker that sleeps 500ms made that theoretical. A sidecar with a two-minute timeout makes it fatal — an extraction would freeze the initiative list, the artifact panel, and every other job in the process.

So `JobWorker` splits:

```go
type JobWorker func(ctx context.Context, job models.Job, item int) (JobCommit, error)
type JobCommit func(tx *gorm.DB) error
```

The worker runs with no transaction open: it reads what it needs, does the slow thing, and returns a closure that writes the result. The runner opens one transaction, calls the closure, and increments the completed count inside it. The per-item boundary the Phase 5 spec promises is unchanged — the work and the count still commit together or not at all — but the window in which the writer is held is now the length of one `UPDATE`, not the length of the work.

Cancellation gets simpler as a side effect: the point at which cancellation is honoured is no longer also the point at which the database is locked, so a cancel never waits on the item it is interrupting.

A worker that needs to read inside the same transaction it writes can do the read in the commit half. Nothing in this phase does.

### Extraction state lives on the artifact, not on the job

A job is how something ran; an artifact is what it produced. The job row already carries state, counts, and a failure code, and it is disposable — a retry makes a new run of the same work. The extraction result is not disposable: it is provenance, and the PRD lists extraction state, error, extractor name and version among the artifact's fields.

Migration 6 therefore adds `extraction_state`, `extraction_error`, `extractor`, `extractor_version`, and `markdown` to `artifacts`. The job carries only `{"artifactId": N}` — an identifier and nothing else, per the Phase 5 rule that params hold identifiers and numbers, never content.

There is no `running` extraction state. In-progress is a property of the job, which already has one, and adding a second place to be in-progress would mean a second place to be wrong after a crash. An artifact is `pending`, `extracted`, or `failed`; it goes back to `pending` when a retry starts.

### The sidecar is verified at startup, and re-checked before the bytes are written

Verification is: the configured path is absolute; it exists; it is a regular file; running it with `--version` inside a short timeout prints the pinned version. That runs once at startup and the result is cached, because the answer cannot change without the install directory changing, and because a version probe per extraction is a subprocess per extraction for no new information.

But the *stat* is cheap and the ordering matters. The PRD requires that a relative, missing, mismatched, or substituted sidecar is rejected **before document bytes are materialized**, so the extraction path re-stats the verified absolute path as its first act, before it creates a directory or writes a byte. A sidecar swapped after startup fails the extraction rather than running.

When verification fails at startup, extraction is not disabled quietly: every extraction of a sidecar-requiring type fails immediately with `sidecar_missing` or `sidecar_version`. The application still runs — a recruiter with a broken install can still keep notes and records — and the failure names itself the moment they try to use it.

### The temporary copy is anonymous, private, and swept

The staging directory is `<data folder>/extract/<24 hex chars>/`, created `0700`, holding one file named `input<ext>`. The extension is carried because the sidecar dispatches on it; nothing else about the original is. No candidate name, no original filename, no artifact display name — the path is an accident, and paths end up in error strings, crash dumps, and shoulder-surfing distance of a screen.

The directory is removed with a `defer` on every exit path, including the panic path, and then again at startup: `NewExtractService` deletes every child of `<data folder>/extract/`. A crash is the only way one survives, and it survives exactly until the next launch. Naming the sweep root `extract/` rather than scattering directories through the data folder is what makes that sweep a single unambiguous `RemoveAll` per child rather than a pattern match over the recruiter's data.

The data folder is where the database already lives, because that is the folder the PRD says is encrypted. A staging directory in the system temp folder would be outside BitLocker's reach, which is the one place these bytes must never be.

### Text bypasses the sidecar entirely

`text/plain` and `text/markdown` extract to their own bytes. No process, no temporary file, no directory. The extractor is recorded as `native-text` so the provenance still says who produced the Markdown. The 10 MB cap applies here too — an artifact may be 25 MB of text, and the extraction contract is 10 MB whatever produced it.

Anything that is neither text nor PDF nor DOCX fails with `unsupported_type`. That is a failure and not a state of its own, because the recruiter's next question is the same either way: why is there nothing to search?

### Failure reasons are codes, and the sidecar's words are never among them

The sidecar reports errors on stderr, and a document parser's errors quote the document: byte offsets, embedded font names, fragments of text. Storing that would put candidate content into a failure record — exactly what Phase 5's coded-reason rule exists to prevent.

So the mapping is one-way and lossy on purpose: `platform.ErrTimeout` → `extract_timeout`, `ErrMemoryLimit` → `extract_memory`, `ErrOutputLimit` → `extract_output`, `ErrExtract` and everything else → `extract_failed`; empty output → `extract_empty`, which is what a scanned or image-only PDF produces and the closest the PoC gets to "this needs OCR". stderr is read only to be discarded.

The codes reuse Phase 5's vocabulary rules, so a failed extraction job and a failed artifact carry the same shape of reason and the same `CHECK` constraint enforces it in both tables.

### The reason-code constraint had to be rebuilt to actually hold

Phase 5 wrote `CHECK (failure_reason = '' OR failure_reason GLOB '[a-z][a-z0-9_]*')` and this phase copied it for artifacts. Writing the test that stores a sentence showed it does nothing: GLOB's `*` matches *any* characters, not the character class beside it, so the pattern constrains only the first letter — and "the candidate's file was missing" begins with a lowercase letter. The rule everyone would read as "codes only" admitted every sentence that starts in lower case.

The condition that holds is the inverse: `col GLOB '[a-z]*' AND NOT col GLOB '*[^a-z0-9_]*' AND length(col) <= 40` — begins with a letter, contains nothing outside the vocabulary, and is short.

It could not be tightened in place. SQLite also accepts a `CHECK` on a column added by `ALTER TABLE ADD COLUMN` and then never evaluates it, so both the new artifact columns and the old jobs column needed something else, and rebuilding two tables to move a constraint is more machinery than the constraint. Migration 6 adds before-insert and before-update triggers on both tables instead. The Phase 5 service check was always correct; what was missing was the database refusing it too, which is the half that matters when the next phase writes to these columns from somewhere new.

### An extraction job belongs to the workspace it was asked for

Phase 5 allows a job with no initiative, and extraction looked like the obvious case: an artifact can be linked to several initiatives or to none. But a job with no initiative shows in *every* workspace's job list, so extracting one CV from one workspace put a row in front of every other one. The request has an obvious owner — the recruiter was standing in a workspace when they made it — so `Extract` takes the initiative and the job belongs there. The artifact's own links are unaffected; this is about where the progress bar appears.

### Markdown is stored and displayed as untrusted text

Whoever wrote the document chose its contents, and they may have chosen a `<script>` tag, an instruction addressed to a language model, an ANSI escape sequence, or a link to somewhere unpleasant. None of that is a problem as long as the Markdown is never rendered, never executed, and never concatenated into a prompt without the phases that will handle that saying so.

The UI shows the extraction in a `<pre>`. There is no Markdown renderer in this application and this phase is not the phase that adds one. That is not a mitigation applied to a risk; it is the absence of the mechanism the risk needs.

### The fake sidecar is a real program

Every containment behaviour worth testing — hangs, floods, children, non-zero exits — needs a process that misbehaves on demand, and a mock cannot be killed by a Job Object. `internal/fakesidecar` is a small Go `main` package that the tests build once into a temporary directory and then point the extraction service at. It answers `--version`, prints Markdown, sleeps forever, prints megabytes, spawns a child that outlives it, or exits non-zero, according to its first argument.

This is what makes the phase testable on a Mac. It is also what makes the Windows gate necessary rather than optional: the fake proves the *contract*, and only the packaged MarkItDown under a real Job Object proves the *containment*. Memory limits in particular are not enforced at all on non-Windows — `runContained` says so — so the memory-cap test is a gate test and is honestly marked as unrun.

## Risks / Trade-offs

- **The Windows-only half stays unrun.** Job Object memory caps, the packaged sidecar, the golden PDF and DOCX fixtures through the real MarkItDown, and the "another local user cannot read the staging directory" permission proof all need the target laptop. They are written and tagged; they join the Phase 1 evidence gap rather than closing it. Everything that can be proven off Windows is proven off Windows.
- **A retry re-extracts from scratch.** There is no partial extraction to resume, and a two-minute ceiling makes resumption worth less than the code it would cost.
- **Markdown lives in the same row as the bytes.** A 25 MB artifact plus 10 MB of Markdown is a wide row, and reads that do not need it must omit the column — as artifact listings already omit `bytes`. A separate table would be cleaner and is not yet worth a join.
- **Nothing re-extracts when the extractor version changes.** The version is recorded so a later phase can find the stale ones. Deciding what to do about them belongs with the phase that has a reason to care.
