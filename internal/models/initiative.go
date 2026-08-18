// Package models defines the GORM data models persisted to SQLite.
package models

import "time"

// InitiativeType enumerates the kinds of initiative a user can create.
type InitiativeType string

// The set of valid initiative types.
const (
	InitiativeTypeJobSearch           InitiativeType = "job_search"
	InitiativeTypeTalentSearch        InitiativeType = "talent_search"
	InitiativeTypeBusinessDevelopment InitiativeType = "business_development"
)

// Valid reports whether t is one of the known initiative types.
func (t InitiativeType) Valid() bool {
	switch t {
	case InitiativeTypeJobSearch, InitiativeTypeTalentSearch, InitiativeTypeBusinessDevelopment:
		return true
	}
	return false
}

// InitiativeStatus is an initiative's lifecycle state. Active and Archived are
// the only two, and an initiative moves freely between them.
type InitiativeStatus string

// The initiative lifecycle states.
const (
	InitiativeActive   InitiativeStatus = "active"
	InitiativeArchived InitiativeStatus = "archived"
)

// Valid reports whether s is one of the two lifecycle states.
func (s InitiativeStatus) Valid() bool {
	return s == InitiativeActive || s == InitiativeArchived
}

// Initiative is a high-level unit of work, similar to an agent session in an
// IDE: it owns everything the user does toward one goal, and references the
// shared records — candidates, roles, companies, contacts — that outlive it.
type Initiative struct {
	ID     uint             `gorm:"primarykey" json:"id"`
	Name   string           `gorm:"not null" json:"name"`
	Type   InitiativeType   `gorm:"not null;index" json:"type"`
	Status InitiativeStatus `gorm:"not null;index;default:active" json:"status"`
	// A Job Search initiative has exactly one candidate; the other types have
	// none. One column is the whole "at most one" rule — it cannot hold two.
	CandidateID *uint     `json:"candidateId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
