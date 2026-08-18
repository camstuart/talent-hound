## ADDED Requirements

### Requirement: Every deletion is blocked, link-only, or transactional
Each deletion SHALL be one of: refused with its reason, a removal of one link only, or a single transaction that cascades to everything derived.

#### Scenario: A cascade is one transaction
- **WHEN** a cascading deletion runs
- **THEN** every step commits together

#### Scenario: A failure at any step changes nothing
- **WHEN** any step of a cascade fails
- **THEN** every affected table holds exactly what it held before

#### Scenario: Each step is exercised
- **WHEN** failure is injected at each cascade step in turn
- **THEN** each one rolls back completely

### Requirement: Success is verified before it is reported
After a cascade commits, the application SHALL query for what should be gone and SHALL report failure if any of it remains.

#### Scenario: Remaining derived data fails the deletion
- **WHEN** a cascade commits but leaves derived data behind
- **THEN** the operation reports failure rather than success

#### Scenario: Verification covers the derived kinds
- **WHEN** verification runs
- **THEN** it checks chunks, retrieval index rows, embeddings, profiles, matches, and exclusively owned artifacts

### Requirement: Verification is scoped so shared evidence is not a failure
Verification queries SHALL be scoped to the deleted entity, so evidence deliberately shared with another record is not mistaken for a failed deletion.

#### Scenario: Shared evidence does not fail verification
- **WHEN** an artifact retained under another link still has chunks
- **THEN** the deletion is reported successful

#### Scenario: Exclusively owned evidence must be gone
- **WHEN** an artifact owned only by the deleted entity still has chunks
- **THEN** the deletion is reported failed

### Requirement: Repeated deletion is safe
Deleting something already deleted SHALL be safe and SHALL NOT affect unrelated records.

#### Scenario: A repeated deletion is harmless
- **WHEN** a deletion is requested twice
- **THEN** the second is safe and reports the record is already gone

#### Scenario: Unrelated records are untouched
- **WHEN** a repeated deletion runs
- **THEN** every other record is exactly as it was

### Requirement: Purging every stale role applies the invariant per role
Purging all stale roles SHALL apply the same rules independently to each, and SHALL report any role that could not be purged rather than partially deleting it.

#### Scenario: Each role is purged independently
- **WHEN** several stale roles are purged together
- **THEN** each one's cascade is its own transaction

#### Scenario: One failure does not affect the others
- **WHEN** one role cannot be purged
- **THEN** it is reported and the others are purged

#### Scenario: A failed purge leaves that role whole
- **WHEN** a role's purge fails
- **THEN** that role and everything derived from it are unchanged
