package models

import "time"

// OwnerKind names what a vector was made from. Source chunks are the only kind
// that exists now; Profile Aspects arrive in Phase 10 and share this storage
// rather than getting a table of their own.
type OwnerKind string

// The retrieval unit kinds this build knows.
const (
	OwnerChunk  OwnerKind = "chunk"
	OwnerAspect OwnerKind = "aspect"
)

// Valid reports whether k is a kind this build knows.
func (k OwnerKind) Valid() bool { return k == OwnerChunk || k == OwnerAspect }

// MetricCosine is the only similarity this build computes. It is stored on the
// space rather than assumed, because a space that does not say what its numbers
// mean is the thing this phase exists to avoid.
const MetricCosine = "cosine"

// EmbeddingSpace is what makes two vectors comparable.
//
// The identity is every column below taken together, and the assignment
// revision is the anchor: endpoint, model, and digest are the visible identity,
// but a parameter change alters the geometry while altering no visible name.
// Phase 8 already made "the configuration changed" into a durable, append-only
// number, so naming that number here means the two phases cannot disagree.
type EmbeddingSpace struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	Endpoint string `gorm:"not null" json:"endpoint"`
	Model    string `gorm:"not null" json:"model"`
	// Empty is honest: the endpoint reported no digest. It does not make two
	// configurations one, because the revision already separated them.
	Digest string `gorm:"not null;default:''" json:"digest"`
	// Revision is the embed assignment revision that produced this space.
	Revision int `gorm:"not null" json:"revision"`
	// Dimensions is learned from the first successful embedding and enforced
	// forever after. A configured count would be a second source of truth about
	// something the endpoint already knows, and getting it wrong fails silently.
	Dimensions int    `gorm:"not null" json:"dimensions"`
	Metric     string `gorm:"not null" json:"metric"`
	// Normalized records whether the endpoint returns unit vectors. Cosine does
	// not need it; a later phase choosing dot product would.
	Normalized bool      `gorm:"not null;default:false" json:"normalized"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Embedding is one retrieval unit's vector within one space.
type Embedding struct {
	ID      uint `gorm:"primarykey" json:"id"`
	SpaceID uint `gorm:"not null" json:"spaceId"`
	// What this vector was made from.
	OwnerKind OwnerKind `gorm:"not null" json:"ownerKind"`
	OwnerID   uint      `gorm:"not null" json:"ownerId"`
	// Repeated from the space so a read can check the blob without a join.
	Dimensions int `gorm:"not null" json:"dimensions"`
	// Little-endian float32, exactly 4×Dimensions bytes. Never sent to the
	// frontend: it is several kilobytes of numbers nothing over there can use.
	Vector    []byte    `gorm:"not null" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Embedding failure codes. They follow the job reason-code rules: short,
// lowercase, and carrying nothing of what was being embedded.
const (
	ReasonEmbedFailed    = "embed_failed"
	ReasonNoEmbedModel   = "no_embed_model"
	ReasonBadVector      = "bad_vector"
	ReasonDimsMismatch   = "dimensions_mismatch"
	ReasonMissingOwner   = "missing_owner"
	ReasonEndpointFailed = "endpoint_failed"
)
