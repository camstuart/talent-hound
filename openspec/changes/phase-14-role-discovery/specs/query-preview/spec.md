## ADDED Requirements

### Requirement: Every search is previewed and confirmed
No Exa request SHALL be sent without the recruiter having seen the exact query and confirmed it.

#### Scenario: Sending requires confirmation
- **WHEN** a query is generated
- **THEN** nothing is sent until the recruiter confirms

#### Scenario: The query is editable before sending
- **WHEN** the recruiter edits the previewed query
- **THEN** the edited text is what would be sent

### Requirement: The previewed query is the sent query, byte for byte
The text shown in the preview SHALL be transmitted unchanged. No template, default filter, or transformation SHALL be applied between the preview and the request.

#### Scenario: The request carries exactly what was previewed
- **WHEN** a previewed query is confirmed and sent
- **THEN** the query the provider receives is byte-for-byte identical to the previewed text

#### Scenario: An edited query is sent as edited
- **WHEN** the recruiter edits the query and confirms
- **THEN** the provider receives exactly the edited text

### Requirement: A cancelled preview leaves no trace
Cancelling a preview SHALL send no request, create no search record, and create no disclosure audit event.

#### Scenario: Cancellation sends nothing
- **WHEN** a preview is cancelled
- **THEN** the provider received no request

#### Scenario: Cancellation records nothing
- **WHEN** a preview is cancelled
- **THEN** no search record and no disclosure event exist for it

### Requirement: A sent search is reproducible
A search that was sent SHALL store the exact visible query on its search record, within the owning initiative.

#### Scenario: The visible query is stored
- **WHEN** a search is sent
- **THEN** its record holds the exact query text that was sent

#### Scenario: The stored query belongs to the initiative
- **WHEN** searches from two initiatives are listed
- **THEN** each search record belongs to the initiative it was run in
