package main

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/chunk"
	"camstuart/talent-hound/internal/models"
)

// ChunkService cuts extracted Markdown into the chunks everything downstream
// retrieves and cites. It is pure computation over text already in the
// database, so a job carries a list of artifacts and does one per item — which
// is what makes the cancellation guarantee real: cancel a run over eight
// artifacts after three, and three are chunked, five are untouched, and the
// fourth left nothing behind.
type ChunkService struct {
	db   *gorm.DB
	jobs *JobService
}

// chunkParams is what a chunking job carries: identifiers, and nothing else.
type chunkParams struct {
	ArtifactIDs []uint `json:"artifactIds"`
}

// NewChunkService returns a ChunkService and registers the chunk worker.
func NewChunkService(db *gorm.DB, jobs *JobService) *ChunkService {
	s := &ChunkService{db: db, jobs: jobs}
	jobs.register("chunk", s.work)
	return s
}

// Chunk queues one artifact for chunking.
func (s *ChunkService) Chunk(artifactID, initiativeID uint) (*models.Job, error) {
	return s.enqueue([]uint{artifactID}, initiativeID)
}

// ChunkAll queues every extracted artifact linked to an initiative. An
// initiative with nothing extracted yields a job with no items, which completes
// immediately: an empty batch is a batch that finished.
func (s *ChunkService) ChunkAll(initiativeID uint) (*models.Job, error) {
	ids := []uint{}
	err := s.db.Model(&models.Artifact{}).
		Where("extraction_state = ?", models.ExtractionExtracted).
		Where("id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)",
			models.LinkInitiative, initiativeID).
		Order("id asc").Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("listing extracted artifacts for initiative %d: %w", initiativeID, err)
	}
	return s.enqueue(ids, initiativeID)
}

func (s *ChunkService) enqueue(ids []uint, initiativeID uint) (*models.Job, error) {
	params, err := json.Marshal(chunkParams{ArtifactIDs: ids})
	if err != nil {
		return nil, fmt.Errorf("encoding chunking params: %w", err)
	}
	return s.jobs.Enqueue(JobInput{
		Kind:         "chunk",
		InitiativeID: initiativeID,
		Params:       string(params),
		TotalItems:   len(ids),
	})
}

// List returns one artifact's chunks in order.
func (s *ChunkService) List(artifactID uint) ([]models.Chunk, error) {
	chunks := []models.Chunk{}
	err := s.db.Where("artifact_id = ?", artifactID).Order("ordinal asc").Find(&chunks).Error
	if err != nil {
		return nil, fmt.Errorf("listing chunks for artifact %d: %w", artifactID, err)
	}
	return chunks, nil
}

// CountForInitiative reports how many chunks the initiative's artifacts have,
// so the workspace can say whether there is anything to search.
func (s *ChunkService) CountForInitiative(initiativeID uint) (int64, error) {
	var n int64
	err := s.db.Model(&models.Chunk{}).
		Where("artifact_id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)",
			models.LinkInitiative, initiativeID).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("counting chunks for initiative %d: %w", initiativeID, err)
	}
	return n, nil
}

// work chunks one artifact per item. The chunking happens with no transaction
// open; only the replacement of the artifact's rows is inside one.
func (s *ChunkService) work(_ context.Context, job models.Job, item int) (JobCommit, error) {
	var p chunkParams
	if err := json.Unmarshal([]byte(job.Params), &p); err != nil {
		return nil, FailReason("bad_params")
	}
	if item < 0 || item >= len(p.ArtifactIDs) {
		return nil, FailReason(models.ReasonChunkFailed)
	}
	id := p.ArtifactIDs[item]

	var artifact models.Artifact
	err := s.db.Select("id", "extraction_state", "markdown").First(&artifact, id).Error
	if err != nil {
		return nil, FailReason(models.ReasonChunkFailed)
	}
	if artifact.ExtractionState != models.ExtractionExtracted {
		return nil, FailReason(models.ReasonNotExtracted)
	}

	chunks := chunk.Split(artifact.Markdown)
	rows := make([]models.Chunk, 0, len(chunks))
	for _, c := range chunks {
		// Asserted before anything is stored: a chunk whose offsets do not
		// select its own text is a citation waiting to mislead.
		if err := chunk.Verify(artifact.Markdown, c); err != nil {
			return nil, FailReason(models.ReasonChunkFailed)
		}
		rows = append(rows, models.Chunk{
			ArtifactID:     id,
			Ordinal:        c.Ordinal,
			Text:           c.Text,
			StartOffset:    c.Start,
			EndOffset:      c.End,
			HeadingPath:    models.StringList(c.HeadingPath),
			TokenCount:     c.TokenCount,
			Hash:           c.Hash,
			Chunker:        chunk.Name,
			ChunkerVersion: chunk.Version,
			ChunkerParams:  chunk.Params,
		})
	}

	return func(tx *gorm.DB) error {
		// Delete then insert, never a merge: an artifact's chunks are always the
		// product of exactly one chunker version and one set of parameters.
		return replaceChunks(tx, id, rows)
	}, nil
}

// replaceChunks swaps an artifact's chunks for a new set inside tx. The FTS
// index follows through the triggers, so it rolls back with them.
func replaceChunks(tx *gorm.DB, artifactID uint, rows []models.Chunk) error {
	if err := dropChunkVectors(tx, artifactID); err != nil {
		return err
	}
	if err := tx.Where("artifact_id = ?", artifactID).Delete(&models.Chunk{}).Error; err != nil {
		return fmt.Errorf("clearing chunks for artifact %d: %w", artifactID, err)
	}
	if len(rows) == 0 {
		return nil
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("storing chunks for artifact %d: %w", artifactID, err)
	}
	return nil
}

// dropChunkVectors deletes the embeddings of an artifact's chunks, inside tx.
//
// A vector whose chunk is gone is not stale data — it is a retrieval result
// that cannot be cited, which is worse than no result. So it goes in the same
// transaction that removes the chunks, exactly as chunks go in the transaction
// that replaces the Markdown they came from.
func dropChunkVectors(tx *gorm.DB, artifactID uint) error {
	err := tx.Where("owner_kind = ?", models.OwnerChunk).
		Where("owner_id IN (SELECT id FROM chunks WHERE artifact_id = ?)", artifactID).
		Delete(&models.Embedding{}).Error
	if err != nil {
		return fmt.Errorf("clearing vectors derived from artifact %d: %w", artifactID, err)
	}
	return nil
}
