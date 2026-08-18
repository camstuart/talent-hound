package models

import "time"

// MaxArtifactBytes is the per-file input limit: 25 MiB. Exactly at the limit is
// accepted; one byte more is refused.
const MaxArtifactBytes = 25 * 1024 * 1024

// LinkTarget is the kind of record an artifact is attached to.
type LinkTarget string

// The record types an artifact can be linked to.
const (
	LinkInitiative LinkTarget = "initiative"
	LinkCandidate  LinkTarget = "candidate"
	LinkRole       LinkTarget = "role"
	LinkCompany    LinkTarget = "company"
	LinkContact    LinkTarget = "contact"
)

// Valid reports whether t is a known link target.
func (t LinkTarget) Valid() bool {
	switch t {
	case LinkInitiative, LinkCandidate, LinkRole, LinkCompany, LinkContact:
		return true
	}
	return false
}

// Artifact is one ingestion occurrence: the original bytes plus the provenance
// that makes them evidence. Two ingestions of identical bytes are two
// artifacts, deliberately — filename, source, and capture time differ.
//
// Everything but DisplayName is immutable, enforced by there being no code path
// that writes it after creation.
type Artifact struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	DisplayName string `gorm:"not null" json:"displayName"`
	// As supplied by whoever ingested it: provenance, never a filesystem path.
	// Empty for pasted text, which has no filename.
	OriginalFilename string `json:"originalFilename"`
	// Detected from the bytes; the filename extension is not consulted.
	MediaType  string    `gorm:"not null" json:"mediaType"`
	ByteLength int64     `gorm:"not null" json:"byteLength"`
	SHA256     string    `gorm:"column:sha256;not null;index" json:"sha256"`
	Source     string    `json:"source"`
	CapturedAt time.Time `gorm:"not null" json:"capturedAt"`
	// Bytes is omitted from listings; read it with the service's Bytes method.
	Bytes []byte `gorm:"not null" json:"-"`

	// What happened when something tried to read this artifact. There is no
	// "running" here: in progress is the job's business.
	ExtractionState ExtractionState `gorm:"not null;default:'pending'" json:"extractionState"`
	// A short code, never a parser's own words — those quote the document.
	ExtractionError string `json:"extractionError"`
	// Who produced Markdown, and which build of them. Provenance, so a later
	// phase can find extractions made by a version it no longer trusts.
	Extractor        string `json:"extractor"`
	ExtractorVersion string `json:"extractorVersion"`
	// Markdown is omitted from listings, like Bytes; read it with the
	// extraction service. Whoever wrote the document chose its contents, so it
	// is text and only text: never rendered, never executed.
	Markdown string `json:"-"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// MaxMarkdownBytes is the extraction contract's ceiling: 10 MB of Markdown per
// artifact, whatever produced it.
const MaxMarkdownBytes = 10 << 20

// ExtractionState is how far an artifact has got towards being readable.
type ExtractionState string

// The extraction states. Anything in progress is a job, not a state here.
const (
	ExtractionPending   ExtractionState = "pending"
	ExtractionExtracted ExtractionState = "extracted"
	ExtractionFailed    ExtractionState = "failed"
)

// Extraction failure codes. They follow the job reason-code rules, and the same
// CHECK constraint shape enforces them in the database.
const (
	ReasonSidecarMissing = "sidecar_missing"
	ReasonSidecarVersion = "sidecar_version"
	ReasonUnsupported    = "unsupported_type"
	ReasonExtractTimeout = "extract_timeout"
	ReasonExtractMemory  = "extract_memory"
	ReasonExtractOutput  = "extract_output"
	ReasonExtractEmpty   = "extract_empty"
	ReasonExtractFailed  = "extract_failed"
)

// ArtifactLink attaches an artifact to one record without copying its bytes.
type ArtifactLink struct {
	ID         uint       `gorm:"primarykey" json:"id"`
	ArtifactID uint       `gorm:"not null" json:"artifactId"`
	TargetType LinkTarget `gorm:"not null" json:"targetType"`
	TargetID   uint       `gorm:"not null" json:"targetId"`
	// Historical marks a role's superseded source. The artifact stays visible
	// so an earlier citation still resolves, and leaves current retrieval so a
	// match is never made against a listing that has been replaced.
	Historical bool      `gorm:"not null;default:false" json:"historical"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
