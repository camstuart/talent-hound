## ADDED Requirements

### Requirement: Artifacts link to initiatives and records without copying bytes
An Artifact SHALL be linkable to initiatives, candidates, roles, companies, and contacts through links that reference it. Linking SHALL NOT copy the bytes, and one artifact SHALL be linkable to several targets at once.

#### Scenario: One artifact linked to several targets
- **WHEN** an artifact is linked to an initiative and to a candidate
- **THEN** both targets resolve the same artifact identifier, and only one copy of the bytes exists

#### Scenario: Linking to a target that does not exist is rejected
- **WHEN** an artifact is linked to a target identifier with no matching record
- **THEN** the link fails with an error and no link is created

#### Scenario: Linking twice to the same target is not duplicated
- **WHEN** an artifact already linked to a target is linked to it again
- **THEN** the result is still one link, and this is not reported as a failure

#### Scenario: Listing an artifact's links
- **WHEN** an artifact's links are listed
- **THEN** every target it is attached to is returned, identified by type and record

### Requirement: Creating an artifact with its first link is atomic
When an ingestion supplies a link target, the artifact and its first link SHALL be created in one transaction.

#### Scenario: A failed link leaves no artifact
- **WHEN** an ingestion names a link target that cannot be linked
- **THEN** neither the artifact nor the link is persisted

#### Scenario: An artifact may be ingested with no target
- **WHEN** an ingestion supplies no link target
- **THEN** the artifact is created with no links and appears in the orphan library

### Requirement: Detaching removes one link only
Detaching SHALL remove exactly one link, leaving the artifact's bytes, provenance, and every other link untouched. The application SHALL NOT expose global artifact deletion in this phase.

#### Scenario: Detaching one link preserves the others
- **WHEN** an artifact linked to two targets is detached from one
- **THEN** the bytes and provenance are unchanged and the other link still resolves the artifact

#### Scenario: Detaching the last link produces a visible orphan
- **WHEN** an artifact's last remaining link is detached
- **THEN** the artifact and its bytes still exist and it appears in the orphan library

#### Scenario: Detaching a link that is not there
- **WHEN** a detach names a target the artifact is not linked to
- **THEN** an error is returned and nothing changes

### Requirement: Orphaned-artifact library
The application SHALL list artifacts with no links, so that an artifact is never lost by having its last link removed.

#### Scenario: Orphan listing reflects current links
- **WHEN** an artifact is linked, then detached, then linked again
- **THEN** it is absent from the orphan listing while linked and present while it has no links

#### Scenario: Orphans keep their bytes and provenance
- **WHEN** an orphaned artifact is read
- **THEN** its bytes, original filename, media type, SHA-256, source, and capture time are all intact
