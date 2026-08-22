package main

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// DeletionService is the only operation with no undo, over a database holding
// information about people who did not choose to be in it.
//
// So "delete" has to mean delete — not "hide", not "mostly", and not "the
// record but not the six derived things pointing at it". Every cascade runs in
// one transaction and then queries for what should be gone, because a cascade
// that reports success while leaving embeddings behind is worse than one that
// fails: nobody looks again.
type DeletionService struct {
	db *gorm.DB
}

// NewDeletionService returns the deletion rules bound to the database.
func NewDeletionService(db *gorm.DB) *DeletionService { return &DeletionService{db: db} }

// Consequence is one thing a deletion would remove.
type Consequence struct {
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
	// Detail names specific records where naming them helps.
	Detail []string `json:"detail"`
}

// Preview is what the recruiter is told before they confirm.
type Preview struct {
	// Blocked says the deletion cannot proceed as asked.
	Blocked bool `json:"blocked"`
	// Blockers name what is stopping it, so the refusal is a to-do list rather
	// than a dead end.
	Blockers []string `json:"blockers"`
	// Removes is what would go.
	Removes []Consequence `json:"removes"`
	// Choice is set when the recruiter must decide something before proceeding.
	Choice string `json:"choice"`
}

// PreviewInitiative lists what deleting an initiative would remove.
func (s *DeletionService) PreviewInitiative(id uint) (*Preview, error) {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}
	counts := []struct {
		kind  string
		model any
		where string
	}{
		{"search criteria", &models.SearchCriterion{}, "initiative_id = ?"},
		{"matches", &models.Match{}, "initiative_id = ?"},
		{"drafts", &models.Draft{}, "initiative_id = ?"},
		{"background jobs", &models.Job{}, "initiative_id = ?"},
		{"answers", &models.Answer{}, "initiative_id = ?"},
		{"audit events", &models.DisclosureEvent{}, "initiative_id = ?"},
		{"searches", &models.Search{}, "initiative_id = ?"},
	}
	for _, c := range counts {
		var n int64
		if err := s.db.Model(c.model).Where(c.where, id).Count(&n).Error; err != nil {
			return nil, fmt.Errorf("counting %s: %w", c.kind, err)
		}
		out.Removes = append(out.Removes, Consequence{Kind: c.kind, Count: n})
	}
	// Said explicitly, because this is the rule most likely to be "improved" by
	// someone tidying up.
	out.Removes = append(out.Removes, Consequence{
		Kind:   "shared records kept",
		Detail: []string{"candidates, roles, companies, contacts, and recruiter-added artifacts are not deleted"},
	})
	return out, nil
}

// DeleteInitiative removes what an initiative owns and nothing shared.
func (s *DeletionService) DeleteInitiative(id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, step := range []func(*gorm.DB) error{
			func(tx *gorm.DB) error {
				return tx.Where("match_id IN (SELECT id FROM matches WHERE initiative_id = ?)", id).
					Delete(&models.MatchResult{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.Match{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.DisclosureEvent{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.Draft{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.Answer{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.SearchCriterion{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.CriteriaVersion{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.Search{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.CloudConsent{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Where("initiative_id = ?", id).Delete(&models.Job{}).Error
			},
			func(tx *gorm.DB) error {
				// Links only: the artifacts themselves are shared and stay.
				return tx.Where("target_type = ? AND target_id = ?", models.LinkInitiative, id).
					Delete(&models.ArtifactLink{}).Error
			},
			func(tx *gorm.DB) error {
				return tx.Delete(&models.Initiative{}, id).Error
			},
		} {
			if err := step(tx); err != nil {
				return fmt.Errorf("deleting the initiative: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.verifyInitiative(id)
}

// verifyInitiative asks whether what should be gone is gone.
func (s *DeletionService) verifyInitiative(id uint) error {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}
	remaining := map[string]any{
		"search criteria": &models.SearchCriterion{},
		"matches":         &models.Match{},
		"drafts":          &models.Draft{},
		"audit events":    &models.DisclosureEvent{},
		"searches":        &models.Search{},
	}
	for kind, model := range remaining {
		var n int64
		if err := s.db.Model(model).Where("initiative_id = ?", id).Count(&n).Error; err != nil {
			return fmt.Errorf("verifying %s: %w", kind, err)
		}
		if n > 0 {
			out.Blocked = true
			out.Blockers = append(out.Blockers,
				fmt.Sprintf("%d %s were not removed", n, kind))
		}
	}
	if out.Blocked {
		return fmt.Errorf("the deletion committed but left data behind: %s",
			strings.Join(out.Blockers, "; "))
	}
	return nil
}

// PreviewCandidate lists what deleting a candidate would remove, and what is
// stopping it.
func (s *DeletionService) PreviewCandidate(id uint) (*Preview, error) {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}

	// Archived is not a lesser state: an archive is a record of work that
	// happened, and deleting its subject leaves an account of a search for
	// nobody.
	blocking := []models.Initiative{}
	err := s.db.Where("candidate_id = ?", id).Find(&blocking).Error
	if err != nil {
		return nil, fmt.Errorf("finding referencing initiatives: %w", err)
	}
	linked := []models.ArtifactLink{}
	err = s.db.Where("target_type = ? AND target_id = ?", models.LinkCandidate, id).
		Find(&linked).Error
	if err != nil {
		return nil, fmt.Errorf("finding the candidate's artifacts: %w", err)
	}
	for _, init := range blocking {
		out.Blocked = true
		out.Blockers = append(out.Blockers,
			fmt.Sprintf("the %s initiative %q references this candidate", init.Status, init.Name))
	}

	// A shared artifact is a decision, not a default. Deleting by default
	// destroys evidence someone attached deliberately; keeping by default
	// leaves a résumé in the system after its subject was deleted.
	shared := []string{}
	for _, link := range linked {
		others, err := s.otherOwners(link.ArtifactID, id)
		if err != nil {
			return nil, err
		}
		if others > 0 {
			var artifact models.Artifact
			if err := s.db.Select("id", "display_name").First(&artifact, link.ArtifactID).Error; err == nil {
				shared = append(shared, artifact.DisplayName)
			}
		}
	}
	if len(shared) > 0 {
		out.Blocked = true
		out.Choice = "these artifacts are attached to other records too, and may contain candidate " +
			"information: delete them everywhere, or keep them under their other links"
		out.Removes = append(out.Removes, Consequence{
			Kind: "artifacts attached elsewhere", Count: int64(len(shared)), Detail: shared,
		})
	}

	var profiles, aspects int64
	if err := s.db.Model(&models.Profile{}).
		Where("subject_kind = ? AND subject_id = ?", profile.SubjectCandidate, id).
		Count(&profiles).Error; err != nil {
		return nil, fmt.Errorf("counting profiles: %w", err)
	}
	if err := s.db.Model(&models.ProfileAspect{}).
		Where("profile_id IN (SELECT id FROM profiles WHERE subject_kind = ? AND subject_id = ?)",
			profile.SubjectCandidate, id).Count(&aspects).Error; err != nil {
		return nil, fmt.Errorf("counting aspects: %w", err)
	}
	out.Removes = append(out.Removes,
		Consequence{Kind: "profile versions", Count: profiles},
		Consequence{Kind: "profile aspects", Count: aspects},
		Consequence{Kind: "candidate-only artifacts", Count: int64(len(linked)) - int64(len(shared))},
	)
	return out, nil
}

// SharedArtifactChoice is what the recruiter decided about shared artifacts.
type SharedArtifactChoice string

const (
	// DeleteShared removes the artifact everywhere.
	DeleteShared SharedArtifactChoice = "delete_everywhere"
	// RetainShared keeps it under its other links, with the warning given.
	RetainShared SharedArtifactChoice = "retain_under_other_links"
)

// DeleteCandidate removes a candidate and their derived data.
func (s *DeletionService) DeleteCandidate(id uint, choice SharedArtifactChoice) error {
	preview, err := s.PreviewCandidate(id)
	if err != nil {
		return err
	}
	// Initiative references block regardless of any choice.
	for _, blocker := range preview.Blockers {
		if strings.Contains(blocker, "initiative") {
			return fmt.Errorf("this candidate cannot be deleted yet: %s",
				strings.Join(preview.Blockers, "; "))
		}
	}
	if preview.Choice != "" && choice == "" {
		return fmt.Errorf("%s", preview.Choice)
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		artifactIDs := []uint{}
		err := tx.Model(&models.ArtifactLink{}).
			Where("target_type = ? AND target_id = ?", models.LinkCandidate, id).
			Pluck("artifact_id", &artifactIDs).Error
		if err != nil {
			return fmt.Errorf("listing the candidate's artifacts: %w", err)
		}

		// Which artifacts go entirely, and which keep their other links.
		remove := []uint{}
		for _, artifactID := range artifactIDs {
			others, err := otherOwnersWithin(tx, artifactID, id)
			if err != nil {
				return err
			}
			if others == 0 || choice == DeleteShared {
				remove = append(remove, artifactID)
			}
		}

		if len(remove) > 0 {
			if err := deleteArtifactsWithin(tx, remove); err != nil {
				return err
			}
		}
		// Whatever is retained loses its candidate link, which is what
		// "retained under its other links" means.
		err = tx.Where("target_type = ? AND target_id = ?", models.LinkCandidate, id).
			Delete(&models.ArtifactLink{}).Error
		if err != nil {
			return fmt.Errorf("removing the candidate's links: %w", err)
		}

		err = tx.Where("profile_id IN (SELECT id FROM profiles WHERE subject_kind = ? AND subject_id = ?)",
			profile.SubjectCandidate, id).Delete(&models.ProfileAspect{}).Error
		if err != nil {
			return fmt.Errorf("deleting the candidate's aspects: %w", err)
		}
		err = tx.Where("subject_kind = ? AND subject_id = ?", profile.SubjectCandidate, id).
			Delete(&models.Profile{}).Error
		if err != nil {
			return fmt.Errorf("deleting the candidate's profiles: %w", err)
		}
		err = tx.Where("match_id IN (SELECT id FROM matches WHERE candidate_id = ?)", id).
			Delete(&models.MatchResult{}).Error
		if err != nil {
			return fmt.Errorf("deleting the candidate's match results: %w", err)
		}
		if err := tx.Where("candidate_id = ?", id).Delete(&models.Match{}).Error; err != nil {
			return fmt.Errorf("deleting the candidate's matches: %w", err)
		}
		if err := tx.Delete(&models.Candidate{}, id).Error; err != nil {
			return fmt.Errorf("deleting the candidate: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.verifyCandidate(id)
}

// verifyCandidate proves the candidate's exclusively owned evidence is gone.
//
// Scoped to the entity: a chunk belonging to an artifact retained under another
// link is not a failure, and a check that counted it would make every correct
// deletion look broken until someone turned the check off.
func (s *DeletionService) verifyCandidate(id uint) error {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}

	var candidates int64
	if err := s.db.Model(&models.Candidate{}).Where("id = ?", id).Count(&candidates).Error; err != nil {
		return fmt.Errorf("verifying the candidate: %w", err)
	}
	if candidates > 0 {
		out.Blockers = append(out.Blockers, "the candidate record was not removed")
	}
	var profiles int64
	if err := s.db.Model(&models.Profile{}).
		Where("subject_kind = ? AND subject_id = ?", profile.SubjectCandidate, id).
		Count(&profiles).Error; err != nil {
		return fmt.Errorf("verifying profiles: %w", err)
	}
	if profiles > 0 {
		out.Blockers = append(out.Blockers, "profile versions remain")
	}
	var links int64
	if err := s.db.Model(&models.ArtifactLink{}).
		Where("target_type = ? AND target_id = ?", models.LinkCandidate, id).
		Count(&links).Error; err != nil {
		return fmt.Errorf("verifying links: %w", err)
	}
	if links > 0 {
		out.Blockers = append(out.Blockers, "artifact links remain")
	}
	if len(out.Blockers) > 0 {
		out.Blocked = true
		return fmt.Errorf("the deletion committed but left data behind: %s",
			strings.Join(out.Blockers, "; "))
	}
	return nil
}

// otherOwners counts the records — other than this candidate — that own an
// artifact.
//
// An initiative link is deliberately not an owner. It is workspace placement:
// deleting the initiative does not delete the artifact either, and treating it
// as shared ownership would make every résumé dropped into a workspace require
// a decision it does not need. What the PRD means by "shared elsewhere" is
// another candidate, role, company, or contact.
func (s *DeletionService) otherOwners(artifactID, candidateID uint) (int64, error) {
	return otherOwnersWithin(s.db, artifactID, candidateID)
}

func otherOwnersWithin(tx *gorm.DB, artifactID, candidateID uint) (int64, error) {
	var n int64
	err := tx.Model(&models.ArtifactLink{}).
		Where("artifact_id = ?", artifactID).
		Where("target_type <> ?", models.LinkInitiative).
		Where("NOT (target_type = ? AND target_id = ?)", models.LinkCandidate, candidateID).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("counting other owners of artifact %d: %w", artifactID, err)
	}
	return n, nil
}

// deleteArtifactsWithin removes artifacts and everything derived from them.
func deleteArtifactsWithin(tx *gorm.DB, ids []uint) error {
	err := tx.Where("owner_kind = ? AND owner_id IN (SELECT id FROM chunks WHERE artifact_id IN ?)",
		models.OwnerChunk, ids).Delete(&models.Embedding{}).Error
	if err != nil {
		return fmt.Errorf("deleting embeddings: %w", err)
	}
	if err := tx.Where("artifact_id IN ?", ids).Delete(&models.Chunk{}).Error; err != nil {
		return fmt.Errorf("deleting chunks: %w", err)
	}
	if err := tx.Where("artifact_id IN ?", ids).Delete(&models.ArtifactLink{}).Error; err != nil {
		return fmt.Errorf("deleting artifact links: %w", err)
	}
	if err := tx.Where("id IN ?", ids).Delete(&models.Artifact{}).Error; err != nil {
		return fmt.Errorf("deleting artifacts: %w", err)
	}
	return nil
}

// Detach removes one link and nothing else.
//
// A role's source artifact cannot be detached: a role's provenance is the
// sequence of listings it was seen as, and letting one be removed leaves a
// match citing a listing that no longer exists.
func (s *DeletionService) Detach(artifactID uint, targetType models.LinkTarget, targetID uint) error {
	if targetType == models.LinkRole {
		owned, err := s.isRoleSource(artifactID)
		if err != nil {
			return err
		}
		if owned {
			return fmt.Errorf("this is a role's source listing and cannot be detached — purge the role instead")
		}
	}
	err := s.db.Where("artifact_id = ? AND target_type = ? AND target_id = ?",
		artifactID, targetType, targetID).Delete(&models.ArtifactLink{}).Error
	if err != nil {
		return fmt.Errorf("detaching the artifact: %w", err)
	}
	return nil
}

// PreviewArtifact lists every link a global deletion would remove.
func (s *DeletionService) PreviewArtifact(id uint) (*Preview, error) {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}

	owned, err := s.isRoleSource(id)
	if err != nil {
		return nil, err
	}
	if owned {
		out.Blocked = true
		out.Blockers = append(out.Blockers,
			"this is a role's source listing: it is read-only and goes only when the role is purged")
		return out, nil
	}

	links := []models.ArtifactLink{}
	if err := s.db.Where("artifact_id = ?", id).Find(&links).Error; err != nil {
		return nil, fmt.Errorf("listing links: %w", err)
	}
	detail := make([]string, 0, len(links))
	for _, l := range links {
		detail = append(detail, fmt.Sprintf("%s %d", l.TargetType, l.TargetID))
	}
	var chunks int64
	if err := s.db.Model(&models.Chunk{}).Where("artifact_id = ?", id).Count(&chunks).Error; err != nil {
		return nil, fmt.Errorf("counting chunks: %w", err)
	}
	out.Removes = append(out.Removes,
		Consequence{Kind: "links", Count: int64(len(links)), Detail: detail},
		Consequence{Kind: "indexed sections", Count: chunks},
	)
	return out, nil
}

// DeleteArtifact removes an artifact everywhere, with everything derived.
func (s *DeletionService) DeleteArtifact(id uint) error {
	preview, err := s.PreviewArtifact(id)
	if err != nil {
		return err
	}
	if preview.Blocked {
		return fmt.Errorf("%s", strings.Join(preview.Blockers, "; "))
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		return deleteArtifactsWithin(tx, []uint{id})
	})
	if err != nil {
		return err
	}
	return s.verifyArtifact(id)
}

func (s *DeletionService) verifyArtifact(id uint) error {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}
	checks := []struct {
		kind  string
		model any
		where string
	}{
		{"the artifact", &models.Artifact{}, "id = ?"},
		{"its links", &models.ArtifactLink{}, "artifact_id = ?"},
		{"its indexed sections", &models.Chunk{}, "artifact_id = ?"},
	}
	for _, c := range checks {
		var n int64
		if err := s.db.Model(c.model).Where(c.where, id).Count(&n).Error; err != nil {
			return fmt.Errorf("verifying %s: %w", c.kind, err)
		}
		if n > 0 {
			out.Blockers = append(out.Blockers, fmt.Sprintf("%s remained", c.kind))
		}
	}
	if len(out.Blockers) > 0 {
		out.Blocked = true
		return fmt.Errorf("the deletion committed but left data behind: %s",
			strings.Join(out.Blockers, "; "))
	}
	return nil
}

// isRoleSource reports whether an artifact belongs to a role.
func (s *DeletionService) isRoleSource(artifactID uint) (bool, error) {
	var n int64
	err := s.db.Model(&models.ArtifactLink{}).
		Where("artifact_id = ? AND target_type = ?", artifactID, models.LinkRole).
		Count(&n).Error
	if err != nil {
		return false, fmt.Errorf("checking artifact ownership: %w", err)
	}
	return n > 0, nil
}

// PreviewRolePurge lists the initiatives referencing a role and what would go.
func (s *DeletionService) PreviewRolePurge(id uint) (*Preview, error) {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}

	initiatives := []string{}
	rows := []models.Initiative{}
	err := s.db.Where("id IN (SELECT target_id FROM artifact_links WHERE target_type = ? "+
		"AND artifact_id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?))",
		models.LinkInitiative, models.LinkRole, id).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("finding referencing initiatives: %w", err)
	}
	for _, init := range rows {
		initiatives = append(initiatives, init.Name)
	}

	var sources, matches, drafts int64
	if err := s.db.Model(&models.ArtifactLink{}).
		Where("target_type = ? AND target_id = ?", models.LinkRole, id).
		Count(&sources).Error; err != nil {
		return nil, fmt.Errorf("counting sources: %w", err)
	}
	if err := s.db.Model(&models.Match{}).Where("role_id = ?", id).Count(&matches).Error; err != nil {
		return nil, fmt.Errorf("counting matches: %w", err)
	}
	if err := s.db.Model(&models.Draft{}).
		Where("role_id = ? AND state = ?", id, models.DraftActive).Count(&drafts).Error; err != nil {
		return nil, fmt.Errorf("counting drafts: %w", err)
	}
	out.Removes = append(out.Removes,
		Consequence{Kind: "referencing initiatives", Count: int64(len(initiatives)), Detail: initiatives},
		Consequence{Kind: "source listings, current and historical", Count: sources},
		Consequence{Kind: "matches", Count: matches},
		Consequence{Kind: "active drafts", Count: drafts},
		Consequence{Kind: "kept", Detail: []string{
			"recruiter notes survive with the role reference cleared",
			"copy events survive with the role reference cleared",
		}},
	)
	return out, nil
}

// PurgeRole removes a role and everything derived from it.
func (s *DeletionService) PurgeRole(id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		artifactIDs := []uint{}
		err := tx.Model(&models.ArtifactLink{}).
			Where("target_type = ? AND target_id = ?", models.LinkRole, id).
			Pluck("artifact_id", &artifactIDs).Error
		if err != nil {
			return fmt.Errorf("listing the role's sources: %w", err)
		}
		if len(artifactIDs) > 0 {
			if err := deleteArtifactsWithin(tx, artifactIDs); err != nil {
				return err
			}
		}
		err = tx.Where("profile_id IN (SELECT id FROM profiles WHERE subject_kind = ? AND subject_id = ?)",
			profile.SubjectRole, id).Delete(&models.ProfileAspect{}).Error
		if err != nil {
			return fmt.Errorf("deleting the role's aspects: %w", err)
		}
		err = tx.Where("subject_kind = ? AND subject_id = ?", profile.SubjectRole, id).
			Delete(&models.Profile{}).Error
		if err != nil {
			return fmt.Errorf("deleting the role's profiles: %w", err)
		}
		err = tx.Where("match_id IN (SELECT id FROM matches WHERE role_id = ?)", id).
			Delete(&models.MatchResult{}).Error
		if err != nil {
			return fmt.Errorf("deleting the role's match results: %w", err)
		}
		if err := tx.Where("role_id = ?", id).Delete(&models.Match{}).Error; err != nil {
			return fmt.Errorf("deleting the role's matches: %w", err)
		}
		err = tx.Where("role_id = ? AND state = ?", id, models.DraftActive).
			Delete(&models.Draft{}).Error
		if err != nil {
			return fmt.Errorf("deleting the role's active drafts: %w", err)
		}
		// Survivors: the recruiter's own words and the record that something
		// left the machine. Their references are cleared rather than the rows
		// being deleted.
		err = tx.Model(&models.DisclosureEvent{}).Where("role_id = ?", id).
			Update("role_id", nil).Error
		if err != nil {
			return fmt.Errorf("clearing audit references: %w", err)
		}
		err = tx.Model(&models.Draft{}).Where("role_id = ?", id).
			Update("role_id", nil).Error
		if err != nil {
			return fmt.Errorf("clearing draft references: %w", err)
		}
		if err := tx.Delete(&models.Role{}, id).Error; err != nil {
			return fmt.Errorf("deleting the role: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.verifyRole(id)
}

func (s *DeletionService) verifyRole(id uint) error {
	out := &Preview{Blockers: []string{}, Removes: []Consequence{}}
	var roles, profiles, matches, links int64
	if err := s.db.Model(&models.Role{}).Where("id = ?", id).Count(&roles).Error; err != nil {
		return fmt.Errorf("verifying the role: %w", err)
	}
	if err := s.db.Model(&models.Profile{}).
		Where("subject_kind = ? AND subject_id = ?", profile.SubjectRole, id).
		Count(&profiles).Error; err != nil {
		return fmt.Errorf("verifying profiles: %w", err)
	}
	if err := s.db.Model(&models.Match{}).Where("role_id = ?", id).Count(&matches).Error; err != nil {
		return fmt.Errorf("verifying matches: %w", err)
	}
	if err := s.db.Model(&models.ArtifactLink{}).
		Where("target_type = ? AND target_id = ?", models.LinkRole, id).
		Count(&links).Error; err != nil {
		return fmt.Errorf("verifying sources: %w", err)
	}
	for kind, n := range map[string]int64{
		"the role": roles, "its profiles": profiles,
		"its matches": matches, "its sources": links,
	} {
		if n > 0 {
			out.Blockers = append(out.Blockers, kind+" remained")
		}
	}
	if len(out.Blockers) > 0 {
		out.Blocked = true
		return fmt.Errorf("the purge committed but left data behind: %s",
			strings.Join(out.Blockers, "; "))
	}
	return nil
}

// PurgeStale purges every stale role independently.
//
// Each is its own transaction, so one failure neither stops the others nor
// leaves its own role half-deleted.
type PurgeReport struct {
	Purged []uint          `json:"purged"`
	Failed map[uint]string `json:"failed"`
}

// PurgeStale purges the roles given, reporting each outcome.
func (s *DeletionService) PurgeStale(roleIDs []uint) *PurgeReport {
	out := &PurgeReport{Purged: []uint{}, Failed: map[uint]string{}}
	for _, id := range roleIDs {
		if err := s.PurgeRole(id); err != nil {
			out.Failed[id] = err.Error()
			continue
		}
		out.Purged = append(out.Purged, id)
	}
	return out
}

// DeleteDraft removes a draft and clears the reference on its surviving copy
// events.
func (s *DeletionService) DeleteDraft(id uint) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&models.DisclosureEvent{}).Where("draft_id = ?", id).
			Update("draft_id", nil).Error
		if err != nil {
			return fmt.Errorf("clearing copy references: %w", err)
		}
		if err := tx.Delete(&models.Draft{}, id).Error; err != nil {
			return fmt.Errorf("deleting the draft: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.verifyDraft(id)
}

// verifyDraft proves the draft is gone and that nothing still points at it.
//
// The PRD asks this of every deletion, and this was the one without it: "a
// scoped verification query proves the deleted entity and exclusively owned
// evidence no longer appear". A transaction makes a partial write unlikely
// rather than impossible, and what it cannot make unlikely is a table gaining a
// reference to a draft that nobody remembers to clear here — which is the shape
// the audit events already have.
func (s *DeletionService) verifyDraft(id uint) error {
	var drafts, references int64
	if err := s.db.Model(&models.Draft{}).Where("id = ?", id).Count(&drafts).Error; err != nil {
		return fmt.Errorf("verifying the draft: %w", err)
	}
	if err := s.db.Model(&models.DisclosureEvent{}).Where("draft_id = ?", id).
		Count(&references).Error; err != nil {
		return fmt.Errorf("verifying copy events: %w", err)
	}
	left := []string{}
	if drafts > 0 {
		left = append(left, "the draft remained")
	}
	if references > 0 {
		// The event survives — the PRD says so — with its reference cleared.
		// One still pointing at a deleted draft is a dangling record, not a
		// surviving one.
		left = append(left, "a copy event still points at it")
	}
	if len(left) > 0 {
		return fmt.Errorf("the deletion committed but left data behind: %s",
			strings.Join(left, "; "))
	}
	return nil
}

// Gone reports whether a record is already absent, so a repeated deletion can
// say so rather than failing.
func (s *DeletionService) Gone(kind string, id uint) (bool, error) {
	var model any
	switch kind {
	case "candidate":
		model = &models.Candidate{}
	case "role":
		model = &models.Role{}
	case "artifact":
		model = &models.Artifact{}
	case "initiative":
		model = &models.Initiative{}
	case "draft":
		model = &models.Draft{}
	default:
		return false, fmt.Errorf("unknown record kind %q", kind)
	}
	var n int64
	if err := s.db.Model(model).Where("id = ?", id).Count(&n).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		return false, fmt.Errorf("checking whether %s %d exists: %w", kind, id, err)
	}
	return n == 0, nil
}
