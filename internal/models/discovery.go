package models

import "time"

// Search is one query actually sent to a provider.
//
// It holds the visible query because reproducing a search needs it, and the
// initiative is where the recruiter already has that information. The audit
// event that records the same disclosure deliberately holds none of it.
type Search struct {
	ID           uint   `gorm:"primarykey" json:"id"`
	InitiativeID uint   `gorm:"not null" json:"initiativeId"`
	Provider     string `gorm:"not null" json:"provider"`
	// Task is what the search was for: a role search or a people search.
	Task string `gorm:"not null;default:'role_search'" json:"task"`
	// Query is exactly what was sent, byte for byte with what was previewed.
	Query        string `gorm:"not null" json:"query"`
	ResultCount  int    `gorm:"not null;default:0" json:"resultCount"`
	SkippedCount int    `gorm:"not null;default:0" json:"skippedCount"`
	// Partial says some records could not be read, so an incomplete answer is
	// never presented as a complete one.
	Partial       bool      `gorm:"not null;default:false" json:"partial"`
	FailureReason string    `gorm:"not null;default:''" json:"failureReason"`
	SentAt        time.Time `gorm:"not null" json:"sentAt"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// DisclosureEvent records that information left the machine.
//
// It records *that*, not *what*. There is no query column, no result column,
// and no content column, because this is the record most likely to be exported,
// reviewed, or retained longest — and a query in it is a candidate's
// professional shape sitting in a compliance artifact forever.
type DisclosureEvent struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	OccurredAt time.Time `gorm:"not null" json:"occurredAt"`
	Provider   string    `gorm:"not null" json:"provider"`
	Task       string    `gorm:"not null" json:"task"`
	// Categories names the kinds of information disclosed — "professional
	// requirements" — never the information.
	Categories   string `gorm:"not null;default:''" json:"categories"`
	InitiativeID *uint  `json:"initiativeId"`
	CandidateID  *uint  `json:"candidateId"`
	RoleID       *uint  `json:"roleId"`
	// DraftID is set for a copy-out event. Like every other reference here it
	// is an identifier: this table records that something happened, never what
	// it said.
	DraftID   *uint     `json:"draftId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// The tasks a disclosure can be for. Role search is the only one this phase
// creates; the cloud tasks arrive with Phase 18.
const (
	TaskRoleSearch = "role_search"
	// TaskCandidateSourcing sends a role's requirements to find people.
	TaskCandidateSourcing = "candidate_sourcing"
	// TaskCandidateEnrich sends one public handle to fetch its public footprint.
	TaskCandidateEnrich = "candidate_enrich"
)

// The providers that can receive a disclosure.
const (
	ProviderExa    = "exa"
	ProviderGitHub = "github"
)

// Discovery failure codes. Short, lowercase, and carrying nothing of the query.
const (
	ReasonRateLimited   = "rate_limited"
	ReasonSearchTimeout = "timeout"
	ReasonOffline       = "offline"
	ReasonUnauthorized  = "unauthorized"
	ReasonMalformed     = "malformed_response"
)
