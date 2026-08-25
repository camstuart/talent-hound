package models

import (
	"fmt"
	"time"
)

// The states a lead moves through. There is no "contacted": once a person is
// in the pool, what happens with them is an Interaction on the candidate.
const (
	LeadNew       = "new"
	LeadPromoted  = "promoted"
	LeadDismissed = "dismissed"
)

var leadStates = []string{LeadNew, LeadPromoted, LeadDismissed}

// leadProviders are the providers that can return a lead.
var leadProviders = []string{ProviderExa}

// Lead is one search result that might be a person worth adding to the pool.
//
// It is deliberately not a Candidate. A candidate is a shared record the
// recruiter has chosen to keep; a lead is a URL, a title, and a snippet the
// provider returned, held only until the recruiter promotes or dismisses it.
// Automatically creating people from web hits would fill the pool with records
// nobody asked for, each one personal data with no one accountable for it.
type Lead struct {
	ID uint `gorm:"primarykey" json:"id"`
	// SearchID is the Search that produced this lead: how a lead is reproduced.
	SearchID     uint   `gorm:"not null;index" json:"searchId"`
	InitiativeID uint   `gorm:"not null;index" json:"initiativeId"`
	RoleID       *uint  `json:"roleId"`
	Provider     string `gorm:"not null" json:"provider"`
	// SourceID is the provider's own identifier, when it gives one.
	SourceID string `json:"sourceId"`
	URL      string `gorm:"not null;index" json:"url"`
	Title    string `json:"title"`
	// Snippet is the provider's text: displayed, never rendered.
	Snippet string `json:"snippet"`
	State   string `gorm:"not null;default:'new'" json:"state"`
	// CandidateID is set when the lead was promoted, or when it was recognised
	// as someone already in the pool.
	CandidateID *uint     `json:"candidateId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Validate normalises l in place and reports the first problem found.
func (l *Lead) Validate() error {
	if l.SearchID == 0 {
		return fmt.Errorf("a lead belongs to a search")
	}
	if l.InitiativeID == 0 {
		return fmt.Errorf("a lead belongs to an initiative")
	}
	if !contains(leadProviders, l.Provider) {
		return fmt.Errorf("unknown lead provider %q", l.Provider)
	}
	var err error
	if l.URL, err = requireText("lead URL", l.URL); err != nil {
		return err
	}
	if l.URL, err = requireAbsoluteURL("lead URL", l.URL); err != nil {
		return err
	}
	if l.State == "" {
		l.State = LeadNew
	}
	if !contains(leadStates, l.State) {
		return fmt.Errorf("unknown lead state %q", l.State)
	}
	return nil
}

func contains(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}
