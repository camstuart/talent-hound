## MODIFIED Requirements

### Requirement: A profile is a versioned derived record
The application SHALL persist a profile as a version carrying the subject it describes, the profile schema version, the classifier prompt version, the `classify` model assignment revision that produced it, a content hash of the sources it was derived from, and its lifecycle state. A version produced by a recruiter edit rather than a model SHALL record no model revision and SHALL still carry a source hash of the evidence in force when it was made.

#### Scenario: A profile records its full provenance
- **WHEN** a profile is created from a source
- **THEN** it records the schema version, the prompt version, the classify assignment revision, and the source content hash

#### Scenario: A profile belongs to exactly one subject
- **WHEN** a profile is created
- **THEN** it names one subject kind and one subject identifier

#### Scenario: A profile carries a lifecycle state
- **WHEN** a profile version is created
- **THEN** it records whether it is proposed, approved, or failed

#### Scenario: A recruiter-made version needs no model
- **WHEN** a version is produced by editing, adding, or removing an aspect
- **THEN** it is stored with no model revision and is a version like any other
