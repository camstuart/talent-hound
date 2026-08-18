## ADDED Requirements

### Requirement: Only out-of-scope, deleted, and Stale roles are excluded
The shortlist SHALL exclude roles outside the initiative's scope, roles that have been deleted, and roles whose profile is Stale. No other condition SHALL exclude a role.

#### Scenario: Another initiative's roles are excluded
- **WHEN** the shortlist is computed for one initiative
- **THEN** roles belonging only to another initiative do not appear

#### Scenario: A stale role is excluded
- **WHEN** a role's listing has changed since it was profiled
- **THEN** it does not appear on the shortlist

#### Scenario: A deleted role is excluded
- **WHEN** a role has been deleted
- **THEN** it does not appear, and nothing about it is retrieved

#### Scenario: A role that fails a must-have is not excluded
- **WHEN** a role conflicts with a must-have criterion
- **THEN** it may still appear on the shortlist

### Requirement: Exclusions are applied before retrieval
Scope, deletion, and staleness SHALL be applied when selecting what to retrieve against, not by filtering results afterwards.

#### Scenario: Excluded roles never reach fusion
- **WHEN** the shortlist is computed with excluded roles present in the database
- **THEN** no excluded role appears in any ranked list

#### Scenario: Exclusion does not shrink the shortlist
- **WHEN** more than twenty eligible roles exist alongside excluded ones
- **THEN** exactly twenty eligible roles are returned, rather than twenty minus the excluded ones

### Requirement: Structured conflicts are carried, not applied
A role retrieved despite conflicting with a structured must-have SHALL remain on the shortlist with the conflict recorded against it.

#### Scenario: A location conflict is visible
- **WHEN** a must-have location criterion says one place and a retrieved role's location says another
- **THEN** the role appears with that conflict noted

#### Scenario: A work-rights conflict is visible
- **WHEN** a role requires work rights the criteria say are absent
- **THEN** the role appears with that conflict noted

#### Scenario: A work-arrangement conflict is visible
- **WHEN** a must-have arrangement conflicts with a role's stated arrangement
- **THEN** the role appears with that conflict noted

#### Scenario: A conflicting role is not hidden
- **WHEN** every retrieved role conflicts with a must-have
- **THEN** all of them appear, rather than the shortlist being empty
