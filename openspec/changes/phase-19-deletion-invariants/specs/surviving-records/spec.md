## ADDED Requirements

### Requirement: Recruiter-authored notes survive a purge
A recruiter's note about a purged role SHALL survive, with its role reference cleared so it reads as referring to something no longer available.

#### Scenario: The note survives
- **WHEN** a role with a recruiter-authored note is purged
- **THEN** the note still exists

#### Scenario: Its reference is cleared
- **WHEN** the note is read after the purge
- **THEN** it no longer points at the purged role

#### Scenario: The note's text is unchanged
- **WHEN** the note survives
- **THEN** its words are exactly as the recruiter wrote them

### Requirement: Audit events survive what they were about
Metadata-only audit events SHALL survive within their initiative when the record they referenced is deleted, with that reference cleared.

#### Scenario: A copy event survives its draft
- **WHEN** a draft with copy events is deleted
- **THEN** the events remain with no draft reference

#### Scenario: A copy event survives its role
- **WHEN** a role referenced by copy events is purged
- **THEN** the events remain with no role reference

#### Scenario: A disclosure event survives its role
- **WHEN** a role referenced by a disclosure event is purged
- **THEN** the event remains, recording that a disclosure happened

#### Scenario: Surviving events stay in their initiative
- **WHEN** events survive a deletion
- **THEN** they are still associated with the initiative they occurred in

### Requirement: Deleting an initiative removes its audit events
An initiative's own audit events SHALL be deleted with it, because they belong to it rather than outliving it.

#### Scenario: The initiative's events go with it
- **WHEN** an initiative with disclosure and copy events is deleted
- **THEN** those events are gone

#### Scenario: Another initiative's events are untouched
- **WHEN** one initiative is deleted
- **THEN** another initiative's events remain
