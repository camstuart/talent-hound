## ADDED Requirements

### Requirement: Only Ready Role Profiles enter automatic assessment
A role SHALL be eligible for automatic assessment only when its current profile is Ready. Failed, Stale, and missing profiles SHALL make the role ineligible, and the reason SHALL be reported rather than the role silently being skipped.

#### Scenario: A Ready role is eligible
- **WHEN** eligibility is checked for a role whose current profile is Ready
- **THEN** it reports eligible

#### Scenario: A Failed role is ineligible with its reason
- **WHEN** eligibility is checked for a role whose profile Failed
- **THEN** it reports ineligible, naming the failure

#### Scenario: A Stale role is ineligible
- **WHEN** eligibility is checked for a role whose source content changed after profiling
- **THEN** it reports ineligible, saying the listing changed since it was profiled

#### Scenario: A role with no profile is ineligible
- **WHEN** eligibility is checked for a role that has never been profiled
- **THEN** it reports ineligible, saying so

### Requirement: Eligibility is decided by one call
Every consumer deciding whether a role may be assessed SHALL use the same eligibility call. No consumer SHALL decide by inspecting a profile's state directly.

#### Scenario: A consumer receives permission and reason together
- **WHEN** a feature needs to know whether a role may be assessed
- **THEN** one call returns both whether it may and, when it may not, why

#### Scenario: Both sides of a match answer the same shape
- **WHEN** a match between a candidate and a role is being considered
- **THEN** the candidate's readiness and the role's eligibility are obtained through calls of the same shape
