## ADDED Requirements

### Requirement: Criteria are proposed but never applied automatically
The application MAY propose Search Criteria from a candidate's approved profile. A proposal SHALL write nothing, and a criterion SHALL be created only by an explicit recruiter action naming which proposals to apply.

#### Scenario: Proposing writes nothing
- **WHEN** criteria are proposed for an initiative
- **THEN** no criterion is created and the criteria version is unchanged

#### Scenario: Applying requires naming the proposals
- **WHEN** the recruiter applies two of five proposals
- **THEN** exactly those two become criteria and the other three do not

#### Scenario: Proposals come only from an approved profile
- **WHEN** criteria are proposed for a candidate whose profile is not approved
- **THEN** no proposal is produced, and the reason is reported

### Requirement: No model output can create or change a criterion
No classifier output, generated text, or conversational answer SHALL create, modify, reprioritize, or remove a Search Criterion. Every write to criteria SHALL originate in an explicit recruiter action.

#### Scenario: A classifier response cannot become a criterion
- **WHEN** a model returns text describing a requirement
- **THEN** nothing is written to the criteria until the recruiter applies it

#### Scenario: The criteria write path takes no model output
- **WHEN** the service surface for criteria is inspected
- **THEN** no method accepts model-generated content as the thing to store

### Requirement: History never becomes a preference
The application SHALL NOT derive a Search Criterion from a candidate's employment history, education history, location history, or compensation history. Aspect types carrying such history SHALL be excluded from proposals.

#### Scenario: A prior employer does not become a preference
- **WHEN** a candidate's approved profile records having worked at a named company
- **THEN** no proposal suggests a criterion preferring that company or ones like it

#### Scenario: A school does not become a preference
- **WHEN** a candidate's approved profile records a named university
- **THEN** no proposal suggests a criterion preferring that school or its graduates

#### Scenario: A past location does not become a location preference
- **WHEN** a candidate's approved profile records where they have worked
- **THEN** no proposal suggests a location criterion

#### Scenario: Past compensation does not become a compensation criterion
- **WHEN** a candidate's approved profile records past or expected pay
- **THEN** no proposal suggests a compensation criterion

#### Scenario: A recruiter may still add any of these by hand
- **WHEN** the recruiter types a location criterion themselves
- **THEN** it is accepted, because a person decided it
