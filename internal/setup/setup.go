// Package setup holds the first-run step order, the remembered pointer to the
// data folder, and the table of models a working installation needs.
//
// Nothing here talks to the machine. The service gathers what is true and this
// package decides what that means, so the wizard's behaviour is a table test
// rather than an integration test.
package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// Step is one first-run step. The order is the PRD's order.
type Step string

// The steps, in the PRD's order.
const (
	StepDataFolder      Step = "data_folder"
	StepEncryption      Step = "encryption"
	StepSidecar         Step = "sidecar"
	StepOllama          Step = "ollama"
	StepModels          Step = "models"
	StepAcknowledgement Step = "acknowledgement"
	StepFirstInitiative Step = "first_initiative"
	// StepComplete is not a step: it is what is left when none remain.
	StepComplete Step = "complete"
)

// Order is every step, in order, so the interface and the tests walk the same
// list.
var Order = []Step{
	StepDataFolder, StepEncryption, StepSidecar, StepOllama,
	StepModels, StepAcknowledgement, StepFirstInitiative,
}

// Scope is what the recruiter may do with this installation.
type Scope string

const (
	// ScopeReal is real candidate data, and requires an encrypted volume.
	ScopeReal Scope = "real"
	// ScopeDemo is an empty workspace on any volume. It refuses real content,
	// and the PRD is explicit that it is not an acceptance environment.
	ScopeDemo Scope = "demo"
)

// Valid reports whether s is one of the two scopes.
func (s Scope) Valid() bool { return s == ScopeReal || s == ScopeDemo }

// AcknowledgementVersion identifies the data-handling text that was accepted.
// A changed text is a new acknowledgement, not a remembered old one.
const AcknowledgementVersion = "1"

// UnencryptedAcceptanceVersion identifies the warning text accepted when the
// recruiter chooses to hold candidate data without disk encryption. A changed
// warning is a new acceptance.
const UnencryptedAcceptanceVersion = "1"

// Checks is what is true about this machine right now. Every field is gathered
// fresh: a remembered fact about a machine is a fact about the machine as it
// was.
type Checks struct {
	DataFolder   string                    `json:"dataFolder"`
	Scope        Scope                     `json:"scope"`
	Encryption   platform.EncryptionStatus `json:"encryption"`
	Sidecar      bool                      `json:"sidecar"`
	SidecarWhy   string                    `json:"sidecarWhy"`
	Ollama       bool                      `json:"ollama"`
	OllamaWhy    string                    `json:"ollamaWhy"`
	Models       map[models.ModelRole]bool `json:"models"`
	Acknowledged string                    `json:"acknowledged"`
	// UnencryptedAccepted is the acceptance version the recruiter recorded
	// for holding data without encryption, or empty.
	UnencryptedAccepted string `json:"unencryptedAccepted"`
	Initiatives         int64  `json:"initiatives"`
}

// Next returns the first unsatisfied step, or StepComplete.
//
// It is recomputed on every call rather than stored. A stored cursor is wrong
// the moment reality moves underneath it — Ollama uninstalled, a model deleted
// to free space, the folder moved to a USB stick.
func Next(c Checks) Step {
	switch {
	case c.DataFolder == "":
		return StepDataFolder
	// Demo scope is allowed on any volume. Real scope needs an encrypted
	// volume, or the recruiter's recorded acceptance of storing without one
	// — a choice made in the open, never an unencrypted volume tolerated
	// quietly.
	case c.Scope != ScopeDemo && c.Encryption != platform.StatusEncrypted &&
		c.UnencryptedAccepted != UnencryptedAcceptanceVersion:
		return StepEncryption
	case !c.Sidecar:
		return StepSidecar
	case !c.Ollama:
		return StepOllama
	case !modelsReady(c.Models):
		return StepModels
	case c.Acknowledged != AcknowledgementVersion:
		return StepAcknowledgement
	case c.Initiatives == 0:
		return StepFirstInitiative
	}
	return StepComplete
}

func modelsReady(present map[models.ModelRole]bool) bool {
	for _, role := range models.ModelRoles {
		if !present[role] {
			return false
		}
	}
	return true
}

// RealDataAllowed reports whether personal data may be held, and why not when
// it may not. "Could not check" and "not encrypted" are different sentences to
// the recruiter and the same answer here — unless the recruiter has accepted
// holding data without encryption, in which case the answer is yes and the
// sentence is a reminder rather than a refusal.
//
// The acceptance exists because most recruiters cannot turn disk encryption
// on, and an application they cannot use protects nobody. It is recorded,
// versioned, and shown, so it is a decision rather than a default.
func RealDataAllowed(scope Scope, status platform.EncryptionStatus, unencryptedAccepted bool) (bool, string) {
	if scope != ScopeReal {
		return false, "this installation is in demo scope: it holds no candidate data"
	}
	switch status {
	case platform.StatusEncrypted:
		return true, ""
	}
	if unencryptedAccepted {
		return true, UnencryptedWarning
	}
	switch status {
	case platform.StatusUnencrypted:
		return false, "the volume holding the data folder is not encrypted"
	case platform.StatusPermissionDenied:
		return false, "the encryption check needs permission this application does not have, " +
			"so the volume cannot be confirmed as encrypted"
	default:
		return false, "the encryption state of the volume could not be checked, " +
			"so it is not treated as encrypted"
	}
}

// UnencryptedWarning is what the recruiter accepts, and what they are reminded
// of afterwards. Version UnencryptedAcceptanceVersion.
const UnencryptedWarning = "candidate data is stored without disk encryption, by your choice: " +
	"anyone with this computer can read it"

// RequiredModel is one role's recommended model and its approximate download
// size, so the recruiter sees what a working installation costs before it
// starts downloading.
type RequiredModel struct {
	Role        models.ModelRole `json:"role"`
	Model       string           `json:"model"`
	ApproxBytes int64            `json:"approxBytes"`
}

// Required is the PoC's recommended local set: roughly 6 GB in total, inside
// the PRD's 4–8 GB envelope.
//
// ponytail: a static table. Ollama's registry knows the real sizes, and asking
// it means a network call during a step whose whole point is telling the
// recruiter what they are about to download before anything is downloaded.
var Required = []RequiredModel{
	{Role: models.RoleEmbed, Model: "nomic-embed-text", ApproxBytes: 274 << 20},
	{Role: models.RoleClassify, Model: "qwen2.5:7b-instruct", ApproxBytes: 4700 << 20},
	{Role: models.RoleGenerate, Model: "qwen2.5:7b-instruct", ApproxBytes: 4700 << 20},
}

// Settings is the whole of what first run remembers: one pointer and two facts.
//
// It lives outside the data folder, because it is the pointer to that folder
// and a pointer inside the thing it points at cannot be followed.
type Settings struct {
	DataFolder          string `json:"dataFolder"`
	Scope               Scope  `json:"scope"`
	Acknowledged        string `json:"acknowledged"`
	UnencryptedAccepted string `json:"unencryptedAccepted"`
}

// SettingsPath returns the settings file's location under dir.
func SettingsPath(dir string) string { return filepath.Join(dir, "setup.json") }

// Load reads the settings from dir. A missing file is an empty Settings, not an
// error: that is what a fresh install looks like.
//
// An unset scope loads as real. Demo is a deliberate choice, and defaulting to
// it would mean a fresh install silently refusing the recruiter's first
// document — while defaulting to real changes nothing about what is permitted,
// because the encryption gate still decides that.
func Load(dir string) (Settings, error) {
	s := Settings{Scope: ScopeReal}
	raw, err := os.ReadFile(SettingsPath(dir)) // #nosec G304 -- application's own config path.
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("reading setup settings: %w", err)
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, fmt.Errorf("reading setup settings: %w", err)
	}
	if s.Scope == "" {
		s.Scope = ScopeReal
	}
	return s, nil
}

// Save writes the settings to dir, creating it if needed.
func Save(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating the settings directory: %w", err)
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("writing setup settings: %w", err)
	}
	if err := os.WriteFile(SettingsPath(dir), raw, 0o600); err != nil {
		return fmt.Errorf("writing setup settings: %w", err)
	}
	return nil
}

// CatalogModel is one choice the model picker offers: what it is for, how
// capable it is in the recruiter's terms, and what downloading it costs.
type CatalogModel struct {
	Role        models.ModelRole `json:"role"`
	Model       string           `json:"model"`
	Purpose     string           `json:"purpose"`
	Power       string           `json:"power"`
	ApproxBytes int64            `json:"approxBytes"`
}

// Catalog is every model the picker offers, a few per role. Sizes are
// approximate for the same reason Required's are: the point is telling the
// recruiter what a download costs before it starts, not accounting for it
// afterwards. Every Required entry appears here so the recommended set is
// always choosable.
var Catalog = []CatalogModel{
	{Role: models.RoleEmbed, Model: "all-minilm", Purpose: "Turns documents into searchable evidence", Power: "fastest, smallest", ApproxBytes: 46 << 20},
	{Role: models.RoleEmbed, Model: "nomic-embed-text", Purpose: "Turns documents into searchable evidence", Power: "recommended", ApproxBytes: 274 << 20},
	{Role: models.RoleClassify, Model: "qwen2.5:3b-instruct", Purpose: "Reads profiles and flags prohibited criteria", Power: "fastest, less thorough", ApproxBytes: 1900 << 20},
	{Role: models.RoleClassify, Model: "qwen2.5:7b-instruct", Purpose: "Reads profiles and flags prohibited criteria", Power: "recommended", ApproxBytes: 4700 << 20},
	{Role: models.RoleGenerate, Model: "llama3.2:3b", Purpose: "Writes assessments, summaries, drafts, and chat", Power: "fastest, simpler writing", ApproxBytes: 2000 << 20},
	{Role: models.RoleGenerate, Model: "qwen2.5:7b-instruct", Purpose: "Writes assessments, summaries, drafts, and chat", Power: "recommended", ApproxBytes: 4700 << 20},
	{Role: models.RoleGenerate, Model: "qwen3:8b", Purpose: "Writes assessments, summaries, drafts, and chat", Power: "most capable, slowest", ApproxBytes: 5200 << 20},
}
