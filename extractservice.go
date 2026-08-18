package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/extract"
	"camstuart/talent-hound/internal/models"
)

// ExtractService turns stored artifacts into Markdown. It owns no lifecycle of
// its own: extraction is a background job like everything else slow, so it is
// cancellable, retryable, and honest after a crash without a second mechanism.
//
// One artifact per job. Batch extraction is a P1 item and the PoC contract is
// deliberately not shaped to accommodate one.
type ExtractService struct {
	db      *gorm.DB
	jobs    *JobService
	sidecar *extract.Sidecar
	dataDir string
}

// extractParams is what an extraction job carries: an identifier, and nothing
// else. Params never hold content.
type extractParams struct {
	ArtifactID uint `json:"artifactId"`
}

// NewExtractService verifies the sidecar, sweeps whatever a previous process
// left in the staging root, and registers the extract worker.
//
// A failed verification does not stop the application: a recruiter with a
// broken install can still keep records, and the failure names itself the
// moment they try to read a document.
func NewExtractService(db *gorm.DB, jobs *JobService, dataDir string) *ExtractService {
	s := &ExtractService{
		db:      db,
		jobs:    jobs,
		sidecar: extract.Verify(context.Background(), extract.DefaultSidecarPath()),
		dataDir: dataDir,
	}
	if !s.sidecar.Available() {
		// Safe to print: it describes the install, not a document.
		log.Println("extraction unavailable:", s.sidecar.Detail())
	}
	if err := extract.Sweep(dataDir); err != nil {
		log.Println("staging sweep failed:", err)
	}
	jobs.register("extract", s.work)
	return s
}

// Extract queues one artifact for conversion and returns without waiting.
// initiativeID is the workspace the request came from, so the job appears where
// the recruiter asked for it; zero for a request that belongs to no workspace.
func (s *ExtractService) Extract(artifactID, initiativeID uint) (*models.Job, error) {
	var artifact models.Artifact
	if err := s.db.Select("id").First(&artifact, artifactID).Error; err != nil {
		return nil, fmt.Errorf("loading artifact %d: %w", artifactID, err)
	}
	params, err := json.Marshal(extractParams{ArtifactID: artifactID})
	if err != nil {
		return nil, fmt.Errorf("encoding extraction params: %w", err)
	}
	// Back to pending before the attempt, so a retry never shows the previous
	// outcome beside a run that has not finished.
	if err := s.setState(s.db, artifactID, models.ExtractionPending, "", extract.Result{}); err != nil {
		return nil, err
	}
	return s.jobs.Enqueue(JobInput{
		Kind:         "extract",
		InitiativeID: initiativeID,
		Params:       string(params),
		TotalItems:   1,
	})
}

// Markdown returns one artifact's extracted text. It is untrusted: whoever
// wrote the document chose its contents, so the caller displays it and does
// nothing else with it.
func (s *ExtractService) Markdown(artifactID uint) (string, error) {
	var artifact models.Artifact
	if err := s.db.Select("markdown").First(&artifact, artifactID).Error; err != nil {
		return "", fmt.Errorf("loading artifact %d: %w", artifactID, err)
	}
	return artifact.Markdown, nil
}

// work is the extract job's worker. The conversion happens in the compute half,
// with no transaction open — a two-minute subprocess inside one would hold
// SQLite's single writer for two minutes.
func (s *ExtractService) work(ctx context.Context, job models.Job, _ int) (JobCommit, error) {
	var p extractParams
	if err := json.Unmarshal([]byte(job.Params), &p); err != nil {
		return nil, FailReason("bad_params")
	}
	var artifact models.Artifact
	if err := s.db.Select("id", "media_type", "bytes").First(&artifact, p.ArtifactID).Error; err != nil {
		return nil, FailReason(models.ReasonExtractFailed)
	}

	res, err := s.sidecar.Run(ctx, s.dataDir, artifact.MediaType, artifact.Bytes)
	if err != nil {
		code := extract.Code(err)
		// A cancelled job leaves the artifact pending: it was not tried, so it
		// has nothing to report. The job carries the cancellation.
		if ctx.Err() != nil {
			return nil, err
		}
		if serr := s.setState(s.db, p.ArtifactID, models.ExtractionFailed, code, extract.Result{}); serr != nil {
			return nil, FailReason(models.ReasonExtractFailed)
		}
		return nil, FailReason(code)
	}
	return func(tx *gorm.DB) error {
		return s.setState(tx, p.ArtifactID, models.ExtractionExtracted, "", res)
	}, nil
}

// setState is the only place an artifact's extraction columns change.
func (s *ExtractService) setState(
	tx *gorm.DB, id uint, state models.ExtractionState, reason string, res extract.Result,
) error {
	if reason != "" && !models.ValidReason(reason) {
		return fmt.Errorf("extraction reason %q is not a code", reason)
	}
	err := tx.Model(&models.Artifact{}).Where("id = ?", id).Updates(map[string]any{
		"extraction_state":  state,
		"extraction_error":  reason,
		"extractor":         res.Extractor,
		"extractor_version": res.Version,
		"markdown":          res.Markdown,
	}).Error
	if err != nil {
		return fmt.Errorf("recording extraction for artifact %d: %w", id, err)
	}
	// Chunks are derived from the Markdown this call just replaced. They go in
	// the same transaction, not a later cleanup: a chunk that outlives its
	// Markdown by a moment is a chunk something could cite in that moment.
	// Vectors first: they are derived from the chunks, and deleting the chunks
	// out from under them would leave the lookup with nothing to name.
	if err := dropChunkVectors(tx, id); err != nil {
		return err
	}
	if err := tx.Where("artifact_id = ?", id).Delete(&models.Chunk{}).Error; err != nil {
		return fmt.Errorf("clearing chunks derived from artifact %d: %w", id, err)
	}
	return nil
}
