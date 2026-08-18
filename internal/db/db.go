// Package db opens the application's SQLite database (via the pure-Go,
// CGO-free driver) and keeps its schema at the version defined by the ordered
// migrations in migrations.go.
package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"camstuart/talent-hound/internal/platform"
)

// busyTimeoutMS bounds how long a contended write waits before returning
// SQLITE_BUSY. Single-user desktop app: a second connection is a background
// job, not a fleet.
const busyTimeoutMS = 5000

// DefaultPath returns the per-user location of the SQLite database file,
// creating its parent directory if needed. The TALENT_HOUND_DB_PATH
// environment variable overrides it (used by E2E tests).
func DefaultPath() (string, error) {
	if p := os.Getenv("TALENT_HOUND_DB_PATH"); p != "" {
		// #nosec G703 -- the path comes from the operator's own environment;
		// this is a local single-user desktop app, not a service.
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return "", fmt.Errorf("creating db directory: %w", err)
		}
		return p, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving user config dir: %w", err)
	}
	dir := filepath.Join(cfg, "talent-hound")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating db directory: %w", err)
	}
	return filepath.Join(dir, "talent-hound.db"), nil
}

// Open opens (creating if necessary) the SQLite database at path, checks its
// integrity, migrates it to the current schema version behind a snapshot, and
// verifies FTS5 support. Use ":memory:" for an ephemeral database.
//
// Every failure returns an error and no connection: a database whose state is
// unknown is never written to.
func Open(path string) (*gorm.DB, error) {
	return open(path, migrations)
}

// open is Open with an injectable migration list, so failure-injection tests
// need no seam in production code.
func open(path string, migs []migration) (*gorm.DB, error) {
	inMemory := strings.Contains(path, ":memory:")
	existed := false
	if !inMemory {
		if st, err := os.Stat(path); err == nil && st.Size() > 0 {
			existed = true
		}
	}

	gdb, err := openRaw(path)
	if err != nil {
		return nil, err
	}

	if existed {
		if err := integrityCheck(gdb); err != nil {
			closeDB(gdb)
			return nil, err
		}
	}

	current, err := schemaVersion(gdb)
	if err != nil {
		closeDB(gdb)
		return nil, err
	}
	latest := latestVersion(migs)
	if current > latest {
		closeDB(gdb)
		return nil, fmt.Errorf("%w: file is at v%d, this build knows up to v%d", ErrFutureSchema, current, latest)
	}

	if current < latest {
		snap := ""
		if existed {
			snap = snapshotPath(path, current)
			if err := snapshot(gdb, snap); err != nil {
				closeDB(gdb)
				return nil, err
			}
		}
		if err := applyMigrations(gdb, migs, current); err != nil {
			closeDB(gdb)
			if snap == "" {
				return nil, err
			}
			if rerr := restore(path, snap); rerr != nil {
				return nil, fmt.Errorf("%w (and %w)", err, rerr)
			}
			return nil, err
		}
	}

	if err := platform.CheckFTS5(gdb); err != nil {
		closeDB(gdb)
		return nil, err
	}
	return gdb, nil
}

// openRaw opens a connection and applies the connection pragmas, with no
// integrity, version, or FTS5 handling.
func openRaw(path string) (*gorm.DB, error) {
	// busy_timeout and foreign_keys are per-connection, and the pool opens more
	// connections than the one Exec below would reach — so they go in the DSN,
	// where every connection gets them.
	// No file: prefix: the driver then strips the query from the path itself, so
	// a Windows path needs no URI escaping.
	//
	// _txlock=immediate takes the write lock when a transaction begins rather
	// than when it first writes: a deferred transaction that upgrades gets
	// SQLITE_BUSY immediately, which busy_timeout does not retry.
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(%d)&_pragma=foreign_keys(1)&_txlock=immediate",
		path, busyTimeoutMS)
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db at %s: %w", path, err)
	}
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		fmt.Sprintf("PRAGMA busy_timeout = %d", busyTimeoutMS),
	}
	for _, p := range pragmas {
		if err := gdb.Exec(p).Error; err != nil {
			closeDB(gdb)
			return nil, fmt.Errorf("applying %q: %w", p, err)
		}
	}
	return gdb, nil
}
