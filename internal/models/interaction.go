package models

import (
	"fmt"
	"time"
)

// Interaction is one thing that happened with a record: a call, a note, a
// placement. The recruiter's own words, so — unlike an artifact — it is
// editable; the companion artifact is replaced on every edit so search always
// reflects the current wording.
type Interaction struct {
	ID uint `gorm:"primarykey" json:"id"`
	// The record this happened with. Initiative is not a valid subject: an
	// interaction happens with a person or organisation, and the initiative it
	// happened under is context, carried by InitiativeID.
	TargetType LinkTarget `gorm:"not null" json:"targetType"`
	TargetID   uint       `gorm:"not null" json:"targetId"`
	Kind       string     `gorm:"not null" json:"kind"`
	// The recruiter's words. Free text: displayed, never rendered.
	Note string `gorm:"not null" json:"note"`
	// When it happened — distinct from CreatedAt, which is when it was logged.
	OccurredAt Date  `gorm:"not null" json:"occurredAt"`
	RoleID     *uint `json:"roleId"`
	InitiativeID *uint `json:"initiativeId"`
	// The evidence copy of the note. Owned by this row: replaced on edit,
	// deleted with it, never detached or renamed on its own.
	ArtifactID uint      `gorm:"not null" json:"artifactId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

var interactionKinds = []string{"call", "meeting", "email", "note", "placement", "application", "rejection"}

// outcomeKinds are the kinds that assert something about a role, so they
// require one.
var outcomeKinds = map[string]bool{"placement": true, "application": true, "rejection": true}

// InteractionKinds returns the valid kinds, in display order.
func InteractionKinds() []string { return interactionKinds }

// Validate normalises i in place and reports the first problem found.
func (i *Interaction) Validate() error {
	if !i.TargetType.Valid() || i.TargetType == LinkInitiative {
		return fmt.Errorf("interaction target must be a candidate, contact, company, or role")
	}
	if i.TargetID == 0 {
		return fmt.Errorf("interaction target record is required")
	}
	known := false
	for _, k := range interactionKinds {
		if i.Kind == k {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("unknown interaction kind %q", i.Kind)
	}
	var err error
	if i.Note, err = requireText("interaction note", i.Note); err != nil {
		return err
	}
	if i.OccurredAt == "" {
		return fmt.Errorf("interaction date is required")
	}
	if err := i.OccurredAt.Validate("interaction date"); err != nil {
		return err
	}
	if outcomeKinds[i.Kind] && (i.RoleID == nil || *i.RoleID == 0) {
		return fmt.Errorf("a %s needs the role it is about", i.Kind)
	}
	return nil
}
