package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// theWorks fills a database with one of everything a report could leak, and
// returns the strings that must never appear in one.
func theWorks(t *testing.T, e *setupEnv) []string {
	t.Helper()
	secret := "sk-live-4f9ac2b7e1d84c05" //nolint:gosec // an invented string, not a credential
	name := "Kalinda Reyes"
	email := "kalinda.reyes@example.invalid"
	phone := "+61 400 111 222"
	address := "12 Wattle Street, Fitzroy"
	filename := "Kalinda Reyes CV (final).pdf"
	query := "senior go engineer melbourne quokkastack"
	draft := "Hi Kalinda — your work on quokkastack caught my eye."
	payload := `{"messages":[{"role":"user","content":"` + draft + `"}]}`
	// A control character and a terminal escape, to prove neither survives.
	nasty := "bell\x07escape\x1b[31mred"

	initiatives := NewInitiativeService(e.db)
	initiative, err := initiatives.Create("A search", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating an initiative: %v", err)
	}

	candidate := models.Candidate{
		FullName: name, Emails: models.StringList{email},
		Phones: models.StringList{phone}, Location: address,
	}
	if err := e.db.Create(&candidate).Error; err != nil {
		t.Fatalf("creating a candidate: %v", err)
	}
	artifact := models.Artifact{
		DisplayName: nasty, OriginalFilename: filename, Source: "upload",
		MediaType: "application/pdf", Bytes: []byte(draft), SHA256: "0", ByteLength: int64(len(draft)),
		CapturedAt: time.Now(),
	}
	if err := e.db.Create(&artifact).Error; err != nil {
		t.Fatalf("creating an artifact: %v", err)
	}
	if err := e.db.Create(&models.Search{
		InitiativeID: initiative.ID, Query: query, Provider: "exa", SentAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("creating a search: %v", err)
	}
	if err := e.db.Create(&models.Draft{
		InitiativeID: initiative.ID, CandidateID: &candidate.ID, Body: draft,
		State: models.DraftActive, Kind: "outreach",
	}).Error; err != nil {
		t.Fatalf("creating a draft: %v", err)
	}
	if err := e.db.Create(&models.DisclosureEvent{
		InitiativeID: &initiative.ID, Task: "search", Provider: "exa", OccurredAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("creating an audit event: %v", err)
	}
	if err := e.db.Create(&models.Job{
		Kind: "extract", State: models.JobFailed, FailureReason: models.ReasonSidecarMissing,
	}).Error; err != nil {
		t.Fatalf("creating a job: %v", err)
	}
	return []string{secret, name, email, phone, address, filename, query, draft, payload,
		"\x07", "\x1b", "quokkastack"}
}

func newDiagnostics(t *testing.T) (*DiagnosticsService, *setupEnv) {
	t.Helper()
	e := newSetupEnv(t)
	return NewDiagnosticsService(e.db, e.setup, e.dataDir), e
}

func TestAReportNamesTheVersionsAndTheFolder(t *testing.T) {
	diag, e := newDiagnostics(t)
	report, err := diag.Diagnostics()
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	if report.Version != Version {
		t.Fatalf("version = %q, want %q", report.Version, Version)
	}
	if report.BuildSchema != db.LatestVersion() || report.SchemaVersion != db.LatestVersion() {
		t.Fatalf("schema versions = %d/%d, want %d",
			report.SchemaVersion, report.BuildSchema, db.LatestVersion())
	}
	if report.DataFolder != e.dataDir {
		t.Fatalf("data folder = %q, want %q", report.DataFolder, e.dataDir)
	}
}

func TestAReportOnAnEmptyDatabaseCountsZeroRatherThanFailing(t *testing.T) {
	diag, _ := newDiagnostics(t)
	report, err := diag.Diagnostics()
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	if len(report.Counts) == 0 {
		t.Fatal("the report counted nothing at all")
	}
	for _, c := range report.Counts {
		if c.Count != 0 {
			t.Fatalf("%s counted %d on an empty database", c.Kind, c.Count)
		}
	}
}

// The report is the one artifact built to be shown to someone else, which makes
// it the one place candidate details would leak by design.
func TestAReportContainsNoContentOfAnyKind(t *testing.T) {
	diag, e := newDiagnostics(t)
	forbidden := theWorks(t, e)

	report, err := diag.Diagnostics()
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshalling the report: %v", err)
	}
	text := string(raw)
	for _, secret := range forbidden {
		if strings.Contains(text, secret) {
			t.Fatalf("the report contains %q", secret)
		}
	}
	// It did count what it refused to quote.
	found := false
	for _, c := range report.Counts {
		if c.Kind == "candidates" && c.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("the report did not count the candidate it refused to name")
	}
}

// A stored credential is reported as stored, never as its value.
func TestAReportNeverCarriesAStoredSecret(t *testing.T) {
	diag, e := newDiagnostics(t)
	secret := "sk-live-4f9ac2b7e1d84c05" //nolint:gosec // an invented string, not a credential
	store := newMemoryStore()
	credentials := &CredentialService{store: store}
	if err := credentials.Store("exa", secret); err != nil {
		t.Fatalf("storing the key: %v", err)
	}
	_ = e

	report, err := diag.Diagnostics()
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	raw, _ := json.Marshal(report)
	if strings.Contains(string(raw), secret) {
		t.Fatal("the report contains the stored secret")
	}
}

func TestTheReportIsSafeToPasteAnywhere(t *testing.T) {
	diag, e := newDiagnostics(t)
	theWorks(t, e)
	report, err := diag.Diagnostics()
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	raw, _ := json.Marshal(report)
	for _, r := range string(raw) {
		if r < 0x20 && r != '\n' && r != '\t' {
			t.Fatalf("the report contains control character %q", r)
		}
	}
}

func TestJobOutcomesAreCodes(t *testing.T) {
	diag, e := newDiagnostics(t)
	theWorks(t, e)
	report, err := diag.Diagnostics()
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	if len(report.Jobs) == 0 {
		t.Fatal("no job outcomes were reported")
	}
	// Every part is a short lowercase code: a job outcome cannot carry a
	// sentence, let alone anything a job read.
	for _, j := range report.Jobs {
		for _, part := range strings.Split(j.Kind, ": ") {
			if part != strings.ToLower(part) || strings.ContainsAny(part, " '\"") {
				t.Fatalf("job outcome %q does not look like a code", j.Kind)
			}
		}
	}
}

func TestTheLogsFolderPathIsReportedEvenWhenItCannotBeOpened(t *testing.T) {
	diag, e := newDiagnostics(t)
	want := filepath.Join(e.dataDir, "logs")
	if got := diag.LogsFolder(); got != want {
		t.Fatalf("logs folder = %q, want %q", got, want)
	}
	// Whether the system file manager opens is not this test's business; the
	// path is what the recruiter needs, and it is returned either way.
	dir, _ := diag.OpenLogsFolder()
	if dir != want {
		t.Fatalf("open reported %q, want %q", dir, want)
	}
}

func TestTheRecoveryProcedureNamesTheResolvedFolder(t *testing.T) {
	diag, e := newDiagnostics(t)
	rec := diag.RecoveryProcedure()
	if rec.DataFolder != e.dataDir {
		t.Fatalf("recovery folder = %q, want %q", rec.DataFolder, e.dataDir)
	}
	joined := strings.Join(rec.Steps, "\n")
	if !strings.Contains(joined, e.dataDir) {
		t.Fatal("the procedure did not name the resolved folder")
	}
	for _, want := range []string{"credential", "model", "snapshot"} {
		if !strings.Contains(strings.ToLower(joined), want) {
			t.Fatalf("the procedure does not mention %q", want)
		}
	}
}

func TestDeleteAllRefusesWithoutTheExactFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("data"), 0o600); err != nil {
		t.Fatalf("writing a file: %v", err)
	}
	diag := NewDiagnosticsService(nil, nil, dir)

	for _, wrong := range []string{"", "yes", "delete", filepath.Dir(dir)} {
		target, err := diag.DeleteAll(wrong)
		if err == nil {
			t.Fatalf("%q deleted everything", wrong)
		}
		if !strings.Contains(err.Error(), target) {
			t.Fatalf("the refusal did not name the folder: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatalf("a refused delete removed the file anyway: %v", err)
	}
}

func TestDeleteAllRemovesTheFolderContentsWhenConfirmed(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0o700); err != nil {
		t.Fatalf("creating a subfolder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "talent-hound.db"), []byte("data"), 0o600); err != nil {
		t.Fatalf("writing a file: %v", err)
	}
	diag := NewDiagnosticsService(nil, nil, dir)

	if _, err := diag.DeleteAll(dir); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the folder: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("%d entries survived", len(entries))
	}
}

// A folder is checked before anything writes to it, and each failure has its
// own reason: discovering a read-only volume during a migration is how a
// recruiter finds out their only copy was on one.
func TestAFolderIsRefusedByItsOwnReason(t *testing.T) {
	t.Run("no database", func(t *testing.T) {
		if err := db.CheckFolder(t.TempDir()); !errors.Is(err, db.ErrNotDataFolder) {
			t.Fatalf("err = %v, want ErrNotDataFolder", err)
		}
	})

	t.Run("read-only", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "ro")
		if err := os.Mkdir(dir, 0o500); err != nil { //nolint:gosec // deliberately read-only
			t.Fatalf("creating the folder: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // restoring a test folder so it can be removed
		if err := db.CheckFolder(dir); !errors.Is(err, db.ErrNotWritable) {
			t.Fatalf("err = %v, want ErrNotWritable", err)
		}
	})

	t.Run("a healthy folder passes", func(t *testing.T) {
		dir := t.TempDir()
		gdb, err := db.Open(filepath.Join(dir, db.FileName))
		if err != nil {
			t.Fatalf("opening: %v", err)
		}
		sqlDB, _ := gdb.DB()
		_ = sqlDB.Close()
		if err := db.CheckFolder(dir); err != nil {
			t.Fatalf("a healthy folder was refused: %v", err)
		}
	})
}

// A copied folder opens with its data intact, and the missing credentials and
// models are recovery steps rather than data loss.
func TestACopiedFolderOpensWithItsData(t *testing.T) {
	source := t.TempDir()
	gdb, err := db.Open(filepath.Join(source, db.FileName))
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	if err := gdb.Create(&models.Candidate{FullName: "Nadia Frost"}).Error; err != nil {
		t.Fatalf("creating a candidate: %v", err)
	}
	sqlDB, _ := gdb.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	copied := filepath.Join(t.TempDir(), "copy")
	if err := os.MkdirAll(copied, 0o700); err != nil {
		t.Fatalf("creating the copy: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(source, db.FileName)) // #nosec G304 -- a temp dir this test made
	if err != nil {
		t.Fatalf("reading the database: %v", err)
	}
	if err := os.WriteFile(filepath.Join(copied, db.FileName), raw, 0o600); err != nil {
		t.Fatalf("writing the copy: %v", err)
	}

	if err := db.CheckFolder(copied); err != nil {
		t.Fatalf("the copy was refused: %v", err)
	}
	reopened, err := db.Open(filepath.Join(copied, db.FileName))
	if err != nil {
		t.Fatalf("opening the copy: %v", err)
	}
	var n int64
	if err := reopened.Model(&models.Candidate{}).Count(&n).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if n != 1 {
		t.Fatalf("%d candidates in the copy, want 1", n)
	}
}

func TestACorruptCopyIsRefusedAndNothingIsWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, db.FileName)
	if err := os.WriteFile(path, []byte("this is not a database"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
	before, err := os.ReadFile(path) // #nosec G304 -- a temp dir this test made
	if err != nil {
		t.Fatalf("reading: %v", err)
	}

	if _, err := db.Open(path); err == nil {
		t.Fatal("a corrupt database opened")
	}
	after, err := os.ReadFile(path) // #nosec G304 -- a temp dir this test made
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("the refused database was modified")
	}
}
