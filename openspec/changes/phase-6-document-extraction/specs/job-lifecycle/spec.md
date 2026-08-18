## MODIFIED Requirements

### Requirement: Each item runs in its own transaction
Each item of a job SHALL be processed in a single transaction that also records that item's completion, so that an item's work and the count claiming it are committed together or not at all. The work an item performs SHALL be computed before that transaction is opened, so that the duration of the work does not determine how long the database's single writer is held.

#### Scenario: Completed items survive a later failure
- **WHEN** a job fails partway through a batch
- **THEN** the work committed by the items that already finished is still present, and the completed count matches it exactly

#### Scenario: An interrupted item leaves nothing behind
- **WHEN** an item's transaction does not commit
- **THEN** neither that item's work nor an increment of the completed count is present

#### Scenario: A slow item does not block other writers
- **WHEN** an item is performing work that takes longer than the database's busy timeout
- **THEN** other writes to the database succeed while that work is in progress, because no transaction is open until the work is done

#### Scenario: Work that fails is never committed
- **WHEN** an item's work fails before it produces a result
- **THEN** no transaction is opened for that item and the job fails with the item's reason
