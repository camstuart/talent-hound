package main

import (
	"fmt"
	"strings"

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
	// Guard refuses personal-data entry in demo scope and on an unencrypted
	// volume. It is checked here, at the write, rather than in the interface:
	// the interface is not the only caller.
	Guard DataGuard
}

// DataGuard decides whether this installation may hold candidate data. It is
// set at startup; nil means unguarded, which is what a test with no setup
// service is.
type DataGuard interface {
	AllowRealData() error
}

func guardAllows(g DataGuard) error {
	if g == nil {
		return nil
	}
	return g.AllowRealData()
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
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
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
	// Editing is entry too. The guard can turn on after the records exist — a
	// volume stops being encrypted, or the recruiter moves to demo scope — and
	// guarding only creation would leave every existing candidate an open field
	// for typing a real name into.
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
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
	// A contact is a person: a full name, an email address and a phone number.
	// The guard was on candidates and artifacts and not on this, so the one
	// record type made entirely of direct identifiers was the one that could be
	// written to an unencrypted disk.
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
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
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
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

// CandidateFilter is the structured search over candidates. Text matches
// names, emails, and location; the rest are exact or range filters. This is a
// filter and behaves like one — the semantic search over evidence is
// SearchService.People, deliberately a different box in the UI.
type CandidateFilter struct {
	Text           string      `json:"text"`
	WorkRights     string      `json:"workRights"`
	EmploymentType string      `json:"employmentType"`
	Arrangement    string      `json:"arrangement"`
	AvailableBy    models.Date `json:"availableBy"`
}

// SearchCandidates returns the candidates matching every given filter, by name.
func (s *RecordService) SearchCandidates(f CandidateFilter) ([]models.Candidate, error) {
	q := s.db.Order("full_name asc")
	if t := strings.TrimSpace(f.Text); t != "" {
		like := "%" + t + "%"
		q = q.Where(
			"full_name LIKE ? COLLATE NOCASE OR preferred_name LIKE ? COLLATE NOCASE"+
				" OR emails LIKE ? COLLATE NOCASE OR location LIKE ? COLLATE NOCASE",
			like, like, like, like)
	}
	if f.WorkRights != "" {
		q = q.Where("work_rights = ?", f.WorkRights)
	}
	if f.EmploymentType != "" {
		q = q.Where("desired_employment_type = ?", f.EmploymentType)
	}
	if f.Arrangement != "" {
		q = q.Where("desired_work_arrangement = ?", f.Arrangement)
	}
	if f.AvailableBy != "" {
		if err := f.AvailableBy.Validate("available-by date"); err != nil {
			return nil, err
		}
		// Unknown availability is kept: a filter must not hide people the
		// recruiter simply has not dated yet.
		q = q.Where("availability = '' OR availability <= ?", f.AvailableBy)
	}
	out := []models.Candidate{}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("searching candidates: %w", err)
	}
	return out, nil
}

// SearchCompanies returns companies whose name matches text, by name.
func (s *RecordService) SearchCompanies(text string) ([]models.Company, error) {
	out := []models.Company{}
	q := s.db.Order("name asc")
	if t := strings.TrimSpace(text); t != "" {
		q = q.Where("name LIKE ? COLLATE NOCASE", "%"+t+"%")
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("searching companies: %w", err)
	}
	return out, nil
}

// SearchContacts returns contacts whose name or email matches text, by name.
func (s *RecordService) SearchContacts(text string) ([]models.Contact, error) {
	out := []models.Contact{}
	q := s.db.Order("full_name asc")
	if t := strings.TrimSpace(text); t != "" {
		like := "%" + t + "%"
		q = q.Where("full_name LIKE ? COLLATE NOCASE OR email LIKE ? COLLATE NOCASE", like, like)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("searching contacts: %w", err)
	}
	return out, nil
}
