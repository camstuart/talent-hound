package main

import (
	"camstuart/talent-hound/internal/db"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/extract"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/setup"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// setupEnv is a setup service over a real database, a fake Ollama, and its own
// config and data folders.
type setupEnv struct {
	*modelEnv
	setup   *SetupService
	confDir string
	dataDir string
}

func newSetupEnv(t *testing.T, installed ...string) *setupEnv {
	t.Helper()
	// The stand-in sidecar, so a wizard test is about the wizard rather than
	// about whether this machine has the packaged sidecar installed.
	t.Setenv(extract.SidecarPathEnv, buildFakeSidecar(t))
	base := newModelEnv(t, installed...)
	confDir, dataDir := t.TempDir(), t.TempDir()
	sv, err := NewSetupService(base.db, base.models, confDir, dataDir)
	if err != nil {
		t.Fatalf("creating the setup service: %v", err)
	}
	return &setupEnv{modelEnv: base, setup: sv, confDir: confDir, dataDir: dataDir}
}

// encrypted forces the gate's answer, so a test about scope is not a test about
// this machine's disk.
func (e *setupEnv) encrypted(status platform.EncryptionStatus) {
	e.setup.mu.Lock()
	e.setup.encryption = status
	e.setup.mu.Unlock()
}

func (e *setupEnv) state(t *testing.T) *SetupStatus {
	t.Helper()
	st, err := e.setup.State()
	if err != nil {
		t.Fatalf("reading setup state: %v", err)
	}
	return st
}

func TestAFreshInstallStartsAtTheDataFolder(t *testing.T) {
	e := newSetupEnv(t)
	// A fresh install: nothing chosen, so the pointer is empty.
	e.setup.mu.Lock()
	e.setup.settings.DataFolder = ""
	e.setup.mu.Unlock()

	if got := e.state(t).Next; got != setup.StepDataFolder {
		t.Fatalf("next step = %q, want the data folder", got)
	}
}

func TestChoosingAFolderRecordsItAndItSurvivesARestart(t *testing.T) {
	e := newSetupEnv(t)
	chosen := filepath.Join(t.TempDir(), "Talent Hound")
	if err := e.setup.ChooseFolder(chosen); err != nil {
		t.Fatalf("choosing a folder: %v", err)
	}

	restarted, err := NewSetupService(e.db, e.models, e.confDir, "")
	if err != nil {
		t.Fatalf("restarting: %v", err)
	}
	st, err := restarted.State()
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if st.DataFolder != chosen {
		t.Fatalf("data folder = %q, want %q", st.DataFolder, chosen)
	}
}

func TestAnUnwritableFolderIsRefusedBeforeAnythingIsRecorded(t *testing.T) {
	if runtimeIsWindows() {
		t.Skip("POSIX permission bits; the Windows ACL proof is a gate test")
	}
	e := newSetupEnv(t)
	dir := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(dir, 0o500); err != nil { //nolint:gosec // deliberately read-only
		t.Fatalf("creating the folder: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // restoring a test folder so it can be removed

	if err := e.setup.ChooseFolder(dir); err == nil {
		t.Fatal("an unwritable folder was accepted")
	} else if !strings.Contains(err.Error(), "written to") {
		t.Fatalf("the refusal did not name writability: %v", err)
	}
	if st := e.state(t); st.DataFolder == dir {
		t.Fatal("the refused folder was recorded anyway")
	}
}

// The acknowledgement is required, is recorded with the version of the text
// accepted, and survives a restart.
func TestTheAcknowledgementIsRequiredAndRecordedWithItsVersion(t *testing.T) {
	e := newSetupEnv(t, setup.Required[0].Model, setup.Required[1].Model)
	e.encrypted(platform.StatusEncrypted)
	assignAll(t, e)

	if got := e.state(t).Next; got != setup.StepAcknowledgement {
		t.Fatalf("next step = %q, want the acknowledgement", got)
	}
	if len(e.setup.Acknowledgements()) == 0 {
		t.Fatal("there is no text to acknowledge")
	}
	if err := e.setup.Acknowledge(); err != nil {
		t.Fatalf("acknowledging: %v", err)
	}
	if got := e.state(t).Next; got != setup.StepFirstInitiative {
		t.Fatalf("next step = %q, want the first initiative", got)
	}

	settings, err := setup.Load(e.confDir)
	if err != nil {
		t.Fatalf("loading settings: %v", err)
	}
	if settings.Acknowledged != setup.AcknowledgementVersion {
		t.Fatalf("acknowledgement recorded as %q, want the version", settings.Acknowledged)
	}
}

func TestSetupIsCompleteOnlyAfterTheFirstInitiative(t *testing.T) {
	e := newSetupEnv(t, setup.Required[0].Model, setup.Required[1].Model)
	e.encrypted(platform.StatusEncrypted)
	assignAll(t, e)
	if err := e.setup.Acknowledge(); err != nil {
		t.Fatalf("acknowledging: %v", err)
	}
	if got := e.state(t).Next; got != setup.StepFirstInitiative {
		t.Fatalf("next step = %q, want the first initiative", got)
	}

	initiatives := NewInitiativeService(e.db)
	if _, err := initiatives.Create("A first search", models.InitiativeTypeTalentSearch, nil); err != nil {
		t.Fatalf("creating an initiative: %v", err)
	}
	st := e.state(t)
	if !st.Complete || st.Next != setup.StepComplete {
		t.Fatalf("setup is not complete: %+v", st.Next)
	}
}

// Cancelling is simply not finishing a step: nothing earlier is lost, and the
// state is the first unsatisfied step again.
func TestCancellingLosesNothingEarlier(t *testing.T) {
	e := newSetupEnv(t, setup.Required[0].Model, setup.Required[1].Model)
	e.encrypted(platform.StatusEncrypted)
	assignAll(t, e)
	if err := e.setup.Acknowledge(); err != nil {
		t.Fatalf("acknowledging: %v", err)
	}

	restarted, err := NewSetupService(e.db, e.models, e.confDir, e.dataDir)
	if err != nil {
		t.Fatalf("restarting: %v", err)
	}
	restarted.mu.Lock()
	restarted.encryption = platform.StatusEncrypted
	restarted.mu.Unlock()
	st, err := restarted.State()
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if !st.Acknowledged {
		t.Fatal("the acknowledgement was lost by restarting")
	}
	if st.Next != setup.StepFirstInitiative {
		t.Fatalf("next step = %q, want the first initiative", st.Next)
	}
}

func TestAMissingModelHoldsSetupAtTheModelsStep(t *testing.T) {
	// Only the embedding model is installed.
	e := newSetupEnv(t, setup.Required[0].Model)
	e.encrypted(platform.StatusEncrypted)
	assignAll(t, e)

	st := e.state(t)
	if st.Next != setup.StepModels {
		t.Fatalf("next step = %q, want the models step", st.Next)
	}
	missing := 0
	for _, m := range st.Models {
		if m.ApproxBytes <= 0 {
			t.Fatalf("model %q was listed with no download size", m.Model)
		}
		if !m.Installed {
			missing++
		}
	}
	if missing == 0 {
		t.Fatal("no model was reported missing")
	}
}

// Declining a pull is an answer, not a failure: setup stays where it is and
// nothing earlier is lost.
func TestDecliningAPullLeavesSetupResumable(t *testing.T) {
	e := newSetupEnv(t, setup.Required[0].Model)
	e.encrypted(platform.StatusEncrypted)
	assignAll(t, e)

	if err := e.setup.DeclineModel(models.RoleGenerate); err != nil {
		t.Fatalf("declining: %v", err)
	}
	st := e.state(t)
	if st.Next != setup.StepModels {
		t.Fatalf("next step = %q, want the models step", st.Next)
	}
	if st.DataFolder == "" {
		t.Fatal("declining a pull lost the data folder")
	}
	for _, m := range st.Models {
		if m.Role == models.RoleGenerate && m.State != models.ModelPullDeclined {
			t.Fatalf("the declined role reports %q", m.State)
		}
	}
}

func TestAFailedPullLeavesSetupResumable(t *testing.T) {
	e := newSetupEnv(t, setup.Required[0].Model)
	e.encrypted(platform.StatusEncrypted)
	assignAll(t, e)
	e.fake.set(func(f *fakeOllama) { f.pullErr = "no space left on device" })

	job, err := e.setup.PullModel(models.RoleGenerate)
	if err != nil {
		t.Fatalf("starting the pull: %v", err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobFailed {
		t.Fatalf("the pull job is %s, want failed", done.State)
	}

	st := e.state(t)
	if st.Next != setup.StepModels {
		t.Fatalf("next step = %q, want the models step", st.Next)
	}
	if st.DataFolder == "" {
		t.Fatal("a failed pull lost the data folder")
	}
}

// A dependency disappearing moves the state back, because the state is
// recomputed rather than remembered.
func TestOllamaDisappearingMovesTheWizardBack(t *testing.T) {
	e := newSetupEnv(t, setup.Required[0].Model, setup.Required[1].Model)
	e.encrypted(platform.StatusEncrypted)
	assignAll(t, e)
	if err := e.setup.Acknowledge(); err != nil {
		t.Fatalf("acknowledging: %v", err)
	}

	e.fake.server.Close()
	st := e.state(t)
	if st.Next != setup.StepOllama {
		t.Fatalf("next step = %q, want the Ollama step", st.Next)
	}
	for _, view := range st.Steps {
		if view.Step == setup.StepOllama && !strings.Contains(view.Detail, "Ollama") {
			t.Fatalf("the Ollama step did not name Ollama: %q", view.Detail)
		}
	}
}

func TestARealScopeOnAnUncheckableVolumeRefusesData(t *testing.T) {
	e := newSetupEnv(t)
	for _, status := range []platform.EncryptionStatus{
		platform.StatusUnencrypted, platform.StatusUnavailable, platform.StatusPermissionDenied,
	} {
		e.encrypted(status)
		if err := e.setup.AllowRealData(); err == nil {
			t.Fatalf("%q permitted real data", status)
		}
	}
	e.encrypted(platform.StatusEncrypted)
	if err := e.setup.AllowRealData(); err != nil {
		t.Fatalf("an encrypted volume refused real data: %v", err)
	}
}

// A blocked real scope stays real: switching the recruiter to demo behind their
// back would be a change to what the application holds, made silently.
func TestABlockedRealScopeDoesNotBecomeDemo(t *testing.T) {
	e := newSetupEnv(t)
	e.encrypted(platform.StatusUnencrypted)
	st := e.state(t)
	if st.Scope != setup.ScopeReal {
		t.Fatalf("scope = %q, want it unchanged", st.Scope)
	}
	if st.RealData || st.RealDataWhy == "" {
		t.Fatal("a blocked scope did not say why")
	}
}

func TestDemoScopeRefusesArtifactsAndCandidates(t *testing.T) {
	e := newSetupEnv(t)
	if err := e.setup.SetScope(setup.ScopeDemo); err != nil {
		t.Fatalf("choosing demo scope: %v", err)
	}
	records := NewRecordService(e.db)
	records.Guard = e.setup
	artifacts := NewArtifactService(e.db)
	artifacts.Guard = e.setup

	if _, err := records.CreateCandidate(models.Candidate{FullName: "Nadia Frost"}); err == nil {
		t.Fatal("demo scope accepted a candidate")
	}
	var candidates int64
	if err := e.db.Model(&models.Candidate{}).Count(&candidates).Error; err != nil {
		t.Fatalf("counting candidates: %v", err)
	}
	if candidates != 0 {
		t.Fatalf("%d candidates were stored in demo scope", candidates)
	}

	if _, err := artifacts.create("notes", "notes.md", "test", []byte("# notes\n"),
		models.LinkInitiative, 1); err == nil {
		t.Fatal("demo scope accepted an artifact")
	}
	var stored int64
	if err := e.db.Model(&models.Artifact{}).Count(&stored).Error; err != nil {
		t.Fatalf("counting artifacts: %v", err)
	}
	if stored != 0 {
		t.Fatalf("%d artifacts were stored in demo scope", stored)
	}
}

// The refusal is at the write, not in the interface: calling the service
// directly is refused identically.
func TestTheScopeRefusalIsAtTheWrite(t *testing.T) {
	e := newSetupEnv(t)
	if err := e.setup.SetScope(setup.ScopeDemo); err != nil {
		t.Fatalf("choosing demo scope: %v", err)
	}
	records := NewRecordService(e.db)
	records.Guard = e.setup
	if _, err := records.CreateCandidate(models.Candidate{FullName: "Nadia Frost"}); err == nil {
		t.Fatal("a direct call was accepted")
	}
}

func TestAnUnknownScopeIsRefused(t *testing.T) {
	e := newSetupEnv(t)
	if err := e.setup.SetScope("everything"); err == nil {
		t.Fatal("an unknown scope was accepted")
	}
}

// assignAll assigns every role a model, so the wizard is past the registry's
// unassigned state and the missing-model case is about the model, not the
// assignment.
func assignAll(t *testing.T, e *setupEnv) {
	t.Helper()
	for _, req := range setup.Required {
		in := AssignInput{Role: req.Role, Model: req.Model, Endpoint: e.fake.server.URL}
		if _, err := e.models.Assign(in); err != nil {
			t.Fatalf("assigning %q: %v", req.Role, err)
		}
	}
}

// The encryption gate judges the folder the data is in, not the one that was
// chosen for later.
//
// Choosing a folder records a preference the next launch opens the database in.
// Until then records go where they were already going. The gate used to check
// the chosen folder, so a recruiter could point the wizard at an encrypted disk,
// be told real data was allowed, and have every record written to the folder
// this process had actually opened — which is the thing the gate exists to
// prevent.
func TestTheEncryptionGateJudgesTheFolderInUse(t *testing.T) {
	e := newSetupEnv(t)
	chosen := t.TempDir()
	svc := e.setup

	asked := ""
	svc.checkEncryption = func(_ context.Context, path string) platform.EncryptionStatus {
		asked = path
		return platform.StatusEncrypted
	}
	if err := svc.ChooseFolder(chosen); err != nil {
		t.Fatalf("choosing: %v", err)
	}

	if asked != e.dataDir {
		t.Fatalf("the gate was asked about %q; the database is in %q", asked, e.dataDir)
	}
	if asked == chosen {
		t.Fatal("the gate judged the folder that was chosen for later, not the one in use")
	}

	state, err := svc.State()
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	// The choice is recorded and reported, because the recruiter made it.
	if state.DataFolder != chosen {
		t.Fatalf("the chosen folder is %q, want %q", state.DataFolder, chosen)
	}
	// And the folder the gate was asked about is the one being written to.

}

// Personal-data entry is refused wherever it happens, not only where a record
// is created.
//
// The guard was on creating a candidate and on artifacts. It was not on editing
// a candidate, and it was not on contacts at all — and a contact is a person:
// a full name, an email address and a phone number, which is the one record
// type made entirely of direct identifiers.
//
// The gate can also turn on after the records exist. A volume stops being
// encrypted, or the recruiter moves to demo scope, and every candidate already
// in the database was an open field for typing a real name into.
func TestEveryPersonalRecordIsRefusedWhenTheGateIsClosed(t *testing.T) {
	e := newSetupEnv(t)
	e.encrypted(platform.StatusEncrypted) // open the gate deliberately, not by this machine's disk
	records := NewRecordService(e.db)
	records.Guard = e.setup

	// While the gate is open: a candidate and a contact, so there is something
	// to try to edit afterwards.
	candidate, err := records.CreateCandidate(models.Candidate{FullName: "Nadia Frost"})
	if err != nil {
		t.Fatalf("creating a candidate: %v", err)
	}
	company, err := records.CreateCompany(models.Company{Name: "Quokkastack"})
	if err != nil {
		t.Fatalf("creating a company: %v", err)
	}
	contact, err := records.CreateContact(models.Contact{
		CompanyID: company.ID, FullName: "Tobias Fenn", Email: "tobias.fenn@example.invalid",
	})
	if err != nil {
		t.Fatalf("creating a contact: %v", err)
	}
	// And editing works while it is open, so what follows is the gate closing
	// rather than the method being broken.
	contact.Title = "Head of Engineering"
	updated, err := records.UpdateContact(*contact)
	if err != nil {
		t.Fatalf("editing a contact with the gate open: %v", err)
	}
	if updated.Title != "Head of Engineering" {
		t.Fatalf("the edit did not take: %+v", updated)
	}

	// The gate closes.
	if err := e.setup.SetScope(setup.ScopeDemo); err != nil {
		t.Fatalf("choosing demo scope: %v", err)
	}

	if _, err := records.CreateContact(models.Contact{
		CompanyID: company.ID, FullName: "Priya Raman", Phone: "+61 400 123 456",
	}); err == nil {
		t.Fatal("a contact — a name, an email and a phone number — was accepted with the gate closed")
	}
	contact.FullName = "Someone Real"
	if _, err := records.UpdateContact(*contact); err == nil {
		t.Fatal("a contact was edited with the gate closed")
	}
	candidate.FullName = "Someone Real"
	if _, err := records.UpdateCandidate(*candidate); err == nil {
		t.Fatal("a candidate was edited with the gate closed")
	}

	// A company is an organization rather than a person, and naming one is
	// ordinary recruiting — the same distinction the search and cloud
	// boundaries draw. It stays allowed, deliberately.
	if _, err := records.CreateCompany(models.Company{Name: "Fernway Health"}); err != nil {
		t.Fatalf("a company was refused with the gate closed: %v", err)
	}
}

// The recruiter acknowledges six things at first run, and the PRD names all
// six.
//
// The existing check is that the list is not empty, which a list of one
// satisfies. These are the data-handling preconditions the recruiter agrees to
// before any candidate data is loaded — authority, retention, the use of public
// information, what a search discloses, what a cloud task discloses, and the
// prohibited-criteria boundary. One quietly dropped is a thing they were never
// asked, and nobody would notice from the screen.
func TestTheRecruiterAcknowledgesEverythingThePRDNames(t *testing.T) {
	e := newSetupEnv(t)
	terms := e.setup.Acknowledgements()
	if len(terms) != 6 {
		t.Fatalf("%d acknowledgements, and the PRD lists six", len(terms))
	}

	// One phrase per precondition, in the PRD's own terms.
	for _, subject := range []string{
		"authority",     // authority to hold and use the data
		"deleting",      // retention and deletion responsibilities
		"republication", // evaluation and outreach, not republication
		"previewed",     // searches disclose, and are previewed
		"cloud",         // cloud tasks have separate consent
		"prohibited",    // the prohibited-criteria boundary
	} {
		found := false
		for _, term := range terms {
			if strings.Contains(strings.ToLower(term), subject) {
				found = true
			}
		}
		if !found {
			t.Errorf("no acknowledgement mentions %q, which the PRD requires", subject)
		}
	}

	// And each is a sentence the recruiter can read, not a code.
	for _, term := range terms {
		if len(term) < 20 || !strings.HasSuffix(term, ".") {
			t.Errorf("%q does not read as something a person agrees to", term)
		}
	}
}

// Choosing a folder that already holds a database says now whether it can be
// opened.
//
// "Selecting a previously copied data folder SHALL run the integrity and
// schema-version checks … and open only if every check passes." The checks ran
// when the database was opened, which is the next launch — so choosing a copy
// whose file was truncated by whatever interrupted the copy said nothing, and
// the application then failed to start. That is the worst moment to learn it.
func TestChoosingACopiedFolderSaysWhetherItCanBeOpened(t *testing.T) {
	e := newSetupEnv(t)

	t.Run("an empty folder is a new installation", func(t *testing.T) {
		if err := e.setup.ChooseFolder(t.TempDir()); err != nil {
			t.Fatalf("an empty folder was refused: %v", err)
		}
	})

	t.Run("a folder holding a database is checked", func(t *testing.T) {
		good := t.TempDir()
		gdb, err := db.Open(filepath.Join(good, db.FileName))
		if err != nil {
			t.Fatalf("creating a data folder: %v", err)
		}
		closeOnCleanup(t, gdb)
		if raw, err := gdb.DB(); err == nil {
			_ = raw.Close()
		}
		if err := e.setup.ChooseFolder(good); err != nil {
			t.Fatalf("a real data folder was refused: %v", err)
		}
	})

	t.Run("a truncated database is refused, by its reason", func(t *testing.T) {
		broken := t.TempDir()
		if err := os.WriteFile(filepath.Join(broken, db.FileName), []byte{}, 0o600); err != nil {
			t.Fatalf("writing the empty file: %v", err)
		}
		err := e.setup.ChooseFolder(broken)
		if err == nil {
			t.Fatal("a folder holding an empty database file was accepted")
		}
		if !strings.Contains(err.Error(), "cannot be opened") {
			t.Fatalf("the refusal does not say what happened: %v", err)
		}
	})
}

// The state says which folder the encryption answer is about.
//
// Choosing a folder records where the next launch opens the database; until
// then the records go where they are already going, and the gate judges that
// one. So between choosing and restarting, the screen would otherwise show the
// folder they picked beside an encryption result for a different folder — two
// true things that read as one false one.
func TestTheStateSaysWhichFolderIsInUse(t *testing.T) {
	e := newSetupEnv(t)

	before, err := e.setup.State()
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if before.FolderInUse != e.dataDir {
		t.Fatalf("the state says %q is in use, and the database is in %q",
			before.FolderInUse, e.dataDir)
	}
	if before.DataFolder != before.FolderInUse {
		t.Fatal("they differ before anything was chosen")
	}

	chosen := t.TempDir()
	if err := e.setup.ChooseFolder(chosen); err != nil {
		t.Fatalf("choosing: %v", err)
	}
	after, err := e.setup.State()
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if after.DataFolder != chosen {
		t.Fatalf("the chosen folder is %q, want %q", after.DataFolder, chosen)
	}
	if after.FolderInUse != e.dataDir {
		t.Fatalf("the folder in use changed to %q without a restart", after.FolderInUse)
	}
}

func TestOllamaReachableNamesTheEndpointInUse(t *testing.T) {
	views := []ModelView{{State: models.ModelEndpointDown}}
	_, why := ollamaReachable(views, "http://127.0.0.1:11435")
	if !strings.Contains(why, "http://127.0.0.1:11435") {
		t.Fatalf("the message must name the endpoint in use, got %q", why)
	}
}
