package main

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// RoleProfileService decomposes roles automatically.
//
// This is Phase 11 with the approval removed, and the asymmetry is deliberate:
// a recruiter running a search sees twenty discovered listings in a sitting, and
// approving twenty decompositions before any matching could happen would defeat
// the workflow the product exists to provide.
//
// What replaces approval is honesty about state. A Role Profile is Ready,
// Failed, or Stale; only Ready is assessed; and neither Failed nor Stale ever
// leaves the screen — a failed decomposition that vanishes is indistinguishable
// from a role that was never discovered, and the recruiter's response to those
// is completely different.
type RoleProfileService struct {
	db       *gorm.DB
	classify *ClassifyService
}

// NewRoleProfileService wires role profiling to the classifier.
func NewRoleProfileService(db *gorm.DB, classify *ClassifyService) *RoleProfileService {
	return &RoleProfileService{db: db, classify: classify}
}

// The role profile lifecycle states. They are computed rather than stored:
// nothing is written that could be out of date.
//
// Named RoleProfile* rather than Role*: models.RoleStale is a different thing
// entirely — a discovered listing that has gone off the market — and two
// meanings of "a stale role" in one codebase is a bug waiting for a reader in a
// hurry.
const (
	RoleProfileReady   = "ready"
	RoleProfileFailed  = "failed"
	RoleProfileStale   = "stale"
	RoleProfileMissing = "unprofiled"
)

// roleRecord names the record a recruiter-authored role aspect cites.
func roleRecord(roleID uint) string { return fmt.Sprintf("role %d", roleID) }

// Profile decomposes a role from its current source content.
func (s *RoleProfileService) Profile(roleID uint) (*models.Profile, error) {
	ids, err := s.sourceChunks(roleID)
	if err != nil {
		return nil, err
	}
	return s.classify.Classify(ClassifyInput{
		SubjectKind: profile.SubjectRole,
		SubjectID:   roleID,
		ChunkIDs:    ids,
	})
}

// sourceChunks lists the chunks of every extracted artifact linked to the role.
func (s *RoleProfileService) sourceChunks(roleID uint) ([]uint, error) {
	ids := []uint{}
	err := s.db.Model(&models.Chunk{}).
		Where("artifact_id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)",
			models.LinkRole, roleID).
		Order("artifact_id asc, ordinal asc").Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("listing source chunks for role %d: %w", roleID, err)
	}
	return ids, nil
}

// CurrentSourceHash is what the role's evidence right now hashes to.
func (s *RoleProfileService) CurrentSourceHash(roleID uint) (string, error) {
	ids, err := s.sourceChunks(roleID)
	if err != nil {
		return "", err
	}
	return hashChunks(s.db, ids)
}

// hashChunks hashes a set of chunks in the given order. Shared with the
// candidate side, because "the evidence changed" has to mean the same thing on
// both halves of a match.
func hashChunks(db *gorm.DB, ids []uint) (string, error) {
	rows := []models.Chunk{}
	if len(ids) > 0 {
		if err := db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return "", fmt.Errorf("loading source chunks: %w", err)
		}
	}
	byID := make(map[uint]models.Chunk, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	sources := make([]profile.Source, 0, len(ids))
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			sources = append(sources, profile.Source{ChunkID: r.ID, Text: r.Text})
		}
	}
	return profile.HashSources(sources), nil
}

// RoleStatus is one role's profile state, for the Research listing.
//
// Every role has one, including roles with no profile: an absence in a list is
// how a failure becomes invisible.
type RoleStatus struct {
	RoleID    uint   `json:"roleId"`
	ProfileID uint   `json:"profileId"`
	State     string `json:"state"`
	// Reason explains Failed and Stale. Empty when Ready.
	Reason string `json:"reason"`
	// Aspects is the profile's content, when it has one.
	Aspects []models.ProfileAspect `json:"aspects"`
}

// Status reports one role's profile state.
func (s *RoleProfileService) Status(roleID uint) (*RoleStatus, error) {
	out := &RoleStatus{RoleID: roleID, State: RoleProfileMissing,
		Reason: "this role has not been profiled yet"}

	current, err := s.classify.Current(profile.SubjectRole, roleID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return out, nil
	}
	out.ProfileID = current.ID
	out.Aspects = current.Aspects

	if current.State == string(models.ProfileFailed) {
		out.State = RoleProfileFailed
		out.Reason = fmt.Sprintf("this listing could not be decomposed (%s) — retry it, or enter the requirements by hand",
			current.FailureReason)
		return out, nil
	}

	hash, err := s.CurrentSourceHash(roleID)
	if err != nil {
		return nil, err
	}
	if hash != current.SourceHash {
		out.State = RoleProfileStale
		out.Reason = "the listing has changed since this profile was made — profile it again"
		return out, nil
	}
	out.State = RoleProfileReady
	return out, nil
}

// RoleEligibility is whether a role may enter automatic assessment.
//
// Same shape as candidate readiness on purpose: matching asks about both sides,
// and two differently-shaped answers would grow two code paths that drift.
//
// Unlike candidates there is no eligible-with-a-warning case. A recruiter knows
// a candidate and can vouch for them from memory; nobody has independent
// knowledge of a listing that changed, and reprofiling is one cheap local call.
type RoleEligibility struct {
	RoleID    uint   `json:"roleId"`
	Eligible  bool   `json:"eligible"`
	Reason    string `json:"reason"`
	ProfileID uint   `json:"profileId"`
}

// Eligibility reports whether this role may be assessed automatically.
//
// Two questions, because the word stale means two things here. A profile is
// stale when the listing changed after it was extracted. A role is Stale when it
// closed, or when thirty days passed without it being seen — and the PRD says
// those are "visibly labeled and excluded from matching".
//
// Only the first was asked. So a discovered role that closed last month, whose
// profile still matched the listing perfectly, was eligible: it was shortlisted,
// and it could be assessed directly. Both callers asked this one function
// precisely so there would be one answer, which is the right shape — the answer
// was just incomplete.
func (s *RoleProfileService) Eligibility(roleID uint) (*RoleEligibility, error) {
	status, err := s.Status(roleID)
	if err != nil {
		return nil, err
	}
	out := &RoleEligibility{RoleID: roleID, ProfileID: status.ProfileID}

	var role models.Role
	if err := s.db.First(&role, roleID).Error; err != nil {
		return nil, fmt.Errorf("reading role %d: %w", roleID, err)
	}
	switch role.LifecycleState {
	case models.RoleStale:
		out.Reason = "this role is stale — it closed, or thirty days passed without it being seen"
		return out, nil
	case models.RolePurged:
		out.Reason = "this role has been purged"
		return out, nil
	}

	if status.State == RoleProfileReady {
		out.Eligible = true
		return out, nil
	}
	out.Reason = status.Reason
	return out, nil
}

// List reports every role's profile state, so the Research view can show the
// ones that need attention rather than only the ones that worked.
func (s *RoleProfileService) List() ([]RoleStatus, error) {
	ids := []uint{}
	if err := s.db.Model(&models.Role{}).Order("id asc").Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	out := make([]RoleStatus, 0, len(ids))
	for _, id := range ids {
		status, err := s.Status(id)
		if err != nil {
			return nil, err
		}
		out = append(out, *status)
	}
	return out, nil
}

// AddAspect records a requirement the recruiter typed, which is how a listing
// that cannot be decomposed still becomes assessable.
//
// Role aspects may carry a priority; candidate aspects may not. That asymmetry
// is the taxonomy's, and it is the only difference from the candidate call.
func (s *RoleProfileService) AddAspect(roleID uint, aspect profile.Aspect) (*models.Profile, error) {
	hash, err := s.CurrentSourceHash(roleID)
	if err != nil {
		return nil, err
	}
	return s.classify.AddRecruiterAspect(profile.SubjectRole, roleID, aspect, roleRecord(roleID), hash)
}

// EditAspect replaces one requirement's wording, as a new version.
//
// It cannot touch the role's source artifact — those are immutable from Phase 4
// — and that is a feature: a recruiter who knows the listing's "5+ years" is
// negotiable can say so, while the citation still points at the listing saying
// five, which is exactly the account a person should see.
func (s *RoleProfileService) EditAspect(roleID uint, ordinal int, wording string, priority profile.Priority) (*models.Profile, error) {
	aspects, base, err := s.workingSet(roleID)
	if err != nil {
		return nil, err
	}
	if ordinal < 0 || ordinal >= len(aspects) {
		return nil, fmt.Errorf("this profile has no requirement %d", ordinal+1)
	}
	edited := aspects[ordinal]
	edited.Wording = strings.TrimSpace(wording)
	edited.Origin = profile.RecruiterSupplied
	edited.Citations = []profile.Citation{{Record: roleRecord(roleID)}}
	if priority != "" {
		edited.Priority = priority
	}
	if problems := profile.Validate(profile.SubjectRole,
		profile.Proposal{Aspects: []profile.Aspect{edited}}, nil); len(problems) > 0 {
		return nil, fmt.Errorf("that edit does not satisfy the contract:\n- %s",
			strings.Join(problems, "\n- "))
	}
	aspects[ordinal] = edited
	return s.newVersion(roleID, base, aspects)
}

// RemoveAspect drops one requirement, as a new version.
func (s *RoleProfileService) RemoveAspect(roleID uint, ordinal int) (*models.Profile, error) {
	aspects, base, err := s.workingSet(roleID)
	if err != nil {
		return nil, err
	}
	if ordinal < 0 || ordinal >= len(aspects) {
		return nil, fmt.Errorf("this profile has no requirement %d", ordinal+1)
	}
	kept := append(append([]profile.Aspect{}, aspects[:ordinal]...), aspects[ordinal+1:]...)
	return s.newVersion(roleID, base, kept)
}

func (s *RoleProfileService) workingSet(roleID uint) ([]profile.Aspect, *models.Profile, error) {
	base, err := s.classify.Current(profile.SubjectRole, roleID)
	if err != nil {
		return nil, nil, err
	}
	if base == nil {
		return nil, nil, fmt.Errorf("this role has no profile to edit")
	}
	aspects, err := s.classify.Aspects(base.ID)
	if err != nil {
		return nil, nil, err
	}
	return aspects, base, nil
}

// newVersion stores an edited requirement set.
//
// The source hash is carried forward from the version being edited, so an edit
// does not make a Ready profile look Stale — the evidence did not move, the
// recruiter's account of it did.
func (s *RoleProfileService) newVersion(roleID uint, base *models.Profile, aspects []profile.Aspect) (*models.Profile, error) {
	identity := profile.Identity{
		SchemaVersion: profile.SchemaVersion,
		PromptVersion: profile.PromptVersion,
		Revision:      0,
		SourceHash:    base.SourceHash,
	}
	return s.classify.store(
		ClassifyInput{SubjectKind: profile.SubjectRole, SubjectID: roleID},
		identity, "", aspects, models.ProfileProposed, "")
}

// Citations resolves a role profile's aspects back to the listing wording they
// came from.
func (s *RoleProfileService) Citations(profileID uint) ([]AspectCitation, error) {
	return citationsOf(s.db, s.classify, profileID)
}
