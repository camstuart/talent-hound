package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/setup"
)

// ModelService is the registry: which model answers for each role, at which
// endpoint, with which parameters, and whether anything has ever proved it good
// enough.
//
// Assignments are append-only. Phase 9 identifies an embedding space by
// endpoint revision and model digest, so what those point at must not be
// editable — an identifier that can change under its referents is not one.
type ModelService struct {
	db     *gorm.DB
	jobs   *JobService
	ollama *platform.Ollama

	// pullState holds what happened last time a role's model was going to be
	// downloaded: declined by the recruiter, or attempted and failed. It is
	// memory rather than a column because neither fact should survive a restart
	// — the next launch should ask again, and retry.
	//
	// ponytail: in-memory, per process. A column when someone complains about
	// being asked twice in a week rather than twice in a session.
	mu        sync.Mutex
	pullState map[models.ModelRole]string
}

// NewModelService returns a registry backed by db and registers the pull
// worker.
func NewModelService(db *gorm.DB, jobs *JobService, ollama *platform.Ollama) *ModelService {
	s := &ModelService{db: db, jobs: jobs, ollama: ollama, pullState: map[models.ModelRole]string{}}
	jobs.register("pull", s.pullWorker)
	return s
}

// Endpoint is the model endpoint this registry talks to, for messages that
// must name it.
func (s *ModelService) Endpoint() string { return s.ollama.Endpoint() }

// checkTimeout bounds an availability check. Long enough for a busy endpoint to
// answer, short enough that a settings screen is not stuck on it.
const checkTimeout = 10 * time.Second

// AssignInput is one assignment request. Endpoint is optional and defaults to
// the local one; naming anything else is refused, because a required role that
// could point at a cloud endpoint would make "local-only" a property of every
// future call site instead of one record.
type AssignInput struct {
	Role     models.ModelRole `json:"role"`
	Endpoint string           `json:"endpoint"`
	Model    string           `json:"model"`
	Digest   string           `json:"digest"`
	// Role parameters as JSON. Settings and numbers, never content.
	Params string `json:"params"`
}

// Assign records a configuration for a role, as a new revision.
//
// Re-assigning exactly what a role already has does nothing and returns the
// current assignment: without that rule, a settings screen's save button
// becomes a revision generator, and every save invalidates derived data that
// did not need invalidating.
func (s *ModelService) Assign(in AssignInput) (*models.ModelAssignment, error) {
	if !in.Role.Valid() {
		return nil, fmt.Errorf("unknown model role %q", in.Role)
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		return nil, fmt.Errorf("the %s role needs a model name", in.Role)
	}
	params, err := checkParams(in.Role, in.Params)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimSpace(in.Endpoint)
	if endpoint == "" {
		endpoint = platform.OllamaBaseURL
	}
	if err := checkLocalEndpoint(endpoint); err != nil {
		return nil, err
	}

	next := models.ModelAssignment{
		Role:       in.Role,
		Endpoint:   endpoint,
		Model:      model,
		Digest:     strings.TrimSpace(in.Digest),
		Params:     params,
		Validation: models.Unvalidated,
	}

	current, err := s.current(in.Role)
	if err != nil {
		return nil, err
	}
	if current != nil && current.SameConfig(next) {
		return current, nil
	}
	next.Revision = 1
	if current != nil {
		next.Revision = current.Revision + 1
	}
	if err := s.db.Create(&next).Error; err != nil {
		return nil, fmt.Errorf("assigning the %s role: %w", in.Role, err)
	}
	return &next, nil
}

// Resolution is a role's current assignment and where it came from.
type Resolution struct {
	Role models.ModelRole `json:"role"`
	// Assignment is nil when the role has no model and none can be inherited.
	Assignment *models.ModelAssignment `json:"assignment"`
	// Inherited is true when this is the generate assignment standing in for
	// classify, which is what "classify defaults to generate" means: no second
	// copy of the same configuration, and it keeps following when generate
	// changes.
	Inherited bool `json:"inherited"`
}

// Resolve returns the model that answers for a role right now.
func (s *ModelService) Resolve(role models.ModelRole) (*Resolution, error) {
	if !role.Valid() {
		return nil, fmt.Errorf("unknown model role %q", role)
	}
	assignment, err := s.current(role)
	if err != nil {
		return nil, err
	}
	if assignment != nil {
		return &Resolution{Role: role, Assignment: assignment}, nil
	}
	if role != models.RoleClassify {
		return &Resolution{Role: role}, nil
	}
	inherited, err := s.current(models.RoleGenerate)
	if err != nil {
		return nil, err
	}
	if inherited == nil {
		return &Resolution{Role: role}, nil
	}
	return &Resolution{Role: role, Assignment: inherited, Inherited: true}, nil
}

// Registry returns every role's current resolution, for the settings view.
func (s *ModelService) Registry() ([]Resolution, error) {
	out := make([]Resolution, 0, len(models.ModelRoles))
	for _, role := range models.ModelRoles {
		res, err := s.Resolve(role)
		if err != nil {
			return nil, err
		}
		out = append(out, *res)
	}
	return out, nil
}

// History returns every revision a role has had, oldest first. Nothing prunes
// it: a few dozen rows in a realistic lifetime, and Phase 9 needs to be able to
// name a revision long after it stopped being current.
func (s *ModelService) History(role models.ModelRole) ([]models.ModelAssignment, error) {
	rows := []models.ModelAssignment{}
	if err := s.db.Where("role = ?", role).Order("revision asc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("listing %s assignments: %w", role, err)
	}
	return rows, nil
}

// MarkValidated records that a benchmark proved this revision good enough.
//
// Nothing calls it yet — Phase 21 owns benchmarks — and that is the point. The
// alternative is a status that defaults to Unvalidated with no way to change,
// which reads as unfinished and invites the next person to flip it with an
// update statement.
func (s *ModelService) MarkValidated(role models.ModelRole, benchmarkRef string) error {
	benchmarkRef = strings.TrimSpace(benchmarkRef)
	if benchmarkRef == "" {
		return fmt.Errorf("marking %s validated needs a benchmark reference", role)
	}
	current, err := s.current(role)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("the %s role has no assignment to validate", role)
	}
	err = s.db.Model(&models.ModelAssignment{}).Where("id = ?", current.ID).Updates(map[string]any{
		"validation":    models.Validated,
		"benchmark_ref": benchmarkRef,
	}).Error
	if err != nil {
		return fmt.Errorf("validating the %s role: %w", role, err)
	}
	return nil
}

// current returns the highest revision for a role, or nil when it has none.
func (s *ModelService) current(role models.ModelRole) (*models.ModelAssignment, error) {
	var row models.ModelAssignment
	err := s.db.Where("role = ?", role).Order("revision desc").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading the %s assignment: %w", role, err)
	}
	return &row, nil
}

// Status is one role's availability, as one of the codes in models.
type Status struct {
	Role      models.ModelRole `json:"role"`
	Model     string           `json:"model"`
	Inherited bool             `json:"inherited"`
	// State is exactly one of the model availability codes. Distinct causes are
	// not collapsed: the recruiter's next action differs for every one of them.
	State string `json:"state"`
}

// Check reports the availability of every role's model.
func (s *ModelService) Check() ([]Status, error) {
	registry, err := s.Registry()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	// One listing answers for every role, and carries no content of any kind —
	// an availability check must never be a reason candidate text leaves the
	// database.
	installed, listErr := s.ollama.Tags(ctx)
	pulling := s.activePulls()

	out := make([]Status, 0, len(registry))
	for _, res := range registry {
		status := Status{Role: res.Role, Inherited: res.Inherited}
		if res.Assignment == nil {
			status.State = models.ModelUnassigned
			out = append(out, status)
			continue
		}
		status.Model = res.Assignment.Model
		switch {
		case pulling[res.Role]:
			status.State = models.ModelPulling
		case listErr != nil:
			status.State = endpointState(listErr)
		case platform.HasModel(installed, res.Assignment.Model):
			status.State = models.ModelReady
		default:
			// Missing, unless something more specific is known about why it is
			// still missing: the recruiter said no, or a pull was tried and did
			// not finish.
			status.State = models.ModelMissing
			if last := s.lastPull(res.Role); last != "" {
				status.State = last
			}
		}
		out = append(out, status)
	}
	return out, nil
}

// endpointState turns an endpoint failure into the state the recruiter acts on.
func endpointState(err error) string {
	switch {
	case errors.Is(err, platform.ErrEndpointTimeout):
		return models.ModelTimeout
	case errors.Is(err, platform.ErrModelMemory):
		return models.ModelOutOfMemory
	case errors.Is(err, platform.ErrMalformedResponse):
		return models.ModelMalformed
	case errors.Is(err, platform.ErrModelNotFound):
		return models.ModelMissing
	}
	return models.ModelEndpointDown
}

// Decline records that the recruiter said no to pulling this role's model.
func (s *ModelService) Decline(role models.ModelRole) error {
	if !role.Valid() {
		return fmt.Errorf("unknown model role %q", role)
	}
	s.setPullState(role, models.ModelPullDeclined)
	return nil
}

func (s *ModelService) setPullState(role models.ModelRole, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state == "" {
		delete(s.pullState, role)
		return
	}
	s.pullState[role] = state
}

func (s *ModelService) lastPull(role models.ModelRole) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pullState[role]
}

// pullParams is what a pull job carries: a role and a model name. Identifiers
// and settings, never content.
type pullParams struct {
	Role  models.ModelRole `json:"role"`
	Model string           `json:"model"`
}

// Pull downloads a role's model in the background. Declining earlier does not
// prevent it: the recruiter changed their mind, which is allowed.
func (s *ModelService) Pull(role models.ModelRole) (*models.Job, error) {
	res, err := s.Resolve(role)
	if err != nil {
		return nil, err
	}
	if res.Assignment == nil {
		return nil, fmt.Errorf("the %s role has no model to pull", role)
	}
	// A decline or an earlier failure is not a bar: the recruiter changed their
	// mind, and this attempt reports its own outcome.
	s.setPullState(role, "")

	params, err := json.Marshal(pullParams{Role: role, Model: res.Assignment.Model})
	if err != nil {
		return nil, fmt.Errorf("encoding pull params: %w", err)
	}
	return s.jobs.Enqueue(JobInput{Kind: "pull", Params: string(params), TotalItems: 1})
}

// pullWorker runs one model download. It writes nothing, so it has no commit
// half: what changed is at the endpoint, and the next check sees it.
func (s *ModelService) pullWorker(ctx context.Context, job models.Job, _ int) (JobCommit, error) {
	var p pullParams
	if err := json.Unmarshal([]byte(job.Params), &p); err != nil {
		return nil, FailReason("bad_params")
	}
	if err := s.ollama.Pull(ctx, p.Model); err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		// The endpoint's own words stay at the endpoint; the job stores a code,
		// and the role remembers that a pull was tried.
		s.setPullState(p.Role, models.ModelPullFailed)
		return nil, FailReason(models.ModelPullFailed)
	}
	return nil, nil
}

// roleParams is the parameter each role accepts, and the only one it accepts.
// A short fixed list is the whole of "unsupported parameters are refused" —
// there is no schema language here and nothing that needs one.
var roleParams = map[models.ModelRole][]string{
	models.RoleEmbed:    {},
	models.RoleClassify: {"temperature", "num_ctx"},
	models.RoleGenerate: {"temperature", "num_ctx", "top_p"},
}

// checkParams validates the parameter JSON and returns it normalized.
func checkParams(role models.ModelRole, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "{}", nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", fmt.Errorf("the %s parameters must be a JSON object", role)
	}
	allowed := roleParams[role]
	for name := range parsed {
		if !slices.Contains(allowed, name) {
			return "", fmt.Errorf("the %s role does not support the parameter %q", role, name)
		}
	}
	// Re-encoded so two orderings of the same parameters are one configuration
	// rather than two revisions.
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return "", fmt.Errorf("encoding the %s parameters: %w", role, err)
	}
	return string(normalized), nil
}

// checkLocalEndpoint refuses anything that is not the local model endpoint.
func checkLocalEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("the model endpoint must be an absolute http or https URL, got %q", endpoint)
	}
	if host := u.Hostname(); host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("a required model role must use the local endpoint, got %q", endpoint)
	}
	return nil
}

// ModelOption is one catalog entry as the picker shows it: the catalog fact
// plus whether the endpoint already holds the model.
type ModelOption struct {
	Role        models.ModelRole `json:"role"`
	Model       string           `json:"model"`
	Purpose     string           `json:"purpose"`
	Power       string           `json:"power"`
	ApproxBytes int64            `json:"approxBytes"`
	Installed   bool             `json:"installed"`
}

// OptionsView is everything the model picker needs in one answer: the catalog
// with installed flags, and how much disk there is for a download to land on.
type OptionsView struct {
	Models        []ModelOption `json:"models"`
	FreeDiskBytes int64         `json:"freeDiskBytes"`
}

// Options lists the curated models a recruiter may choose from. An endpoint
// that cannot be asked leaves every model un-flagged rather than failing the
// whole answer: the picker is still usable, only the "already available"
// highlights go missing.
func (s *ModelService) Options() (*OptionsView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	installed, listErr := s.ollama.Tags(ctx)

	view := &OptionsView{Models: make([]ModelOption, 0, len(setup.Catalog))}
	for _, c := range setup.Catalog {
		view.Models = append(view.Models, ModelOption{
			Role:        c.Role,
			Model:       c.Model,
			Purpose:     c.Purpose,
			Power:       c.Power,
			ApproxBytes: c.ApproxBytes,
			Installed:   listErr == nil && platform.HasModel(installed, c.Model),
		})
	}
	// Downloads land on the user's own volume wherever Ollama keeps its store,
	// so that volume's free space is the one that decides whether a download
	// fits. A home folder that cannot be asked leaves the count at zero, which
	// the picker shows as unknown rather than as room to spare.
	if home, err := os.UserHomeDir(); err == nil {
		if free, err := platform.FreeDiskBytes(home); err == nil {
			view.FreeDiskBytes = free
		}
	}
	return view, nil
}

// activePulls reports the roles whose model download is underway right now,
// read from the job queue so any window of any session sees the same answer.
func (s *ModelService) activePulls() map[models.ModelRole]bool {
	var jobs []models.Job
	if err := s.db.Where("kind = ? AND state IN ?", "pull",
		[]models.JobState{models.JobQueued, models.JobRunning}).Find(&jobs).Error; err != nil {
		return nil
	}
	out := map[models.ModelRole]bool{}
	for _, job := range jobs {
		var p pullParams
		if json.Unmarshal([]byte(job.Params), &p) == nil {
			out[p.Role] = true
		}
	}
	return out
}
