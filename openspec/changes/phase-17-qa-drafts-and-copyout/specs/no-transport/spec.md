## ADDED Requirements

### Requirement: The application cannot send outreach
The application SHALL contain no facility for sending email, SMS, LinkedIn messages, or any other outreach. No setting, flag, or configuration SHALL enable one.

#### Scenario: No sender exists in the source
- **WHEN** the repository is scanned for mail and messaging senders
- **THEN** none is present

#### Scenario: No sender is reachable at runtime
- **WHEN** a full generate, edit, copy, and discard cycle runs
- **THEN** no connection is opened to a mail or messaging service

#### Scenario: No configuration enables sending
- **WHEN** the settings surface is inspected
- **THEN** nothing offers to send, and no credential is collected for a sender

### Requirement: The absence is asserted, not assumed
The application SHALL carry an automated check proving no transport exists, so that adding one fails visibly.

#### Scenario: Adding a sender fails the check
- **WHEN** a mail or messaging sender is introduced
- **THEN** the transport check fails

#### Scenario: The check runs with the ordinary test suite
- **WHEN** the test suite runs
- **THEN** the transport check runs with it
