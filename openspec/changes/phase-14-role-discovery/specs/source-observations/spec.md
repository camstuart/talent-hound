## ADDED Requirements

### Requirement: Unchanged content updates the retrieval time only
When a rediscovered role's content fingerprint is unchanged, the application SHALL update its retrieval time and SHALL NOT create an artifact.

#### Scenario: The same content creates no artifact
- **WHEN** a role is rediscovered with identical content
- **THEN** its retrieval date moves and the artifact count is unchanged

#### Scenario: Derived data is not invalidated
- **WHEN** a role is rediscovered with identical content
- **THEN** its Role Profile does not become stale, because nothing it was derived from changed

### Requirement: Changed content creates a new current source
When a rediscovered role's content differs, the application SHALL create a new immutable current source artifact, make the previous source historical, and mark derived data stale.

#### Scenario: Changed content creates an artifact
- **WHEN** a role is rediscovered with different content
- **THEN** a new artifact holds the new content and is the role's current source

#### Scenario: The previous source becomes historical
- **WHEN** a new current source is created
- **THEN** the previous one is marked historical rather than deleted

#### Scenario: Historical sources leave current retrieval
- **WHEN** a historical source exists
- **THEN** it is excluded from current retrieval and matching

#### Scenario: Historical sources remain visible
- **WHEN** a role's sources are listed
- **THEN** the historical ones appear, so an earlier citation can still be resolved

#### Scenario: Derived data goes stale
- **WHEN** a role's current source changes
- **THEN** its Role Profile is reported stale

### Requirement: Roles go stale by age or closing date
A role SHALL be reported Stale when its retrieval is older than thirty days, or when its closing date has passed. Rediscovery SHALL return a Stale role to Active.

#### Scenario: Thirty days makes a role stale
- **WHEN** a role's retrieval date is more than thirty days before the current time
- **THEN** it is Stale

#### Scenario: The boundary is exact
- **WHEN** a role's retrieval date is exactly thirty days old
- **THEN** it is not yet Stale, and one moment later it is

#### Scenario: A passed closing date makes a role stale
- **WHEN** a role's closing date is in the past
- **THEN** it is Stale regardless of when it was retrieved

#### Scenario: Rediscovery reactivates
- **WHEN** a Stale role is found again
- **THEN** it becomes Active

#### Scenario: Staleness uses a supplied clock
- **WHEN** staleness is evaluated in a test
- **THEN** the current time is supplied rather than read from the machine
