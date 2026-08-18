## ADDED Requirements

### Requirement: One immutable artifact per ingestion
Each ingestion SHALL create exactly one Artifact holding the original bytes and its provenance: original filename, detected media type, byte length, SHA-256, source, and capture time. Artifacts SHALL NOT be deduplicated, and the stored bytes and provenance SHALL NOT be changed by any later operation.

#### Scenario: Bytes round-trip exactly
- **WHEN** an artifact is ingested and read back
- **THEN** the returned bytes are byte-identical to those submitted, for plain text, Markdown, PDF, DOCX, and arbitrary binary content alike

#### Scenario: Identical bytes create two artifacts
- **WHEN** the same bytes are ingested twice
- **THEN** two artifacts exist with different identifiers, each with its own filename, source, and capture time, and neither replaces the other

#### Scenario: Provenance is recorded at ingestion
- **WHEN** an artifact is created
- **THEN** its byte length matches the stored bytes, its SHA-256 matches those bytes, and its capture time is set by the application rather than supplied by the caller

#### Scenario: Provenance cannot be edited
- **WHEN** any update operation the application exposes is applied to an artifact
- **THEN** the original filename, media type, byte length, SHA-256, source, capture time, and bytes are all unchanged

### Requirement: Editable display name
An Artifact SHALL carry a display name that the recruiter can change, separate from its immutable original filename. The display name SHALL default to the original filename when one is supplied.

#### Scenario: Renaming changes only the display name
- **WHEN** an artifact's display name is changed
- **THEN** the new display name is stored and the original filename, media type, byte length, SHA-256, source, capture time, and bytes are unchanged

#### Scenario: Blank display name is rejected
- **WHEN** an artifact is renamed to an empty or whitespace-only name
- **THEN** the rename fails with an error naming the field and the stored display name is unchanged

#### Scenario: Duplicate display names are allowed
- **WHEN** two artifacts are given the same display name
- **THEN** both are stored, because a display name is a label and the identifier is the artifact's own

### Requirement: Size limit
The application SHALL refuse an ingestion larger than the 25 MB per-file limit, and SHALL accept one exactly at the limit.

#### Scenario: Exactly at the limit is accepted
- **WHEN** an artifact of exactly the limit in bytes is ingested
- **THEN** it is stored and its byte length equals the limit

#### Scenario: One byte over the limit is refused
- **WHEN** an artifact one byte larger than the limit is ingested
- **THEN** ingestion fails with an error naming the limit and no artifact or link is created

#### Scenario: Zero bytes is accepted
- **WHEN** an artifact with no bytes is ingested
- **THEN** it is stored with a byte length of zero and the SHA-256 of empty input, because an empty file is still an ingestion that happened

### Requirement: Safe handling of untrusted metadata
The application SHALL store supplied filenames verbatim as provenance and SHALL NOT treat them as filesystem paths. Media type SHALL be determined from the bytes rather than from the filename.

#### Scenario: Media type is taken from the content
- **WHEN** an artifact's filename extension disagrees with what its bytes actually are
- **THEN** the media type recorded is the one detected from the bytes, and the ingestion is not refused

#### Scenario: Unicode and path-like filenames survive unchanged
- **WHEN** an artifact is ingested with a filename containing non-Latin characters, or one that looks like a filesystem path
- **THEN** the original filename is stored exactly as supplied, nothing is written outside the database, and the value is never used to open a file

#### Scenario: Missing filename is allowed
- **WHEN** an artifact is ingested with no filename, as pasted text has none
- **THEN** it is stored with an empty original filename and a display name supplied by the recruiter
