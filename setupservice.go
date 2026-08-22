package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/extract"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/setup"
)

// Version is the application version, shown in the interface and in the
// diagnostic report. Both read this one constant so they cannot disagree.
const Version = "0.1.0-poc"

// checkDeadline bounds the two checks that shell out, so a wedged dependency
// cannot wedge the wizard.
const checkDeadline = 25 * time.Second

// SetupService runs first run and holds the two facts and one pointer that
// survive it.
//
// The wizard's position is not among them: it is recomputed from what is true
// every time it is asked for.
type SetupService struct {
	// checkEncryption, when set, replaces the platform check. Only tests set
	// it, and only to see which folder was asked about: two folders on one
	// volume give the same answer, so the answer cannot show which was
	// examined, and which was examined is the whole of the rule.
	checkEncryption func(ctx context.Context, path string) platform.EncryptionStatus

	db      *gorm.DB
	modelSv *ModelService
	confDir string
	dataDir string

	mu       sync.RWMutex
	settings setup.Settings
	// encryption is the last startup check's result, so the write guard does not
	// shell out on every candidate.
	encryption platform.EncryptionStatus
}

// NewSetupService loads the remembered settings and runs the startup encryption
// check. confDir is where the settings pointer lives — outside the data folder.
func NewSetupService(db *gorm.DB, modelSv *ModelService, confDir, dataDir string) (*SetupService, error) {
	settings, err := setup.Load(confDir)
	if err != nil {
		return nil, err
	}
	if settings.DataFolder == "" && dataDir != "" {
		settings.DataFolder = dataDir
	}
	s := &SetupService{db: db, modelSv: modelSv, confDir: confDir, dataDir: dataDir, settings: settings}
	s.Recheck()
	return s, nil
}

// Recheck runs the volume encryption check again. It runs at startup and
// whenever the recruiter asks, because a data folder can move to an
// unencrypted volume long after first run.
//
// It checks the folder the data is in, which is not always the folder the
// recruiter chose. Choosing one records a preference that the next launch
// opens the database in; until then the records go where they were already
// going. Checking the chosen folder instead meant a recruiter could point the
// wizard at an encrypted disk, be told real data was allowed, and have every
// record written to the folder this process actually opened — which is the
// gate that exists to stop exactly that.
func (s *SetupService) Recheck() {
	s.mu.Lock()
	folder := s.dataDir
	if folder == "" {
		folder = s.settings.DataFolder
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), checkDeadline)
	defer cancel()
	check := s.checkEncryption
	if check == nil {
		check = platform.VolumeEncryption
	}
	status := check(ctx, folder)

	s.mu.Lock()
	s.encryption = status
	s.mu.Unlock()
}

// AllowRealData is the write guard the record and artifact services consult. It
// is a method with no parameter for the same reason Phase 18's cloud boundary
// is: there is nothing to pass that says yes.
func (s *SetupService) AllowRealData() error {
	s.mu.RLock()
	scope, status := s.settings.Scope, s.encryption
	s.mu.RUnlock()
	if ok, _ := setup.RealDataAllowed(scope, status); ok {
		return nil
	}
	_, why := setup.RealDataAllowed(scope, status)
	return fmt.Errorf("this installation cannot hold candidate data: %s", why)
}

// ScopeState is what the operating state needs, and nothing else: it is read
// from memory, because a status strip that redraws on every change must not
// spawn the sidecar and call Ollama each time.
type ScopeState struct {
	Scope       setup.Scope               `json:"scope"`
	Encryption  platform.EncryptionStatus `json:"encryption"`
	RealData    bool                      `json:"realData"`
	RealDataWhy string                    `json:"realDataWhy"`
	Version     string                    `json:"version"`
}

// Scope reports the current scope and what it permits, without running a check.
func (s *SetupService) Scope() *ScopeState {
	s.mu.RLock()
	scope, status := s.settings.Scope, s.encryption
	s.mu.RUnlock()
	allowed, why := setup.RealDataAllowed(scope, status)
	return &ScopeState{
		Scope: scope, Encryption: status, RealData: allowed,
		RealDataWhy: why, Version: Version,
	}
}

// StepView is one step and whether it is satisfied, with the reason when not.
type StepView struct {
	Step      setup.Step `json:"step"`
	Satisfied bool       `json:"satisfied"`
	Detail    string     `json:"detail"`
}

// ModelView is one required model, its size, and whether it is installed.
type ModelView struct {
	Role        models.ModelRole `json:"role"`
	Model       string           `json:"model"`
	ApproxBytes int64            `json:"approxBytes"`
	Installed   bool             `json:"installed"`
	State       string           `json:"state"`
}

// SetupStatus is the whole first-run picture in one call: the step to be on, every
// step's state, the models, and what the recruiter may do with this
// installation.
type SetupStatus struct {
	Next       setup.Step  `json:"next"`
	Complete   bool        `json:"complete"`
	Steps      []StepView  `json:"steps"`
	Models     []ModelView `json:"models"`
	DataFolder string      `json:"dataFolder"`
	// FolderInUse is where this process actually opened the database, which is
	// not the chosen folder until the application restarts. The encryption
	// status below is about this one, because this is where the records go.
	FolderInUse  string                    `json:"folderInUse"`
	Scope        setup.Scope               `json:"scope"`
	Encryption   platform.EncryptionStatus `json:"encryption"`
	RealData     bool                      `json:"realData"`
	RealDataWhy  string                    `json:"realDataWhy"`
	Version      string                    `json:"version"`
	Acknowledged bool                      `json:"acknowledged"`
}

// State gathers what is true and reports the first unsatisfied step.
func (s *SetupService) State() (*SetupStatus, error) {
	s.mu.RLock()
	settings, status := s.settings, s.encryption
	s.mu.RUnlock()

	sidecarOK, sidecarWhy := s.sidecar()
	modelViews, err := s.models()
	if err != nil {
		return nil, err
	}
	ollamaOK, ollamaWhy := ollamaReachable(modelViews)

	var initiatives int64
	if err := s.db.Model(&models.Initiative{}).Count(&initiatives).Error; err != nil {
		return nil, fmt.Errorf("counting initiatives: %w", err)
	}

	present := map[models.ModelRole]bool{}
	for _, m := range modelViews {
		present[m.Role] = present[m.Role] || m.Installed
	}
	checks := setup.Checks{
		DataFolder:   settings.DataFolder,
		Scope:        settings.Scope,
		Encryption:   status,
		Sidecar:      sidecarOK,
		SidecarWhy:   sidecarWhy,
		Ollama:       ollamaOK,
		OllamaWhy:    ollamaWhy,
		Models:       present,
		Acknowledged: settings.Acknowledged,
		Initiatives:  initiatives,
	}
	next := setup.Next(checks)
	allowed, why := setup.RealDataAllowed(settings.Scope, status)

	return &SetupStatus{
		Next:         next,
		Complete:     next == setup.StepComplete,
		Steps:        stepViews(checks, next),
		Models:       modelViews,
		DataFolder:   settings.DataFolder,
		FolderInUse:  s.dataDir,
		Scope:        settings.Scope,
		Encryption:   status,
		RealData:     allowed,
		RealDataWhy:  why,
		Version:      Version,
		Acknowledged: settings.Acknowledged == setup.AcknowledgementVersion,
	}, nil
}

// stepViews reports every step's state. A step after the current one is not
// "failed" — it is simply not reached yet, and saying otherwise sends the
// recruiter chasing a dependency that was never checked.
func stepViews(c setup.Checks, next setup.Step) []StepView {
	reached := true
	out := make([]StepView, 0, len(setup.Order))
	for _, step := range setup.Order {
		if step == next {
			reached = false
		}
		view := StepView{Step: step, Satisfied: reached}
		if step == next {
			view.Detail = detailFor(step, c)
		}
		out = append(out, view)
	}
	return out
}

func detailFor(step setup.Step, c setup.Checks) string {
	switch step {
	case setup.StepDataFolder:
		return "choose the folder that will hold every record, document, and index"
	case setup.StepEncryption:
		_, why := setup.RealDataAllowed(setup.ScopeReal, c.Encryption)
		return why
	case setup.StepSidecar:
		return c.SidecarWhy
	case setup.StepOllama:
		return c.OllamaWhy
	case setup.StepModels:
		return "the models below are needed before anything can be indexed or written"
	case setup.StepAcknowledgement:
		return "the data-handling preconditions have not been acknowledged"
	case setup.StepFirstInitiative:
		return "create the first initiative to finish setting up"
	}
	return ""
}

// sidecar verifies the bundled extraction sidecar and its pinned version, and
// says which of the two failed rather than reporting extraction as broken.
func (s *SetupService) sidecar() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), checkDeadline)
	defer cancel()
	sc := extract.Verify(ctx, extract.DefaultSidecarPath())
	if sc.Available() {
		return true, ""
	}
	switch sc.Reason() {
	case models.ReasonSidecarVersion:
		return false, fmt.Sprintf("the extraction sidecar at %s reports version %q, and this build "+
			"is pinned to %s", extract.DefaultSidecarPath(), sc.Version(), extract.PinnedSidecarVersion)
	default:
		return false, fmt.Sprintf("the extraction sidecar was not found at %s",
			extract.DefaultSidecarPath())
	}
}

// roleStatus is one role's registry answer, narrowed to what first run needs.
type roleStatus struct{ Model, State string }

// models reports every required model with its size and whether it is
// installed, reusing the registry's availability codes.
func (s *SetupService) models() ([]ModelView, error) {
	statuses, err := s.modelSv.Check()
	if err != nil {
		return nil, err
	}
	byRole := map[models.ModelRole]roleStatus{}
	for _, st := range statuses {
		byRole[st.Role] = roleStatus{Model: st.Model, State: st.State}
	}
	out := make([]ModelView, 0, len(setup.Required))
	for _, req := range setup.Required {
		view := ModelView{Role: req.Role, Model: req.Model, ApproxBytes: req.ApproxBytes}
		if st, ok := byRole[req.Role]; ok {
			if st.Model != "" {
				view.Model = st.Model
			}
			view.State = st.State
			view.Installed = st.State == models.ModelReady
		}
		out = append(out, view)
	}
	return out, nil
}

// ollamaReachable reads the endpoint's state out of the model check: one
// listing already answered the question, and asking again would be a second
// call that can disagree with the first.
func ollamaReachable(views []ModelView) (bool, string) {
	for _, v := range views {
		switch v.State {
		case models.ModelEndpointDown:
			return false, fmt.Sprintf("Ollama is not reachable at %s", platform.OllamaBaseURL)
		case models.ModelTimeout:
			return false, fmt.Sprintf("Ollama did not answer in time at %s", platform.OllamaBaseURL)
		}
	}
	return true, ""
}

// ChooseFolder records the data folder after checking it can actually be used.
//
// ponytail: the choice takes effect at the next start, because the database is
// opened once at startup. Re-opening every service against a new handle mid-run
// is a lot of machinery for a step the recruiter does once.
func (s *SetupService) ChooseFolder(path string) error {
	if path == "" {
		return errors.New("choose a folder for Talent Hound's data")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving the folder: %w", err)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return fmt.Errorf("the folder cannot be created: %w", err)
	}
	// Writability is checked here rather than discovered during a migration.
	probe := filepath.Join(abs, ".th-write-check")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return fmt.Errorf("the folder cannot be written to: %w", err)
	}
	if err := os.Remove(probe); err != nil {
		return fmt.Errorf("the folder cannot be written to: %w", err)
	}

	// A folder that already holds a database is a recovery, and the recruiter
	// finds out now whether it can be opened.
	//
	// The checks used to run only when the database was opened, which is the
	// next launch: choosing a copy whose file was truncated by whatever
	// interrupted the copy said nothing, and the application then failed to
	// start. That is the worst moment to learn it, because the recruiter has
	// already replaced the working installation in their mind.
	//
	// An empty folder is a new installation and stays acceptable — the check
	// refuses one for holding no database, which is the whole point of a fresh
	// start.
	if _, err := os.Stat(filepath.Join(abs, db.FileName)); err == nil {
		if err := db.CheckFolder(abs); err != nil {
			return fmt.Errorf("that folder cannot be opened: %w", err)
		}
	}

	s.mu.Lock()
	s.settings.DataFolder = abs
	settings := s.settings
	s.mu.Unlock()
	if err := setup.Save(s.confDir, settings); err != nil {
		return err
	}
	s.Recheck()
	return nil
}

// SetScope records the scope. Demo is available on any volume; real is not
// blocked here, because being blocked from holding data is what the encryption
// gate reports, not a reason to silently switch the recruiter to demo.
func (s *SetupService) SetScope(scope setup.Scope) error {
	if !scope.Valid() {
		return fmt.Errorf("unknown scope %q", scope)
	}
	s.mu.Lock()
	s.settings.Scope = scope
	settings := s.settings
	s.mu.Unlock()
	return setup.Save(s.confDir, settings)
}

// Acknowledgements is the data-handling text the recruiter accepts, returned
// rather than hard-coded in the interface so the accepted version and the
// displayed text cannot drift apart.
func (s *SetupService) Acknowledgements() []string {
	return []string{
		"I have the authority to hold and use the candidate data I load.",
		"I am responsible for retaining and deleting that data appropriately.",
		"Public information is used for evaluation and recruiter-controlled outreach, not republication.",
		"Searches may disclose generalized professional information, and are always previewed first.",
		"Optional cloud tasks have their own payload and consent controls.",
		"Some criteria are prohibited, and it is my responsibility not to search on them.",
	}
}

// Acknowledge records acceptance of the current data-handling text.
func (s *SetupService) Acknowledge() error {
	s.mu.Lock()
	s.settings.Acknowledged = setup.AcknowledgementVersion
	settings := s.settings
	s.mu.Unlock()
	return setup.Save(s.confDir, settings)
}

// PullModel starts a pull for one role's model, as a background job.
func (s *SetupService) PullModel(role models.ModelRole) (*models.Job, error) {
	return s.modelSv.Pull(role)
}

// DeclineModel records that the recruiter said no. Setup stays where it is.
func (s *SetupService) DeclineModel(role models.ModelRole) error {
	return s.modelSv.Decline(role)
}
