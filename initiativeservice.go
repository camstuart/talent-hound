package main

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// InitiativeService exposes initiative CRUD to the frontend.
type InitiativeService struct {
	db *gorm.DB
}

// NewInitiativeService returns an InitiativeService backed by db.
func NewInitiativeService(db *gorm.DB) *InitiativeService {
	return &InitiativeService{db: db}
}

// Create persists a new initiative and returns it.
func (s *InitiativeService) Create(name string, initiativeType models.InitiativeType) (*models.Initiative, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("initiative name must not be empty")
	}
	if !initiativeType.Valid() {
		return nil, fmt.Errorf("unknown initiative type %q", initiativeType)
	}
	initiative := &models.Initiative{Name: name, Type: initiativeType}
	if err := s.db.Create(initiative).Error; err != nil {
		return nil, fmt.Errorf("creating initiative: %w", err)
	}
	return initiative, nil
}

// List returns all initiatives, oldest first.
func (s *InitiativeService) List() ([]models.Initiative, error) {
	var initiatives []models.Initiative
	if err := s.db.Order("created_at asc, id asc").Find(&initiatives).Error; err != nil {
		return nil, fmt.Errorf("listing initiatives: %w", err)
	}
	return initiatives, nil
}
