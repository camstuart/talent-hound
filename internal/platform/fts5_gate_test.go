//go:build windowsgate

package platform_test

import (
	"path/filepath"
	"testing"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/platform"
)

// TestGateFTS5 proves FTS5 works in the resolved CGO-free SQLite build against
// a disk-backed database. A missing FTS5 must fail this test, never skip it.
func TestGateFTS5(t *testing.T) {
	gdb, err := db.Open(filepath.Join(t.TempDir(), "fts5gate.db"))
	if err != nil {
		t.Fatalf("opening disk-backed db: %v", err)
	}
	t.Cleanup(func() {
		if raw, err := gdb.DB(); err == nil {
			_ = raw.Close() // Windows will not delete an open database file
		}
	})
	if err := platform.CheckFTS5(gdb); err != nil {
		t.Fatalf("FTS5 gate failed: %v", err)
	}
}
