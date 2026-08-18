## Why

Everything slow in this product is still ahead of us: extraction, chunking, indexing, profile decomposition, role search, and twenty-role assessment batches. Each of those will need progress, cancellation, retry, and an honest answer after a crash. Building the lifecycle once, now, before any pipeline exists, is what stops five pipelines from inventing five queues and five kinds of partial state. Phase 4 gave us the evidence those pipelines will read; this phase gives us the way they run.

## What Changes

- Persist background jobs through Queued → Running → Completed, Failed, or Cancelled, with every illegal transition refused.
- Record total and completed item counts, kept accurate through cancellation rather than replaced by a partial state.
- Record a failure reason drawn from a fixed vocabulary of codes, so no content can reach the job record.
- Add cancellation, honoured at item boundaries, and manual retry of a Failed or Cancelled job.
- Mark jobs found unfinished after a restart as Failed with reason `interrupted`, exactly once, and allow retry.
- Run each item in its own transaction, so completed items survive cancellation and the interrupted item rolls back.

## Capabilities

### New Capabilities
- `job-lifecycle`: the states a job moves through, the counts it carries, the redacted failure reason, and the per-item transaction boundary.
- `job-control`: cancelling, retrying, and recovering jobs after a restart, idempotently.

### Modified Capabilities
<!-- none: this phase adds a migration to the Phase 2 list and changes no behaviour an existing spec describes -->

## Impact

- `internal/db/migrations.go`: migration 5 — `jobs`.
- New `internal/models/job.go` (state enum, transition table, reason codes); new `jobservice.go` registered in `main.go`.
- `frontend/bindings/` regenerated; the initiative workspace gains a jobs panel with state, progress, cancel, and retry.
- New Go tests over the transition table, cancellation timing, per-item durability, restart recovery, panic and error handling, and idempotence; new Vitest and Playwright coverage driving a controllable demo job.
- No real pipeline is wired to this: the only registered worker is the demo job the tests drive. Phase 6 onward supplies the real ones.
- Jobs are in-process and single-threaded per job. No cross-process queue, no scheduler, no priorities.
