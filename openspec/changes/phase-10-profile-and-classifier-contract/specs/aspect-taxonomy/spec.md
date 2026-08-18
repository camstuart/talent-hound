## ADDED Requirements

### Requirement: The aspect type list is closed
A Profile Aspect SHALL have exactly one type from the controlled list: skill, responsibility, experience, qualification, seniority, location, work arrangement, work rights, employment type, compensation, and other. A type outside the list SHALL fail validation, and SHALL NOT be stored, renamed, or mapped to another type.

#### Scenario: Every listed type is accepted
- **WHEN** a proposal contains one valid aspect of each type in the controlled list
- **THEN** validation passes and every aspect is stored with the type it declared

#### Scenario: An unlisted type fails the proposal
- **WHEN** a proposal contains an aspect whose type is not in the controlled list
- **THEN** validation fails naming that type, and no aspect from the proposal is stored

#### Scenario: The database refuses an unlisted type
- **WHEN** an aspect row with a type outside the controlled list is written directly
- **THEN** the database refuses it

### Requirement: Role priority is never invented
A Profile Aspect belonging to a role SHALL carry a priority of must-have, nice-to-have, or unspecified. An aspect whose source does not support must-have or nice-to-have SHALL be unspecified, and no later step SHALL promote it. Aspects belonging to a candidate SHALL NOT carry an employer priority.

#### Scenario: An absent priority becomes unspecified
- **WHEN** a role proposal contains an aspect with no priority stated
- **THEN** the stored aspect has priority unspecified

#### Scenario: Unclear wording stays unspecified
- **WHEN** a role source describes a requirement in wording that supports neither must-have nor nice-to-have
- **THEN** the aspect is stored as unspecified and never as must-have

#### Scenario: An unsupported priority fails the proposal
- **WHEN** a proposal contains an aspect with a priority outside the three permitted values
- **THEN** validation fails and no aspect from the proposal is stored

#### Scenario: A candidate aspect carries no employer priority
- **WHEN** a candidate proposal contains an aspect with a must-have priority
- **THEN** validation fails, because employer priority is not a property of a candidate's evidence

### Requirement: Structured values are restricted per type
Location, work arrangement, work rights, employment type, and compensation aspects MAY carry a normalized structured value, restricted to the fields defined for that type. Any other type SHALL NOT carry one, and a field name outside the defined set SHALL fail validation rather than be ignored. The original source wording SHALL remain available regardless.

#### Scenario: A defined structured field is accepted
- **WHEN** a location aspect carries only the structured fields defined for location
- **THEN** validation passes and both the structured value and the source wording are stored

#### Scenario: An invented structured field fails the proposal
- **WHEN** an aspect carries a structured field that is not defined for its type
- **THEN** validation fails naming the field, rather than dropping it silently

#### Scenario: A type with no structured form carries none
- **WHEN** a skill aspect carries a structured value
- **THEN** validation fails

#### Scenario: An absent structured value is legal
- **WHEN** a work arrangement aspect states wording the model could not normalize and carries no structured value
- **THEN** validation passes, because "the source does not say" is a true and useful answer

### Requirement: An aspect records whether it was extracted or recruiter supplied
Every Profile Aspect SHALL record its origin as either extracted or recruiter supplied. The two SHALL remain distinguishable for the life of the aspect, and a recruiter supplied aspect SHALL be visibly labelled as such wherever it is used as evidence.

#### Scenario: An extracted aspect keeps its origin
- **WHEN** an aspect is produced by the classifier from a document
- **THEN** its origin is extracted

#### Scenario: A recruiter supplied aspect keeps its origin
- **WHEN** the recruiter authors an aspect directly
- **THEN** its origin is recruiter supplied, and it is not confusable with an extracted one

### Requirement: Source wording is preserved
Every Profile Aspect SHALL preserve the wording of its source. Normalization SHALL add a structured value beside the wording rather than replace it.

#### Scenario: Normalization does not overwrite wording
- **WHEN** a compensation aspect is normalized into a structured value
- **THEN** the source wording remains stored and retrievable alongside it
