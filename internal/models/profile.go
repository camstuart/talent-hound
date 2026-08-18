package models

import "time"

// ProfileState is where a profile is in its lifecycle.
//
// Proposed and Approved matter from Phase 11, where a Candidate Profile must be
// approved before search or matching; Failed exists here because the contract
// can fail and the failure has to be visible and retryable.
type ProfileState string

// The profile lifecycle states.
const (
	ProfileProposed ProfileState = "proposed"
	ProfileApproved ProfileState = "approved"
	ProfileFailed   ProfileState = "failed"
)

// Profile is one versioned derived record about one subject.
//
// It carries everything that could change what it means: the schema it was
// shaped by, the prompt that asked for it, the model revision that answered,
// and a hash of the sources it read. A profile without those is a claim with no
// way to tell whether it is still about the same thing.
type Profile struct {
	ID uint `gorm:"primarykey" json:"id"`
	// SubjectKind is "candidate" or "role"; SubjectID is that record's id.
	SubjectKind string `gorm:"not null" json:"subjectKind"`
	SubjectID   uint   `gorm:"not null" json:"subjectId"`
	// Version counts up per subject. Nothing overwrites: a later classification
	// adds a version, and Phase 11 diffs them.
	Version       int    `gorm:"not null" json:"version"`
	State         string `gorm:"not null;default:'proposed'" json:"state"`
	SchemaVersion string `gorm:"not null" json:"schemaVersion"`
	PromptVersion string `gorm:"not null" json:"promptVersion"`
	// ModelRevision is the classify assignment revision that answered, which is
	// why Phase 8 made those append-only.
	ModelRevision int    `gorm:"not null" json:"modelRevision"`
	ModelName     string `gorm:"not null;default:''" json:"modelName"`
	SourceHash    string `gorm:"not null" json:"sourceHash"`
	// Identity is the hash of the four above: two profiles with the same
	// identity are the same derived record.
	Identity      string `gorm:"not null" json:"identity"`
	FailureReason string `gorm:"not null;default:''" json:"failureReason"`
	// ApprovedAt is when a person said yes, and ApprovedSourceHash is what they
	// said yes about. Staleness is the comparison between the second and the
	// evidence in force now — it is not a stored state, because a stored state
	// needs something to notice, and that something is what goes missing.
	ApprovedAt         *time.Time      `json:"approvedAt"`
	ApprovedSourceHash string          `gorm:"not null;default:''" json:"approvedSourceHash"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
	Aspects            []ProfileAspect `gorm:"-" json:"aspects"`
}

// ProfileAspect is one typed, citable statement within a profile.
type ProfileAspect struct {
	ID        uint `gorm:"primarykey" json:"id"`
	ProfileID uint `gorm:"not null" json:"profileId"`
	// Ordinal keeps the classifier's order, which is roughly the source's.
	Ordinal int    `gorm:"not null" json:"ordinal"`
	Type    string `gorm:"not null" json:"type"`
	// Wording as the source put it. Text a stranger wrote: displayed, never
	// rendered, never executed.
	Wording string `gorm:"not null" json:"wording"`
	// Structured is the normalized value as JSON, restricted to the fields
	// defined for Type. "{}" means the source did not say, which is a fact.
	Structured string `gorm:"not null;default:'{}'" json:"structured"`
	Priority   string `gorm:"not null;default:'unspecified'" json:"priority"`
	Origin     string `gorm:"not null;default:'extracted'" json:"origin"`
	// Citations is the evidence, as JSON: chunk and quote for extracted
	// aspects, a record reference for recruiter supplied ones.
	Citations string    `gorm:"not null;default:'[]'" json:"citations"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Classification failure codes. Short, lowercase, and carrying nothing of the
// source document — a failure reason is not a place a stranger's text belongs.
const (
	ReasonNoClassifyModel = "no_classify_model"
	ReasonInvalidProposal = "invalid_proposal"
	ReasonClassifyFailed  = "classify_failed"
	ReasonNoSources       = "no_sources"
)
