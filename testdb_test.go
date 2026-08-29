package main

import (
	"testing"

	"gorm.io/gorm"
)

// closeOnCleanup releases the database's connection pool when the test ends.
// On Windows an open SQLite handle keeps the file locked, and t.TempDir's
// cleanup then fails the test even though every assertion passed; macOS and
// Linux happily unlink an open file, which is why this was never noticed there.
func closeOnCleanup(t testing.TB, gdb *gorm.DB) {
	t.Helper()
	t.Cleanup(func() {
		if raw, err := gdb.DB(); err == nil {
			_ = raw.Close()
		}
	})
}
