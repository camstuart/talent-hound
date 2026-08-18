## ADDED Requirements

### Requirement: One canonical hash covers every decision-relevant input
Each assessment SHALL store one input hash computed over: the approved Candidate Profile version and lifecycle state; the Role Profile version and lifecycle state; the Search Criteria version; the content hashes of the evidence used; the structured-comparison and ranking-rule versions; the generation endpoint configuration revision; the model digest or immutable revision; the prompt-template and output-schema versions; the generation parameters; and the role's staleness state.

#### Scenario: Every listed input changes the hash
- **WHEN** each listed input is changed one at a time
- **THEN** the hash changes for every one of them

#### Scenario: Unchanged inputs produce the same hash
- **WHEN** nothing changes
- **THEN** the hash is identical

#### Scenario: The hash is recorded with the assessment
- **WHEN** an assessment is stored
- **THEN** its input hash is stored with it

### Requirement: Serialization is canonical
The hash SHALL be computed over a serialization whose field order and encoding are fixed, so that map iteration order, process restarts, and machine differences cannot change it.

#### Scenario: Repeated hashing within a process agrees
- **WHEN** the same inputs are hashed many times in one process
- **THEN** every result is identical

#### Scenario: Hashing across process restarts agrees
- **WHEN** the same inputs are hashed in a separate process
- **THEN** the result is identical to the in-process one

#### Scenario: Map ordering cannot affect it
- **WHEN** inputs containing maps are hashed repeatedly
- **THEN** the result does not vary with iteration order

### Requirement: The hash is the only caching rule
A stored assessment SHALL be reused if and only if its recomputed input hash matches. No age, timestamp, or heuristic SHALL cause reuse or invalidation.

#### Scenario: A matching hash reuses the stored result
- **WHEN** assessment is requested and the recomputed hash matches a stored one
- **THEN** the stored result is reused and no model call is made

#### Scenario: A differing hash recomputes
- **WHEN** any input changes
- **THEN** the stored result is not reused

#### Scenario: Presentation-only changes do not invalidate
- **WHEN** criteria are reordered without their content changing
- **THEN** the hash is unchanged and stored results are reused

#### Scenario: Reassessment recomputes only what changed
- **WHEN** a batch is reassessed after one role's profile changed
- **THEN** that role is recomputed and the others are reused
