## ADDED Requirements

### Requirement: Matches are ordered by the stated sequence
Assessed matches SHALL be ordered by: first, having no unmet must-haves in either direction; then fewer total unmet must-haves across both directions; then fewer total unknown must-haves; then more total met nice-to-haves; then higher retrieval position from the shortlist; and finally by role identifier.

#### Scenario: A match with no unmet must-haves leads
- **WHEN** one match has no unmet must-haves and another has one
- **THEN** the first comes before the second

#### Scenario: Fewer unmet must-haves wins
- **WHEN** two matches both have unmet must-haves and one has fewer
- **THEN** the one with fewer comes first

#### Scenario: Fewer unknown must-haves breaks the next tie
- **WHEN** two matches have equal unmet must-have counts and differ in unknown must-haves
- **THEN** the one with fewer unknowns comes first

#### Scenario: More met nice-to-haves breaks the next tie
- **WHEN** the earlier steps are equal and one match meets more nice-to-haves
- **THEN** that one comes first

#### Scenario: Retrieval position breaks the next tie
- **WHEN** all earlier steps are equal and one match came from a higher shortlist position
- **THEN** that one comes first

#### Scenario: Role identifier makes the order total
- **WHEN** every earlier step is equal
- **THEN** the lower role identifier comes first, on every run

#### Scenario: Repeated ordering is identical
- **WHEN** the same matches are ordered twice
- **THEN** the two orders are identical

### Requirement: Unmet must-haves sort down but never hide
A match with unmet must-haves SHALL remain in the result list.

#### Scenario: A failing match is still listed
- **WHEN** a match fails several must-haves
- **THEN** it appears at the bottom of the order rather than being removed

#### Scenario: All matches failing does not empty the list
- **WHEN** every match has unmet must-haves
- **THEN** all of them are listed

### Requirement: Unspecified requirements are assessed but do not rank
A requirement whose priority is unspecified SHALL be assessed and displayed, and SHALL count as neither a must-have nor a nice-to-have in the ordering, unless the recruiter changes its priority.

#### Scenario: An unspecified requirement is assessed
- **WHEN** a role requirement has unspecified priority
- **THEN** it produces a per-aspect result that is shown

#### Scenario: An unspecified requirement does not change the order
- **WHEN** two matches differ only in the results of their unspecified requirements
- **THEN** their order is unchanged

#### Scenario: Changing its priority changes the order
- **WHEN** the recruiter makes an unspecified requirement a must-have
- **THEN** it counts in the ordering from then on
