// Package db opens the application's SQLite database (via the pure-Go,
// CGO-free driver) and keeps its schema migrated.
package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"camstuart/talent-hound/internal/models"
)

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

// Open opens (creating if necessary) the SQLite database at path and runs
// schema migrations. Use ":memory:" for an ephemeral in-memory database.
func Open(path string) (*gorm.DB, error) {
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db at %s: %w", path, err)
	}
	if err := gdb.AutoMigrate(&models.Initiative{}); err != nil {
		return nil, fmt.Errorf("migrating schema: %w", err)
	}
	return gdb, nil
}
