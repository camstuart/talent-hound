package setup

import (
	"os"
	"path/filepath"
	"testing"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// ready is a machine on which every check passes, so each test can spoil
// exactly one thing.
func ready() Checks {
	return Checks{
		DataFolder:   `C:\Talent Hound`,
		Scope:        ScopeReal,
		Encryption:   platform.StatusEncrypted,
		Sidecar:      true,
		Ollama:       true,
		Models:       map[models.ModelRole]bool{models.RoleEmbed: true, models.RoleClassify: true, models.RoleGenerate: true},
		Acknowledged: AcknowledgementVersion,
		Initiatives:  1,
	}
}

func TestTheFirstUnsatisfiedStepIsTheOneToBeOn(t *testing.T) {
	cases := []struct {
		name  string
		spoil func(*Checks)
		want  Step
	}{
		{"fresh install", func(c *Checks) { *c = Checks{} }, StepDataFolder},
		{"no folder", func(c *Checks) { c.DataFolder = "" }, StepDataFolder},
		{"unencrypted", func(c *Checks) { c.Encryption = platform.StatusUnencrypted }, StepEncryption},
		{"check unavailable", func(c *Checks) { c.Encryption = platform.StatusUnavailable }, StepEncryption},
		{"permission denied", func(c *Checks) { c.Encryption = platform.StatusPermissionDenied }, StepEncryption},
		{"no sidecar", func(c *Checks) { c.Sidecar = false }, StepSidecar},
		{"no ollama", func(c *Checks) { c.Ollama = false }, StepOllama},
		{"one model missing", func(c *Checks) { c.Models[models.RoleClassify] = false }, StepModels},
		{"unacknowledged", func(c *Checks) { c.Acknowledged = "" }, StepAcknowledgement},
		{"old acknowledgement", func(c *Checks) { c.Acknowledged = "0" }, StepAcknowledgement},
		{"no initiative", func(c *Checks) { c.Initiatives = 0 }, StepFirstInitiative},
		{"everything done", func(*Checks) {}, StepComplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ready()
			tc.spoil(&c)
			if got := Next(c); got != tc.want {
				t.Fatalf("Next() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A later step is never reached while an earlier one is unsatisfied — checked
// by spoiling two at once and seeing the earlier one win.
func TestAnEarlierUnsatisfiedStepWins(t *testing.T) {
	c := ready()
	c.DataFolder = ""
	c.Ollama = false
	c.Acknowledged = ""
	if got := Next(c); got != StepDataFolder {
		t.Fatalf("Next() = %q, want the data folder step", got)
	}
}

// The state is computed, so a dependency disappearing moves it back rather than
// leaving a stored cursor pointing at a step that no longer holds.
func TestOllamaDisappearingMovesTheStateBack(t *testing.T) {
	c := ready()
	if got := Next(c); got != StepComplete {
		t.Fatalf("setup was not complete to start with: %q", got)
	}
	c.Ollama = false
	if got := Next(c); got != StepOllama {
		t.Fatalf("Next() = %q, want the Ollama step again", got)
	}
}

func TestDemoScopeNeedsNoEncryptedVolume(t *testing.T) {
	c := ready()
	c.Scope = ScopeDemo
	c.Encryption = platform.StatusUnencrypted
	if got := Next(c); got != StepComplete {
		t.Fatalf("Next() = %q, want demo scope to pass the encryption step", got)
	}
}

func TestOnlyAnEncryptedVolumeInRealScopePermitsData(t *testing.T) {
	cases := []struct {
		scope  Scope
		status platform.EncryptionStatus
		want   bool
	}{
		{ScopeReal, platform.StatusEncrypted, true},
		{ScopeReal, platform.StatusUnencrypted, false},
		{ScopeReal, platform.StatusUnavailable, false},
		{ScopeReal, platform.StatusPermissionDenied, false},
		{ScopeDemo, platform.StatusEncrypted, false},
		{ScopeDemo, platform.StatusUnencrypted, false},
	}
	for _, tc := range cases {
		ok, why := RealDataAllowed(tc.scope, tc.status)
		if ok != tc.want {
			t.Fatalf("RealDataAllowed(%q, %q) = %v, want %v", tc.scope, tc.status, ok, tc.want)
		}
		if !ok && why == "" {
			t.Fatalf("RealDataAllowed(%q, %q) refused without a reason", tc.scope, tc.status)
		}
	}
}

// "Could not check" and "not encrypted" are different sentences, because the
// recruiter's next action differs.
func TestTheRefusalDistinguishesUnknownFromUnencrypted(t *testing.T) {
	_, unencrypted := RealDataAllowed(ScopeReal, platform.StatusUnencrypted)
	_, unknown := RealDataAllowed(ScopeReal, platform.StatusUnavailable)
	_, denied := RealDataAllowed(ScopeReal, platform.StatusPermissionDenied)
	if unencrypted == unknown || unknown == denied || unencrypted == denied {
		t.Fatalf("three causes gave the same sentence:\n%s\n%s\n%s", unencrypted, unknown, denied)
	}
}

func TestEveryRoleHasARequiredModelWithASize(t *testing.T) {
	for _, role := range models.ModelRoles {
		found := false
		for _, req := range Required {
			if req.Role != role {
				continue
			}
			found = true
			if req.Model == "" || req.ApproxBytes <= 0 {
				t.Fatalf("role %q has no model name or no size", role)
			}
		}
		if !found {
			t.Fatalf("role %q has no required model", role)
		}
	}
}

func TestSettingsSurviveARestartAndAMissingFileIsAFreshInstall(t *testing.T) {
	dir := t.TempDir()

	fresh, err := Load(dir)
	if err != nil {
		t.Fatalf("loading a missing file: %v", err)
	}
	if fresh.DataFolder != "" || fresh.Acknowledged != "" {
		t.Fatalf("a missing file was not a fresh install: %+v", fresh)
	}
	// Demo is a deliberate choice, so an unset scope is real and the encryption
	// gate decides what is permitted.
	if fresh.Scope != ScopeReal {
		t.Fatalf("unset scope loaded as %q, want %q", fresh.Scope, ScopeReal)
	}

	want := Settings{DataFolder: filepath.Join(dir, "data"), Scope: ScopeDemo, Acknowledged: AcknowledgementVersion}
	if err := Save(dir, want); err != nil {
		t.Fatalf("saving: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if got != want {
		t.Fatalf("settings did not survive: got %+v, want %+v", got, want)
	}
}

// The pointer to the data folder is not inside the data folder: a pointer
// inside the thing it points at cannot be followed.
func TestTheSettingsFileIsNotInsideTheDataFolder(t *testing.T) {
	conf, data := t.TempDir(), t.TempDir()
	if err := Save(conf, Settings{DataFolder: data, Scope: ScopeReal}); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if _, err := os.Stat(SettingsPath(data)); !os.IsNotExist(err) {
		t.Fatalf("the settings file was written into the data folder")
	}
	if _, err := os.Stat(SettingsPath(conf)); err != nil {
		t.Fatalf("the settings file is not in the config folder: %v", err)
	}
}

func TestCatalogCoversEveryRoleAndTheRequiredSet(t *testing.T) {
	byRole := map[models.ModelRole][]CatalogModel{}
	for _, c := range Catalog {
		if c.Model == "" || c.Purpose == "" || c.Power == "" || c.ApproxBytes <= 0 {
			t.Fatalf("catalog entry %+v is missing a field the picker shows", c)
		}
		byRole[c.Role] = append(byRole[c.Role], c)
	}
	for _, role := range models.ModelRoles {
		if len(byRole[role]) == 0 {
			t.Fatalf("no catalog entries for role %s", role)
		}
	}
	for _, req := range Required {
		found := false
		for _, c := range byRole[req.Role] {
			if c.Model == req.Model && c.ApproxBytes == req.ApproxBytes {
				found = true
			}
		}
		if !found {
			t.Fatalf("required model %s for %s is not in the catalog with the same size", req.Model, req.Role)
		}
	}
}
