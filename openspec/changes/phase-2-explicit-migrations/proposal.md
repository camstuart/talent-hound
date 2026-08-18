## Why

The database is opened with GORM `AutoMigrate`, which cannot express the virtual tables, triggers, and rebuild paths FTS5 needs, has no version number, and cannot be rolled back. Every later phase writes irreplaceable recruiter data into this file, and the PoC's only backup story is a folder copy plus a pre-migration snapshot. The schema foundation has to become versioned and recoverable before any product data lands on it.

## What Changes

- Replace `AutoMigrate` with an ordered explicit SQL migration runner and a schema version recorded in the database file.
- Take a database snapshot before applying any pending migration, and restore it when a migration fails.
- Run a SQLite integrity check before migrating or recovering a database.
- Refuse to open the database — without writing to it — when the integrity check fails, when the schema is newer than this build, or when a migration fails after restore.
- Run the FTS5 smoke test at startup so an FTS5-less build fails visibly before personal data is accepted.
- Resolve the database path from the selected data folder without silently switching databases.
- Define concurrent-open behaviour for the disk-backed database.

## Capabilities

### New Capabilities
- `database-schema`: versioned schema migration, pre-migration snapshot and restore, integrity checking, FTS5 startup verification, and safe database opening.

### Modified Capabilities
<!-- none: no existing specs -->

## Impact

- `internal/db/db.go` rewritten: `Open` no longer calls `AutoMigrate`.
- New `internal/db/migrations.go` (the ordered list) and `internal/db/migrate.go` (runner, snapshot, restore, integrity check).
- New file-backed Go tests, including historical-version and failure-injection fixtures.
- `internal/models/` structs stop driving the schema; they must match the migrations, which become the source of truth.
- Existing developer and E2E databases created by `AutoMigrate` are adopted at version 1 rather than recreated.
