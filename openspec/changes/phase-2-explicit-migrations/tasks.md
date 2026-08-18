## 1. Migration runner

- [x] 1.1 Add `internal/db/migrations.go`: ordered `[]migration{Version, Name, SQL}` with migration 1 as the `CREATE TABLE IF NOT EXISTS` baseline matching `AutoMigrate`'s `initiatives` table and index
- [x] 1.2 Add `internal/db/migrate.go`: read `PRAGMA user_version`, reject a future version without writing, apply each pending migration and its version bump in one transaction
- [x] 1.3 Rewrite `Open` to run pragmas (WAL, foreign keys, busy timeout), integrity check, migrate, FTS5 smoke test — and to stop calling `AutoMigrate`

## 2. Snapshot and restore

- [x] 2.1 Snapshot pending-migration source state with `VACUUM INTO <folder>/snapshots/pre-v<N>.db`; skip for `:memory:` and when nothing is pending
- [x] 2.2 Restore on migration failure: close the connection, replace the database file, remove `-wal`/`-shm`, integrity-check the restored file, return a distinct error
- [x] 2.3 Distinguish snapshot-failure, migration-failure, and restore-failure errors

## 3. Integrity and FTS5

- [x] 3.1 Run `PRAGMA integrity_check` on existing files before snapshot or migration; refuse to open on failure
- [x] 3.2 Call `platform.CheckFTS5` at the end of `Open`; assert it leaves no residual table

## 4. Path resolution

- [x] 4.1 Keep `TALENT_HOUND_DB_PATH` as the explicit override and the per-user default otherwise; create parents with owner-only permissions
- [x] 4.2 Test that the resolved path is stable across calls and that the override wins

## 5. Tests against real files

- [x] 5.1 Zero-to-current on a new file; current-database reopen is idempotent and creates no snapshot
- [x] 5.2 Historical fixture at each earlier version migrates forward with its rows intact
- [x] 5.3 `AutoMigrate`-shaped database is adopted at version 1 and its schema matches a freshly created one
- [x] 5.4 Future version is rejected with the file unmodified (compare file hash)
- [x] 5.5 Failing migration restores a byte-valid, integrity-checked snapshot at the pre-migration version and leaves no partially applied schema
- [x] 5.6 Snapshot failure aborts before any migration (injected by occupying the `snapshots` name with a file — a read-only folder blocks WAL creation and fails the open itself, before the snapshot)
- [x] 5.7 Corrupt database is refused without writes
- [x] 5.8 Interrupted migration re-applies cleanly on the next open
- [x] 5.9 Concurrent open of the same file: contended write waits within the busy timeout or returns a distinct busy error

## 6. Exit gate

- [x] 6.1 Existing initiative records survive an upgrade fixture (E2E database shape included)
- [x] 6.2 `just qa` and all three test layers pass; Playwright E2E still starts against the real backend (`just vuln` still fails on the pre-existing Go stdlib advisories that need a go1.26.6 toolchain)
