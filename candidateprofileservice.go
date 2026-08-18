package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// CandidateProfileService is where a person decides whether the model's account
// of a candidate is true enough to act on.
//
// Nothing in Phases 6 through 10 can answer that. A citation that resolves
// proves the model did not invent the sentence; it does not prove the sentence
// means what the aspect says. So a recruiter says yes, and this service's job is
// to make that yes cheap to give, hard to give by accident, and impossible to
// forget was given about a particular set of evidence.
type CandidateProfileService struct {
	db       *gorm.DB
	classify *ClassifyService
	records  *RecordService
}

// NewCandidateProfileService wires the profile lifecycle to the classifier.
func NewCandidateProfileService(db *gorm.DB, classify *ClassifyService, records *RecordService) *CandidateProfileService {
	return &CandidateProfileService{db: db, classify: classify, records: records}
}

// recruiterRecord names the record a recruiter-authored aspect cites.
func recruiterRecord(candidateID uint) string {
	return fmt.Sprintf("candidate %d", candidateID)
}

// Classify builds a Proposed profile from the candidate's record and their
// linked, extracted artifacts.
//
// The structured record is a source too: a candidate whose availability the
// recruiter typed should not produce a profile that does not know when they are
// available. It is simply a source with a different kind of citation, which the
// contract already supports.
func (s *CandidateProfileService) Classify(candidateID uint) (*models.Profile, error) {
	chunkIDs, err := s.sourceChunks(candidateID)
	if err != nil {
		return nil, err
	}
	if len(chunkIDs) == 0 {
		// No documents is not a failure: the record alone may be enough, and a
		// hand-built profile is a first-class profile.
		return s.fromRecordOnly(candidateID)
	}
	return s.classify.Classify(ClassifyInput{
		SubjectKind: profile.SubjectCandidate,
		SubjectID:   candidateID,
		ChunkIDs:    chunkIDs,
	})
}

// fromRecordOnly makes a version out of the structured record alone.
func (s *CandidateProfileService) fromRecordOnly(candidateID uint) (*models.Profile, error) {
	aspects, err := s.recordAspects(candidateID)
	if err != nil {
		return nil, err
	}
	if len(aspects) == 0 {
		return nil, fmt.Errorf("this candidate has no extracted documents and no structured details to build a profile from")
	}
	hash, err := s.CurrentSourceHash(candidateID)
	if err != nil {
		return nil, err
	}
	var current *models.Profile
	for _, a := range aspects {
		current, err = s.classify.AddRecruiterAspect(
			profile.SubjectCandidate, candidateID, a, recruiterRecord(candidateID), hash)
		if err != nil && !strings.Contains(err.Error(), "duplicates aspect") {
			return nil, err
		}
	}
	if current == nil {
		return s.classify.Current(profile.SubjectCandidate, candidateID)
	}
	return current, nil
}

// recordAspects turns the structured candidate record into aspects. They are
// recruiter supplied because a person typed them, and they cite the record.
func (s *CandidateProfileService) recordAspects(candidateID uint) ([]profile.Aspect, error) {
	c, err := s.records.GetCandidate(candidateID)
	if err != nil {
		return nil, err
	}
	record := recruiterRecord(candidateID)
	cite := []profile.Citation{{Record: record}}
	aspects := []profile.Aspect{}
	// No structured values: the record's fields are free text a recruiter
	// typed, and normalizing them is the classifier's job when a document says
	// the same thing more precisely.
	add := func(t profile.AspectType, wording string) {
		if strings.TrimSpace(wording) == "" {
			return
		}
		aspects = append(aspects, profile.Aspect{
			Type: t, Wording: wording,
			Origin: profile.RecruiterSupplied, Citations: cite,
		})
	}
	add(profile.Location, c.Location)
	add(profile.WorkRights, c.WorkRights)
	add(profile.EmploymentType, c.DesiredEmploymentType)
	add(profile.WorkArrangement, c.DesiredWorkArrangement)
	return aspects, nil
}

// sourceChunks lists the chunks of every extracted artifact linked to the
// candidate. This is also what the current source hash is computed over, so
// "the evidence changed" and "what we classify from changed" are the same fact.
func (s *CandidateProfileService) sourceChunks(candidateID uint) ([]uint, error) {
	ids := []uint{}
	err := s.db.Model(&models.Chunk{}).
		Where("artifact_id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)",
			models.LinkCandidate, candidateID).
		Order("artifact_id asc, ordinal asc").Pluck("id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("listing source chunks for candidate %d: %w", candidateID, err)
	}
	return ids, nil
}

// CurrentSourceHash is what "the evidence right now" hashes to.
func (s *CandidateProfileService) CurrentSourceHash(candidateID uint) (string, error) {
	ids, err := s.sourceChunks(candidateID)
	if err != nil {
		return "", err
	}
	return hashChunks(s.db, ids)
}

// Approve stamps a version as the one search and matching use.
func (s *CandidateProfileService) Approve(profileID uint) (*models.Profile, error) {
	var row models.Profile
	if err := s.db.First(&row, profileID).Error; err != nil {
		return nil, fmt.Errorf("loading profile %d: %w", profileID, err)
	}
	if row.SubjectKind != string(profile.SubjectCandidate) {
		return nil, fmt.Errorf("only Candidate Profiles are approved; profile %d is a %s profile",
			profileID, row.SubjectKind)
	}
	if row.State == string(models.ProfileFailed) {
		return nil, fmt.Errorf("a failed profile cannot be approved — retry it, or build one by hand")
	}
	hash, err := s.CurrentSourceHash(row.SubjectID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	err = s.db.Model(&models.Profile{}).Where("id = ?", profileID).Updates(map[string]any{
		"state":                models.ProfileApproved,
		"approved_at":          now,
		"approved_source_hash": hash,
	}).Error
	if err != nil {
		return nil, fmt.Errorf("approving profile %d: %w", profileID, err)
	}
	return s.Approved(row.SubjectID)
}

// Approved returns the candidate's approved version, or nil when there is none.
//
// The newest approval wins: approving again after a source change is how
// staleness is cleared, so the latest stamp is the one in force.
func (s *CandidateProfileService) Approved(candidateID uint) (*models.Profile, error) {
	var row models.Profile
	err := s.db.Where("subject_kind = ? AND subject_id = ? AND approved_at IS NOT NULL",
		profile.SubjectCandidate, candidateID).
		Order("approved_at desc, version desc").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading the approved profile for candidate %d: %w", candidateID, err)
	}
	row.Aspects, err = s.aspects(row.ID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// Readiness is the gate. Everything that consumes a Candidate Profile asks this
// rather than inspecting a state, because Stale is the case that matters — a
// stale approved profile is usable with a warning, not blocked, and four copies
// of that rule would eventually disagree about it.
type Readiness struct {
	CandidateID uint `json:"candidateId"`
	Ready       bool `json:"ready"`
	// Reason is why not, when not ready. Empty when ready.
	Reason string `json:"reason"`
	// Warning is carried alongside permission, never instead of it.
	Warning string `json:"warning"`
	Stale   bool   `json:"stale"`
	// ProfileID is the version in use, when there is one.
	ProfileID uint `json:"profileId"`
}

// Readiness reports whether this candidate may be searched and matched.
func (s *CandidateProfileService) Readiness(candidateID uint) (*Readiness, error) {
	out := &Readiness{CandidateID: candidateID}
	approved, err := s.Approved(candidateID)
	if err != nil {
		return nil, err
	}
	if approved == nil {
		current, err := s.classify.Current(profile.SubjectCandidate, candidateID)
		if err != nil {
			return nil, err
		}
		switch {
		case current == nil:
			out.Reason = "this candidate has no profile yet"
		case current.State == string(models.ProfileFailed):
			out.Reason = fmt.Sprintf("this candidate's profile could not be built (%s) — retry it, or build one by hand",
				current.FailureReason)
		default:
			out.Reason = "this candidate's profile has not been approved yet"
		}
		return out, nil
	}

	out.Ready = true
	out.ProfileID = approved.ID
	hash, err := s.CurrentSourceHash(candidateID)
	if err != nil {
		return nil, err
	}
	if hash != approved.ApprovedSourceHash {
		out.Stale = true
		out.Warning = "the evidence has changed since this profile was approved — review and approve it again"
	}
	return out, nil
}

// EditAspect replaces one aspect's wording and structured value, as a new
// version.
//
// Making an edit a version rather than a mutation costs a row and buys the
// property that every profile ever on screen was, at some moment, exactly what
// somebody looked at. The edited aspect becomes recruiter supplied, because a
// person now asserts it.
func (s *CandidateProfileService) EditAspect(
	candidateID uint, ordinal int, wording string, structured map[string]any,
) (*models.Profile, error) {
	aspects, base, err := s.workingSet(candidateID)
	if err != nil {
		return nil, err
	}
	if ordinal < 0 || ordinal >= len(aspects) {
		return nil, fmt.Errorf("this profile has no aspect %d", ordinal+1)
	}
	edited := aspects[ordinal]
	edited.Wording = strings.TrimSpace(wording)
	edited.Structured = structured
	edited.Origin = profile.RecruiterSupplied
	edited.Citations = []profile.Citation{{Record: recruiterRecord(candidateID)}}

	if problems := profile.Validate(profile.SubjectCandidate,
		profile.Proposal{Aspects: []profile.Aspect{edited}}, nil); len(problems) > 0 {
		return nil, fmt.Errorf("that edit does not satisfy the contract:\n- %s",
			strings.Join(problems, "\n- "))
	}
	aspects[ordinal] = edited
	return s.newVersion(candidateID, base, aspects)
}

// RemoveAspect drops one aspect, as a new version.
func (s *CandidateProfileService) RemoveAspect(candidateID uint, ordinal int) (*models.Profile, error) {
	aspects, base, err := s.workingSet(candidateID)
	if err != nil {
		return nil, err
	}
	if ordinal < 0 || ordinal >= len(aspects) {
		return nil, fmt.Errorf("this profile has no aspect %d", ordinal+1)
	}
	kept := append(append([]profile.Aspect{}, aspects[:ordinal]...), aspects[ordinal+1:]...)
	return s.newVersion(candidateID, base, kept)
}

// AddAspect records a fact the recruiter asserted.
func (s *CandidateProfileService) AddAspect(candidateID uint, aspect profile.Aspect) (*models.Profile, error) {
	hash, err := s.CurrentSourceHash(candidateID)
	if err != nil {
		return nil, err
	}
	return s.classify.AddRecruiterAspect(
		profile.SubjectCandidate, candidateID, aspect, recruiterRecord(candidateID), hash)
}

// workingSet returns the aspects an edit applies to, and the version they came
// from. Edits apply to the version in use: the approved one when there is one,
// otherwise the newest.
func (s *CandidateProfileService) workingSet(candidateID uint) ([]profile.Aspect, *models.Profile, error) {
	base, err := s.InUse(candidateID)
	if err != nil {
		return nil, nil, err
	}
	if base == nil {
		return nil, nil, fmt.Errorf("this candidate has no profile to edit")
	}
	aspects, err := s.classify.Aspects(base.ID)
	if err != nil {
		return nil, nil, err
	}
	return aspects, base, nil
}

// InUse returns the version the application acts on: the approved one when
// there is one, otherwise the newest.
func (s *CandidateProfileService) InUse(candidateID uint) (*models.Profile, error) {
	approved, err := s.Approved(candidateID)
	if err != nil || approved != nil {
		return approved, err
	}
	return s.classify.Current(profile.SubjectCandidate, candidateID)
}

// newVersion stores an edited aspect set as a new Proposed version.
func (s *CandidateProfileService) newVersion(
	candidateID uint, base *models.Profile, aspects []profile.Aspect,
) (*models.Profile, error) {
	identity := profile.Identity{
		SchemaVersion: profile.SchemaVersion,
		PromptVersion: profile.PromptVersion,
		// A recruiter-made version had no model. Zero is honest here, and it is
		// why the identity hash distinguishes it from any classified version.
		Revision:   0,
		SourceHash: base.SourceHash,
	}
	return s.classify.store(
		ClassifyInput{SubjectKind: profile.SubjectCandidate, SubjectID: candidateID},
		identity, "", aspects, models.ProfileProposed, "")
}

// aspects loads a version's aspects as stored rows.
func (s *CandidateProfileService) aspects(profileID uint) ([]models.ProfileAspect, error) {
	rows := []models.ProfileAspect{}
	err := s.db.Where("profile_id = ?", profileID).Order("ordinal asc").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing aspects of profile %d: %w", profileID, err)
	}
	return rows, nil
}

// DiffSide is one version's account of an aspect, for display.
type DiffSide struct {
	Ordinal    int            `json:"ordinal"`
	Type       string         `json:"type"`
	Wording    string         `json:"wording"`
	Structured map[string]any `json:"structured"`
	Origin     string         `json:"origin"`
}

// Conflict is one aspect both versions describe, differently.
type Conflict struct {
	Approved DiffSide `json:"approved"`
	Proposed DiffSide `json:"proposed"`
}

// Diff is a reclassification presented against what was approved.
type Diff struct {
	ApprovedProfileID uint       `json:"approvedProfileId"`
	ProposedProfileID uint       `json:"proposedProfileId"`
	Additions         []DiffSide `json:"additions"`
	Removals          []DiffSide `json:"removals"`
	Conflicts         []Conflict `json:"conflicts"`
}

// DiffAgainstApproved compares a proposed version with the approved one.
//
// It is a pure function of two stored versions: no model is called, so the same
// two versions always produce the same three lists. Correspondence is a
// heuristic — same type plus agreeing structured field, else substantially
// overlapping wording — and when it is wrong it produces a conflict that was
// really an addition and a removal, which is a mildly annoying review rather
// than a wrong profile.
func (s *CandidateProfileService) DiffAgainstApproved(candidateID, proposedID uint) (*Diff, error) {
	approved, err := s.Approved(candidateID)
	if err != nil {
		return nil, err
	}
	if approved == nil {
		return nil, fmt.Errorf("this candidate has no approved profile to compare against")
	}
	approvedAspects, err := s.classify.Aspects(approved.ID)
	if err != nil {
		return nil, err
	}
	proposedAspects, err := s.classify.Aspects(proposedID)
	if err != nil {
		return nil, err
	}

	out := &Diff{
		ApprovedProfileID: approved.ID,
		ProposedProfileID: proposedID,
		Additions:         []DiffSide{},
		Removals:          []DiffSide{},
		Conflicts:         []Conflict{},
	}
	matched := make([]bool, len(approvedAspects))
	for i, p := range proposedAspects {
		j := correspondent(p, approvedAspects, matched)
		if j < 0 {
			out.Additions = append(out.Additions, side(i, p))
			continue
		}
		matched[j] = true
		if profile.MeaningKey(p) == profile.MeaningKey(approvedAspects[j]) {
			// Identical: neither an addition, a removal, nor a conflict.
			continue
		}
		out.Conflicts = append(out.Conflicts, Conflict{
			Approved: side(j, approvedAspects[j]),
			Proposed: side(i, p),
		})
	}
	for j, a := range approvedAspects {
		if !matched[j] {
			out.Removals = append(out.Removals, side(j, a))
		}
	}
	return out, nil
}

// ResolveConflicts produces a new version taking, for each conflict, either the
// approved aspect or the proposed one. Neither compared version is modified.
func (s *CandidateProfileService) ResolveConflicts(
	candidateID, proposedID uint, takeProposed []int,
) (*models.Profile, error) {
	diff, err := s.DiffAgainstApproved(candidateID, proposedID)
	if err != nil {
		return nil, err
	}
	take := make(map[int]bool, len(takeProposed))
	for _, i := range takeProposed {
		take[i] = true
	}
	approved, err := s.Approved(candidateID)
	if err != nil {
		return nil, err
	}
	approvedAspects, err := s.classify.Aspects(approved.ID)
	if err != nil {
		return nil, err
	}
	proposedAspects, err := s.classify.Aspects(proposedID)
	if err != nil {
		return nil, err
	}

	// Start from the approved account, swap in the chosen proposed sides, then
	// append what the proposal added.
	result := append([]profile.Aspect{}, approvedAspects...)
	for i, c := range diff.Conflicts {
		if !take[i] {
			continue
		}
		if c.Approved.Ordinal < len(result) && c.Proposed.Ordinal < len(proposedAspects) {
			result[c.Approved.Ordinal] = proposedAspects[c.Proposed.Ordinal]
		}
	}
	for _, add := range diff.Additions {
		if add.Ordinal < len(proposedAspects) {
			result = append(result, proposedAspects[add.Ordinal])
		}
	}
	base := approved
	return s.newVersion(candidateID, base, result)
}

// correspondent finds the approved aspect a proposed one is about, or -1.
func correspondent(p profile.Aspect, approved []profile.Aspect, matched []bool) int {
	for j, a := range approved {
		if matched[j] || a.Type != p.Type {
			continue
		}
		if sharesStructuredField(a, p) || wordingOverlaps(a.Wording, p.Wording) {
			return j
		}
	}
	return -1
}

// sharesStructuredField reports whether two aspects normalize the same field,
// which is the strongest signal that they are about the same thing.
func sharesStructuredField(a, b profile.Aspect) bool {
	for name := range a.Structured {
		if _, ok := b.Structured[name]; ok {
			return true
		}
	}
	return false
}

// wordingOverlaps is the fallback: enough shared words that these are two
// accounts of one fact rather than two facts.
func wordingOverlaps(a, b string) bool {
	left := wordSet(a)
	right := wordSet(b)
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	shared := 0
	for w := range left {
		if right[w] {
			shared++
		}
	}
	smaller := min(len(left), len(right))
	// Half the shorter side. Loose enough to pair a reworded requirement,
	// tight enough not to pair two unrelated skills that both say "experience".
	return shared*2 >= smaller
}

func wordSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:()[]\"'")
		if len(w) > 2 {
			out[w] = true
		}
	}
	return out
}

func side(ordinal int, a profile.Aspect) DiffSide {
	return DiffSide{
		Ordinal:    ordinal,
		Type:       string(a.Type),
		Wording:    a.Wording,
		Structured: a.Structured,
		Origin:     string(a.Origin),
	}
}

// AspectCitation is one aspect's evidence, resolved for display.
type AspectCitation struct {
	Ordinal int `json:"ordinal"`
	// Record is set for recruiter supplied aspects.
	Record string `json:"record"`
	// Artifact and Location are set for extracted ones.
	ArtifactID   uint   `json:"artifactId"`
	ArtifactName string `json:"artifactName"`
	Location     string `json:"location"`
	// Text a stranger wrote: displayed, never rendered, never executed.
	Text string `json:"text"`
}

// Citations resolves a version's aspects back to what they came from, so a
// recruiter reviewing a profile can see the source wording without leaving the
// screen.
func (s *CandidateProfileService) Citations(profileID uint) ([]AspectCitation, error) {
	return citationsOf(s.db, s.classify, profileID)
}

// citationsOf is the shared resolver: candidates and roles cite the same way,
// so they read the same way too.
func citationsOf(db *gorm.DB, classify *ClassifyService, profileID uint) ([]AspectCitation, error) {
	aspects, err := classify.Aspects(profileID)
	if err != nil {
		return nil, err
	}
	out := []AspectCitation{}
	for i, a := range aspects {
		for _, c := range a.Citations {
			if c.Record != "" {
				out = append(out, AspectCitation{Ordinal: i, Record: c.Record, Text: a.Wording})
				continue
			}
			var chunk models.Chunk
			if err := db.First(&chunk, c.ChunkID).Error; err != nil {
				// The evidence is gone. Say so rather than quoting nothing.
				out = append(out, AspectCitation{
					Ordinal:  i,
					Location: fmt.Sprintf("the cited section is no longer available (chunk %d)", c.ChunkID),
				})
				continue
			}
			var artifact models.Artifact
			if err := db.Select("id", "display_name").First(&artifact, chunk.ArtifactID).Error; err != nil {
				return nil, fmt.Errorf("loading artifact %d: %w", chunk.ArtifactID, err)
			}
			out = append(out, AspectCitation{
				Ordinal:      i,
				ArtifactID:   artifact.ID,
				ArtifactName: artifact.DisplayName,
				Location:     fmt.Sprintf("%s (section %d)", artifact.DisplayName, chunk.Ordinal+1),
				Text:         chunk.Text,
			})
		}
	}
	return out, nil
}
