package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/scrub"
)

// DiagnosticsService produces the local, redacted report; opens the logs
// folder; and performs delete-all.
//
// The report is assembled from facts the application already holds — versions,
// availability, counts, codes. It never reads a table for content. That is the
// difference between a report that is safe and a report that is redacted: a
// redactor that misses one field produces something that looks safe.
type DiagnosticsService struct {
	db      *gorm.DB
	setup   *SetupService
	dataDir string
}

// NewDiagnosticsService returns a DiagnosticsService over the given data folder.
func NewDiagnosticsService(gdb *gorm.DB, setupSv *SetupService, dataDir string) *DiagnosticsService {
	return &DiagnosticsService{db: gdb, setup: setupSv, dataDir: dataDir}
}

// Count is one kind of record and how many exist. The kind is a fixed label;
// the number is a number. Neither can carry content.
type Count struct {
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
}

// Report is the whole diagnostic picture.
type Report struct {
	Version       string  `json:"version"`
	SchemaVersion int     `json:"schemaVersion"`
	BuildSchema   int     `json:"buildSchema"`
	Platform      string  `json:"platform"`
	DataFolder    string  `json:"dataFolder"`
	LogsFolder    string  `json:"logsFolder"`
	Encryption    string  `json:"encryption"`
	Scope         string  `json:"scope"`
	RealData      bool    `json:"realData"`
	Sidecar       string  `json:"sidecar"`
	Ollama        string  `json:"ollama"`
	Models        []Count `json:"models"`
	Counts        []Count `json:"counts"`
	// Jobs are outcomes as codes: the failure_reason column is a short code by
	// construction, so nothing a job touched can reach this report through it.
	Jobs []Count `json:"jobs"`
}

// countable is every table the report counts, with the label it reports. Adding
// a table here adds a number, never a string.
var countable = []struct {
	kind  string
	model any
}{
	{"initiatives", &models.Initiative{}},
	{"candidates", &models.Candidate{}},
	{"companies", &models.Company{}},
	{"contacts", &models.Contact{}},
	{"roles", &models.Role{}},
	{"artifacts", &models.Artifact{}},
	{"artifact links", &models.ArtifactLink{}},
	{"chunks", &models.Chunk{}},
	{"embeddings", &models.Embedding{}},
	{"profiles", &models.Profile{}},
	{"profile aspects", &models.ProfileAspect{}},
	{"search criteria", &models.SearchCriterion{}},
	{"matches", &models.Match{}},
	{"drafts", &models.Draft{}},
	{"audit events", &models.DisclosureEvent{}},
}

// Diagnostics builds the report.
func (s *DiagnosticsService) Diagnostics() (*Report, error) {
	schema, err := db.SchemaVersion(s.db)
	if err != nil {
		return nil, err
	}
	report := &Report{
		Version:       Version,
		SchemaVersion: schema,
		BuildSchema:   db.LatestVersion(),
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
		DataFolder:    s.dataDir,
		LogsFolder:    s.LogsFolder(),
		Sidecar:       "unknown",
		Ollama:        "unknown",
	}

	if s.setup != nil {
		state, err := s.setup.State()
		if err != nil {
			return nil, err
		}
		report.Encryption = string(state.Encryption)
		report.Scope = string(state.Scope)
		report.RealData = state.RealData
		report.Sidecar = availability(stepSatisfied(state, "sidecar"))
		report.Ollama = availability(stepSatisfied(state, "ollama"))
		for _, m := range state.Models {
			// The role and the availability code — not the model name, which the
			// recruiter chose and which is theirs, not a fact about the machine.
			report.Models = append(report.Models, Count{Kind: string(m.Role) + ": " + m.State, Count: 1})
		}
	}

	for _, c := range countable {
		var n int64
		if err := s.db.Model(c.model).Count(&n).Error; err != nil {
			return nil, fmt.Errorf("counting %s: %w", c.kind, err)
		}
		report.Counts = append(report.Counts, Count{Kind: c.kind, Count: n})
	}

	type row struct {
		State  string
		Reason string
		N      int64
	}
	rows := []row{}
	err = s.db.Model(&models.Job{}).
		Select("state as state, failure_reason as reason, count(*) as n").
		Group("state").Group("failure_reason").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("counting jobs: %w", err)
	}
	for _, r := range rows {
		kind := r.State
		if r.Reason != "" {
			kind += ": " + r.Reason
		}
		report.Jobs = append(report.Jobs, Count{Kind: kind, Count: r.N})
	}

	report.clean()
	return report, nil
}

// clean is the second line, not the first: every string in the report already
// comes from a fixed label, a version, a path, or a code. Passing them through
// the scrubber and stripping control characters means a report is safe to paste
// anywhere even if a future field is added carelessly.
func (r *Report) clean() {
	safe := func(s string) string { return stripControl(scrub.Text(s, scrub.Identifiers{})) }
	r.Version = safe(r.Version)
	r.Platform = safe(r.Platform)
	// The two paths are stripped of control characters but not scrubbed: they
	// are the application's own resolved folders, and the scrubber would eat the
	// digits out of them.
	r.DataFolder = stripControl(r.DataFolder)
	r.LogsFolder = stripControl(r.LogsFolder)
	r.Encryption = safe(r.Encryption)
	r.Scope = safe(r.Scope)
	r.Sidecar = safe(r.Sidecar)
	r.Ollama = safe(r.Ollama)
	for _, list := range [][]Count{r.Models, r.Counts, r.Jobs} {
		for i := range list {
			list[i].Kind = safe(list[i].Kind)
		}
	}
}

// stripControl removes control characters and escape sequences, so a report is
// safe in a terminal as well as in the interface.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func stepSatisfied(state *SetupStatus, step string) bool {
	for _, s := range state.Steps {
		if string(s.Step) == step {
			return s.Satisfied
		}
	}
	return false
}

func availability(ok bool) string {
	if ok {
		return "available"
	}
	return "unavailable"
}

// openDeadline bounds the file-manager launch: it is a convenience, and a
// convenience must not hang the interface.
const openDeadline = 10 * time.Second

// LogsFolder is where local logs are kept: inside the data folder, so the one
// folder the recruiter copies holds them too.
func (s *DiagnosticsService) LogsFolder() string {
	return filepath.Join(s.dataDir, "logs")
}

// OpenLogsFolder asks the system to open the logs folder and returns its
// resolved path either way — the path is the thing the recruiter actually
// needs, and a file manager that does not open is not a reason to withhold it.
func (s *DiagnosticsService) OpenLogsFolder() (string, error) {
	dir := s.LogsFolder()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return dir, fmt.Errorf("creating the logs folder: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), openDeadline)
	defer cancel()
	opener := "xdg-open"
	switch runtime.GOOS {
	case "windows":
		opener = "explorer"
	case "darwin":
		opener = "open"
	}
	// #nosec G204 -- a fixed opener name and the application's own path.
	cmd := exec.CommandContext(ctx, opener, dir)
	if err := cmd.Start(); err != nil {
		return dir, fmt.Errorf("the logs folder is at %s, and could not be opened: %w", dir, err)
	}
	return dir, nil
}

// Recovery is what the recruiter has to do by hand after opening a copied
// folder, and where that folder is.
type Recovery struct {
	DataFolder string   `json:"dataFolder"`
	Steps      []string `json:"steps"`
}

// RecoveryProcedure documents the copied-folder path, naming the resolved
// folder rather than a generic location.
func (s *DiagnosticsService) RecoveryProcedure() *Recovery {
	return &Recovery{
		DataFolder: s.dataDir,
		Steps: []string{
			"Close Talent Hound completely, then copy " + s.dataDir + " somewhere safe.",
			"On the new machine, install Talent Hound and Ollama.",
			"Start Talent Hound and select the copied folder as the data folder.",
			"The database is checked for integrity and its schema version before it opens; " +
				"a snapshot is taken before any migration, and restored if one fails.",
			"Re-enter provider credentials: they live in the Windows credential store, not in the folder.",
			"Re-download any missing models: they live in Ollama's storage, not in the folder.",
		},
	}
}

// CheckFolder reports whether a folder can be opened as a data folder, without
// writing anything to it beyond a removed probe file.
func (s *DiagnosticsService) CheckFolder(dir string) error { return db.CheckFolder(dir) }

// DeleteAll removes the data folder's contents. The confirmation must be the
// resolved path itself: confirming "yes" to a folder described in words is how
// the wrong folder gets deleted.
func (s *DiagnosticsService) DeleteAll(confirmation string) (string, error) {
	target := filepath.Clean(s.dataDir)
	if target == "" || target == "." || target == string(filepath.Separator) {
		return target, errors.New("refusing to delete: the data folder is not a specific folder")
	}
	if filepath.Clean(strings.TrimSpace(confirmation)) != target {
		return target, fmt.Errorf("nothing was deleted: to delete everything, confirm the exact folder %s", target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return target, fmt.Errorf("reading the data folder: %w", err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(target, e.Name())); err != nil {
			return target, fmt.Errorf("deleting %s: %w", e.Name(), err)
		}
	}
	return target, nil
}
