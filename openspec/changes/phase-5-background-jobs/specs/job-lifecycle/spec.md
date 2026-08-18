## ADDED Requirements

### Requirement: Background jobs are persisted through one lifecycle
A background job SHALL be persisted with its kind, state, total item count, and completed item count. A job SHALL be in exactly one of Queued, Running, Completed, Failed, or Cancelled, and its state SHALL survive application restart.

#### Scenario: A newly enqueued job is queued with its counts
- **WHEN** a job is enqueued for a known kind
- **THEN** it is persisted in the Queued state with its total item count, a completed count of zero, and no failure reason

#### Scenario: A job with no items completes
- **WHEN** a job is enqueued with a total of zero items
- **THEN** it reaches Completed with a completed count of zero, because an empty batch is a batch that finished

#### Scenario: An unknown job kind is refused at enqueue
- **WHEN** a job is enqueued for a kind that has no registered worker
- **THEN** enqueue fails with an error naming the kind and no job is persisted

#### Scenario: A finished job keeps its counts
- **WHEN** a job that processed every item is read back
- **THEN** its state is Completed and its completed count equals its total count

### Requirement: Illegal state transitions are refused
The application SHALL permit only the transitions Queued → Running, Queued → Cancelled, Running → Completed, Running → Failed, and Running → Cancelled. Every other transition SHALL be refused and SHALL leave the stored state unchanged.

#### Scenario: Every illegal transition is rejected
- **WHEN** each ordered pair of states that is not a permitted transition is applied to a job
- **THEN** every one of them is refused with an error naming both states, and the job's stored state is unchanged

#### Scenario: A terminal state is final
- **WHEN** any transition is applied to a job that is Completed, Failed, or Cancelled
- **THEN** it is refused, because a finished job has no next state

### Requirement: Progress is counted per item
A job SHALL record how many of its items have completed. The completed count SHALL only ever increase, SHALL never exceed the total, and SHALL reflect only items whose work was committed.

#### Scenario: Progress advances as items finish
- **WHEN** a job's items complete one by one
- **THEN** the completed count read from the database after each item equals the number of items whose work was committed

#### Scenario: An item that fails does not count as completed
- **WHEN** an item's work fails
- **THEN** the completed count does not include it, and the job moves to Failed

### Requirement: Each item runs in its own transaction
Each item of a job SHALL be processed in a single transaction that also records that item's completion, so that an item's work and the count claiming it are committed together or not at all.

#### Scenario: Completed items survive a later failure
- **WHEN** a job fails partway through a batch
- **THEN** the work committed by the items that already finished is still present, and the completed count matches it exactly

#### Scenario: An interrupted item leaves nothing behind
- **WHEN** an item's transaction does not commit
- **THEN** neither that item's work nor an increment of the completed count is present

### Requirement: Failure reasons carry no content
A failed job SHALL record a failure reason drawn from a fixed vocabulary of short codes. The application SHALL NOT store free text, recruiter content, candidate content, or error text from a worker in the job record.

#### Scenario: A worker's coded failure is stored
- **WHEN** a worker fails with a declared reason code
- **THEN** the job is Failed and its reason is that code

#### Scenario: An undeclared error becomes a generic code
- **WHEN** a worker fails with an error that carries no declared reason code
- **THEN** the job is Failed with the generic worker-error code, and no part of the error text is stored

#### Scenario: A worker panic fails the job rather than the application
- **WHEN** a worker panics
- **THEN** the job is Failed with the panic reason code, no job is left Running, and the application keeps serving other requests

#### Scenario: An invalid reason code is refused
- **WHEN** a failure reason that is not a short lowercase code is written
- **THEN** the write is refused, because a reason that can hold a sentence can hold a candidate's name
