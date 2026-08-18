## ADDED Requirements

### Requirement: Jobs can be cancelled
The recruiter SHALL be able to cancel a Queued or Running job. Cancellation SHALL be honoured at the next item boundary, SHALL record the completed and total counts reached, and SHALL NOT introduce a separate partial state.

#### Scenario: Cancelling before the job starts
- **WHEN** a Queued job is cancelled before any item begins
- **THEN** it becomes Cancelled with a completed count of zero and no item is ever processed

#### Scenario: Cancelling between items
- **WHEN** a running job is cancelled after some items have finished
- **THEN** it becomes Cancelled, its completed count is the number of items that committed, and no further item is started

#### Scenario: Cancelling during an item
- **WHEN** cancellation is requested while an item is in flight
- **THEN** that item's transaction rolls back, the completed count does not include it, and the job becomes Cancelled

#### Scenario: Cancelling after the job has finished
- **WHEN** a Completed or Failed job is cancelled
- **THEN** the request is refused with an error naming the job's state and the stored state is unchanged

#### Scenario: Cancelling twice is not an error
- **WHEN** a job that is already Cancelled, or already has a cancellation pending, is cancelled again
- **THEN** the request succeeds without changing anything

### Requirement: Failed and cancelled jobs can be retried
The recruiter SHALL be able to retry a Failed or Cancelled job. Retry SHALL return the job to Queued, reset its completed count, and clear its failure reason.

#### Scenario: Retrying a failed job runs it again
- **WHEN** a Failed job is retried
- **THEN** it returns to Queued with a completed count of zero and no failure reason, and it runs again

#### Scenario: Retrying a cancelled job clears the cancellation
- **WHEN** a Cancelled job is retried
- **THEN** the pending cancellation is cleared, so the retried run is not cancelled by the earlier request

#### Scenario: Retrying an unfinished job is refused
- **WHEN** a Queued or Running job is retried
- **THEN** the request is refused with an error naming the job's state, because the job has not stopped

#### Scenario: Retrying a completed job is refused
- **WHEN** a Completed job is retried
- **THEN** the request is refused, because there is nothing to retry

#### Scenario: Repeated retry requests are idempotent
- **WHEN** a retry is requested twice for the same failed job
- **THEN** the first returns it to Queued and the second is refused as a request against a job that has not stopped, leaving exactly one queued run

### Requirement: Jobs interrupted by a restart are recovered exactly once
On start-up the application SHALL mark every job left unfinished by a previous run as Failed with the reason `interrupted`, before any job can be started, and SHALL leave already-finished jobs untouched.

#### Scenario: A job left running becomes failed and interrupted
- **WHEN** the application starts and finds a job in the Running state from a previous run
- **THEN** that job is Failed with the reason `interrupted`, keeping the completed count it had reached

#### Scenario: A job left queued is interrupted too
- **WHEN** the application starts and finds a Queued job from a previous run
- **THEN** that job is Failed with the reason `interrupted`, because the queue did not survive the process that held it

#### Scenario: Recovery happens once
- **WHEN** the application starts twice in succession
- **THEN** jobs interrupted by the first restart are not changed again by the second, and jobs that finished normally are never touched

#### Scenario: An interrupted job can be retried
- **WHEN** a job that was failed as `interrupted` is retried
- **THEN** it returns to Queued and runs again

### Requirement: Job state is visible to the recruiter
The application SHALL show, for each job, its kind, state, completed and total item counts, and failure reason when it has one, and SHALL offer cancel for unfinished jobs and retry for stopped ones.

#### Scenario: Progress is visible while a job runs
- **WHEN** a running job's items complete
- **THEN** the displayed state and completed-of-total counts follow it

#### Scenario: A failed job shows its reason and offers retry
- **WHEN** a job has failed
- **THEN** its reason is displayed and a retry control is offered

#### Scenario: A finished job offers no cancel
- **WHEN** a job is Completed, Failed, or Cancelled
- **THEN** no cancel control is offered for it

### Requirement: Cancelled jobs are listed separately
Cancelled jobs SHALL be listed in their own tab rather than among the jobs the recruiter is currently working with, and the tab SHALL show how many there are so that none disappears silently. Queued, Running, Completed, and Failed jobs SHALL remain in the main list.

#### Scenario: Cancelling moves a job out of the main list
- **WHEN** a job the recruiter is watching is cancelled
- **THEN** it leaves the main job list and appears in the cancelled tab, which reports the new count

#### Scenario: The cancelled tab keeps the job's outcome
- **WHEN** a cancelled job is viewed in the cancelled tab
- **THEN** its kind, cancelled state, and completed-of-total counts are shown, and retry is offered

#### Scenario: Failures stay in the main list
- **WHEN** a job fails
- **THEN** it remains in the main list, because a failure is news the recruiter has not already been told
