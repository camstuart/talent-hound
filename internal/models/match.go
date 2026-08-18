package models

import "time"

// Match is one candidate assessed against one role, in both directions.
//
// It is a conclusion, and a conclusion is only valid while the inputs that
// produced it are — which is what InputHash records. There is no timestamp
// rule, no age, and no heuristic: the hash matches or the match is recomputed.
type Match struct {
	ID           uint `gorm:"primarykey" json:"id"`
	InitiativeID uint `gorm:"not null" json:"initiativeId"`
	CandidateID  uint `gorm:"not null" json:"candidateId"`
	RoleID       uint `gorm:"not null" json:"roleId"`
	// InputHash is the sole caching rule.
	InputHash string `gorm:"not null" json:"inputHash"`
	// The counts ranking needs, summed across both directions.
	UnmetMustHaves   int `gorm:"not null;default:0" json:"unmetMustHaves"`
	UnknownMustHaves int `gorm:"not null;default:0" json:"unknownMustHaves"`
	MetNiceToHaves   int `gorm:"not null;default:0" json:"metNiceToHaves"`
	// RetrievalPosition is the shortlist position this role came from.
	RetrievalPosition int       `gorm:"not null;default:0" json:"retrievalPosition"`
	FailureReason     string    `gorm:"not null;default:''" json:"failureReason"`
	AssessedAt        time.Time `gorm:"not null" json:"assessedAt"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	// Results is loaded alongside for display; it is not a GORM association.
	Results []MatchResult `gorm:"-" json:"results"`
	// Stale says the recomputed hash no longer matches. Computed, never stored.
	Stale bool `gorm:"-" json:"stale"`
	// RoleTitle is carried for display so the panel needs no second query.
	RoleTitle string `gorm:"-" json:"roleTitle"`
}

// MatchResult is one requirement's outcome in one direction.
type MatchResult struct {
	ID        uint   `gorm:"primarykey" json:"id"`
	MatchID   uint   `gorm:"not null" json:"matchId"`
	Direction string `gorm:"not null" json:"direction"`
	Ordinal   int    `gorm:"not null" json:"ordinal"`
	// Requirement is what was assessed, in the source's own words.
	Requirement string `gorm:"not null" json:"requirement"`
	Priority    string `gorm:"not null;default:'unspecified'" json:"priority"`
	// Result is met, not_met, or unknown, and nothing else.
	Result string `gorm:"not null" json:"result"`
	// Reason is the assessor's short explanation. Text a model wrote about text
	// a stranger wrote: displayed, never rendered.
	Reason string `gorm:"not null;default:''" json:"reason"`
	// Citations is the evidence, as JSON. A met result with none of it never
	// reaches this table.
	Citations string    `gorm:"not null;default:'[]'" json:"citations"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Assessment failure codes. Short, lowercase, and carrying nothing of the
// evidence or the requirement.
const (
	ReasonNoGenerateModel = "no_generate_model"
	ReasonUncitedMet      = "uncited_met"
	ReasonBadResultState  = "bad_result_state"
	ReasonBadCitation     = "unresolvable_citation"
	ReasonAssessFailed    = "assess_failed"
	ReasonNotAssessable   = "not_assessable"
)
