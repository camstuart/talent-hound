## Context

`internal/db.Open` calls `gorm.AutoMigrate(&models.Initiative{})` on every start. That cannot create FTS5 virtual tables or triggers, has no version number, no snapshot, and no failure path — a schema change that half-applies leaves the recruiter's only copy of their data in an undefined state. Phase 1 proved FTS5 works in the CGO-free driver; this phase makes the schema versioned and recoverable so later phases can add virtual tables safely.

## Goals / Non-Goals

**Goals:**
- One ordered list of explicit SQL migrations, applied atomically, with the version stored in the file.
- A snapshot before any pending migration, restored on failure, with opening refused rather than degraded.
- Integrity check before touching an existing database.
- FTS5 verified at startup by reusing `platform.CheckFTS5`.
- Tests against real files, not only `:memory:`.

**Non-Goals:**
- No down-migrations. Recovery is snapshot restore; a rollback DSL is a second schema language to keep correct.
- No migration CLI, no embedded migration framework, no generated migration scaffolding.
- No data-folder selection UI (Phase 20) — this phase only resolves the path it is given.
- No FTS5 tables yet; only the smoke test. Content migrations arrive with the features that need them.

## Decisions

**Schema version lives in `PRAGMA user_version`, not a `schema_migrations` table.**
The project plan says "schema-version table"; `user_version` is a 32-bit integer already in the database header, set inside the same transaction as the migration it belongs to, so version and schema can never disagree. A table would add a schema object that itself needs migrating and buys only an application history log, which the snapshot filenames and logs already give. Deviation recorded here deliberately.

**Migrations are `[]migration{Version, Name, SQL}` with plain SQL strings, applied via `gdb.Exec` inside `gdb.Transaction`.**
Alternative: `golang-migrate` or `goose`. Rejected — a new dependency, a file-embedding scheme, and a driver shim for ~20 lines of loop. SQLite runs DDL transactionally, so the statement batch and the `PRAGMA user_version = N` commit or roll back together.

**Migration 1 is a `CREATE TABLE IF NOT EXISTS` baseline matching what `AutoMigrate` produced.**
Existing developer and E2E databases sit at `user_version = 0` with the `initiatives` table already there. `IF NOT EXISTS` adopts them at version 1 with no data loss and no detection logic. New databases get the identical table. From migration 2 onward, plain `CREATE`/`ALTER` — the baseline is the only forgiving one.

**Snapshots use `VACUUM INTO '<snapshot>'`.**
Alternative: copy the file (plus `-wal`/`-shm`) while open. Rejected — `VACUUM INTO` is one statement that writes a consistent, already-integrity-clean database including committed WAL content, with no CGO backup API and no multi-file copy race. Snapshots land in `<data-folder>/snapshots/pre-v<N>.db`, one per source version, overwritten on retry.

**Restore closes the connection, replaces the database file, and deletes any `-wal`/`-shm` sidecars, then returns an error.**
A restored database that then opens successfully would hide the failure; the recruiter must be told the migration failed. The restored file is integrity-checked before `Open` returns, so a failed restore is reported distinctly from a failed migration.

**`:memory:` skips snapshotting and integrity checking, not migrating.**
Nothing to restore and nothing to corrupt. Go service tests keep using `db.Open(":memory:")` and still get the real schema.

**Concurrency is `PRAGMA busy_timeout` plus WAL mode.**
Alternative: a process-wide open-database registry or an advisory lock file. Rejected for a single-user desktop app — WAL lets the one reader/writer pair coexist and `busy_timeout` converts the racy failure into a bounded wait with a distinct error at the end.

**Failure injection is a test-only migration list, not build tags or interfaces.**
The runner takes the migration slice as a parameter; the exported `Open` passes the real one. Tests pass a list whose last entry is deliberately invalid SQL. No mock, no interface, no seam in production code.

## Risks / Trade-offs

- `VACUUM INTO` rewrites the whole file — on a large database the snapshot costs time and disk. Acceptable at PoC scale; Phase 20 documents the folder copy for real backup.
- `user_version` is a single integer with no history — a corrupted header loses the version. The integrity check runs first, and a version newer than this build is refused rather than guessed.
- Refusing to open on any failure means a corrupt file blocks the app entirely. That is the intended trade: never write into a database whose state is unknown.
- The baseline migration must match `AutoMigrate`'s output exactly, or adopted databases drift from new ones. A test opens an `AutoMigrate`-shaped database and asserts the resulting schema matches a fresh one.

## Migration Plan

Existing databases (all at `user_version = 0`) are adopted by migration 1 without data loss; the E2E database and any local developer database keep working. `internal/models` structs stay for GORM query use but no longer create tables. Rollback is reverting `internal/db` — the schema itself is unchanged by this phase.

## Open Questions

- Does the snapshot belong beside the database or in a `snapshots/` subfolder the recruiter sees during folder copy? Currently a subfolder; Phase 20's recovery documentation may move it.
- How many pre-migration snapshots to retain. Currently one per source version, overwritten.
