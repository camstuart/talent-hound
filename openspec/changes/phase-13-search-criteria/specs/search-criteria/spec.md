## ADDED Requirements

### Requirement: Search Criteria belong to an initiative and are separate from candidate facts
Search Criteria SHALL be stored against an initiative, not a candidate, and SHALL be editable and versioned independently of any Candidate Profile. Editing a criterion SHALL NOT modify any profile, and editing a profile SHALL NOT modify any criterion.

#### Scenario: Criteria and profiles are edited separately
- **WHEN** a criterion's wording is changed
- **THEN** no Candidate Profile version is created and no aspect is modified

#### Scenario: A profile edit leaves criteria alone
- **WHEN** a Candidate Profile aspect is edited
- **THEN** the initiative's criteria and their version are unchanged

#### Scenario: Criteria are versioned separately
- **WHEN** the criteria set changes
- **THEN** the criteria version changes and no profile version does

### Requirement: A criterion carries a recruiter-selected priority
Every Search Criterion SHALL be must-have or nice-to-have, selected by the recruiter. There SHALL be no unspecified priority for criteria, and no priority SHALL be inferred.

#### Scenario: A criterion is saved with its priority
- **WHEN** the recruiter adds a criterion as nice-to-have
- **THEN** it is stored as nice-to-have

#### Scenario: An unsupported priority is refused
- **WHEN** a criterion is offered with any priority other than must-have or nice-to-have
- **THEN** it is refused

### Requirement: Ordering is presentation, not weighting
Search Criteria SHALL have an order the recruiter controls. Reordering SHALL NOT change the criteria version, and SHALL NOT affect how criteria are used in discovery or matching.

#### Scenario: Reordering does not change the version
- **WHEN** criteria are reordered
- **THEN** the criteria version is unchanged

#### Scenario: Changing content does change the version
- **WHEN** a criterion's wording or priority changes, or one is added or removed
- **THEN** the criteria version changes

#### Scenario: Order is preserved for display
- **WHEN** criteria are listed after reordering
- **THEN** they appear in the order the recruiter set

### Requirement: The criteria version identifies the intent an assessment was made under
The application SHALL expose the current criteria version for an initiative so that derived results can record which intent produced them.

#### Scenario: The current version is reportable
- **WHEN** the criteria version for an initiative is requested
- **THEN** a value is returned that changes exactly when the criteria content changes

#### Scenario: An initiative with no criteria has a version
- **WHEN** an initiative has no criteria yet
- **THEN** a version is still reported, so a result made against no criteria is distinguishable from one made against some
