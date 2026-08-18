## ADDED Requirements

### Requirement: Direct fetching is deny-by-default with an empty allowlist
The application SHALL refuse to fetch any page directly unless its host appears on a developer-maintained allowlist. That allowlist SHALL ship empty, because no source has completed the access review the PRD requires.

#### Scenario: Every host is refused by default
- **WHEN** a direct fetch is attempted for any host
- **THEN** it is refused, naming the allowlist as the reason

#### Scenario: The shipped allowlist is empty
- **WHEN** the allowlist is inspected in a shipped build
- **THEN** it contains no entries

#### Scenario: The refusal is not user-overridable
- **WHEN** the service surface is inspected for a way to bypass the allowlist
- **THEN** no flag, setting, or parameter permits it

### Requirement: Named sources are never automated
SEEK, LinkedIn, authenticated pages, robots-disallowed paths, and anti-bot challenges SHALL never be fetched automatically, and SHALL remain refused even if their hosts were added to the allowlist.

#### Scenario: SEEK is refused
- **WHEN** a direct fetch of a SEEK URL is attempted
- **THEN** it is refused

#### Scenario: LinkedIn is refused
- **WHEN** a direct fetch of a LinkedIn URL is attempted
- **THEN** it is refused

#### Scenario: Allowlisting does not enable a denied source
- **WHEN** a denied host is present on the allowlist
- **THEN** fetching it is still refused

#### Scenario: No browser automation exists
- **WHEN** the application is inspected for browser control
- **THEN** none is present and none can be enabled

### Requirement: Manual paste completes insufficient content without claiming automation
The recruiter SHALL be able to paste or attach listing content manually. Such content SHALL be recorded as recruiter-supplied rather than as automatically retrieved.

#### Scenario: Pasted content becomes a role source
- **WHEN** the recruiter pastes listing text for a role
- **THEN** it is stored as an artifact linked to that role and usable for profiling

#### Scenario: Pasted content does not claim automated provenance
- **WHEN** pasted content is inspected
- **THEN** its source records that a person supplied it, not that it was retrieved from the listing

#### Scenario: Pasted content can complete thin provider text
- **WHEN** the provider returned too little text to profile a role
- **THEN** pasting the listing lets profiling proceed
