package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
)

// Failure kinds returned by Open. All of them mean "the database was not
// opened"; none of them ever returns a usable connection.
var (
	ErrFutureSchema = errors.New("database schema is newer than this build")
	ErrIntegrity    = errors.New("database failed integrity check")
	ErrSnapshot     = errors.New("pre-migration snapshot failed")
	ErrMigration    = errors.New("schema migration failed")
	ErrRestore      = errors.New("restoring the pre-migration snapshot failed")
)

// schemaVersion reads the version recorded in the database header.
func schemaVersion(gdb *gorm.DB) (int, error) {
	var v int
	if err := gdb.Raw("PRAGMA user_version").Scan(&v).Error; err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return v, nil
}

// integrityCheck runs SQLite's own structural check over an existing file.
func integrityCheck(gdb *gorm.DB) error {
	var rows []string
	if err := gdb.Raw("PRAGMA integrity_check").Scan(&rows).Error; err != nil {
		return fmt.Errorf("%w: %w", ErrIntegrity, err)
	}
	if len(rows) != 1 || rows[0] != "ok" {
		return fmt.Errorf("%w: %v", ErrIntegrity, rows)
	}
	return nil
}

// snapshotPath is where the pre-migration copy of a database at version v is
// written: a snapshots/ subfolder beside the database, inside the data folder
// the recruiter copies for recovery.
func snapshotPath(dbPath string, v int) string {
	return filepath.Join(filepath.Dir(dbPath), "snapshots", fmt.Sprintf("pre-v%d.db", v))
}

// snapshot writes a consistent copy of the open database to dst. VACUUM INTO
// produces a complete, already-consistent database file (including committed
// WAL content), so there is no multi-file copy race to get wrong.
func snapshot(gdb *gorm.DB, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("%w: creating snapshot directory: %w", ErrSnapshot, err)
	}
	// VACUUM INTO refuses to overwrite; a leftover from an earlier failed
	// attempt is stale by definition.
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%w: removing stale snapshot: %w", ErrSnapshot, err)
	}
	if err := gdb.Exec("VACUUM INTO ?", dst).Error; err != nil {
		return fmt.Errorf("%w: %w", ErrSnapshot, err)
	}
	return nil
}

// applyMigrations runs every migration newer than from, each one atomically
// with its own version bump.
func applyMigrations(gdb *gorm.DB, migs []migration, from int) error {
	for _, m := range migs {
		if m.Version <= from {
			continue
		}
		err := gdb.Transaction(func(tx *gorm.DB) error {
			for _, s := range m.SQL {
				if err := tx.Exec(s).Error; err != nil {
					return fmt.Errorf("statement %q: %w", s, err)
				}
			}
			// PRAGMA takes no bound parameters; m.Version is an int constant
			// from the migration list, never user input.
			return tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.Version)).Error
		})
		if err != nil {
			return fmt.Errorf("%w: migration %d (%s): %w", ErrMigration, m.Version, m.Name, err)
		}
	}
	return nil
}

// restore puts the snapshot back in place of a half-migrated database and
// verifies the result. The connection must already be closed.
func restore(dbPath, snap string) error {
	if err := os.Rename(snap, dbPath); err != nil {
		return fmt.Errorf("%w: %w", ErrRestore, err)
	}
	// The snapshot is a single self-contained file; sidecars belong to the
	// database we just discarded.
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%w: removing %s: %w", ErrRestore, dbPath+suffix, err)
		}
	}
	gdb, err := openRaw(dbPath)
	if err != nil {
		return fmt.Errorf("%w: reopening restored database: %w", ErrRestore, err)
	}
	defer closeDB(gdb)
	if err := integrityCheck(gdb); err != nil {
		return fmt.Errorf("%w: %w", ErrRestore, err)
	}
	return nil
}

// closeDB releases the file handles; the file cannot be replaced on Windows
// while they are open.
func closeDB(gdb *gorm.DB) {
	if sqlDB, err := gdb.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
