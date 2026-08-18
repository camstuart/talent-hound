package models

import "time"

// Chunk is one addressable piece of an artifact's extracted Markdown: the unit
// retrieval returns and citations resolve through.
//
// StartOffset and EndOffset are byte offsets into that Markdown, and the
// contract is that they select exactly Text. That is checked when a citation is
// resolved, not assumed — an offset that is trusted rather than verified is a
// citation pointing confidently at the wrong sentence.
type Chunk struct {
	ID         uint `gorm:"primarykey" json:"id"`
	ArtifactID uint `gorm:"not null" json:"artifactId"`
	// Position within its artifact, from zero, without gaps.
	Ordinal int `gorm:"not null" json:"ordinal"`
	// Text a stranger wrote: displayed, never rendered, never executed.
	Text        string `gorm:"not null" json:"text"`
	StartOffset int    `gorm:"not null" json:"startOffset"`
	EndOffset   int    `gorm:"not null" json:"endOffset"`
	// The headings above this chunk, outermost first.
	HeadingPath StringList `gorm:"not null;default:'[]'" json:"headingPath"`
	// Whitespace-separated words, not a model's tokens — see the chunker
	// parameters, which name the method that produced this number.
	TokenCount int    `gorm:"not null" json:"tokenCount"`
	Hash       string `gorm:"not null" json:"hash"`
	// Which chunker made this, so a later version can find what it made stale.
	Chunker        string    `gorm:"not null" json:"chunker"`
	ChunkerVersion string    `gorm:"not null" json:"chunkerVersion"`
	ChunkerParams  string    `gorm:"not null;default:'{}'" json:"chunkerParams"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Chunking failure codes. They follow the job reason-code rules.
const (
	ReasonNotExtracted = "not_extracted"
	ReasonChunkFailed  = "chunk_failed"
)
