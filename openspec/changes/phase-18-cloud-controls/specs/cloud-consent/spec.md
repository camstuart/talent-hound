## ADDED Requirements

### Requirement: Consent is bound to initiative, endpoint revision, and task
An approval SHALL authorize exactly one initiative, one endpoint revision, and one task type. It SHALL NOT authorize any other combination.

#### Scenario: Another initiative is not authorized
- **WHEN** a task is approved in one initiative and attempted in another
- **THEN** it is refused

#### Scenario: Another task is not authorized
- **WHEN** drafting is approved and assessment is attempted
- **THEN** assessment is refused

#### Scenario: Another endpoint revision is not authorized
- **WHEN** the endpoint changes and a previously approved task is attempted
- **THEN** it is refused

#### Scenario: An approval matches all three or nothing
- **WHEN** an approval is looked up
- **THEN** it matches only an exact initiative, endpoint revision, and task

### Requirement: Changing the endpoint resets approvals
Changing the cloud endpoint SHALL produce a new revision, and no prior approval SHALL apply to it.

#### Scenario: An endpoint change invalidates approvals
- **WHEN** the endpoint URL changes after tasks were approved
- **THEN** every task requires approval again

#### Scenario: The reset takes effect before the next request
- **WHEN** a request is attempted immediately after an endpoint change
- **THEN** it is refused rather than sent under the old approval

### Requirement: Approvals can be revoked
The recruiter SHALL be able to revoke any approval, and the revocation SHALL take effect before the next request.

#### Scenario: A revoked task is refused
- **WHEN** an approval is revoked and the task is attempted
- **THEN** it is refused

#### Scenario: Revocation does not affect other approvals
- **WHEN** one task's approval is revoked
- **THEN** other approved tasks remain approved

#### Scenario: Nothing is sent between revocation and refusal
- **WHEN** a task is attempted after revocation
- **THEN** the cloud endpoint receives no request

### Requirement: Everything starts denied
Every task SHALL be denied until explicitly approved.

#### Scenario: A new initiative approves nothing
- **WHEN** a new initiative is created
- **THEN** no cloud task is approved for it

#### Scenario: A newly configured endpoint approves nothing
- **WHEN** a cloud endpoint is configured for the first time
- **THEN** no task is approved
