## ADDED Requirements

### Requirement: Diagnostics are local, redacted, and built from facts
A diagnostic report SHALL contain only the application version, schema version, resolved folder paths, encryption status, dependency availability and versions, configured model roles, record counts by kind, and recent job outcomes as codes.

#### Scenario: A report is produced with no records present
- **WHEN** a diagnostic report is requested on an empty database
- **THEN** it is produced, and reports zero counts rather than failing

#### Scenario: A report names the versions and the paths
- **WHEN** a diagnostic report is requested
- **THEN** it states the application version, the schema version, and the resolved data folder

### Requirement: A diagnostic report never contains content
A diagnostic report SHALL NOT contain document contents, candidate details, search queries, request payloads, draft contents, filenames, or credentials.

#### Scenario: Candidate details are absent
- **WHEN** a report is produced from a database holding candidates, artifacts, drafts, searches, and cloud payloads
- **THEN** no candidate name, email, phone, address, filename, query, payload, or draft text appears in it

#### Scenario: A stored secret is absent
- **WHEN** a provider credential is present
- **THEN** no part of it appears in the report, and the report states only whether a credential is stored

#### Scenario: Control characters are stripped
- **WHEN** a fixture contains control characters and terminal escape sequences
- **THEN** the report is safe to display and contains none of them

### Requirement: The logs folder can be opened
The application SHALL offer an action that opens the logs folder, and SHALL report the resolved path when it cannot be opened.

#### Scenario: The logs folder path is reported
- **WHEN** the logs folder action is used
- **THEN** the resolved path is available to the recruiter whether or not the system file manager opens

### Requirement: There is no telemetry
The application SHALL contain no telemetry endpoint, SDK, background reporter, or opt-in control.

#### Scenario: No telemetry request during offline work
- **WHEN** a full local workflow runs with network observation
- **THEN** no telemetry request is made

#### Scenario: No telemetry control exists
- **WHEN** the service surface and settings are inspected
- **THEN** no telemetry setting, endpoint, or reporter exists to enable

### Requirement: Delete-all names the exact folder
A delete-all action SHALL display the exact resolved data-folder path and SHALL require confirmation of that path before removing anything.

#### Scenario: A mismatched confirmation deletes nothing
- **WHEN** the confirmation does not match the resolved path
- **THEN** nothing is deleted

#### Scenario: The matching confirmation removes the folder
- **WHEN** the confirmation matches the resolved path
- **THEN** the folder's contents are removed, and the removal is reported
