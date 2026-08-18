package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/criteria"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// CriteriaService holds the recruiter's search intent.
//
// Two rules meet here and they are the same mechanism seen twice. Criteria are
// the only thing that drives discovery, so criteria are the only place to keep
// intent separate from evidence, and the only place to stop an unlawful search.
//
// The block is deterministic and the warning is not, deliberately. A model
// asked "is this discriminatory?" is a model that will sometimes say no; a
// model that hard-blocks on its own judgement will eventually block something
// lawful with no recourse. So the list refuses, and the model only warns.
type CriteriaService struct {
	db       *gorm.DB
	registry *ModelService
	model    Classifier
	profiles *CandidateProfileService
}

// NewCriteriaService wires criteria to the registry and the profile gate.
func NewCriteriaService(db *gorm.DB, registry *ModelService, model Classifier, profiles *CandidateProfileService) *CriteriaService {
	return &CriteriaService{db: db, registry: registry, model: model, profiles: profiles}
}

// proxyTimeout bounds the advisory check. Short, because it is advisory: a slow
// model must not make adding a criterion feel broken.
const proxyTimeout = 20 * time.Second

// CriterionInput is one criterion the recruiter wrote.
//
// Note what it does not contain: anything a model produced. Every write path to
// criteria takes an explicit recruiter action as its whole input, which is what
// makes "no model output can create a criterion" an invariant rather than a
// workflow note.
type CriterionInput struct {
	InitiativeID uint   `json:"initiativeId"`
	Text         string `json:"text"`
	Priority     string `json:"priority"`
}

// Add stores one criterion, after the deterministic block and the advisory
// check.
func (s *CriteriaService) Add(in CriterionInput) (*models.SearchCriterion, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return nil, fmt.Errorf("a criterion needs something to say")
	}
	if err := checkPriority(in.Priority); err != nil {
		return nil, err
	}
	// Before anything else and before any model: a refusal must not depend on
	// an endpoint being up.
	if found := criteria.Check(text); found != nil {
		return nil, found
	}
	if in.InitiativeID == 0 {
		return nil, fmt.Errorf("a criterion belongs to an initiative")
	}

	warning := s.proxyWarning(text)
	row := models.SearchCriterion{
		InitiativeID: in.InitiativeID,
		Text:         text,
		Priority:     in.Priority,
		Warning:      warning,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var next int
		err := tx.Model(&models.SearchCriterion{}).
			Where("initiative_id = ?", in.InitiativeID).
			Select("COALESCE(MAX(position), -1) + 1").Scan(&next).Error
		if err != nil {
			return fmt.Errorf("finding the next criterion position: %w", err)
		}
		row.Position = next
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("storing the criterion: %w", err)
		}
		return bumpCriteriaVersion(tx, in.InitiativeID)
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Edit changes a criterion's wording or priority, as a content change.
func (s *CriteriaService) Edit(id uint, text, priority string) (*models.SearchCriterion, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("a criterion needs something to say")
	}
	if err := checkPriority(priority); err != nil {
		return nil, err
	}
	if found := criteria.Check(text); found != nil {
		return nil, found
	}
	var row models.SearchCriterion
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, fmt.Errorf("loading criterion %d: %w", id, err)
	}
	// Re-evaluated because the wording changed, and stored — not recomputed on
	// read, so the warning cannot move under the reader.
	warning := s.proxyWarning(text)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&models.SearchCriterion{}).Where("id = ?", id).Updates(map[string]any{
			"text":     text,
			"priority": priority,
			"warning":  warning,
		}).Error
		if err != nil {
			return fmt.Errorf("updating criterion %d: %w", id, err)
		}
		return bumpCriteriaVersion(tx, row.InitiativeID)
	})
	if err != nil {
		return nil, err
	}
	row.Text, row.Priority, row.Warning = text, priority, warning
	return &row, nil
}

// Remove deletes a criterion. A content change, so the version moves.
func (s *CriteriaService) Remove(id uint) error {
	var row models.SearchCriterion
	if err := s.db.First(&row, id).Error; err != nil {
		return fmt.Errorf("loading criterion %d: %w", id, err)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.SearchCriterion{}, id).Error; err != nil {
			return fmt.Errorf("removing criterion %d: %w", id, err)
		}
		return bumpCriteriaVersion(tx, row.InitiativeID)
	})
}

// Reorder sets the presentation order.
//
// It deliberately does not bump the version: ordering is not weighting, and a
// version bump on reorder would invalidate every assessment because somebody
// dragged a row — which teaches recruiters that the staleness indicator means
// nothing.
func (s *CriteriaService) Reorder(initiativeID uint, orderedIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		rows := []models.SearchCriterion{}
		if err := tx.Where("initiative_id = ?", initiativeID).Find(&rows).Error; err != nil {
			return fmt.Errorf("loading criteria: %w", err)
		}
		known := make(map[uint]bool, len(rows))
		for _, r := range rows {
			known[r.ID] = true
		}
		for _, id := range orderedIDs {
			if !known[id] {
				return fmt.Errorf("criterion %d does not belong to this initiative", id)
			}
		}
		for position, id := range orderedIDs {
			err := tx.Model(&models.SearchCriterion{}).Where("id = ?", id).
				Update("position", position).Error
			if err != nil {
				return fmt.Errorf("repositioning criterion %d: %w", id, err)
			}
		}
		return nil
	})
}

// List returns an initiative's criteria in the recruiter's order.
func (s *CriteriaService) List(initiativeID uint) ([]models.SearchCriterion, error) {
	rows := []models.SearchCriterion{}
	err := s.db.Where("initiative_id = ?", initiativeID).
		Order("position asc, id asc").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing criteria for initiative %d: %w", initiativeID, err)
	}
	return rows, nil
}

// Version reports the criteria version in force.
//
// An initiative with no criteria still has one, so a result made against no
// criteria is distinguishable from one made against some.
func (s *CriteriaService) Version(initiativeID uint) (int, error) {
	var row models.CriteriaVersion
	err := s.db.Where("initiative_id = ?", initiativeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("loading the criteria version: %w", err)
	}
	return row.Version, nil
}

// Blocked returns the provisional protected categories, so a screen can say
// what cannot be a criterion rather than only that something is refused.
func (s *CriteriaService) Blocked() []string {
	cats := criteria.Categories()
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, string(c))
	}
	return out
}

// Proposal is a criterion the application suggests. It is not a criterion.
type Proposal struct {
	Text     string `json:"text"`
	Priority string `json:"priority"`
	// From names the aspect it was drawn from, so the recruiter can see why.
	From string `json:"from"`
}

// proposableTypes are the aspect types that describe what a person can do or
// needs, rather than where they have been.
//
// Location and compensation are absent, and so are the history-bearing readings
// of experience: the PRD's rule is that preferences are never inferred from
// resume history, and the structural form of that rule is that the proposer
// never sees those aspects at all. A recruiter who wants a location criterion
// types one, which is a person deciding.
var proposableTypes = []profile.AspectType{
	profile.Skill,
	profile.Responsibility,
	profile.Qualification,
	profile.Seniority,
	profile.WorkRights,
	profile.EmploymentType,
	profile.WorkArrangement,
}

// institutionMarkers are the words that turn a proposable aspect back into
// history. A qualification is a proposable thing to want — "a postgraduate
// qualification in computer science" — right up until it names the school, at
// which point it is a fact about where this person has been.
//
// ponytail: a word list over the wording, checked after the type filter. It is
// the second of two coarse filters and the recruiter's review is the third; a
// structured "institution" field on the aspect would be the real fix, and Phase
// 10's taxonomy deliberately does not have one.
var institutionMarkers = []string{
	"university", "college", "school", "institute", "academy", "polytechnic",
	"tafe", "campus",
}

// namesAnInstitution reports whether wording identifies a particular place
// someone studied or worked, rather than a thing they can do.
func namesAnInstitution(text string) bool {
	lowered := strings.ToLower(text)
	for _, marker := range institutionMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	// "Senior engineer at Northwind" — the employer, named.
	if strings.Contains(lowered, " at ") {
		return true
	}
	return false
}

// Propose suggests criteria from a candidate's approved profile and writes
// nothing at all.
func (s *CriteriaService) Propose(initiativeID, candidateID uint) ([]Proposal, error) {
	ready, err := s.profiles.Readiness(candidateID)
	if err != nil {
		return nil, err
	}
	if !ready.Ready {
		return nil, fmt.Errorf("criteria can only be proposed from an approved profile: %s", ready.Reason)
	}
	approved, err := s.profiles.Approved(candidateID)
	if err != nil || approved == nil {
		return nil, fmt.Errorf("this candidate has no approved profile")
	}

	existing, err := s.List(initiativeID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(existing))
	for _, c := range existing {
		seen[strings.ToLower(strings.TrimSpace(c.Text))] = true
	}

	out := []Proposal{}
	for _, a := range approved.Aspects {
		if !slices.Contains(proposableTypes, profile.AspectType(a.Type)) {
			continue
		}
		text := strings.TrimSpace(a.Wording)
		if text == "" || seen[strings.ToLower(text)] {
			continue
		}
		// A proposal that could not be stored is not worth showing.
		if criteria.Check(text) != nil {
			continue
		}
		// Type is not enough on its own: a qualification that names the school
		// is history wearing a proposable type.
		if namesAnInstitution(text) {
			continue
		}
		seen[strings.ToLower(text)] = true
		out = append(out, Proposal{
			Text:     text,
			Priority: models.CriterionNiceToHave,
			From:     a.Type,
		})
	}
	return out, nil
}

// Apply turns the proposals the recruiter chose into criteria.
//
// The chosen ones are named by the caller. Nothing applies a proposal because
// it looked good, and there is no path from a model's output to this method
// that does not pass through a person picking items off a list.
func (s *CriteriaService) Apply(initiativeID uint, chosen []Proposal) ([]models.SearchCriterion, error) {
	out := []models.SearchCriterion{}
	for _, p := range chosen {
		priority := p.Priority
		if priority == "" {
			priority = models.CriterionNiceToHave
		}
		row, err := s.Add(CriterionInput{
			InitiativeID: initiativeID,
			Text:         p.Text,
			Priority:     priority,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	return out, nil
}

// proxyWarning asks the classify model whether a criterion might be a proxy for
// a protected attribute.
//
// It returns an empty string on any failure, including no model at all. An
// unavailable model must not become a block: blocks are deterministic, and a
// block that depends on an endpoint being up is not.
func (s *CriteriaService) proxyWarning(text string) string {
	if s.model == nil {
		return ""
	}
	res, err := s.registry.Resolve(models.RoleClassify)
	if err != nil || res.Assignment == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), proxyTimeout)
	defer cancel()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"proxy":  map[string]any{"type": "boolean"},
			"reason": map[string]any{"type": "string"},
		},
		"required":             []any{"proxy", "reason"},
		"additionalProperties": false,
	}
	prompt := "You review one recruitment search criterion for a possible indirect proxy for a " +
		"protected attribute (age, sex, gender identity, sexual orientation, race or national " +
		"origin, religion, disability, family or carer status, pregnancy, marital status, " +
		"political opinion, union membership).\n\n" +
		"Answer proxy=true only when the wording would in practice select for one of those " +
		"without naming it — for example \"digital native\" or \"recent graduate\" for age. " +
		"Ordinary professional requirements, and lawful right-to-work requirements, are not " +
		"proxies. Give one short sentence of reason.\n\n" +
		"Criterion: " + text

	raw, err := s.model.Chat(ctx, res.Assignment.Model, prompt, schema)
	if err != nil {
		return ""
	}
	var out struct {
		Proxy  bool   `json:"proxy"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil || !out.Proxy {
		return ""
	}
	reason := strings.TrimSpace(out.Reason)
	if reason == "" {
		reason = "this wording may act as a proxy for a protected attribute"
	}
	return reason
}

// checkPriority refuses anything but the two the recruiter may choose.
func checkPriority(priority string) error {
	if priority != models.CriterionMustHave && priority != models.CriterionNiceToHave {
		return fmt.Errorf("a criterion is must-have or nice-to-have, got %q", priority)
	}
	return nil
}

// bumpCriteriaVersion records that the intent changed.
func bumpCriteriaVersion(tx *gorm.DB, initiativeID uint) error {
	var row models.CriteriaVersion
	err := tx.Where("initiative_id = ?", initiativeID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = models.CriteriaVersion{InitiativeID: initiativeID, Version: 2}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("recording the criteria version: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("loading the criteria version: %w", err)
	}
	err = tx.Model(&models.CriteriaVersion{}).Where("initiative_id = ?", initiativeID).
		Update("version", row.Version+1).Error
	if err != nil {
		return fmt.Errorf("bumping the criteria version: %w", err)
	}
	return nil
}
