package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// ResumeDrop is one resume arriving on a Job Search Initiative.
//
// CandidateID selects an existing candidate; leaving it zero creates one from
// FullName. Exactly one of those happens, and either way the whole thing is one
// transaction — the failure mode to avoid is the orphan, a candidate row with
// no evidence or an artifact attached to nothing, both invisible in the UI and
// permanent in the database.
type ResumeDrop struct {
	InitiativeID uint `json:"initiativeId"`
	// CandidateID attaches to an existing candidate. Zero creates a new one.
	CandidateID uint `json:"candidateId"`
	// FullName names the candidate to create. Required when CandidateID is zero.
	FullName         string `json:"fullName"`
	DisplayName      string `json:"displayName"`
	OriginalFilename string `json:"originalFilename"`
	Source           string `json:"source"`
	DataBase64       string `json:"dataBase64"`
}

// ResumeIntake is the result of a completed drop.
type ResumeIntake struct {
	Candidate *models.Candidate `json:"candidate"`
	Artifact  *models.Artifact  `json:"artifact"`
	// Created says whether the candidate was made by this drop, so the UI can
	// say "added" rather than "attached".
	Created bool `json:"created"`
}

// DropResume creates a candidate and an artifact, or neither.
//
// There is no partial outcome and no cancellation path in the backend: a drop
// that is cancelled in the UI simply never calls this. That is the whole of
// "cancellation creates neither".
func (s *CandidateProfileService) DropResume(in ResumeDrop) (*ResumeIntake, error) {
	data, err := base64.StdEncoding.DecodeString(in.DataBase64)
	if err != nil {
		return nil, fmt.Errorf("resume bytes are not valid base64: %w", err)
	}
	if int64(len(data)) > models.MaxArtifactBytes {
		return nil, fmt.Errorf("resume is %d bytes, over the %d byte limit", len(data), models.MaxArtifactBytes)
	}
	if in.InitiativeID == 0 {
		return nil, fmt.Errorf("a resume drop needs the initiative it was dropped on")
	}
	filename := strings.TrimSpace(in.OriginalFilename)
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = filename
	}
	if displayName == "" {
		return nil, fmt.Errorf("a dropped resume needs a filename or a name to show")
	}

	out := &ResumeIntake{}
	sum := sha256.Sum256(data)
	artifact := &models.Artifact{
		DisplayName:      displayName,
		OriginalFilename: filename,
		MediaType:        detectMediaType(data, filename),
		ByteLength:       int64(len(data)),
		SHA256:           hex.EncodeToString(sum[:]),
		Source:           strings.TrimSpace(in.Source),
		CapturedAt:       time.Now().UTC(),
		Bytes:            data,
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		candidate := &models.Candidate{ID: in.CandidateID}
		if in.CandidateID == 0 {
			candidate = &models.Candidate{FullName: strings.TrimSpace(in.FullName)}
			if err := candidate.Validate(); err != nil {
				return err
			}
			if err := tx.Create(candidate).Error; err != nil {
				return fmt.Errorf("creating the candidate: %w", err)
			}
			out.Created = true
		} else if err := tx.First(candidate, in.CandidateID).Error; err != nil {
			return fmt.Errorf("loading candidate %d: %w", in.CandidateID, err)
		}

		if err := tx.Create(artifact).Error; err != nil {
			return fmt.Errorf("storing the resume: %w", err)
		}
		// Both links, in the same transaction as both rows. A resume belongs to
		// the person and appears in the workspace it arrived in.
		if err := linkWithin(tx, artifact.ID, models.LinkCandidate, candidate.ID); err != nil {
			return err
		}
		if err := linkWithin(tx, artifact.ID, models.LinkInitiative, in.InitiativeID); err != nil {
			return err
		}
		out.Candidate = candidate
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The bytes are not sent back: the caller asked to store them, not to read
	// them again.
	artifact.Bytes = nil
	out.Artifact = artifact
	return out, nil
}
