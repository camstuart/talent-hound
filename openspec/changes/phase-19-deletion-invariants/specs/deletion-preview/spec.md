## ADDED Requirements

### Requirement: A deletion is previewed before it is confirmed
Before any destructive action, the application SHALL list the exact records and links that would be removed, and any blockers.

#### Scenario: The preview lists what goes
- **WHEN** a deletion is previewed
- **THEN** it names the records and links that would be removed

#### Scenario: The preview lists blockers
- **WHEN** a deletion would be refused
- **THEN** the preview names what is blocking it

#### Scenario: Previewing changes nothing
- **WHEN** a deletion is previewed and not confirmed
- **THEN** nothing is deleted

### Requirement: Detach and global deletion are confirmed differently
The confirmation for removing one link SHALL be visibly different from the confirmation for removing an artifact everywhere.

#### Scenario: Detach says it removes one link
- **WHEN** a detach is confirmed
- **THEN** the confirmation says the bytes and other links remain

#### Scenario: Global deletion says it removes everything
- **WHEN** a global deletion is confirmed
- **THEN** the confirmation lists every link that will go

#### Scenario: The two cannot be mistaken for each other
- **WHEN** both confirmations are shown
- **THEN** they differ in wording and in what they list
