package main

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// InitiativeService exposes the initiative lifecycle to the frontend.
type InitiativeService struct {
	db *gorm.DB
}

// NewInitiativeService returns an InitiativeService backed by db.
func NewInitiativeService(db *gorm.DB) *InitiativeService {
	return &InitiativeService{db: db}
}

// Create persists a new initiative and returns it. candidateIDs must hold
// exactly one existing candidate for a Job Search initiative and none for the
// other types: it is a slice so that "more than one" is a rejectable request
// rather than a shape the caller cannot express.
func (s *InitiativeService) Create(name string, initiativeType models.InitiativeType, candidateIDs []uint) (*models.Initiative, error) {
	initiative := &models.Initiative{Name: name, Type: initiativeType, Status: models.InitiativeActive}
	var err error
	if initiative.Name, err = requireName(name); err != nil {
		return nil, err
	}
	if !initiativeType.Valid() {
		return nil, fmt.Errorf("unknown initiative type %q", initiativeType)
	}

	if initiativeType == models.InitiativeTypeJobSearch {
		if len(candidateIDs) != 1 {
			return nil, fmt.Errorf("a job search initiative needs exactly one candidate, got %d", len(candidateIDs))
		}
		if err := s.requireCandidate(candidateIDs[0]); err != nil {
			return nil, err
		}
		initiative.CandidateID = &candidateIDs[0]
	} else if len(candidateIDs) > 0 {
		return nil, fmt.Errorf("a %s initiative does not take a candidate", initiativeType)
	}

	if err := s.db.Create(initiative).Error; err != nil {
		return nil, fmt.Errorf("creating initiative: %w", err)
	}
	return initiative, nil
}

// List returns initiatives oldest first. Archived ones are left out unless
// asked for, so the sidebar does not grow forever.
func (s *InitiativeService) List(includeArchived bool) ([]models.Initiative, error) {
	q := s.db.Order("created_at asc, id asc")
	if !includeArchived {
		q = q.Where("status = ?", models.InitiativeActive)
	}
	initiatives := []models.Initiative{}
	if err := q.Find(&initiatives).Error; err != nil {
		return nil, fmt.Errorf("listing initiatives: %w", err)
	}
	return initiatives, nil
}

// Get returns one initiative by ID.
func (s *InitiativeService) Get(id uint) (*models.Initiative, error) {
	var initiative models.Initiative
	if err := s.db.First(&initiative, id).Error; err != nil {
		return nil, fmt.Errorf("loading initiative %d: %w", id, err)
	}
	return &initiative, nil
}

// Rename changes an initiative's label. Duplicate names are allowed: the name
// is a label, and the identifier is the initiative's own.
func (s *InitiativeService) Rename(id uint, name string) (*models.Initiative, error) {
	name, err := requireName(name)
	if err != nil {
		return nil, err
	}
	initiative, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(initiative).Update("name", name).Error; err != nil {
		return nil, fmt.Errorf("renaming initiative %d: %w", id, err)
	}
	return initiative, nil
}

// Archive moves an Active initiative to Archived, keeping every reference.
func (s *InitiativeService) Archive(id uint) (*models.Initiative, error) {
	return s.setStatus(id, models.InitiativeActive, models.InitiativeArchived)
}

// Reopen moves an Archived initiative back to Active.
func (s *InitiativeService) Reopen(id uint) (*models.Initiative, error) {
	return s.setStatus(id, models.InitiativeArchived, models.InitiativeActive)
}

// setStatus performs one lifecycle transition, rejecting it when the initiative
// is not in the expected state.
func (s *InitiativeService) setStatus(id uint, from, to models.InitiativeStatus) (*models.Initiative, error) {
	initiative, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if initiative.Status != from {
		return nil, fmt.Errorf("initiative %d is already %s", id, initiative.Status)
	}
	if err := s.db.Model(initiative).Update("status", to).Error; err != nil {
		return nil, fmt.Errorf("setting initiative %d to %s: %w", id, to, err)
	}
	return initiative, nil
}

// Delete removes an initiative and the rows it exclusively owns. It never
// removes a candidate, role, company, contact, or recruiter-added artifact —
// those are shared records that outlive any one initiative.
//
// The initiative owns nothing yet; every later phase that gives it owned rows
// adds them to this transaction, and TestDeleteLeavesSharedRecords is what
// catches the day one of them cascades into a shared table by accident.
func (s *InitiativeService) Delete(id uint) error {
	if _, err := s.Get(id); err != nil {
		return err
	}
	if err := s.db.Delete(&models.Initiative{}, id).Error; err != nil {
		return fmt.Errorf("deleting initiative %d: %w", id, err)
	}
	return nil
}

// requireCandidate reports whether the candidate exists, with a message the UI
// can show as-is.
func (s *InitiativeService) requireCandidate(id uint) error {
	var n int64
	if err := s.db.Model(&models.Candidate{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return fmt.Errorf("checking candidate %d: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("candidate %d does not exist", id)
	}
	return nil
}

// requireName trims and rejects an empty initiative name.
func requireName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("initiative name must not be empty")
	}
	return trimmed, nil
}
