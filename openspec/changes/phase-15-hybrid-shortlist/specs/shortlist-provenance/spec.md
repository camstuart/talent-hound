## ADDED Requirements

### Requirement: Every shortlisted role says why it is there
Each entry SHALL record which criteria and which candidate aspects retrieved it, and by which method, so a recruiter can see why it was selected without re-running anything.

#### Scenario: An entry names what retrieved it
- **WHEN** a role is on the shortlist
- **THEN** it lists the criteria and aspects that matched it

#### Scenario: An entry names the method
- **WHEN** a role was retrieved lexically, semantically, or both
- **THEN** the entry says which

#### Scenario: Provenance is recorded, not reconstructed
- **WHEN** a shortlist is inspected after it was computed
- **THEN** its provenance is the one recorded at the time, not a fresh retrieval

### Requirement: A shortlist records the intent it was computed under
A shortlist SHALL record the criteria version and the embedding space it used, so a later reader can tell whether it reflects the current intent.

#### Scenario: The criteria version is recorded
- **WHEN** a shortlist is computed
- **THEN** it records the criteria version in force

#### Scenario: A changed criteria version is detectable
- **WHEN** criteria change after a shortlist was computed
- **THEN** the shortlist's recorded version differs from the current one
