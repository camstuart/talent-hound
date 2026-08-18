package main

import (
	"fmt"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// RecordService exposes the shared CRM records — candidates, roles, companies,
// and contacts — to the frontend. They are shared by reference: an initiative
// points at them, and deleting the initiative never removes them.
//
// ponytail: one service for four record types. Split it when one of them grows
// behaviour the others do not share.
type RecordService struct {
	db *gorm.DB
}

// NewRecordService returns a RecordService backed by db.
func NewRecordService(db *gorm.DB) *RecordService {
	return &RecordService{db: db}
}

// Deletion of these records is deliberately absent: every deletion invariant —
// blocked-while-referenced, cascade, and the shared-artifact warning — lands
// together in Phase 19. A half-enforced deletion rule is worse than none.

// CreateCandidate validates and persists a candidate.
func (s *RecordService) CreateCandidate(candidate models.Candidate) (*models.Candidate, error) {
	candidate.ID = 0
	if err := candidate.Validate(); err != nil {
		return nil, err
	}
	if err := s.db.Create(&candidate).Error; err != nil {
		return nil, fmt.Errorf("creating candidate: %w", err)
	}
	return &candidate, nil
}

// UpdateCandidate validates and saves an existing candidate.
func (s *RecordService) UpdateCandidate(candidate models.Candidate) (*models.Candidate, error) {
	if err := candidate.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.GetCandidate(candidate.ID); err != nil {
		return nil, err
	}
	if err := s.db.Save(&candidate).Error; err != nil {
		return nil, fmt.Errorf("updating candidate %d: %w", candidate.ID, err)
	}
	return &candidate, nil
}

// GetCandidate returns one candidate by ID.
func (s *RecordService) GetCandidate(id uint) (*models.Candidate, error) {
	var candidate models.Candidate
	if err := s.db.First(&candidate, id).Error; err != nil {
		return nil, fmt.Errorf("loading candidate %d: %w", id, err)
	}
	return &candidate, nil
}

// ListCandidates returns every candidate, newest first.
func (s *RecordService) ListCandidates() ([]models.Candidate, error) {
	candidates := []models.Candidate{}
	if err := s.db.Order("id desc").Find(&candidates).Error; err != nil {
		return nil, fmt.Errorf("listing candidates: %w", err)
	}
	return candidates, nil
}

// CreateRole validates and persists a role.
func (s *RecordService) CreateRole(role models.Role) (*models.Role, error) {
	role.ID = 0
	if err := s.validateRole(&role); err != nil {
		return nil, err
	}
	if err := s.db.Create(&role).Error; err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}
	return &role, nil
}

// UpdateRole validates and saves an existing role.
func (s *RecordService) UpdateRole(role models.Role) (*models.Role, error) {
	if err := s.validateRole(&role); err != nil {
		return nil, err
	}
	if _, err := s.GetRole(role.ID); err != nil {
		return nil, err
	}
	if err := s.db.Save(&role).Error; err != nil {
		return nil, fmt.Errorf("updating role %d: %w", role.ID, err)
	}
	return &role, nil
}

// GetRole returns one role by ID.
func (s *RecordService) GetRole(id uint) (*models.Role, error) {
	var role models.Role
	if err := s.db.First(&role, id).Error; err != nil {
		return nil, fmt.Errorf("loading role %d: %w", id, err)
	}
	return &role, nil
}

// ListRoles returns every role, newest first.
func (s *RecordService) ListRoles() ([]models.Role, error) {
	roles := []models.Role{}
	if err := s.db.Order("id desc").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	return roles, nil
}

// validateRole applies the model's own rules and then checks the optional
// company reference, which only the database can answer.
func (s *RecordService) validateRole(role *models.Role) error {
	if err := role.Validate(); err != nil {
		return err
	}
	if role.CompanyID != nil {
		if _, err := s.GetCompany(*role.CompanyID); err != nil {
			return err
		}
	}
	return nil
}

// CreateCompany validates and persists a company.
func (s *RecordService) CreateCompany(company models.Company) (*models.Company, error) {
	company.ID = 0
	if err := company.Validate(); err != nil {
		return nil, err
	}
	if err := s.db.Create(&company).Error; err != nil {
		return nil, fmt.Errorf("creating company: %w", err)
	}
	return &company, nil
}

// UpdateCompany validates and saves an existing company.
func (s *RecordService) UpdateCompany(company models.Company) (*models.Company, error) {
	if err := company.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.GetCompany(company.ID); err != nil {
		return nil, err
	}
	if err := s.db.Save(&company).Error; err != nil {
		return nil, fmt.Errorf("updating company %d: %w", company.ID, err)
	}
	return &company, nil
}

// GetCompany returns one company by ID.
func (s *RecordService) GetCompany(id uint) (*models.Company, error) {
	var company models.Company
	if err := s.db.First(&company, id).Error; err != nil {
		return nil, fmt.Errorf("loading company %d: %w", id, err)
	}
	return &company, nil
}

// ListCompanies returns every company by name.
func (s *RecordService) ListCompanies() ([]models.Company, error) {
	companies := []models.Company{}
	if err := s.db.Order("name asc, id asc").Find(&companies).Error; err != nil {
		return nil, fmt.Errorf("listing companies: %w", err)
	}
	return companies, nil
}

// CreateContact validates and persists a contact at an existing company.
func (s *RecordService) CreateContact(contact models.Contact) (*models.Contact, error) {
	contact.ID = 0
	if err := contact.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.GetCompany(contact.CompanyID); err != nil {
		return nil, err
	}
	if err := s.db.Create(&contact).Error; err != nil {
		return nil, fmt.Errorf("creating contact: %w", err)
	}
	return &contact, nil
}

// UpdateContact validates and saves an existing contact.
func (s *RecordService) UpdateContact(contact models.Contact) (*models.Contact, error) {
	if err := contact.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.GetContact(contact.ID); err != nil {
		return nil, err
	}
	if _, err := s.GetCompany(contact.CompanyID); err != nil {
		return nil, err
	}
	if err := s.db.Save(&contact).Error; err != nil {
		return nil, fmt.Errorf("updating contact %d: %w", contact.ID, err)
	}
	return &contact, nil
}

// GetContact returns one contact by ID.
func (s *RecordService) GetContact(id uint) (*models.Contact, error) {
	var contact models.Contact
	if err := s.db.First(&contact, id).Error; err != nil {
		return nil, fmt.Errorf("loading contact %d: %w", id, err)
	}
	return &contact, nil
}

// ContactsAtCompany is the PoC's whole warm-path story: how many people we
// already know at a company, and who they are.
type ContactsAtCompany struct {
	Company  models.Company   `json:"company"`
	Count    int              `json:"count"`
	Contacts []models.Contact `json:"contacts"`
}

// ContactsAtCompany returns the contacts known at one company. A company with
// no contacts is an empty result; a company that does not exist is an error, so
// a mistyped reference is never read as "nobody there".
func (s *RecordService) ContactsAtCompany(companyID uint) (*ContactsAtCompany, error) {
	company, err := s.GetCompany(companyID)
	if err != nil {
		return nil, err
	}
	contacts := []models.Contact{}
	if err := s.db.Where("company_id = ?", companyID).Order("full_name asc, id asc").Find(&contacts).Error; err != nil {
		return nil, fmt.Errorf("listing contacts at company %d: %w", companyID, err)
	}
	return &ContactsAtCompany{Company: *company, Count: len(contacts), Contacts: contacts}, nil
}
