## ADDED Requirements

### Requirement: Structured constraints are compared without a model
Location, work arrangement, work rights, employment type, and compensation SHALL be compared by deterministic code over normalized values. No model SHALL be consulted for these comparisons.

#### Scenario: Each structured type is compared deterministically
- **WHEN** each of the five types is compared
- **THEN** the result depends only on the two normalized values

#### Scenario: The same comparison always gives the same answer
- **WHEN** a structured comparison is repeated
- **THEN** it produces the same result every time, on any machine

#### Scenario: No model call is made
- **WHEN** only structured requirements are assessed
- **THEN** no generation call occurs

### Requirement: Unknown on either side yields unknown
When either side of a structured comparison is absent or unknown, the result SHALL be unknown rather than met or not met.

#### Scenario: An absent role value yields unknown
- **WHEN** the role states no work arrangement
- **THEN** the comparison is unknown

#### Scenario: An absent candidate value yields unknown
- **WHEN** the candidate states no work rights
- **THEN** the comparison is unknown

#### Scenario: An explicit unknown yields unknown
- **WHEN** either side's normalized value is unknown
- **THEN** the comparison is unknown

### Requirement: Compensation compares ranges rather than points
A compensation comparison SHALL treat stated minima and maxima as a range and SHALL report met when the ranges overlap, not met when they cannot, and unknown when either is unstated.

#### Scenario: Overlapping ranges are met
- **WHEN** the role offers 170,000–190,000 and the candidate wants at least 180,000
- **THEN** the comparison is met

#### Scenario: Disjoint ranges are not met
- **WHEN** the role offers up to 150,000 and the candidate wants at least 180,000
- **THEN** the comparison is not met

#### Scenario: A missing amount is unknown
- **WHEN** either side states no amount
- **THEN** the comparison is unknown

#### Scenario: Different currencies are unknown rather than compared
- **WHEN** the two sides state different currencies
- **THEN** the comparison is unknown rather than a conversion
