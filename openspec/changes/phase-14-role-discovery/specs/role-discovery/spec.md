## ADDED Requirements

### Requirement: Discovered roles are cached with provenance
A role discovered through a search SHALL be stored with its source, source ID, canonical URL, retrieval date, and discovered origin. Content supplied by the provider SHALL be stored as an artifact owned by that role.

#### Scenario: A discovered role records where it came from
- **WHEN** a search returns a listing
- **THEN** a role exists carrying the source, source ID, canonical URL, retrieval date, and discovered origin

#### Scenario: Provider content becomes a role-owned artifact
- **WHEN** a search returns listing content
- **THEN** an artifact holds that content and is linked to the role

#### Scenario: A discovered role is distinguishable from an entered one
- **WHEN** a role is created by discovery
- **THEN** its origin is discovered, not recruiter entered

### Requirement: Role identity resolves by a fixed precedence
The application SHALL identify a discovered listing against existing roles by source ID first, then canonical URL, then content fingerprint. The first match SHALL win, and the order SHALL NOT vary.

#### Scenario: Source ID matches first
- **WHEN** a result shares a source ID with an existing role but has a different URL
- **THEN** it resolves to that role

#### Scenario: Canonical URL matches when there is no source ID
- **WHEN** a result has no source ID and shares a canonical URL with an existing role
- **THEN** it resolves to that role

#### Scenario: Content fingerprint matches last
- **WHEN** a result has neither a matching source ID nor a matching URL but identical content to an existing role's current source
- **THEN** it resolves to that role

#### Scenario: Nothing matching creates a new role
- **WHEN** a result matches on none of the three
- **THEN** a new role is created

#### Scenario: The precedence is deterministic when signals disagree
- **WHEN** a result's source ID matches one role and its canonical URL matches another
- **THEN** it resolves to the source ID match, every time

### Requirement: Provider failures are reported, not silently partial
Pagination, duplicates, missing fields, malformed records, rate limits, timeouts, and offline conditions SHALL each be handled explicitly. A partial result SHALL be reported as partial rather than presented as complete.

#### Scenario: Duplicate results collapse to one role
- **WHEN** a response contains the same listing twice
- **THEN** one role exists for it

#### Scenario: A malformed record does not discard the whole response
- **WHEN** a response contains one unusable record among usable ones
- **THEN** the usable ones are stored and the response is reported as partial

#### Scenario: A rate limit is reported as itself
- **WHEN** the provider rate-limits the request
- **THEN** the failure says so rather than reporting no results

#### Scenario: A timeout is distinguishable from an empty result
- **WHEN** the request times out
- **THEN** the failure says so, and no search is recorded as having returned nothing

#### Scenario: Being offline is distinguishable from both
- **WHEN** the provider cannot be reached
- **THEN** the failure says so
