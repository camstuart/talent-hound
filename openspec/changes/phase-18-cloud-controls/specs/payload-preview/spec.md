## ADDED Requirements

### Requirement: The first use of a task shows the actual payload
Before the first cloud request for a task, the recruiter SHALL be shown the payload that would be sent. The previewed payload SHALL be transmitted unchanged.

#### Scenario: The first send is previewed
- **WHEN** a task is approved and used for the first time
- **THEN** the payload was shown before anything was sent

#### Scenario: The preview equals the request
- **WHEN** a previewed payload is sent
- **THEN** the endpoint receives it byte for byte

#### Scenario: Cancelling sends nothing
- **WHEN** a preview is cancelled
- **THEN** no request is sent and no audit event is created

### Requirement: Payloads remain previewable
After the first use, the recruiter SHALL still be able to preview a task's payload before sending.

#### Scenario: A later payload can be previewed
- **WHEN** a previously used task is about to run again
- **THEN** its payload can be shown first

#### Scenario: Previewing does not send
- **WHEN** a payload is previewed
- **THEN** nothing is transmitted

### Requirement: Cloud chat previews every send
Every cloud chat request SHALL require an explicit payload selection and a preview, not only the first.

#### Scenario: Each chat send is previewed
- **WHEN** a second chat message is sent to the cloud
- **THEN** its payload was previewed first

#### Scenario: A chat payload is explicitly selected
- **WHEN** a cloud chat request is prepared
- **THEN** what it contains is chosen rather than assembled implicitly

### Requirement: Failures send nothing unexpected
A cancelled preview, a denied task, an offline endpoint, a timeout, a provider error, or a removed credential SHALL each result in nothing being sent beyond what was approved and previewed.

#### Scenario: A denied task transmits nothing
- **WHEN** an unapproved task is attempted
- **THEN** the endpoint receives no request

#### Scenario: A removed credential disables the provider
- **WHEN** the cloud credential is removed
- **THEN** requests are refused and no local information is deleted

#### Scenario: An offline endpoint is reported as itself
- **WHEN** the endpoint cannot be reached
- **THEN** the failure says so rather than reporting an empty result
