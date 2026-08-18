## ADDED Requirements

### Requirement: Initiative creation with a valid type
The application SHALL create initiatives of the three defined types — Job Search, Talent Search, and Business Development — and SHALL reject any other type and any empty name.

#### Scenario: Each valid type can be created
- **WHEN** an initiative is created with a non-empty name and one of the three defined types
- **THEN** it is persisted with that type, an Active lifecycle state, and a creation timestamp

#### Scenario: Unknown type is rejected
- **WHEN** an initiative is created with a type outside the defined set
- **THEN** creation fails with an error naming the rejected type and nothing is persisted

#### Scenario: Blank name is rejected
- **WHEN** an initiative is created with a name that is empty or only whitespace
- **THEN** creation fails with an error naming the field and nothing is persisted

### Requirement: Job Search initiatives reference exactly one candidate
A Job Search Initiative SHALL reference exactly one Candidate. Talent Search and Business Development initiatives SHALL NOT require a candidate.

#### Scenario: Job Search without a candidate is rejected
- **WHEN** a Job Search initiative is created with no candidate
- **THEN** creation fails with an error naming the candidate requirement and nothing is persisted

#### Scenario: Job Search with more than one candidate is rejected
- **WHEN** a Job Search initiative is created referencing more than one candidate
- **THEN** creation fails with an error stating that exactly one candidate is required and nothing is persisted

#### Scenario: Job Search with one candidate is created
- **WHEN** a Job Search initiative is created referencing exactly one existing candidate
- **THEN** it is persisted and the candidate is reachable from the initiative without the candidate record being copied

#### Scenario: Other types need no candidate
- **WHEN** a Talent Search or Business Development initiative is created with no candidate
- **THEN** it is persisted successfully

#### Scenario: Reference to a missing candidate is rejected
- **WHEN** a Job Search initiative references a candidate that does not exist
- **THEN** creation fails with an error and nothing is persisted

### Requirement: Initiative rename
The application SHALL allow an initiative to be renamed without changing its type, lifecycle state, references, or creation time.

#### Scenario: Rename persists
- **WHEN** an initiative is renamed to a valid non-empty name
- **THEN** the new name is persisted, and the type, lifecycle state, referenced records, and creation time are unchanged

#### Scenario: Rename to a blank name is rejected
- **WHEN** an initiative is renamed to an empty or whitespace-only name
- **THEN** the rename fails with an error and the stored name is unchanged

#### Scenario: Duplicate names are allowed
- **WHEN** an initiative is renamed to a name another initiative already uses
- **THEN** the rename succeeds, because the name is a label and the identifier is the initiative's own

### Requirement: Archive and reopen
An initiative SHALL move between Active and Archived, and SHALL NOT enter any other state. Archiving SHALL preserve every reference the initiative holds.

#### Scenario: Archiving preserves references
- **WHEN** an Active initiative referencing records is archived
- **THEN** its state becomes Archived and every referenced record is still reachable from it and still exists independently

#### Scenario: Reopening restores the Active state
- **WHEN** an Archived initiative is reopened
- **THEN** its state becomes Active with all references intact

#### Scenario: Redundant transitions are rejected
- **WHEN** an Active initiative is archived twice, or an Archived initiative is reopened twice
- **THEN** the second attempt fails with an error naming the current state and nothing changes

#### Scenario: Listing separates active from archived
- **WHEN** initiatives are listed
- **THEN** Active initiatives are returned by default and Archived initiatives are retrievable explicitly

### Requirement: Initiative deletion never removes shared records
Deleting an initiative SHALL delete the initiative and the records it exclusively owns, and SHALL NOT delete any Candidate, Role, Company, Contact, or recruiter-added Artifact.

#### Scenario: Deleting an initiative leaves shared records
- **WHEN** an initiative referencing a candidate, a role, a company, and a contact is deleted
- **THEN** the initiative is gone and all four records still exist and remain referencable by other initiatives

#### Scenario: Deleting one of two referencing initiatives
- **WHEN** two initiatives reference the same candidate and one is deleted
- **THEN** the other initiative still resolves that candidate

#### Scenario: Archived initiatives can be deleted
- **WHEN** an Archived initiative is deleted
- **THEN** it is removed under the same rules as an Active one
