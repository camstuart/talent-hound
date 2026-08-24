package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// InteractionService records what happened with a CRM record — calls, notes,
// placements — and keeps each note's evidence copy flowing through the same
// pipeline as any other artifact. The note is the recruiter's own words, so it
// is editable; the artifact is replaced wholesale on every edit.
type InteractionService struct {
	db     *gorm.DB
	chunks *ChunkService
	Guard  DataGuard
}

// NewInteractionService returns an InteractionService backed by db.
func NewInteractionService(db *gorm.DB, chunks *ChunkService) *InteractionService {
	return &InteractionService{db: db, chunks: chunks}
}

// InteractionInput is one logged or edited interaction. Zero RoleID and
// InitiativeID mean none; an empty OccurredAt means today.
type InteractionInput struct {
	ID           uint              `json:"id"`
	TargetType   models.LinkTarget `json:"targetType"`
	TargetID     uint              `json:"targetId"`
	Kind         string            `json:"kind"`
	Note         string            `json:"note"`
	OccurredAt   models.Date       `json:"occurredAt"`
	RoleID       uint              `json:"roleId"`
	InitiativeID uint              `json:"initiativeId"`
}

// row builds the model from the input, defaulting the date to today.
func (in InteractionInput) row() models.Interaction {
	i := models.Interaction{
		ID:         in.ID,
		TargetType: in.TargetType,
		TargetID:   in.TargetID,
		Kind:       in.Kind,
		Note:       in.Note,
		OccurredAt: in.OccurredAt,
	}
	if i.OccurredAt == "" {
		i.OccurredAt = models.Date(time.Now().UTC().Format("2006-01-02"))
	}
	if in.RoleID != 0 {
		id := in.RoleID
		i.RoleID = &id
	}
	if in.InitiativeID != 0 {
		id := in.InitiativeID
		i.InitiativeID = &id
	}
	return i
}

// Log records one interaction and its evidence artifact, atomically.
func (s *InteractionService) Log(in InteractionInput) (*models.Interaction, error) {
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
	interaction := in.row()
	if err := interaction.Validate(); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		artifact, err := s.evidenceWithin(tx, &interaction)
		if err != nil {
			return err
		}
		interaction.ArtifactID = artifact.ID
		if err := tx.Create(&interaction).Error; err != nil {
			return fmt.Errorf("creating interaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Enqueued after commit: a job must never race a transaction it depends on.
	if _, err := s.chunks.Chunk(interaction.ArtifactID, in.InitiativeID); err != nil {
		return nil, fmt.Errorf("queueing the note for indexing: %w", err)
	}
	return &interaction, nil
}

// evidenceWithin creates the note's artifact inside tx and links it to the
// interaction's target. The Markdown is set here — this application wrote the
// note, so there is nothing to extract.
func (s *InteractionService) evidenceWithin(tx *gorm.DB, i *models.Interaction) (*models.Artifact, error) {
	subject, err := targetName(tx, i.TargetType, i.TargetID)
	if err != nil {
		return nil, err
	}
	header := fmt.Sprintf("# %s with %s, %s", titleCase(i.Kind), subject, i.OccurredAt)
	if i.RoleID != nil {
		var role models.Role
		if err := tx.Select("title").First(&role, *i.RoleID).Error; err != nil {
			return nil, fmt.Errorf("loading role %d: %w", *i.RoleID, err)
		}
		header += fmt.Sprintf(" — re: %s", role.Title)
	}
	markdown := header + "\n\n" + i.Note + "\n"
	data := []byte(markdown)
	sum := sha256.Sum256(data)
	artifact := &models.Artifact{
		DisplayName:      fmt.Sprintf("%s — %s", titleCase(i.Kind), i.OccurredAt),
		MediaType:        "text/markdown",
		ByteLength:       int64(len(data)),
		SHA256:           hex.EncodeToString(sum[:]),
		Source:           "interaction",
		CapturedAt:       time.Now().UTC(),
		Bytes:            data,
		Markdown:         markdown,
		ExtractionState:  models.ExtractionExtracted,
		Extractor:        "interaction",
		ExtractorVersion: "1",
	}
	if err := tx.Create(artifact).Error; err != nil {
		return nil, fmt.Errorf("storing the note: %w", err)
	}
	if err := linkWithin(tx, artifact.ID, i.TargetType, i.TargetID); err != nil {
		return nil, err
	}
	return artifact, nil
}

// titleCase uppercases the first ASCII byte of an interaction kind, which are
// all lowercase ASCII codes — a tiny stand-in for the deprecated strings.Title.
func titleCase(kind string) string {
	if kind == "" {
		return kind
	}
	b := []byte(kind)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}

// targetName loads the display name of the record an interaction is about, and
// in doing so proves the record exists.
func targetName(tx *gorm.DB, targetType models.LinkTarget, id uint) (string, error) {
	table, column := "", ""
	switch targetType {
	case models.LinkCandidate:
		table, column = "candidates", "full_name"
	case models.LinkContact:
		table, column = "contacts", "full_name"
	case models.LinkCompany:
		table, column = "companies", "name"
	case models.LinkRole:
		table, column = "roles", "title"
	default:
		return "", fmt.Errorf("interaction target must be a candidate, contact, company, or role")
	}
	var name string
	err := tx.Raw("SELECT "+column+" FROM "+table+" WHERE id = ?", id).Scan(&name).Error
	if err != nil {
		return "", fmt.Errorf("loading %s %d: %w", targetType, id, err)
	}
	if name == "" {
		return "", fmt.Errorf("%s %d does not exist", targetType, id)
	}
	return name, nil
}

// TimelineEntry is one interaction with the display names its links resolve to,
// so the panel needs no second query.
type TimelineEntry struct {
	models.Interaction
	RoleTitle      string `json:"roleTitle"`
	InitiativeName string `json:"initiativeName"`
}

// Timeline returns a record's history, newest first.
func (s *InteractionService) Timeline(targetType models.LinkTarget, targetID uint) ([]TimelineEntry, error) {
	if !targetType.Valid() || targetType == models.LinkInitiative {
		return nil, fmt.Errorf("interaction target must be a candidate, contact, company, or role")
	}
	entries := []TimelineEntry{}
	err := s.db.Raw(`
		SELECT i.*, COALESCE(r.title, '') AS role_title,
		       COALESCE(n.name, '') AS initiative_name
		FROM interactions i
		LEFT JOIN roles r ON r.id = i.role_id
		LEFT JOIN initiatives n ON n.id = i.initiative_id
		WHERE i.target_type = ? AND i.target_id = ?
		ORDER BY i.occurred_at DESC, i.id DESC`,
		targetType, targetID).Scan(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("loading the timeline: %w", err)
	}
	return entries, nil
}

// Update edits an interaction and replaces its evidence artifact, so search
// always reflects the current wording. The target cannot change: a note about
// someone else is a new interaction.
func (s *InteractionService) Update(in InteractionInput) (*models.Interaction, error) {
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
	var existing models.Interaction
	if err := s.db.First(&existing, in.ID).Error; err != nil {
		return nil, fmt.Errorf("loading interaction %d: %w", in.ID, err)
	}
	interaction := in.row()
	interaction.TargetType, interaction.TargetID = existing.TargetType, existing.TargetID
	interaction.CreatedAt = existing.CreatedAt
	if err := interaction.Validate(); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Create the new artifact first so the interaction always has a valid reference.
		artifact, err := s.evidenceWithin(tx, &interaction)
		if err != nil {
			return err
		}
		interaction.ArtifactID = artifact.ID
		if err := tx.Save(&interaction).Error; err != nil {
			return fmt.Errorf("updating interaction: %w", err)
		}
		// Now safe to delete the old artifact.
		if err := deleteArtifactsWithin(tx, []uint{existing.ArtifactID}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.chunks.Chunk(interaction.ArtifactID, in.InitiativeID); err != nil {
		return nil, fmt.Errorf("queueing the note for indexing: %w", err)
	}
	return &interaction, nil
}

// Delete removes an interaction and everything derived from its note.
func (s *InteractionService) Delete(id uint) error {
	var existing models.Interaction
	if err := s.db.First(&existing, id).Error; err != nil {
		return fmt.Errorf("loading interaction %d: %w", id, err)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete the interaction first to clear the reference, then delete the artifact.
		if err := tx.Delete(&models.Interaction{}, id).Error; err != nil {
			return fmt.Errorf("deleting interaction: %w", err)
		}
		if err := deleteArtifactsWithin(tx, []uint{existing.ArtifactID}); err != nil {
			return err
		}
		return nil
	})
}
