## Context

The PRD fixes the shape of this: Queued → Running → Completed, Failed, or Cancelled; cancellation stores completed and total counts rather than becoming its own partial state; a job found Running after a restart becomes Failed with reason **interrupted** and may be retried manually; and completed per-item results survive batch cancellation because each is independently valid. The interesting part is not the state machine — it is where the transaction boundaries sit, because that is what decides whether a cancelled twenty-role batch keeps its nineteen finished roles.

## Goals / Non-Goals

**Goals:**
- One lifecycle every later pipeline can use without inventing its own.
- Cancellation that is deterministic wherever it lands: before start, between items, during an item, after the end.
- Completed work that survives cancellation, and an interrupted item that leaves nothing behind.
- An honest state after a crash, produced exactly once.
- A failure reason that cannot carry recruiter or candidate content.

**Non-Goals:**
- No real pipeline. The only worker registered in this phase is the demo job the tests drive.
- No cross-process queue, no scheduler, no priorities, no concurrency limits, no backoff.
- No automatic retry. Retry is a recruiter action, because a silent retry loop hides the failure the PRD says must be visible.
- No resume-from-item. Retry re-runs the job from the start; workers are responsible for their own idempotence.
- No job history pruning. Phase 19 owns deletion.

## Decisions

**One `jobs` table, no queue table and no item table.**
A job row carries kind, optional initiative, params, state, total and completed counts, reason, and timestamps. Per-item rows would let a retry skip finished items, but nothing in the PoC needs that and an item table is a second thing to keep consistent with the counts.
`ponytail:` counts, not item rows; add an items table when a pipeline actually needs resume-from-item rather than re-run.

**The state machine is a table, and it is the only place transitions are decided.**
`models.JobState` plus an `allowed` map from state to permitted successors. Every write to `state` goes through one guarded update, so an illegal transition is impossible rather than merely untested. The exhaustive test walks all 25 pairs.

**A pending cancellation request lives in memory; only the outcome is durable.**
The first design made `cancel_requested` a column, and it deadlocked on its own guarantee: SQLite has a single writer, so while an item's transaction is open no other connection can write, and a cancellation that needed a write would wait out the very item it was trying to interrupt. The request is therefore a map plus a `context.CancelFunc` — the context wakes an item that is sleeping or waiting on a subprocess, and the runner writes the durable Cancelled state at the item boundary, when no transaction is open. Nothing is lost by not persisting the request: a request that does not outlive the process is moot, because the restart sweep fails the job as `interrupted` anyway.

**A worker holds the write lock for as long as its item runs.**
The per-item transaction is what makes work and count agree, but it is also a write lock on the whole database. A worker should therefore do its slow compute — model call, subprocess, HTTP — *before* it writes through `tx`, so the lock is held for the persist and not for the minute. `ponytail:` a convention, not a mechanism; split `JobWorker` into compute and commit halves if a worker ever gets this wrong in a way that shows.

**Cancellation is honoured at item boundaries, and the in-flight item rolls back.**
Each item runs inside its own transaction along with the increment of `completed_items`, so an item and the count that claims it finished commit together or not at all. A cancellation seen mid-item aborts that transaction: the count never claims work that rolled back. This is the whole reason the transaction boundary is per item rather than per job.

**A job with zero items completes immediately.**
`total_items` of zero is a legitimate batch — nothing matched — and it reaches Completed with 0/0 rather than being refused. Refusing it would make every caller special-case an empty result set.

**The failure reason is a code, not a message.**
`failure_reason` accepts only `^[a-z][a-z0-9_]{0,39}$`, validated before the write. Workers return `jobs.Fail("extraction_failed")`; anything else becomes `worker_error`, and a panic becomes `panic`. Redaction by construction beats redaction by careful string handling, because the string that leaks candidate content is always the one nobody thought about. Detail belongs in the worker's own domain rows, which have their own rules; the job record is a status, not a log.

**The restart sweep runs in `NewJobService` and covers Queued as well as Running.**
The PRD names Running; a Queued job is in exactly the same position, because the queue lives in this process and died with it. Both become Failed with reason `interrupted`. Running the sweep in the constructor means it happens once per process, before any binding can be called, with no extra line in `main.go` for a future service to forget — and the E2E server build gets it for free.

**Retry reuses the job row.**
Failed or Cancelled → Queued, resetting `completed_items` to zero and clearing the reason. A new row per attempt would make "how did this job end" a question about which row you looked at. The counts are the current attempt's; the previous attempt's counts are not history worth a table.

**Workers are registered in a map from kind to function, in Go.**
`Register("demo", fn)`. Kinds are not data the frontend supplies: a job whose kind has no registered worker fails at enqueue rather than at run, so a typo cannot produce a job nothing will ever execute.

**One goroutine per running job, started by `Enqueue`.**
No worker pool. This is a single-user desktop app where the expensive thing is a CPU-bound model call the recruiter watches; a pool would add a queue depth to reason about with nothing to gain. `ponytail:` unbounded goroutines, one per job; add a semaphore when two pipelines can realistically overlap and contend for the same CPU.

## Risks / Trade-offs

- Cancellation latency is one item. A twenty-minute batch of one-minute items cancels within about a minute, not instantly. The context cancel shortens that for anything that waits; a genuinely uninterruptible in-process computation is the ceiling, and the fix would be finer-grained items, not a finer-grained lifecycle.
- Retry re-runs everything, so a batch that fails on item nineteen redoes the first eighteen. Acceptable while retries are rare and recruiter-initiated; resume needs the item table above.
- The reason vocabulary means a worker's genuinely novel failure arrives as `worker_error` until someone adds a code. That is the trade for a record that cannot leak content.
- Jobs run only while the app is open. A batch left running when the recruiter quits comes back Failed with `interrupted` rather than resuming. Making it resumable means a durable per-item boundary the PoC does not need.

## Migration Plan

Migration 5 adds `jobs`; no existing table changes, so every current database migrates forward untouched. Rollback is Phase 2's snapshot restore.

**Cancelled jobs live in their own tab, not in the main list.**
A cancelled job is a thing the recruiter already knows about — they cancelled it. Mixing it into the active list pushes the work they are watching down the screen behind decisions they have already made. The main tab carries Queued, Running, Completed, and Failed; a Cancelled tab holds the rest, with a count on the tab so nothing disappears silently. Failed stays in the main list because a failure is news.

## Open Questions

- Should a cancelled job's counts show as `19/20` or as `19 of 20 before cancelling`? Currently the raw counts, with the state supplying the meaning.
- Should the jobs panel show every job or only the open initiative's? Currently the open initiative's, plus jobs with no initiative in the same list.
