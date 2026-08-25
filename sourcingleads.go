package main

import (
	"fmt"
	"net/url"
	"strings"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// LeadView is a lead as the list shows it: the row, plus the name of the
// candidate it was recognised as and the host it came from, so the panel needs
// no second query.
type LeadView struct {
	models.Lead
	CandidateName string `json:"candidateName"`
	Host          string `json:"host"`
}

// Leads lists an initiative's leads, newest search first. An empty state lists
// every state.
func (s *SourcingService) Leads(initiativeID uint, state string) ([]LeadView, error) {
	rows := []models.Lead{}
	q := s.db.Where("initiative_id = ?", initiativeID)
	if state != "" {
		q = q.Where("state = ?", state)
	}
	if err := q.Order("search_id desc, id asc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("listing leads: %w", err)
	}
	out := make([]LeadView, 0, len(rows))
	for _, lead := range rows {
		view := LeadView{Lead: lead, Host: hostOf(lead.URL)}
		if lead.CandidateID != nil {
			var c models.Candidate
			if err := s.db.Select("full_name").First(&c, *lead.CandidateID).Error; err == nil {
				view.CandidateName = c.FullName
			}
		}
		out = append(out, view)
	}
	return out, nil
}

// Dismiss marks a lead as not wanted. The row stays, so a resend shows it as
// already seen rather than as new.
func (s *SourcingService) Dismiss(leadID uint) error {
	lead, err := s.lead(leadID)
	if err != nil {
		return err
	}
	if lead.State == models.LeadPromoted {
		return fmt.Errorf("this lead was promoted — dismiss the candidate instead")
	}
	if err := s.db.Model(lead).Update("state", models.LeadDismissed).Error; err != nil {
		return fmt.Errorf("dismissing the lead: %w", err)
	}
	return nil
}

// Suggest guesses the candidate fields a lead's title supports, for the
// recruiter to correct. It writes nothing, and it does not read the snippet:
// a name is on the page's title or it is the recruiter's to type.
func (s *SourcingService) Suggest(leadID uint) (*models.Candidate, error) {
	lead, err := s.lead(leadID)
	if err != nil {
		return nil, err
	}
	name := lead.Title
	for _, sep := range []string{" — ", " – ", " - ", " | ", " · ", ", ", " ("} {
		if i := strings.Index(name, sep); i > 0 {
			name = name[:i]
		}
	}
	name = strings.TrimSpace(name)
	if provider, handle := identityFromURL(lead.URL); provider == models.IdentityGitHub && strings.EqualFold(name, handle) {
		// A login is not a name.
		name = ""
	}
	return &models.Candidate{
		FullName:   name,
		SourceNote: "Sourced from " + lead.Provider + " on " + s.now().Format("2006-01-02"),
	}, nil
}

// Promote turns a lead into a candidate, in one step: the record, an identity
// for the page, and the snippet kept as evidence under the candidate.
//
// The recruiter's corrected fields are what is saved; the lead's own text is
// evidence, not fields.
func (s *SourcingService) Promote(leadID uint, in models.Candidate) (*models.Candidate, error) {
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
	lead, err := s.lead(leadID)
	if err != nil {
		return nil, err
	}
	if lead.State != models.LeadNew {
		return nil, fmt.Errorf("this lead is %s and cannot be promoted", lead.State)
	}
	if lead.CandidateID != nil {
		return nil, fmt.Errorf("this lead is already someone in the pool — open that candidate instead")
	}
	in.ID = 0
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if in.SourceNote == "" {
		in.SourceNote = "Sourced from " + lead.Provider + " on " + s.now().Format("2006-01-02")
	}

	// The snippet becomes an unattached artifact first; the transaction then
	// links it. A failed transaction leaves an orphan, which is removed below.
	body := "# " + lead.Title + "\n\n" + lead.URL + "\n\n" + lead.Snippet + "\n"
	title := lead.Title
	if title == "" {
		title = lead.URL
	}
	artifact, err := s.artifacts.create(title, "", lead.Provider+":"+lead.URL, []byte(body), "", 0)
	if err != nil {
		return nil, err
	}

	candidate := in
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&candidate).Error; err != nil {
			return fmt.Errorf("creating the candidate: %w", err)
		}
		provider, handle := identityFromURL(lead.URL)
		if provider == "" {
			provider, handle = models.IdentityWebsite, hostOf(lead.URL)
		}
		identity := models.Identity{
			CandidateID: candidate.ID, Provider: provider, Handle: handle, URL: lead.URL,
		}
		if err := identity.Validate(); err != nil {
			return err
		}
		if err := tx.Create(&identity).Error; err != nil {
			return fmt.Errorf("recording the identity: %w", err)
		}
		link := models.ArtifactLink{
			ArtifactID: artifact.ID, TargetType: models.LinkCandidate, TargetID: candidate.ID,
		}
		if err := tx.Create(&link).Error; err != nil {
			return fmt.Errorf("attaching the evidence: %w", err)
		}
		return tx.Model(lead).Updates(map[string]any{
			"state": models.LeadPromoted, "candidate_id": candidate.ID,
		}).Error
	})
	if err != nil {
		_ = s.db.Delete(&models.Artifact{}, artifact.ID).Error
		return nil, err
	}
	return &candidate, nil
}

func (s *SourcingService) lead(id uint) (*models.Lead, error) {
	var lead models.Lead
	if err := s.db.First(&lead, id).Error; err != nil {
		return nil, fmt.Errorf("finding lead %d: %w", id, err)
	}
	return &lead, nil
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}
