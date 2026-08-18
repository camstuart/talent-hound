## ADDED Requirements

### Requirement: Dropping a resume creates one candidate and one artifact, or neither
Dropping a resume onto a Job Search Initiative SHALL create exactly one Candidate, one Artifact, and the links between them, in a single transaction. Cancelling SHALL create neither.

#### Scenario: A completed drop creates both
- **WHEN** a resume is dropped onto a Job Search Initiative and confirmed
- **THEN** exactly one Candidate and one Artifact exist, and the artifact is linked to both the candidate and the initiative

#### Scenario: A cancelled drop creates nothing
- **WHEN** a resume drop is cancelled
- **THEN** no Candidate and no Artifact were created

#### Scenario: A failure part-way creates nothing
- **WHEN** creating the candidate succeeds but linking the artifact fails
- **THEN** neither the candidate nor the artifact remains

### Requirement: A resume may attach to an existing candidate instead
Dropping a resume SHALL support attaching it to an already-selected Candidate rather than creating a new one, without creating a duplicate Candidate.

#### Scenario: Attaching to a selected candidate
- **WHEN** a resume is dropped with an existing candidate selected
- **THEN** the artifact is linked to that candidate and no new Candidate is created

### Requirement: Classification combines structured data and artifacts
Classifying a candidate SHALL use both the candidate's structured record and their linked, extracted artifacts as sources, preserving the origin of each.

#### Scenario: Both sources reach the profile
- **WHEN** a candidate with structured details and an extracted resume is classified
- **THEN** the profile may contain aspects from both, and each aspect's origin identifies which

#### Scenario: Record-derived aspects cite the record
- **WHEN** an aspect derives from the candidate's structured record rather than a document
- **THEN** it carries recruiter supplied origin and cites that record

#### Scenario: Document-derived aspects cite a chunk
- **WHEN** an aspect derives from an artifact
- **THEN** it carries extracted origin and cites a source chunk that resolves
