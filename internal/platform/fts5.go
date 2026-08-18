// Package platform holds the Windows-native mechanisms the PoC depends on.
// Each file is one mechanism, proved by a build-tagged gate test (see
// openspec/changes/phase-1-windows-platform-gates).
package platform

import (
	"fmt"

	"gorm.io/gorm"
)

// CheckFTS5 exercises the full FTS5 lifecycle — create, insert, MATCH, delete,
// rebuild — on db. It returns an error naming FTS5 if the driver lacks it.
// Phase 2 wires this in as the startup smoke test.
func CheckFTS5(db *gorm.DB) error {
	stmts := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts5_smoke USING fts5(body)`,
		`INSERT INTO fts5_smoke(body) VALUES ('talent hound smoke row'), ('second row')`,
	}
	// Registered before the statements run so a half-created smoke table is
	// still cleaned up.
	defer func() { _ = db.Exec(`DROP TABLE IF EXISTS fts5_smoke`).Error }()
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("fts5 unavailable in this sqlite build (%q): %w", s, err)
		}
	}

	var n int64
	if err := db.Raw(`SELECT count(*) FROM fts5_smoke WHERE fts5_smoke MATCH 'smoke'`).Scan(&n).Error; err != nil {
		return fmt.Errorf("fts5 MATCH query failed: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("fts5 MATCH returned %d rows, want 1", n)
	}
	if err := db.Exec(`DELETE FROM fts5_smoke WHERE body MATCH 'smoke'`).Error; err != nil {
		return fmt.Errorf("fts5 delete failed: %w", err)
	}
	if err := db.Exec(`INSERT INTO fts5_smoke(fts5_smoke) VALUES ('rebuild')`).Error; err != nil {
		return fmt.Errorf("fts5 rebuild failed: %w", err)
	}
	if err := db.Raw(`SELECT count(*) FROM fts5_smoke WHERE fts5_smoke MATCH 'smoke'`).Scan(&n).Error; err != nil {
		return fmt.Errorf("fts5 MATCH after rebuild failed: %w", err)
	}
	if n != 0 {
		return fmt.Errorf("fts5 MATCH after delete+rebuild returned %d rows, want 0", n)
	}
	return nil
}
