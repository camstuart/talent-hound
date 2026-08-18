## 1. Schema

- [x] 1.1 Migration 5: `jobs` (kind, optional initiative, params JSON, state, total/completed items, failure reason, started/finished timestamps)
- [x] 1.2 CHECK constraints on state and on the failure-reason code shape; index on (state) and on (initiative)
- [x] 1.3 No item table — counts only, per the design's resume-from-item trade-off

## 2. Model

- [x] 2.1 `internal/models/job.go`: `JobState` enum, the transition table, and `CanTransition`
- [x] 2.2 Failure reason codes as constants plus a validator; `interrupted`, `worker_error`, `panic` at minimum
- [x] 2.3 No state written outside the guarded transition helper

## 3. Service and runner

- [x] 3.1 `jobservice.go`: `Enqueue`, `Get`, `List`, `ListForInitiative`, `Cancel`, `Retry`
- [x] 3.2 Worker registry keyed by kind; unknown kind refused at enqueue
- [x] 3.3 One goroutine per job; each item in its own transaction with the completed-count increment
- [x] 3.4 Cancellation: in-memory request plus a context cancel, honoured at item boundaries — no write while an item holds the single SQLite writer
- [x] 3.5 Panic recovery fails the job rather than the process
- [x] 3.6 Restart sweep in `NewJobService`: Queued and Running from a previous run become Failed/`interrupted`
- [x] 3.7 Demo worker (`demo` kind, params for item delay and a failing item) — the only worker this phase registers; `register` is unexported so the worker function never crosses the JSON boundary
- [x] 3.8 Register the service in `main.go` and regenerate bindings

## 4. Frontend

- [x] 4.1 Jobs panel for the open initiative: kind, state, completed-of-total, reason
- [x] 4.2 Cancel for unfinished jobs, retry for stopped ones, nothing for the wrong state
- [x] 4.3 Cancelled jobs in their own tab with a count; queued, running, completed, and failed stay in the main list
- [x] 4.4 Progress polling while any job is unfinished, stopped when none is
- [x] 4.5 Backend messages surfaced verbatim, as elsewhere

## 5. Tests

- [x] 5.1 Exhaustive transition table: all 25 ordered pairs, legal ones accepted and illegal ones refused with the state unchanged
- [x] 5.2 Cancellation before start, between items, during an item, and after completion
- [x] 5.3 Completed per-item work survives cancellation; the in-flight item rolls back
- [x] 5.4 Restart converts Queued and Running to Failed/`interrupted` exactly once and permits retry
- [x] 5.5 Repeated cancel is idempotent; repeated retry leaves exactly one queued run
- [x] 5.6 Worker panic, worker error, and an undeclared error code leave no Running job and no partial item data
- [x] 5.7 Zero-item job completes; unknown kind is refused at enqueue
- [x] 5.8 Failure reason rejects free text and stores only codes
- [x] 5.9 Vitest over the jobs panel: progress, counts, cancel, failure reason, retry, and cancelled jobs moving to their own tab
- [x] 5.10 Playwright starts, cancels, and retries the demo job through the real backend
- [x] 5.11 Fixtures are synthetic only — no real candidate information anywhere

## 6. Exit gate

- [x] 6.1 Every later slow operation can use this lifecycle without inventing another queue or partial state
- [x] 6.2 `just check` passes: `qa` plus all three test layers (`just vuln` still fails on the pre-existing Go stdlib advisories that need a go1.26.6 toolchain — unchanged by this phase)
