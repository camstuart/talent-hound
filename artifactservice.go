package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// ArtifactService stores evidence: one artifact per ingestion, with the bytes
// and the provenance that make them citable. Nothing here can change a stored
// artifact except its display name — there is no update path for the rest.
//
// Global deletion is deliberately absent. Detaching removes one link; Phase 19
// owns every deletion invariant.
type ArtifactService struct {
	db *gorm.DB
	// Guard refuses artifacts in demo scope and on an unencrypted volume.
	Guard DataGuard
}

// NewArtifactService returns an ArtifactService backed by db.
func NewArtifactService(db *gorm.DB) *ArtifactService {
	return &ArtifactService{db: db}
}

// ArtifactInput is one ingestion request. Bytes cross the Wails boundary as
// base64, because that boundary is JSON.
type ArtifactInput struct {
	DisplayName      string `json:"displayName"`
	OriginalFilename string `json:"originalFilename"`
	Source           string `json:"source"`
	DataBase64       string `json:"dataBase64"`
	// Optional first link. An empty target type ingests an unattached artifact,
	// which is how the orphan library gets its first residents.
	TargetType models.LinkTarget `json:"targetType"`
	TargetID   uint              `json:"targetId"`
}

// Create ingests one artifact, optionally attaching it to a first target.
func (s *ArtifactService) Create(in ArtifactInput) (*models.Artifact, error) {
	data, err := base64.StdEncoding.DecodeString(in.DataBase64)
	if err != nil {
		return nil, fmt.Errorf("artifact bytes are not valid base64: %w", err)
	}
	return s.create(in.DisplayName, in.OriginalFilename, in.Source, data, in.TargetType, in.TargetID)
}

// create is Create with raw bytes, so tests need no base64 round trip.
func (s *ArtifactService) create(
	displayName, filename, source string,
	data []byte,
	targetType models.LinkTarget,
	targetID uint,
) (*models.Artifact, error) {
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
	// Checked before the row is built: a refused ingestion never allocates a
	// 25 MB blob into a transaction.
	if int64(len(data)) > models.MaxArtifactBytes {
		return nil, fmt.Errorf("artifact is %d bytes, over the %d byte limit", len(data), models.MaxArtifactBytes)
	}

	filename = strings.TrimSpace(filename)
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = filename
	}
	if displayName == "" {
		return nil, fmt.Errorf("artifact display name must not be empty when there is no filename")
	}

	sum := sha256.Sum256(data)
	artifact := &models.Artifact{
		DisplayName:      displayName,
		OriginalFilename: filename,
		MediaType:        detectMediaType(data, filename),
		ByteLength:       int64(len(data)),
		SHA256:           hex.EncodeToString(sum[:]),
		Source:           strings.TrimSpace(source),
		// Set here, never by the caller: capture time is provenance.
		CapturedAt: time.Now().UTC(),
		Bytes:      data,
	}

	// The artifact and its first link commit together or not at all: a failed
	// link must not leave an orphan nobody asked for.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(artifact).Error; err != nil {
			return fmt.Errorf("creating artifact: %w", err)
		}
		if targetType == "" {
			return nil
		}
		return linkWithin(tx, artifact.ID, targetType, targetID)
	})
	if err != nil {
		return nil, err
	}
	return artifact, nil
}

// Get returns one artifact's provenance. The bytes are left out: read them with
// Bytes when they are actually needed.
func (s *ArtifactService) Get(id uint) (*models.Artifact, error) {
	var artifact models.Artifact
	if err := s.db.Omit("bytes", "markdown").First(&artifact, id).Error; err != nil {
		return nil, fmt.Errorf("loading artifact %d: %w", id, err)
	}
	return &artifact, nil
}

// Bytes returns one artifact's original bytes, base64-encoded for the frontend.
func (s *ArtifactService) Bytes(id uint) (string, error) {
	data, err := s.bytes(id)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// bytes is Bytes without the encoding, for callers inside Go.
func (s *ArtifactService) bytes(id uint) ([]byte, error) {
	var artifact models.Artifact
	if err := s.db.Select("bytes").First(&artifact, id).Error; err != nil {
		return nil, fmt.Errorf("loading artifact %d bytes: %w", id, err)
	}
	return artifact.Bytes, nil
}

// List returns every artifact, newest first, without their bytes.
func (s *ArtifactService) List() ([]models.Artifact, error) {
	return s.list(s.db.Order("id desc"))
}

// ListForTarget returns the artifacts linked to one record.
func (s *ArtifactService) ListForTarget(targetType models.LinkTarget, targetID uint) ([]models.Artifact, error) {
	if !targetType.Valid() {
		return nil, fmt.Errorf("unknown link target type %q", targetType)
	}
	q := s.db.Where(
		"id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)",
		targetType, targetID,
	).Order("id desc")
	return s.list(q)
}

// ListOrphans returns artifacts with no links. An orphan is a query, not a
// stored flag: nothing can drift out of step with the links that define it.
func (s *ArtifactService) ListOrphans() ([]models.Artifact, error) {
	q := s.db.Where("id NOT IN (SELECT artifact_id FROM artifact_links)").Order("id desc")
	return s.list(q)
}

// list runs a prepared query without loading blobs.
func (s *ArtifactService) list(q *gorm.DB) ([]models.Artifact, error) {
	artifacts := []models.Artifact{}
	if err := q.Omit("bytes", "markdown").Find(&artifacts).Error; err != nil {
		return nil, fmt.Errorf("listing artifacts: %w", err)
	}
	return artifacts, nil
}

// Rename changes the display name, and only the display name.
func (s *ArtifactService) Rename(id uint, displayName string) (*models.Artifact, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, fmt.Errorf("artifact display name must not be empty")
	}
	artifact, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if err := s.db.Model(artifact).Update("display_name", displayName).Error; err != nil {
		return nil, fmt.Errorf("renaming artifact %d: %w", id, err)
	}
	return artifact, nil
}

// Link attaches an artifact to a record. Linking something already linked is
// not a failure — the caller asked for a state that already holds.
func (s *ArtifactService) Link(artifactID uint, targetType models.LinkTarget, targetID uint) error {
	if _, err := s.Get(artifactID); err != nil {
		return err
	}
	return linkWithin(s.db, artifactID, targetType, targetID)
}

// Detach removes exactly one link. The bytes, the provenance, and every other
// link are untouched.
func (s *ArtifactService) Detach(artifactID uint, targetType models.LinkTarget, targetID uint) error {
	result := s.db.Where("artifact_id = ? AND target_type = ? AND target_id = ?", artifactID, targetType, targetID).
		Delete(&models.ArtifactLink{})
	if result.Error != nil {
		return fmt.Errorf("detaching artifact %d: %w", artifactID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("artifact %d is not linked to %s %d", artifactID, targetType, targetID)
	}
	return nil
}

// Links returns every target an artifact is attached to.
func (s *ArtifactService) Links(artifactID uint) ([]models.ArtifactLink, error) {
	links := []models.ArtifactLink{}
	if err := s.db.Where("artifact_id = ?", artifactID).Order("id asc").Find(&links).Error; err != nil {
		return nil, fmt.Errorf("listing links for artifact %d: %w", artifactID, err)
	}
	return links, nil
}

// linkWithin creates one link inside whatever transaction it is given, after
// checking that the target really exists. The link column is polymorphic, so
// this check is the only thing standing in for a foreign key.
func linkWithin(tx *gorm.DB, artifactID uint, targetType models.LinkTarget, targetID uint) error {
	if !targetType.Valid() {
		return fmt.Errorf("unknown link target type %q", targetType)
	}
	var target any
	switch targetType {
	case models.LinkInitiative:
		target = &models.Initiative{}
	case models.LinkCandidate:
		target = &models.Candidate{}
	case models.LinkRole:
		target = &models.Role{}
	case models.LinkCompany:
		target = &models.Company{}
	case models.LinkContact:
		target = &models.Contact{}
	}
	var n int64
	if err := tx.Model(target).Where("id = ?", targetID).Count(&n).Error; err != nil {
		return fmt.Errorf("checking %s %d: %w", targetType, targetID, err)
	}
	if n == 0 {
		return fmt.Errorf("%s %d does not exist", targetType, targetID)
	}

	var existing int64
	if err := tx.Model(&models.ArtifactLink{}).
		Where("artifact_id = ? AND target_type = ? AND target_id = ?", artifactID, targetType, targetID).
		Count(&existing).Error; err != nil {
		return fmt.Errorf("checking existing link: %w", err)
	}
	if existing > 0 {
		return nil
	}

	link := &models.ArtifactLink{ArtifactID: artifactID, TargetType: targetType, TargetID: targetID}
	if err := tx.Create(link).Error; err != nil {
		return fmt.Errorf("linking artifact %d to %s %d: %w", artifactID, targetType, targetID, err)
	}
	return nil
}

// refinements narrow the honest but coarse type that content sniffing reports
// for container formats. The extension is consulted only here: for anything
// else the bytes decide, so a .pdf full of text is stored as text.
var refinements = map[string]map[string]string{
	"text/plain": {
		".md":       "text/markdown",
		".markdown": "text/markdown",
	},
	"application/zip": {
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	},
}

// detectMediaType reports what the bytes actually are.
func detectMediaType(data []byte, filename string) string {
	detected := http.DetectContentType(data)
	base := strings.TrimSpace(strings.Split(detected, ";")[0])
	if byExt, ok := refinements[base]; ok {
		if refined, ok := byExt[strings.ToLower(path.Ext(filename))]; ok {
			return refined
		}
	}
	return detected
}
