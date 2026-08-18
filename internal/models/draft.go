package models

import "time"

// Draft is a pitch or an outreach message the recruiter will send themselves.
//
// The application drafts; the recruiter sends. That division is the product's
// position rather than a limitation to remove later — an application that could
// send outreach can make a mistake nobody caught, in someone's name, to a
// person who did not ask to hear from it.
type Draft struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	InitiativeID uint   `gorm:"not null" json:"initiativeId"`
	CandidateID  *uint  `json:"candidateId"`
	RoleID       *uint  `json:"roleId"`
	Kind         string `gorm:"not null" json:"kind"`
	// State is active or discarded. Copying is neither: it is an event.
	State   string `gorm:"not null;default:'active'" json:"state"`
	Subject string `gorm:"not null;default:''" json:"subject"`
	// Body is what the recruiter will paste. Text a model wrote about a real
	// person: displayed, never rendered, and edited before it goes anywhere.
	Body string `gorm:"not null" json:"body"`
	// Claims is the claim-to-evidence map as it was at generation, as JSON. It
	// is not recomputed on read: re-running retrieval against text the
	// recruiter has since edited would describe a different draft.
	Claims    string    `gorm:"not null;default:'[]'" json:"claims"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Copies is how many times it has been copied out, loaded for display.
	Copies int `gorm:"-" json:"copies"`
}

// The two draft states, and nothing else.
const (
	DraftActive    = "active"
	DraftDiscarded = "discarded"
)

// The two draft kinds.
const (
	DraftPitch    = "pitch"
	DraftOutreach = "outreach"
)

// Answer is one question asked in one initiative, and what came back.
type Answer struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	InitiativeID uint   `gorm:"not null" json:"initiativeId"`
	Question     string `gorm:"not null" json:"question"`
	// Answer is empty when Supported is false: an unsupported answer carries no
	// factual assertion, because the alternative to "I cannot find that" is
	// invention.
	Answer string `gorm:"not null;default:''" json:"answer"`
	// Supported says the evidence backs it.
	Supported bool      `gorm:"not null;default:false" json:"supported"`
	Citations string    `gorm:"not null;default:'[]'" json:"citations"`
	AskedAt   time.Time `gorm:"not null" json:"askedAt"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Proposals are structured changes the answer suggested. They are values on
	// their way to a screen, never rows.
	Proposals []string `gorm:"-" json:"proposals"`
}

// TaskCopiedOut is the audit task a copy records. There is deliberately no
// "sent": the application cannot send, and an audit vocabulary that could
// express one would be a vocabulary someone reads as a capability.
const TaskCopiedOut = "copied_out"

// Q&A and drafting failure codes.
const (
	ReasonNoAnswerModel     = "no_answer_model"
	ReasonUnsupportedAnswer = "unsupported_answer"
	ReasonBadDraft          = "bad_draft"
)

// CloudConsent is one approval: one initiative, one endpoint revision, one
// task.
//
// There is no row that could match more broadly, which is what makes "consent
// does not generalize" a property of the schema rather than of a query someone
// might write differently next time.
type CloudConsent struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	InitiativeID     uint      `gorm:"not null" json:"initiativeId"`
	EndpointRevision int       `gorm:"not null" json:"endpointRevision"`
	Task             string    `gorm:"not null" json:"task"`
	ApprovedAt       time.Time `gorm:"not null" json:"approvedAt"`
	// RevokedAt is set when the recruiter takes it back. The row stays, so what
	// was permitted remains answerable.
	RevokedAt *time.Time `json:"revokedAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// CloudEndpointRow is one cloud configuration, append-only like the model
// registry's assignments — a revision has to identify something that cannot
// change under the approvals pointing at it.
type CloudEndpointRow struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	URL       string    `gorm:"not null" json:"url"`
	Model     string    `gorm:"not null;default:''" json:"model"`
	Revision  int       `gorm:"not null" json:"revision"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName is explicit because GORM would pluralise this into
// "cloud_endpoint_rows"; the migration names the table.
func (CloudEndpointRow) TableName() string { return "cloud_endpoints" }
