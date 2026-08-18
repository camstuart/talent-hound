package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
)

// FileName is the database file inside a data folder, so recovery can look for
// the folder's contents rather than asking the recruiter for a file.
const FileName = "talent-hound.db"

// ErrNotDataFolder is a folder that holds no Talent Hound database.
var ErrNotDataFolder = errors.New("this folder holds no Talent Hound database")

// ErrNotWritable is a folder that cannot be written to.
//
// It is checked before anything else, because discovering it during a migration
// is how a recruiter finds out their only copy was on a read-only volume.
var ErrNotWritable = errors.New("this folder cannot be written to")

// SchemaVersion reports the schema version recorded in the database.
func SchemaVersion(gdb *gorm.DB) (int, error) { return schemaVersion(gdb) }

// LatestVersion reports the schema version this build knows.
func LatestVersion() int { return latestVersion(migrations) }

// CheckFolder verifies a folder can be opened as a data folder before any check
// that would write to it. It reports the first failure by its own name rather
// than a single "cannot open".
func CheckFolder(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("the folder cannot be read: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: it is a file, not a folder", ErrNotDataFolder)
	}
	probe := filepath.Join(dir, ".th-write-check")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("%w: %w", ErrNotWritable, err)
	}
	if err := os.Remove(probe); err != nil {
		return fmt.Errorf("%w: %w", ErrNotWritable, err)
	}
	if st, err := os.Stat(filepath.Join(dir, FileName)); err != nil || st.Size() == 0 {
		return ErrNotDataFolder
	}
	return nil
}
