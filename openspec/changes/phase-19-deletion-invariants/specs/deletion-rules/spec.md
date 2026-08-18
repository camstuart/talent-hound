## ADDED Requirements

### Requirement: Deleting an initiative removes what it owns and nothing shared
Deleting an initiative SHALL delete its criteria, matches, drafts, jobs, and audit events. It SHALL NOT delete candidates, roles, companies, contacts, or recruiter-added artifacts.

#### Scenario: Owned records go
- **WHEN** an initiative with criteria, matches, drafts, and jobs is deleted
- **THEN** none of them remains

#### Scenario: Shared records stay
- **WHEN** an initiative referencing a candidate and a role is deleted
- **THEN** the candidate and the role still exist

#### Scenario: Recruiter-added artifacts stay
- **WHEN** an initiative with attached recruiter-added artifacts is deleted
- **THEN** the artifacts remain, with their other links intact

### Requirement: Candidate deletion is blocked by any referencing initiative
A candidate SHALL NOT be deleted while any initiative references them, whether Active or Archived. The refusal SHALL name the referencing initiatives.

#### Scenario: An active initiative blocks
- **WHEN** deletion is attempted for a candidate referenced by an active initiative
- **THEN** it is refused, naming that initiative

#### Scenario: An archived initiative blocks equally
- **WHEN** deletion is attempted for a candidate referenced only by an archived initiative
- **THEN** it is refused, naming that initiative

#### Scenario: Deletion succeeds once references are gone
- **WHEN** every referencing initiative has been deleted
- **THEN** the candidate can be deleted

### Requirement: Deleting a candidate removes their derived data
Deleting a candidate SHALL remove their profile, structured data, candidate-only artifacts, aspects, embeddings, and derived retrieval data.

#### Scenario: The profile and its aspects go
- **WHEN** a candidate with an approved profile is deleted
- **THEN** no profile or aspect for them remains

#### Scenario: Candidate-only artifacts go
- **WHEN** a candidate whose artifacts are linked only to them is deleted
- **THEN** those artifacts, their chunks, and their embeddings are gone

#### Scenario: Derived retrieval data goes
- **WHEN** a candidate is deleted
- **THEN** searching returns nothing derived from their evidence

### Requirement: A shared candidate artifact requires an explicit choice
When a candidate's artifact is also linked elsewhere, deletion SHALL be blocked until the recruiter chooses to delete the artifact globally or to retain it under its other links, having been warned it may contain candidate information.

#### Scenario: A shared artifact blocks deletion
- **WHEN** a candidate has an artifact also linked to a role
- **THEN** deletion is refused until a choice is made

#### Scenario: Choosing global deletion removes it everywhere
- **WHEN** the recruiter chooses global deletion
- **THEN** the artifact and all its links are gone

#### Scenario: Choosing retention keeps it under its other links
- **WHEN** the recruiter chooses retention
- **THEN** the artifact survives with its other links and without its candidate link

#### Scenario: The warning is given before retention
- **WHEN** retention is offered
- **THEN** it says the artifact may contain candidate information

### Requirement: Detaching and deleting an artifact are different operations
Detaching a recruiter-added artifact SHALL remove one link only, leaving the bytes and every other link. Deleting one globally SHALL list every link before confirmation, then remove every link and all derived data.

#### Scenario: Detach removes one link
- **WHEN** an artifact linked to two things is detached from one
- **THEN** the other link and the bytes remain

#### Scenario: Global deletion lists the links first
- **WHEN** global deletion is requested
- **THEN** every existing link is listed before it proceeds

#### Scenario: Global deletion removes everything derived
- **WHEN** an artifact is deleted globally
- **THEN** its links, chunks, and embeddings are gone

### Requirement: Exa source artifacts are read-only
A role's source artifact SHALL NOT be detached or deleted individually. It SHALL be removed only by purging its role.

#### Scenario: Detaching a source artifact is refused
- **WHEN** detaching a role's source artifact is attempted
- **THEN** it is refused, saying to purge the role instead

#### Scenario: Deleting a source artifact is refused
- **WHEN** deleting a role's source artifact globally is attempted
- **THEN** it is refused

#### Scenario: Purging the role removes it
- **WHEN** the role is purged
- **THEN** its current and historical source artifacts are gone

### Requirement: Purging a role removes its derived data and lists its references first
Purging a discovered role SHALL list the initiatives referencing it, then delete the role, its current and historical source artifacts, its profile, aspects, embeddings, matches, and active drafts.

#### Scenario: Referencing initiatives are listed
- **WHEN** a purge is previewed
- **THEN** every initiative referencing the role is listed

#### Scenario: Everything derived goes
- **WHEN** a role is purged
- **THEN** its sources, profile, aspects, embeddings, matches, and active drafts are gone

#### Scenario: Historical sources go too
- **WHEN** a role with superseded sources is purged
- **THEN** the historical ones are gone as well

### Requirement: Deleting a draft removes its content only
Deleting a draft SHALL remove the draft. Existing copy audit events SHALL survive within their initiative with their draft reference cleared.

#### Scenario: The draft goes
- **WHEN** a draft is deleted
- **THEN** it no longer exists

#### Scenario: Its copy events survive
- **WHEN** a draft with copy events is deleted
- **THEN** those events still exist, with no draft reference
