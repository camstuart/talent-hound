## ADDED Requirements

### Requirement: Ordered explicit schema migrations
The application SHALL define its schema as an ordered list of explicit SQL migrations, each with a unique increasing version, and SHALL record the applied version inside the database file. Migrations SHALL be the only source of schema truth; ORM auto-migration SHALL NOT run.

#### Scenario: New database migrates from zero
- **WHEN** a database file that does not yet exist is opened
- **THEN** every migration is applied in version order and the recorded schema version equals the newest migration's version

#### Scenario: Historical database migrates forward with records intact
- **WHEN** a database recorded at an earlier schema version is opened
- **THEN** only the migrations newer than that version are applied, in order, and all pre-existing rows are still present afterwards

#### Scenario: Reopening a current database is idempotent
- **WHEN** a database already at the newest version is opened again
- **THEN** no migration runs, no snapshot is created, and the schema version is unchanged

#### Scenario: Unknown future version is rejected
- **WHEN** a database records a schema version newer than the newest migration this build knows
- **THEN** opening fails with an error naming both versions and the database file is not written to

#### Scenario: Each migration is atomic
- **WHEN** a migration's SQL fails partway through
- **THEN** that migration's statements and its version bump are rolled back together, leaving no partially applied schema

### Requirement: Pre-migration snapshot and restore
The application SHALL create a snapshot of the database file before applying any pending migration, and SHALL restore that snapshot and refuse to open the database when a migration fails.

#### Scenario: Snapshot precedes pending migrations
- **WHEN** an existing database has pending migrations
- **THEN** a snapshot is written inside the data folder before the first migration statement runs

#### Scenario: Failing migration restores the snapshot
- **WHEN** a migration fails
- **THEN** the database file is restored from the snapshot, the restored file passes an integrity check, its schema version is the pre-migration version, and opening returns an error rather than a usable database

#### Scenario: Snapshot creation failure aborts before any migration
- **WHEN** the snapshot cannot be created (read-only folder or insufficient space)
- **THEN** opening fails with an error naming the snapshot, and no migration is applied

#### Scenario: Interrupted migration is recoverable on the next open
- **WHEN** the process is interrupted mid-migration and the database is opened again
- **THEN** the recorded schema version is either the pre-migration or the post-migration version, never an intermediate state, and the pending migrations are re-applied cleanly

### Requirement: Integrity checking before migration or recovery
The application SHALL run a SQLite integrity check on an existing database before migrating it or recovering it, and SHALL refuse to open a database that fails the check.

#### Scenario: Corrupt database is refused
- **WHEN** an existing database file fails the integrity check
- **THEN** opening fails with an error identifying the corruption, and neither migrations nor writes are attempted

#### Scenario: Recovered folder is checked before migration
- **WHEN** a copied data folder's database is opened
- **THEN** the integrity check runs before any snapshot or migration, and a failure leaves the recruiter's copy untouched

### Requirement: FTS5 verified at startup
The application SHALL verify FTS5 support in the resolved SQLite build during startup, before any personal data can be accepted.

#### Scenario: FTS5 unavailable fails startup
- **WHEN** the SQLite build does not provide FTS5
- **THEN** opening fails with an error naming FTS5, and the application does not present a data-entry path

#### Scenario: FTS5 smoke test leaves no residue
- **WHEN** the startup FTS5 check succeeds
- **THEN** no table, index, or row created by the check remains in the database

### Requirement: Database path resolution
The application SHALL resolve the database path from the selected data folder and SHALL NOT change which database is opened without an explicit configuration change.

#### Scenario: Explicit override is honoured
- **WHEN** an explicit database path is configured
- **THEN** that exact path is opened and no per-user default location is consulted

#### Scenario: Default location is stable
- **WHEN** no explicit path is configured
- **THEN** the same per-user path is resolved on every startup, and its parent directory is created with owner-only permissions

### Requirement: Concurrent open behaviour
Disk-backed database access SHALL have defined behaviour when a second connection contends for the same file.

#### Scenario: Contended write waits rather than failing immediately
- **WHEN** two connections to the same database file write concurrently
- **THEN** the second waits up to the configured busy timeout and either succeeds or returns a distinct busy error, and neither connection corrupts the file
