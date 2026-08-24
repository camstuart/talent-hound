package models

import (
	"slices"
	"time"
)

// ModelRole is one of the three local assignments the registry always holds.
type ModelRole string

// The model roles, per the PRD.
const (
	// RoleEmbed produces source chunk and Profile Aspect embeddings.
	RoleEmbed ModelRole = "embed"
	// RoleClassify decomposes profiles and flags prohibited criteria. It
	// defaults to the generate model by inheritance, not by a copied row.
	RoleClassify ModelRole = "classify"
	// RoleGenerate writes assessments, summaries, drafts, and chat.
	RoleGenerate ModelRole = "generate"
)

// ModelRoles is every role, so the settings view and the tests can walk them.
var ModelRoles = []ModelRole{RoleEmbed, RoleClassify, RoleGenerate}

// Valid reports whether r is a known role.
func (r ModelRole) Valid() bool { return slices.Contains(ModelRoles, r) }

// ValidationStatus is whether a model has passed the held-out benchmarks. The
// application does not judge a model at runtime; this records whether someone
// else did.
type ValidationStatus string

// The validation statuses. Everything starts Unvalidated.
const (
	Unvalidated ValidationStatus = "unvalidated"
	Validated   ValidationStatus = "validated"
)

// Valid reports whether v is a known status.
func (v ValidationStatus) Valid() bool { return v == Unvalidated || v == Validated }

// ModelAssignment is one configuration a role has had. Rows are append-only:
// the current assignment for a role is its highest revision.
//
// Immutability is the point. Phase 9 identifies an embedding space by endpoint
// revision and model digest, and an identifier that points at a row somebody
// can edit is not an identifier.
type ModelAssignment struct {
	ID   uint      `gorm:"primarykey" json:"id"`
	Role ModelRole `gorm:"not null" json:"role"`
	// Revision counts this role's configurations from one. It changes when the
	// endpoint, model, digest, or parameters change, and not otherwise.
	Revision int `gorm:"not null" json:"revision"`
	// Always the local endpoint: the cloud is a task-level override in a later
	// phase, never a required role here.
	Endpoint string `gorm:"not null" json:"endpoint"`
	Model    string `gorm:"not null" json:"model"`
	// The endpoint's immutable identifier for the model, when it reports one.
	// Empty is honest: it means the endpoint did not say.
	Digest string `json:"digest"`
	// Role parameters as JSON. Settings and numbers, never content.
	Params     string           `gorm:"not null;default:'{}'" json:"params"`
	Validation ValidationStatus `gorm:"not null;default:'unvalidated'" json:"validation"`
	// What proved this model good enough, when anything did. Phase 21 owns
	// benchmarks; until then nothing can fill this in.
	BenchmarkRef string    `json:"benchmarkRef"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// SameConfig reports whether two assignments describe the same configuration.
// Re-assigning what a role already has must not create a revision: the settings
// screen's save button is not a reason to invalidate derived data.
func (a ModelAssignment) SameConfig(b ModelAssignment) bool {
	return a.Endpoint == b.Endpoint && a.Model == b.Model &&
		a.Digest == b.Digest && a.Params == b.Params
}

// Model availability codes. They follow the job reason-code rules, and they are
// distinct because the recruiter's next action differs for each.
const (
	// ModelReady is the endpoint reachable and the model installed.
	ModelReady = "ready"
	// ModelUnassigned is a role with no assignment — not a missing model.
	ModelUnassigned = "unassigned"
	// ModelEndpointDown is the local endpoint refusing connections.
	ModelEndpointDown = "endpoint_unavailable"
	// ModelMissing is a reachable endpoint that does not have the model.
	ModelMissing = "model_missing"
	// ModelPullDeclined is the recruiter having said no, for this session.
	ModelPullDeclined = "pull_declined"
	// ModelPullFailed is a pull that was attempted and did not finish.
	ModelPullFailed = "pull_failed"
	// ModelPulling is a download underway right now.
	ModelPulling = "pulling"
	// ModelTimeout is an endpoint that accepted the connection and went quiet.
	ModelTimeout = "timeout"
	// ModelMalformed is an answer that was not the shape the contract says.
	ModelMalformed = "malformed_response"
	// ModelOutOfMemory is a model that will not load in the memory available.
	ModelOutOfMemory = "out_of_memory"
)
