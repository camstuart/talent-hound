## ADDED Requirements

### Requirement: Model availability is reported as distinct states
Checking a role SHALL report exactly one state: ready, endpoint unavailable, model missing, pull declined, pull failed, timed out, malformed response, or out of memory. Distinct causes SHALL NOT be collapsed into one failure, because the recruiter's next action differs for each.

#### Scenario: A ready model reports ready
- **WHEN** the endpoint is reachable and the assigned model is present
- **THEN** the role reports ready

#### Scenario: The endpoint being down is distinguishable
- **WHEN** the local endpoint cannot be reached
- **THEN** the role reports that the endpoint is unavailable, not that the model is missing

#### Scenario: A missing model is distinguishable and names itself
- **WHEN** the endpoint is reachable but the assigned model is not installed
- **THEN** the role reports the model as missing and names the model that would have to be pulled

#### Scenario: A timeout is distinguishable
- **WHEN** the endpoint accepts a connection but does not answer within the check's time limit
- **THEN** the role reports a timeout rather than an unavailable endpoint

#### Scenario: A malformed answer is distinguishable
- **WHEN** the endpoint answers with something that is not the expected shape
- **THEN** the role reports a malformed response

#### Scenario: A memory failure is distinguishable
- **WHEN** the endpoint reports that the model cannot be loaded for lack of memory
- **THEN** the role reports an out-of-memory state rather than a generic failure

#### Scenario: An unassigned role is not a failure state
- **WHEN** a role has no assignment
- **THEN** it reports as unassigned rather than as a model that is missing

### Requirement: A missing model can be pulled, and declining is remembered
A missing model SHALL be pullable through the background job lifecycle. Declining the pull SHALL be reported as its own state for the life of the session, and SHALL NOT prevent pulling later.

#### Scenario: A pull runs as a background job
- **WHEN** a pull is requested for a missing model
- **THEN** a job is enqueued for it and the request returns without waiting

#### Scenario: A failed pull is distinguishable from a missing model
- **WHEN** a pull is attempted and fails
- **THEN** the role reports that the pull failed, with a short reason code, rather than reporting the model as merely missing

#### Scenario: Declining is remembered until the application restarts
- **WHEN** a pull is declined for a role
- **THEN** that role reports the pull as declined, and reports the model as missing again after a restart

#### Scenario: A declined pull can still be run
- **WHEN** a pull is requested for a role whose pull was previously declined
- **THEN** the pull runs

### Requirement: Each role sends the payload its endpoint contract requires
Calls made on behalf of a role SHALL send the model assigned to that role and the parameters recorded with that assignment.

#### Scenario: The embed role calls the embeddings endpoint
- **WHEN** a check or call is made for the `embed` role
- **THEN** the request goes to the embeddings endpoint carrying the assigned embedding model

#### Scenario: The generate role calls the chat endpoint
- **WHEN** a check or call is made for the `generate` role
- **THEN** the request goes to the chat endpoint carrying the assigned model, with streaming off

#### Scenario: The classify role requests constrained output
- **WHEN** a call is made for the `classify` role with an output schema
- **THEN** the request carries that schema as the response format

#### Scenario: A check sends no candidate content
- **WHEN** a role's availability is checked
- **THEN** the request contains no artifact, chunk, or record content
