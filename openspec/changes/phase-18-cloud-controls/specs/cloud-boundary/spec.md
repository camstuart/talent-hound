## ADDED Requirements

### Requirement: Some tasks can never use the cloud
Raw candidate artifacts, Candidate Profile extraction, and candidate embeddings SHALL never be sent to a cloud endpoint. No configuration, setting, or flag SHALL permit it.

#### Scenario: Candidate profile extraction is refused
- **WHEN** a cloud override is requested for Candidate Profile extraction
- **THEN** it is refused, and the refusal cites the boundary rather than a missing approval

#### Scenario: Embeddings are refused
- **WHEN** a cloud override is requested for embeddings
- **THEN** it is refused

#### Scenario: Raw candidate artifacts are refused
- **WHEN** a payload would contain a raw candidate artifact
- **THEN** it is refused

#### Scenario: No parameter permits a denied task
- **WHEN** the service surface is inspected for a way to permit a denied task
- **THEN** none exists

#### Scenario: A fake cloud endpoint receives nothing denied
- **WHEN** every task is attempted under every configuration the tests can produce
- **THEN** the cloud endpoint receives no candidate artifact, no candidate extraction, and no embedding request

### Requirement: The local configuration is always required
The cloud endpoint SHALL be a per-task override and SHALL NOT replace the required local model configuration.

#### Scenario: Local roles remain required
- **WHEN** a cloud endpoint is configured
- **THEN** the local embed, classify, and generate assignments are still required

#### Scenario: Removing the cloud endpoint leaves the application working
- **WHEN** the cloud endpoint is removed
- **THEN** every task runs locally as before

### Requirement: Eligible tasks are enumerated
The tasks a cloud override may cover SHALL be exactly: role extraction, assessment, drafting, and chat.

#### Scenario: Each eligible task can be approved
- **WHEN** an approval is requested for each of the four
- **THEN** each is possible

#### Scenario: An unlisted task cannot be approved
- **WHEN** an approval is requested for a task outside the four
- **THEN** it is refused
