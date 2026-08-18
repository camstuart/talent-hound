package models

import "time"

// SearchCriterion is one thing the recruiter is looking for, in one initiative.
//
// It belongs to the initiative rather than to a candidate on purpose: a resume
// saying someone worked at Northwind is not a statement that they want to
// again, and treating history as preference is how a search quietly narrows to
// more of the same without anyone deciding that.
type SearchCriterion struct {
	ID           uint `gorm:"primarykey" json:"id"`
	InitiativeID uint `gorm:"not null" json:"initiativeId"`
	// Position is presentation. The PRD is explicit that ordering is not
	// weighting, so reordering does not change the criteria version.
	Position int `gorm:"not null" json:"position"`
	// Text the recruiter wrote. Displayed, never rendered, never executed.
	Text string `gorm:"not null" json:"text"`
	// Priority is must_have or nice_to_have. There is no unspecified: a
	// criterion is a choice, and an unweighted one would be a preference nobody
	// expressed.
	Priority string `gorm:"not null" json:"priority"`
	// Warning is an advisory recorded when the criterion was written — a
	// possible proxy for a protected attribute. It never blocks, and it is not
	// recalculated on read, because a warning that appears and disappears as the
	// loaded model changes is a warning nobody will trust.
	Warning   string    `gorm:"not null;default:''" json:"warning"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName is explicit because GORM's pluraliser turns SearchCriterion into
// "search_criterions", and the migration — which is the source of truth for the
// schema — names the table search_criteria.
func (SearchCriterion) TableName() string { return "search_criteria" }

// CriteriaVersion identifies the search intent in force for an initiative, so a
// derived assessment can record which intent produced it.
type CriteriaVersion struct {
	InitiativeID uint      `gorm:"primarykey" json:"initiativeId"`
	Version      int       `gorm:"not null;default:1" json:"version"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// TableName keeps GORM from pluralising this into criteria_versions_versions or
// similar; the migration names the table.
func (CriteriaVersion) TableName() string { return "criteria_versions" }

// The two criterion priorities. Unlike role aspects there is no unspecified.
const (
	CriterionMustHave   = "must_have"
	CriterionNiceToHave = "nice_to_have"
)
