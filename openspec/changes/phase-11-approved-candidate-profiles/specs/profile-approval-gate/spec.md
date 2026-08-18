## ADDED Requirements

### Requirement: Search and matching are blocked until an initial approval
A candidate SHALL NOT be used for search or matching while their profile is missing, Proposed, or Failed. The block SHALL be reported with the reason, not as an empty result.

#### Scenario: A candidate with no profile is blocked
- **WHEN** readiness is checked for a candidate with no profile
- **THEN** it reports not ready, because no profile exists

#### Scenario: A Proposed profile is blocked
- **WHEN** readiness is checked for a candidate whose only profile is Proposed
- **THEN** it reports not ready, because the profile has not been approved

#### Scenario: A Failed profile is blocked
- **WHEN** readiness is checked for a candidate whose latest profile is Failed
- **THEN** it reports not ready, naming the failure

#### Scenario: An approved profile is ready
- **WHEN** readiness is checked for a candidate with an approved profile derived from the current sources
- **THEN** it reports ready with no warning

#### Scenario: A stale approved profile is ready with a warning
- **WHEN** readiness is checked for a candidate whose approved profile is stale
- **THEN** it reports ready and carries the staleness warning

### Requirement: The readiness rule has exactly one implementation
Every consumer of Candidate Profiles SHALL determine usability through the same readiness check. No consumer SHALL decide usability by inspecting a profile's state directly.

#### Scenario: A new consumer inherits the rule
- **WHEN** a feature needs to know whether a candidate may be matched
- **THEN** it calls the readiness check and receives both the permission and any warning

#### Scenario: The warning and the permission cannot disagree
- **WHEN** readiness reports a candidate usable
- **THEN** any applicable warning is returned by the same call
