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

// Initiative is a high-level unit of work, similar to an agent session in an
// IDE: it owns everything the user does toward one goal.
type Initiative struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Name      string         `gorm:"not null" json:"name"`
	Type      InitiativeType `gorm:"not null;index" json:"type"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}
