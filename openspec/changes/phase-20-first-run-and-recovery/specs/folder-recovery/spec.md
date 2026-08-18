## ADDED Requirements

### Requirement: A copied data folder can be opened
Selecting a previously copied data folder SHALL run the integrity and schema-version checks, snapshot before applying migrations, and open only if every check passes.

#### Scenario: A healthy copied folder opens
- **WHEN** a folder copied while the application was closed is selected
- **THEN** integrity passes, migrations apply behind a snapshot, and the data is present

#### Scenario: Missing credentials and models do not block the data
- **WHEN** a copied folder is opened on a machine with no stored credentials and no downloaded models
- **THEN** the data opens, and the missing credentials and models are reported as guided recovery steps rather than as data loss

### Requirement: A failed check never opens or overwrites the only copy
A failed pre-check, integrity check, or migration SHALL leave the selected folder unopened and unmodified.

#### Scenario: A corrupt database is refused
- **WHEN** the selected folder holds a corrupt database
- **THEN** it is refused with the integrity failure, and nothing is written

#### Scenario: A failing migration restores the snapshot
- **WHEN** a migration fails partway
- **THEN** the snapshot is restored, the folder is not opened, and the failure names the migration

#### Scenario: A read-only folder fails before any write
- **WHEN** the selected folder cannot be written to
- **THEN** it is refused before the integrity check, naming writability as the reason

#### Scenario: A folder with no database is refused
- **WHEN** the selected folder holds no database file
- **THEN** it is refused as not a Talent Hound data folder

#### Scenario: A future schema is refused
- **WHEN** the database is at a schema version newer than this build
- **THEN** it is refused, naming both versions, and no migration runs

### Requirement: The recovery procedure is documented in the application
The application SHALL document copying the data folder while fully closed, reinstalling, selecting the copy, re-entering credentials, and re-downloading models.

#### Scenario: The procedure names the resolved folder
- **WHEN** the recovery documentation is shown
- **THEN** it names the current resolved data folder rather than a generic location
