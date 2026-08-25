package models

import (
	"fmt"
	"strings"
	"time"
)

// The places a candidate can be identified. LinkedIn is here so a surfaced
// profile URL can be remembered; nothing ever fetches it.
const (
	IdentityGitHub   = "github"
	IdentityWebsite  = "website"
	IdentityLinkedIn = "linkedin"
	IdentityHN       = "hn"
)

var identityProviders = []string{IdentityGitHub, IdentityWebsite, IdentityLinkedIn, IdentityHN}

// IdentityProviders returns the valid providers, in display order.
func IdentityProviders() []string { return identityProviders }

// Identity is one public handle a candidate is known by. It is how a search
// result is recognised as someone already in the pool, and how enrichment
// finds the same person again — a durable key rather than a note.
type Identity struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	CandidateID uint   `gorm:"not null;index" json:"candidateId"`
	Provider    string `gorm:"not null" json:"provider"`
	// Handle is the login, domain, or profile slug — unique per provider.
	Handle string `gorm:"not null" json:"handle"`
	URL    string `gorm:"not null" json:"url"`
	// VerifiedAt is the last day the provider confirmed the handle exists.
	VerifiedAt Date      `json:"verifiedAt"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Validate normalises i in place and reports the first problem found.
func (i *Identity) Validate() error {
	if i.CandidateID == 0 {
		return fmt.Errorf("an identity belongs to a candidate")
	}
	if !contains(identityProviders, i.Provider) {
		return fmt.Errorf("unknown identity provider %q", i.Provider)
	}
	handle := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(i.Handle), "@"))
	if i.Provider == IdentityGitHub {
		// Logins are case-insensitive on GitHub; one spelling is stored so the
		// unique index means what it says.
		handle = strings.ToLower(handle)
	}
	var err error
	if i.Handle, err = requireText("identity handle", handle); err != nil {
		return err
	}
	if i.URL, err = requireText("identity URL", i.URL); err != nil {
		return err
	}
	if i.URL, err = requireAbsoluteURL("identity URL", i.URL); err != nil {
		return err
	}
	return i.VerifiedAt.Validate("identity verified date")
}
