## ADDED Requirements

### Requirement: A profile is a versioned derived record
The application SHALL persist a profile as a version carrying the subject it describes, the profile schema version, the classifier prompt version, the `classify` model assignment revision that produced it, and a content hash of the sources it was derived from.

#### Scenario: A profile records its full provenance
- **WHEN** a profile is created from a source
- **THEN** it records the schema version, the prompt version, the classify assignment revision, and the source content hash

#### Scenario: A profile belongs to exactly one subject
- **WHEN** a profile is created
- **THEN** it names one subject kind and one subject identifier

### Requirement: A change to the contract changes derived identity
A change to the profile schema version, the classifier prompt version, or the `classify` assignment revision SHALL produce a different derived identity for a profile of the same subject and sources. Existing profiles SHALL NOT be modified by such a change.

#### Scenario: A schema version change changes identity
- **WHEN** the profile schema version changes and the same subject is classified from the same sources
- **THEN** the resulting profile has a different derived identity from the previous one

#### Scenario: A prompt version change changes identity
- **WHEN** the classifier prompt version changes and the same subject is classified from the same sources
- **THEN** the resulting profile has a different derived identity

#### Scenario: A model change changes identity
- **WHEN** the classify role is assigned a different model, producing a new revision, and the same subject is classified from the same sources
- **THEN** the resulting profile has a different derived identity

#### Scenario: Unchanged inputs yield the same identity
- **WHEN** the same subject is classified from the same sources with schema, prompt, and model unchanged
- **THEN** the derived identity is the same

#### Scenario: A source change changes identity
- **WHEN** the source content changes and the subject is classified again
- **THEN** the derived identity differs, because the sources are part of it

### Requirement: The classify role's model is resolved through the registry
Classification SHALL use the model resolved for the `classify` role, including the case where `classify` inherits `generate`. Classification SHALL fail visibly when no model resolves for the role.

#### Scenario: An inherited classify model is used and recorded
- **WHEN** classify is unassigned and generate has an assignment
- **THEN** classification uses the generate model and records the revision that answered

#### Scenario: No resolvable model fails visibly
- **WHEN** neither classify nor generate has an assignment
- **THEN** classification fails with a coded reason and persists nothing

### Requirement: Profile versions accumulate rather than overwrite
Creating a profile for a subject that already has one SHALL add a version rather than modify the existing record. Earlier versions SHALL remain retrievable.

#### Scenario: A second classification adds a version
- **WHEN** a subject with an existing profile is classified again
- **THEN** a new profile version exists and the earlier one is unchanged

#### Scenario: The current profile is the newest version
- **WHEN** a subject's current profile is requested
- **THEN** the highest version for that subject is returned
