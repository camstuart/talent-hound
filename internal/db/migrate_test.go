package db

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// broken returns the real migration list plus a deliberately invalid one, for
// failure injection.
func broken() []migration {
	return append(append([]migration{}, migrations...), migration{
		Version: latestVersion(migrations) + 1,
		Name:    "deliberately_broken",
		SQL: []string{
			"CREATE TABLE canary (id integer)",
			"THIS IS NOT SQL",
		},
	})
}

func dbPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "talent-hound.db")
}

func mustOpen(t *testing.T, path string) *gorm.DB {
	t.Helper()
	gdb, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%s): %v", path, err)
	}
	t.Cleanup(func() { closeDB(gdb) })
	return gdb
}

func version(t *testing.T, gdb *gorm.DB) int {
	t.Helper()
	v, err := schemaVersion(gdb)
	if err != nil {
		t.Fatalf("schemaVersion: %v", err)
	}
	return v
}

func schemaOf(t *testing.T, gdb *gorm.DB) []string {
	t.Helper()
	var sql []string
	if err := gdb.Raw("SELECT sql FROM sqlite_master WHERE sql IS NOT NULL ORDER BY name").Scan(&sql).Error; err != nil {
		t.Fatalf("reading schema: %v", err)
	}
	return sql
}

func hash(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path) // #nosec G304 -- test-controlled temp path
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return sha256.Sum256(b)
}

func TestOpenNewDatabaseMigratesToCurrent(t *testing.T) {
	path := dbPath(t)
	gdb := mustOpen(t, path)

	if got, want := version(t, gdb), latestVersion(migrations); got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}
	if err := gdb.Create(&models.Initiative{Name: "n", Type: models.InitiativeTypeJobSearch}).Error; err != nil {
		t.Fatalf("insert into migrated schema: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "snapshots")); !os.IsNotExist(err) {
		t.Fatalf("a brand-new database should not be snapshotted (stat err = %v)", err)
	}
}

func TestReopenIsIdempotent(t *testing.T) {
	path := dbPath(t)
	first := mustOpen(t, path)
	before := schemaOf(t, first)
	closeDB(first)

	second := mustOpen(t, path)
	if got, want := version(t, second), latestVersion(migrations); got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}
	after := schemaOf(t, second)
	if len(before) != len(after) {
		t.Fatalf("schema changed on reopen:\n%v\n%v", before, after)
	}
	if _, err := os.Stat(snapshotPath(path, latestVersion(migrations))); !os.IsNotExist(err) {
		t.Fatalf("reopening a current database should not snapshot (stat err = %v)", err)
	}
}

// A database at version 0 with rows in it — the shape every existing developer
// and E2E database has — must be adopted, not recreated.
func TestAutoMigrateShapedDatabaseIsAdopted(t *testing.T) {
	path := dbPath(t)
	legacy, err := openRaw(path)
	if err != nil {
		t.Fatalf("openRaw: %v", err)
	}
	// The literal DDL AutoMigrate emitted, seeded with raw SQL: the current
	// models have moved on, and this fixture must stay the old shape forever.
	for _, s := range []string{
		"CREATE TABLE `initiatives` (`id` integer PRIMARY KEY AUTOINCREMENT,`name` text NOT NULL," +
			"`type` text NOT NULL,`created_at` datetime,`updated_at` datetime)",
		"CREATE INDEX `idx_initiatives_type` ON `initiatives`(`type`)",
		"INSERT INTO `initiatives` (`name`,`type`) VALUES ('legacy','talent_search')",
	} {
		if err := legacy.Exec(s).Error; err != nil {
			t.Fatalf("seeding legacy database (%q): %v", s, err)
		}
	}
	closeDB(legacy)

	gdb := mustOpen(t, path)
	if got, want := version(t, gdb), latestVersion(migrations); got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}
	var got models.Initiative
	if err := gdb.First(&got, "name = ?", "legacy").Error; err != nil {
		t.Fatalf("legacy row lost: %v", err)
	}

	fresh := mustOpen(t, dbPath(t))
	adopted, created := schemaOf(t, gdb), schemaOf(t, fresh)
	if len(adopted) != len(created) {
		t.Fatalf("adopted schema differs from a fresh one:\n%v\n%v", adopted, created)
	}
	for i := range adopted {
		if adopted[i] != created[i] {
			t.Errorf("schema object %d differs:\n adopted: %s\n fresh:   %s", i, adopted[i], created[i])
		}
	}
}

func TestHistoricalVersionMigratesForwardWithRowsIntact(t *testing.T) {
	// The list only has v1 today, so simulate a future build: open with an
	// extra migration, then reopen with it to prove a v1 database with rows
	// steps forward without losing them.
	future := append(append([]migration{}, migrations...), migration{
		Version: latestVersion(migrations) + 1,
		Name:    "add_note_column",
		SQL:     []string{"ALTER TABLE initiatives ADD COLUMN note text"},
	})

	path := dbPath(t)
	old := mustOpen(t, path)
	if err := old.Create(&models.Initiative{Name: "kept", Type: models.InitiativeTypeJobSearch}).Error; err != nil {
		t.Fatalf("seeding: %v", err)
	}
	closeDB(old)

	gdb, err := open(path, future)
	if err != nil {
		t.Fatalf("migrating forward: %v", err)
	}
	defer closeDB(gdb)

	if got, want := version(t, gdb), latestVersion(future); got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}
	var n int64
	if err := gdb.Model(&models.Initiative{}).Where("name = ?", "kept").Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("row count = %d (err %v), want 1", n, err)
	}
	if err := gdb.Exec("UPDATE initiatives SET note = 'x'").Error; err != nil {
		t.Fatalf("new column missing: %v", err)
	}
	if _, err := os.Stat(snapshotPath(path, latestVersion(migrations))); err != nil {
		t.Fatalf("expected a pre-migration snapshot: %v", err)
	}
}

func TestFutureVersionIsRejectedWithoutWrites(t *testing.T) {
	path := dbPath(t)
	ahead, err := open(path, broken()[:1])
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if err := ahead.Exec("PRAGMA user_version = 999").Error; err != nil {
		t.Fatalf("bumping version: %v", err)
	}
	closeDB(ahead)
	before := hash(t, path)

	gdb, err := Open(path)
	if err == nil {
		closeDB(gdb)
		t.Fatal("Open of a future-version database succeeded, want failure")
	}
	if !errors.Is(err, ErrFutureSchema) {
		t.Fatalf("error = %v, want ErrFutureSchema", err)
	}
	if hash(t, path) != before {
		t.Fatal("the database file was modified while rejecting a future version")
	}
}

func TestFailingMigrationRestoresSnapshot(t *testing.T) {
	path := dbPath(t)
	seed := mustOpen(t, path)
	if err := seed.Create(&models.Initiative{Name: "survivor", Type: models.InitiativeTypeJobSearch}).Error; err != nil {
		t.Fatalf("seeding: %v", err)
	}
	closeDB(seed)

	gdb, err := open(path, broken())
	if err == nil {
		closeDB(gdb)
		t.Fatal("broken migration succeeded, want failure")
	}
	if !errors.Is(err, ErrMigration) {
		t.Fatalf("error = %v, want ErrMigration", err)
	}
	if errors.Is(err, ErrRestore) {
		t.Fatalf("restore itself failed: %v", err)
	}

	// The restored file must be usable, at the pre-migration version, with the
	// row intact and no trace of the failed migration.
	after := mustOpen(t, path)
	if got, want := version(t, after), latestVersion(migrations); got != want {
		t.Fatalf("restored schema version = %d, want %d", got, want)
	}
	if err := integrityCheck(after); err != nil {
		t.Fatalf("restored database fails integrity check: %v", err)
	}
	var n int64
	if err := after.Model(&models.Initiative{}).Where("name = ?", "survivor").Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("row count = %d (err %v), want 1", n, err)
	}
	var tables []string
	if err := after.Raw("SELECT name FROM sqlite_master WHERE name = 'canary'").Scan(&tables).Error; err != nil {
		t.Fatalf("checking for partial schema: %v", err)
	}
	if len(tables) != 0 {
		t.Fatal("the failed migration left a partially applied schema")
	}
}

func TestSnapshotFailureAbortsBeforeMigrating(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "talent-hound.db")
	seed := mustOpen(t, path)
	closeDB(seed)

	// Block snapshot creation without making the database itself unopenable:
	// occupy the snapshots directory name with a regular file.
	if err := os.WriteFile(filepath.Join(dir, "snapshots"), []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("blocking snapshot directory: %v", err)
	}

	future := append(append([]migration{}, migrations...), migration{
		Version: latestVersion(migrations) + 1,
		Name:    "add_note_column",
		SQL:     []string{"ALTER TABLE initiatives ADD COLUMN note text"},
	})
	gdb, err := open(path, future)
	if err == nil {
		closeDB(gdb)
		t.Fatal("Open succeeded with an unwritable snapshot directory, want failure")
	}
	if !errors.Is(err, ErrSnapshot) {
		t.Fatalf("error = %v, want ErrSnapshot", err)
	}

	if err := os.Remove(filepath.Join(dir, "snapshots")); err != nil {
		t.Fatalf("unblocking snapshot directory: %v", err)
	}
	check := mustOpen(t, path)
	if got, want := version(t, check), latestVersion(migrations); got != want {
		t.Fatalf("version = %d after aborted migration, want %d", got, want)
	}
}

func TestCorruptDatabaseIsRefused(t *testing.T) {
	path := dbPath(t)
	seed := mustOpen(t, path)
	for i := 0; i < 200; i++ {
		if err := seed.Create(&models.Initiative{Name: "pad", Type: models.InitiativeTypeJobSearch}).Error; err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}
	closeDB(seed)

	// Keep the header and the schema pages intact so the file still opens as a
	// database, and scribble over the data pages in its back half. Corrupting
	// the schema itself fails the connection instead, which is a different
	// (also safe) refusal than the one under test.
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0o600) // #nosec G304 -- test temp path
	if err != nil {
		t.Fatalf("opening for corruption: %v", err)
	}
	// The last page is a leaf of the initiatives table: enough for
	// PRAGMA integrity_check to object, without breaking the schema pages the
	// connection itself has to read.
	from := st.Size() - 1024
	garbage := make([]byte, st.Size()-from)
	for i := range garbage {
		garbage[i] = byte(i%251 + 1)
	}
	if _, err := f.WriteAt(garbage, from); err != nil {
		t.Fatalf("corrupting: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	gdb, err := Open(path)
	if err == nil {
		closeDB(gdb)
		t.Fatal("Open of a corrupt database succeeded, want failure")
	}
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("error = %v, want ErrIntegrity", err)
	}
}

// An interrupted migration leaves the file at the pre-migration version (the
// transaction never committed); the next open re-applies it cleanly.
func TestInterruptedMigrationReappliesCleanly(t *testing.T) {
	path := dbPath(t)
	seed := mustOpen(t, path)
	closeDB(seed)

	future := append(append([]migration{}, migrations...), migration{
		Version: latestVersion(migrations) + 1,
		Name:    "add_note_column",
		SQL:     []string{"ALTER TABLE initiatives ADD COLUMN note text"},
	})

	// Simulate the interruption: apply the migration's statements in a
	// transaction that is rolled back instead of committed.
	interrupted, err := openRaw(path)
	if err != nil {
		t.Fatalf("openRaw: %v", err)
	}
	tx := interrupted.Begin()
	if err := tx.Exec(future[len(future)-1].SQL[0]).Error; err != nil {
		t.Fatalf("in-flight statement: %v", err)
	}
	tx.Rollback()
	closeDB(interrupted)

	gdb, err := open(path, future)
	if err != nil {
		t.Fatalf("reopening after interruption: %v", err)
	}
	defer closeDB(gdb)
	if got, want := version(t, gdb), latestVersion(future); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
	if err := gdb.Exec("UPDATE initiatives SET note = 'x'").Error; err != nil {
		t.Fatalf("re-applied migration incomplete: %v", err)
	}
}

func TestFTS5SmokeTestLeavesNoResidue(t *testing.T) {
	gdb := mustOpen(t, dbPath(t))
	var names []string
	if err := gdb.Raw("SELECT name FROM sqlite_master WHERE name LIKE 'fts5_smoke%'").Scan(&names).Error; err != nil {
		t.Fatalf("querying schema: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("FTS5 smoke test left %v behind", names)
	}
}

func TestConcurrentWritersDoNotCorrupt(t *testing.T) {
	path := dbPath(t)
	a := mustOpen(t, path)
	b := mustOpen(t, path)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, conn := range []*gorm.DB{a, b} {
		wg.Add(1)
		go func(g *gorm.DB) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if err := g.Create(&models.Initiative{Name: "c", Type: models.InitiativeTypeJobSearch}).Error; err != nil {
					errs <- err
					return
				}
			}
		}(conn)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		// A busy timeout expiry is defined behaviour; silent corruption is not.
		t.Logf("concurrent write returned: %v", err)
	}
	if err := integrityCheck(a); err != nil {
		t.Fatalf("integrity after concurrent writes: %v", err)
	}
}

func TestDefaultPathHonoursOverrideAndIsStable(t *testing.T) {
	want := filepath.Join(t.TempDir(), "nested", "custom.db")
	t.Setenv("TALENT_HOUND_DB_PATH", want)

	first, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	second, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath (second): %v", err)
	}
	if first != want || second != want {
		t.Fatalf("DefaultPath = %q/%q, want %q", first, second, want)
	}
	st, err := os.Stat(filepath.Dir(want))
	if err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o700 {
		t.Fatalf("parent directory mode = %v, want 0700", st.Mode().Perm())
	}
}

func TestOpenInMemoryStillMigrates(t *testing.T) {
	gdb := mustOpen(t, ":memory:")
	if got, want := version(t, gdb), latestVersion(migrations); got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}
	if err := gdb.Create(&models.Initiative{Name: "mem", Type: models.InitiativeTypeJobSearch}).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
}
