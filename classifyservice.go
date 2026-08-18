package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// Classifier is the model the classify role resolves to. An interface because
// the contract has to be provable without a model: every rule here is about
// what happens to a response, and a real model is a slow way to produce one.
type Classifier interface {
	Chat(ctx context.Context, model, prompt string, schema map[string]any) (string, error)
}

// ClassifyService turns source chunks into a profile, or into a visible
// failure. There is no third outcome.
//
// A classifier that persists the aspects it got right and drops the ones it got
// wrong produces a profile that looks complete and is silently missing the
// requirement the recruiter cared about — which is worse than no profile at
// all, because every later call site treats it as fact.
type ClassifyService struct {
	db       *gorm.DB
	registry *ModelService
	model    Classifier
}

// classifyTimeout bounds one model call. A decomposition is a page of output
// from a local model, so this is generous rather than tight.
const classifyTimeout = 3 * time.Minute

// NewClassifyService returns a classifier bound to the registry.
func NewClassifyService(db *gorm.DB, registry *ModelService, model Classifier) *ClassifyService {
	return &ClassifyService{db: db, registry: registry, model: model}
}

// ClassifyInput is one classification request.
type ClassifyInput struct {
	SubjectKind profile.SubjectKind `json:"subjectKind"`
	SubjectID   uint                `json:"subjectId"`
	// ChunkIDs are the source chunks the classifier may read and cite. Nothing
	// else resolves: a citation to a chunk not in this list fails.
	ChunkIDs []uint `json:"chunkIds"`
}

// Classify decomposes the given chunks into a profile version.
//
// The call is made, the response validated, and on failure exactly one repair
// attempt is made carrying the problems found. A second failure is Failed and
// retryable — the recruiter may fix the source, change the model, or enter the
// aspects by hand, all of which are better than a loop.
func (s *ClassifyService) Classify(in ClassifyInput) (*models.Profile, error) {
	if !in.SubjectKind.Valid() {
		return nil, fmt.Errorf("unknown subject kind %q", in.SubjectKind)
	}
	sources, err := s.loadSources(in.ChunkIDs)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, s.fail(in, models.ReasonNoSources, 0, "",
			fmt.Errorf("classifying needs at least one source chunk"))
	}

	res, err := s.registry.Resolve(models.RoleClassify)
	if err != nil {
		return nil, err
	}
	if res.Assignment == nil {
		return nil, s.fail(in, models.ReasonNoClassifyModel, 0, "",
			fmt.Errorf("no model resolves for the classify role — assign classify or generate"))
	}
	assignment := *res.Assignment

	ctx, cancel := context.WithTimeout(context.Background(), classifyTimeout)
	defer cancel()

	schema := profile.Schema(in.SubjectKind)
	prompt := profile.Prompt(in.SubjectKind, sources)

	proposal, problems, raw, err := s.attempt(ctx, assignment.Model, prompt, schema, in.SubjectKind, sources)
	if err != nil {
		return nil, s.fail(in, models.ReasonClassifyFailed, assignment.Revision, assignment.Model, err)
	}
	if len(problems) > 0 {
		// Exactly one repair. The failure modes are bimodal: malformed JSON is
		// usually fixed immediately, and an invented aspect type is usually
		// invented again — so a loop buys nothing but the recruiter's time.
		repair := profile.RepairPrompt(raw, problems)
		proposal, problems, _, err = s.attempt(ctx, assignment.Model, prompt+"\n\n"+repair,
			schema, in.SubjectKind, sources)
		if err != nil {
			return nil, s.fail(in, models.ReasonClassifyFailed, assignment.Revision, assignment.Model, err)
		}
	}
	if len(problems) > 0 {
		return nil, s.fail(in, models.ReasonInvalidProposal, assignment.Revision, assignment.Model,
			fmt.Errorf("the classifier's output did not satisfy the contract after one repair attempt:\n- %s",
				strings.Join(problems, "\n- ")))
	}

	identity := profile.Identity{
		SchemaVersion: profile.SchemaVersion,
		PromptVersion: profile.PromptVersion,
		Revision:      assignment.Revision,
		SourceHash:    profile.HashSources(sources),
	}
	return s.store(in, identity, assignment.Model, proposal.Aspects, models.ProfileProposed, "")
}

// attempt makes one model call and validates what comes back.
func (s *ClassifyService) attempt(
	ctx context.Context, model, prompt string, schema map[string]any,
	kind profile.SubjectKind, sources []profile.Source,
) (profile.Proposal, []string, string, error) {
	raw, err := s.model.Chat(ctx, model, prompt, schema)
	if err != nil {
		// The endpoint's own words can quote the document it was given, so they
		// stay at the endpoint and the caller stores a code.
		return profile.Proposal{}, nil, "", fmt.Errorf("the classify model did not answer")
	}
	proposal, problems := profile.ParseProposal(raw)
	if len(problems) > 0 {
		return proposal, problems, raw, nil
	}
	return proposal, profile.Validate(kind, proposal, sources), raw, nil
}

// AddRecruiterAspect records a fact the recruiter asserted, as a new profile
// version.
//
// It has no artifact and cites the record the recruiter authored it into —
// which is the same citation rule in a different currency: something in the
// database says a person asserted this. Letting recruiter aspects cite nothing
// would make "cited" mean two things depending on origin, and Phase 17's drafts
// would have to quote both.
func (s *ClassifyService) AddRecruiterAspect(
	kind profile.SubjectKind, subjectID uint, aspect profile.Aspect, record string,
) (*models.Profile, error) {
	if !kind.Valid() {
		return nil, fmt.Errorf("unknown subject kind %q", kind)
	}
	record = strings.TrimSpace(record)
	if record == "" {
		return nil, fmt.Errorf("a recruiter supplied aspect must name the record it came from")
	}
	aspect.Origin = profile.RecruiterSupplied
	aspect.Citations = []profile.Citation{{Record: record}}

	current, err := s.Current(kind, subjectID)
	if err != nil {
		return nil, err
	}
	aspects := []profile.Aspect{}
	identity := profile.Identity{SchemaVersion: profile.SchemaVersion, PromptVersion: profile.PromptVersion}
	modelName := ""
	if current != nil {
		existing, err := s.Aspects(current.ID)
		if err != nil {
			return nil, err
		}
		aspects = existing
		identity.Revision = current.ModelRevision
		identity.SourceHash = current.SourceHash
		modelName = current.ModelName
	}
	// Only the new aspect is validated. The stored ones were validated against
	// the sources of the classification that produced them, and those sources
	// are not this call's to re-resolve — running them through again with no
	// sources would fail every chunk citation in the profile.
	if problems := profile.Validate(kind, profile.Proposal{Aspects: []profile.Aspect{aspect}}, nil); len(problems) > 0 {
		return nil, fmt.Errorf("that aspect does not satisfy the contract:\n- %s",
			strings.Join(problems, "\n- "))
	}
	key := profile.MeaningKey(aspect)
	for i, existing := range aspects {
		if profile.MeaningKey(existing) == key {
			return nil, fmt.Errorf("that aspect duplicates aspect %d, which the profile already has", i+1)
		}
	}
	aspects = append(aspects, aspect)
	state := models.ProfileProposed
	if current != nil {
		state = models.ProfileState(current.State)
	}
	return s.store(ClassifyInput{SubjectKind: kind, SubjectID: subjectID},
		identity, modelName, aspects, state, "")
}

// store writes a whole profile version in one transaction, or none of it.
func (s *ClassifyService) store(
	in ClassifyInput, identity profile.Identity, modelName string,
	aspects []profile.Aspect, state models.ProfileState, reason string,
) (*models.Profile, error) {
	row := models.Profile{
		SubjectKind:   string(in.SubjectKind),
		SubjectID:     in.SubjectID,
		State:         string(state),
		SchemaVersion: identity.SchemaVersion,
		PromptVersion: identity.PromptVersion,
		ModelRevision: identity.Revision,
		ModelName:     modelName,
		SourceHash:    identity.SourceHash,
		Identity:      identity.Hash(),
		FailureReason: reason,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		next, err := s.nextVersion(tx, in.SubjectKind, in.SubjectID)
		if err != nil {
			return err
		}
		row.Version = next
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("storing the profile: %w", err)
		}
		if len(aspects) == 0 {
			return nil
		}
		rows := make([]models.ProfileAspect, 0, len(aspects))
		for i, a := range aspects {
			structured, err := json.Marshal(orEmptyObject(a.Structured))
			if err != nil {
				return fmt.Errorf("encoding aspect %d: %w", i+1, err)
			}
			citations, err := json.Marshal(a.Citations)
			if err != nil {
				return fmt.Errorf("encoding aspect %d citations: %w", i+1, err)
			}
			priority := a.Priority
			if priority == "" {
				priority = profile.Unspecified
			}
			origin := a.Origin
			if origin == "" {
				origin = profile.Extracted
			}
			rows = append(rows, models.ProfileAspect{
				ProfileID:  row.ID,
				Ordinal:    i,
				Type:       string(a.Type),
				Wording:    a.Wording,
				Structured: string(structured),
				Priority:   string(priority),
				Origin:     string(origin),
				Citations:  string(citations),
			})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("storing the profile's aspects: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	row.Aspects, err = s.aspectRows(row.ID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// fail records a Failed profile version and returns the error to show.
//
// The failure is a row rather than only an error because it has to be visible
// and retryable: the PRD requires a failed profile to stay on screen and
// support retry or manual entry.
func (s *ClassifyService) fail(in ClassifyInput, reason string, revision int, modelName string, cause error) error {
	identity := profile.Identity{
		SchemaVersion: profile.SchemaVersion,
		PromptVersion: profile.PromptVersion,
		Revision:      revision,
	}
	if _, err := s.store(in, identity, modelName, nil, models.ProfileFailed, reason); err != nil {
		return fmt.Errorf("%w (and recording the failure also failed: %w)", cause, err)
	}
	return cause
}

func (s *ClassifyService) nextVersion(tx *gorm.DB, kind profile.SubjectKind, subjectID uint) (int, error) {
	var highest int
	err := tx.Model(&models.Profile{}).
		Where("subject_kind = ? AND subject_id = ?", kind, subjectID).
		Select("COALESCE(MAX(version), 0)").Scan(&highest).Error
	if err != nil {
		return 0, fmt.Errorf("finding the next profile version: %w", err)
	}
	return highest + 1, nil
}

// Current returns a subject's newest profile version, or nil when it has none.
func (s *ClassifyService) Current(kind profile.SubjectKind, subjectID uint) (*models.Profile, error) {
	var row models.Profile
	err := s.db.Where("subject_kind = ? AND subject_id = ?", kind, subjectID).
		Order("version desc").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading the current profile: %w", err)
	}
	row.Aspects, err = s.aspectRows(row.ID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// History returns every version for a subject, oldest first.
func (s *ClassifyService) History(kind profile.SubjectKind, subjectID uint) ([]models.Profile, error) {
	rows := []models.Profile{}
	err := s.db.Where("subject_kind = ? AND subject_id = ?", kind, subjectID).
		Order("version asc").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing profile versions: %w", err)
	}
	return rows, nil
}

// aspectRows loads a profile's stored aspects in order.
func (s *ClassifyService) aspectRows(profileID uint) ([]models.ProfileAspect, error) {
	rows := []models.ProfileAspect{}
	err := s.db.Where("profile_id = ?", profileID).Order("ordinal asc").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing aspects of profile %d: %w", profileID, err)
	}
	return rows, nil
}

// Aspects returns a profile's aspects in the contract's own shape, for callers
// that are going to validate or re-store them.
func (s *ClassifyService) Aspects(profileID uint) ([]profile.Aspect, error) {
	rows, err := s.aspectRows(profileID)
	if err != nil {
		return nil, err
	}
	out := make([]profile.Aspect, 0, len(rows))
	for _, r := range rows {
		a := profile.Aspect{
			Type:     profile.AspectType(r.Type),
			Wording:  r.Wording,
			Priority: profile.Priority(r.Priority),
			Origin:   profile.Origin(r.Origin),
		}
		if err := json.Unmarshal([]byte(r.Structured), &a.Structured); err != nil {
			return nil, fmt.Errorf("reading aspect %d's structured value: %w", r.ID, err)
		}
		if err := json.Unmarshal([]byte(r.Citations), &a.Citations); err != nil {
			return nil, fmt.Errorf("reading aspect %d's citations: %w", r.ID, err)
		}
		out = append(out, a)
	}
	return out, nil
}

// loadSources fetches the chunks a classification may read, in the order asked
// for, and is the only thing that defines what a citation can resolve against.
func (s *ClassifyService) loadSources(ids []uint) ([]profile.Source, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows := []models.Chunk{}
	if err := s.db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("loading source chunks: %w", err)
	}
	byID := make(map[uint]models.Chunk, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	out := make([]profile.Source, 0, len(ids))
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			out = append(out, profile.Source{ChunkID: r.ID, Text: r.Text})
		}
	}
	return out, nil
}

// orEmptyObject keeps a nil map out of the stored JSON, so "{}" always means
// "the source did not say" rather than "null, somehow".
func orEmptyObject(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
