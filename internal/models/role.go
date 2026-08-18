package models

import (
	"fmt"
	"strings"
	"time"
)

// RoleOrigin records how a role entered the system, which decides which
// lifecycle states it can occupy.
type RoleOrigin string

// The two role origins.
const (
	RoleOriginRecruiterEntered RoleOrigin = "recruiter_entered"
	RoleOriginDiscovered       RoleOrigin = "discovered"
)

// Valid reports whether o is a known origin.
func (o RoleOrigin) Valid() bool {
	return o == RoleOriginRecruiterEntered || o == RoleOriginDiscovered
}

// RoleLifecycle is a role's lifecycle state. The two origins have disjoint
// state sets: a discovered role goes stale and is purged, a recruiter-entered
// one is filled or closed.
type RoleLifecycle string

// The lifecycle states, grouped by the origin that can use them.
const (
	RoleActive RoleLifecycle = "active"
	RoleStale  RoleLifecycle = "stale"
	RolePurged RoleLifecycle = "purged"

	RoleOpen   RoleLifecycle = "open"
	RoleFilled RoleLifecycle = "filled"
	RoleClosed RoleLifecycle = "closed"
)

// ValidFor reports whether s is a state a role of that origin can occupy.
func (s RoleLifecycle) ValidFor(origin RoleOrigin) bool {
	switch origin {
	case RoleOriginDiscovered:
		return s == RoleActive || s == RoleStale || s == RolePurged
	case RoleOriginRecruiterEntered:
		return s == RoleOpen || s == RoleFilled || s == RoleClosed
	}
	return false
}

// Role is a recruiter-entered or publicly discovered hiring requirement. Its
// responsibilities and requirements are profile aspects, not columns here.
type Role struct {
	ID    uint   `gorm:"primarykey" json:"id"`
	Title string `gorm:"not null" json:"title"`
	// A discovered role names an employer that may have no Company record yet,
	// so the name stands on its own and the reference is optional.
	CompanyName     string `json:"companyName"`
	CompanyID       *uint  `gorm:"index" json:"companyId"`
	Location        string `json:"location"`
	WorkArrangement string `json:"workArrangement"`
	EmploymentType  string `json:"employmentType"`
	// Compensation or rate, when the source states one.
	Compensation   Compensation  `gorm:"embedded;embeddedPrefix:comp_" json:"compensation"`
	PublishedOn    Date          `json:"publishedOn"`
	ClosingOn      Date          `json:"closingOn"`
	RetrievedOn    Date          `json:"retrievedOn"`
	SourceID       string        `json:"sourceId"`
	CanonicalURL   string        `json:"canonicalUrl"`
	Source         string        `json:"source"`
	Origin         RoleOrigin    `gorm:"not null" json:"origin"`
	LifecycleState RoleLifecycle `gorm:"not null" json:"lifecycleState"`
	// ContentHash fingerprints the current source content, so rediscovering a
	// listing can tell "nothing happened" from "the listing changed".
	ContentHash string `gorm:"not null;default:''" json:"contentHash"`
	// RetrievedAt is when the listing was last seen. Staleness is measured from
	// it, against a clock the caller supplies.
	RetrievedAt *time.Time `json:"retrievedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Validate normalises r in place and reports the first problem found.
func (r *Role) Validate() error {
	var err error
	if r.Title, err = requireText("role title", r.Title); err != nil {
		return err
	}
	if !r.Origin.Valid() {
		return fmt.Errorf("unknown role origin %q", r.Origin)
	}
	if !r.LifecycleState.ValidFor(r.Origin) {
		return fmt.Errorf("lifecycle state %q is not valid for a %s role", r.LifecycleState, r.Origin)
	}
	if r.CanonicalURL, err = requireAbsoluteURL("role canonical URL", r.CanonicalURL); err != nil {
		return err
	}
	r.CompanyName = strings.TrimSpace(r.CompanyName)
	r.Location = strings.TrimSpace(r.Location)
	r.WorkArrangement = strings.TrimSpace(r.WorkArrangement)
	r.EmploymentType = strings.TrimSpace(r.EmploymentType)
	r.SourceID = strings.TrimSpace(r.SourceID)
	r.Source = strings.TrimSpace(r.Source)
	for field, d := range map[string]Date{
		"role published date": r.PublishedOn,
		"role closing date":   r.ClosingOn,
		"role retrieved date": r.RetrievedOn,
	} {
		if err := d.Validate(field); err != nil {
			return err
		}
	}
	if r.ClosingOn.Before(r.PublishedOn) {
		return fmt.Errorf("role closing date %s precedes its published date %s", r.ClosingOn, r.PublishedOn)
	}
	return r.Compensation.Validate("role compensation")
}
