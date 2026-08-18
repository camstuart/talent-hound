package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// profileEnv is a classifyEnv with the candidate lifecycle wired in.
type profileEnv struct {
	*classifyEnv
	records  *RecordService
	profiles *CandidateProfileService
}

func newProfileEnv(t *testing.T) *profileEnv {
	t.Helper()
	base := newClassifyEnv(t)
	records := NewRecordService(base.db)
	return &profileEnv{
		classifyEnv: base,
		records:     records,
		profiles:    NewCandidateProfileService(base.db, base.classify, records),
	}
}

const resumeMarkdown = `# Kalinda Reyes

## Experience

Senior platform engineer at Northwind. Go and SQLite in production since 2019.

## Location

Melbourne, hybrid preferred.
`

// candidateWithResume creates the one candidate these tests use, attaches an
// extracted resume, and chunks it.
func (e *profileEnv) candidateWithResume(t *testing.T) uint {
	t.Helper()
	const name = "Kalinda Reyes"
	const markdown = resumeMarkdown
	c, err := e.records.CreateCandidate(models.Candidate{FullName: name})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	a, err := e.artifacts.create(name+" resume", "resume.md", "test", []byte(markdown),
		models.LinkCandidate, c.ID)
	if err != nil {
		t.Fatalf("attaching resume: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state":  models.ExtractionExtracted,
		"extractor":         "native-text",
		"extractor_version": "1",
		"markdown":          markdown,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	chunks := e.chunkAndWait(t, a.ID)
	e.chunks2 = chunks
	return c.ID
}

// answer scripts a classifier response citing whichever chunk holds a phrase.
func (e *profileEnv) answer(t *testing.T, aspects ...profile.Aspect) string {
	t.Helper()
	const quote = skillQuote
	id := e.chunkQuoting(t)
	for i := range aspects {
		if aspects[i].Citations == nil {
			aspects[i].Citations = []profile.Citation{{ChunkID: id, Quote: quote}}
		}
	}
	return jsonProposal(t, aspects)
}

func jsonProposal(t *testing.T, aspects []profile.Aspect) string {
	t.Helper()
	raw, err := jsonMarshal(profile.Proposal{Aspects: aspects})
	if err != nil {
		t.Fatalf("encoding a proposal: %v", err)
	}
	return raw
}

func TestANewCandidateProfileIsProposedAndBlocksMatching(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)

	// No profile at all: blocked, with a reason rather than an empty result.
	r, err := e.profiles.Readiness(id)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if r.Ready || !strings.Contains(r.Reason, "no profile") {
		t.Fatalf("a candidate with no profile reported ready=%v reason=%q", r.Ready, r.Reason)
	}

	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{e.answer(t,
		profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production"})}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if p.State != string(models.ProfileProposed) {
		t.Fatalf("a new profile is %q, want proposed", p.State)
	}

	r, err = e.profiles.Readiness(id)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if r.Ready || !strings.Contains(r.Reason, "not been approved") {
		t.Fatalf("a Proposed profile reported ready=%v reason=%q", r.Ready, r.Reason)
	}
}

func TestApprovalUnblocksAndFreezesTheVersion(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{e.answer(t,
		profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production"})}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}

	stamped, err := e.profiles.Approve(p.ID)
	if err != nil {
		t.Fatalf("approving: %v", err)
	}
	if stamped == nil || stamped.ApprovedAt == nil {
		t.Fatalf("approval returned %+v with no approval stamp", stamped)
	}
	if stamped.State != string(models.ProfileApproved) {
		t.Errorf("approved profile is in state %q", stamped.State)
	}
	r, err := e.profiles.Readiness(id)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if !r.Ready || r.Stale || r.Warning != "" {
		t.Fatalf("an approved profile reported ready=%v stale=%v warning=%q", r.Ready, r.Stale, r.Warning)
	}
	if r.ProfileID != p.ID {
		t.Errorf("readiness names profile %d, want the approved %d", r.ProfileID, p.ID)
	}

	// The cited evidence still resolves.
	cites, err := e.profiles.Citations(p.ID)
	if err != nil {
		t.Fatalf("resolving citations: %v", err)
	}
	if len(cites) == 0 {
		t.Fatal("an approved profile resolved no citations")
	}
	for _, c := range cites {
		if c.Record == "" && !strings.Contains(c.Text, "Go and SQLite") {
			t.Errorf("a citation resolved to text that does not contain the quote: %q", c.Text)
		}
	}
}

func TestASourceChangeMakesTheApprovedProfileStaleButStillUsable(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{e.answer(t,
		profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production"})}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}

	// Another resume arrives. The evidence moved under the approval.
	second, err := e.artifacts.create("second resume", "second.md", "test",
		[]byte("# Kalinda Reyes\n\nAlso Kubernetes and Terraform.\n"),
		models.LinkCandidate, id)
	if err != nil {
		t.Fatalf("attaching a second resume: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", second.ID).Updates(map[string]any{
		"extraction_state":  models.ExtractionExtracted,
		"extractor":         "native-text",
		"extractor_version": "1",
		"markdown":          "# Kalinda Reyes\n\nAlso Kubernetes and Terraform.\n",
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	e.chunkAndWait(t, second.ID)

	r, err := e.profiles.Readiness(id)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	// Usable with a warning, not blocked: that distinction is why the gate is
	// one call rather than four state checks.
	if !r.Ready {
		t.Fatal("a stale approved profile was blocked — it should be usable with a warning")
	}
	if !r.Stale || r.Warning == "" {
		t.Fatalf("a changed source did not produce staleness: stale=%v warning=%q", r.Stale, r.Warning)
	}

	// The approved version is untouched.
	approved, err := e.profiles.Approved(id)
	if err != nil || approved == nil {
		t.Fatalf("approved profile: %v %v", approved, err)
	}
	if approved.ID != p.ID {
		t.Errorf("the approved version changed from %d to %d without an approval", p.ID, approved.ID)
	}
}

// Every way a source can change makes the profile stale.
func TestEveryKindOfSourceChangeProducesStaleness(t *testing.T) {
	cases := []struct {
		name   string
		change func(t *testing.T, e *profileEnv, candidateID, artifactID uint)
	}{
		{
			name: "the extracted content is replaced",
			change: func(t *testing.T, e *profileEnv, _, artifactID uint) {
				replaced := "# Kalinda Reyes\n\n## Experience\n\nNow mostly Rust.\n"
				err := e.db.Model(&models.Artifact{}).Where("id = ?", artifactID).
					Update("markdown", replaced).Error
				if err != nil {
					t.Fatalf("replacing markdown: %v", err)
				}
				e.chunkAndWait(t, artifactID)
			},
		},
		{
			name: "the artifact is detached",
			change: func(t *testing.T, e *profileEnv, candidateID, artifactID uint) {
				err := e.db.Where("artifact_id = ? AND target_type = ? AND target_id = ?",
					artifactID, models.LinkCandidate, candidateID).
					Delete(&models.ArtifactLink{}).Error
				if err != nil {
					t.Fatalf("detaching: %v", err)
				}
			},
		},
		{
			name: "the chunks are removed entirely",
			change: func(t *testing.T, e *profileEnv, _, artifactID uint) {
				err := e.db.Where("artifact_id = ?", artifactID).Delete(&models.Chunk{}).Error
				if err != nil {
					t.Fatalf("removing chunks: %v", err)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newProfileEnv(t)
			id := e.candidateWithResume(t)
			var artifactID uint
			if err := e.db.Model(&models.Chunk{}).Select("artifact_id").
				Limit(1).Scan(&artifactID).Error; err != nil {
				t.Fatalf("finding the artifact: %v", err)
			}
			e.assignClassify(t, "synthetic-classify")
			e.model.responses = []string{e.answer(t,
				profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production"})}
			p, err := e.profiles.Classify(id)
			if err != nil {
				t.Fatalf("classifying: %v", err)
			}
			if _, err := e.profiles.Approve(p.ID); err != nil {
				t.Fatalf("approving: %v", err)
			}

			c.change(t, e, id, artifactID)

			r, err := e.profiles.Readiness(id)
			if err != nil {
				t.Fatalf("readiness: %v", err)
			}
			if !r.Stale {
				t.Fatalf("%s did not make the approved profile stale", c.name)
			}
			if !r.Ready {
				t.Fatalf("%s blocked the profile instead of warning about it", c.name)
			}
		})
	}
}

func TestReapprovalClearsStaleness(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	valid := e.answer(t,
		profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production"})
	e.model.responses = []string{valid, valid, valid}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}

	// Change the evidence, reclassify, approve the new version.
	if err := e.db.Model(&models.Chunk{}).Where("id > 0").
		Update("text", "Go and SQLite in production, plus Kubernetes.").Error; err != nil {
		t.Fatalf("editing chunks: %v", err)
	}
	stale, err := e.profiles.Readiness(id)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if !stale.Stale {
		t.Fatal("the profile did not go stale")
	}

	next, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("reclassifying: %v", err)
	}
	if _, err := e.profiles.Approve(next.ID); err != nil {
		t.Fatalf("re-approving: %v", err)
	}
	fresh, err := e.profiles.Readiness(id)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if fresh.Stale || fresh.Warning != "" {
		t.Fatalf("re-approval did not clear staleness: stale=%v warning=%q", fresh.Stale, fresh.Warning)
	}
	if fresh.ProfileID != next.ID {
		t.Errorf("the version in use is %d, want the newly approved %d", fresh.ProfileID, next.ID)
	}
}

// Reclassification must not touch the approved version. The strongest form of
// "never silently overwritten" is that reclassification does not write to it.
func TestReclassificationNeverOverwritesAnApprovedAspect(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")

	e.model.responses = []string{
		e.answer(t,
			profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production since 2019"}),
	}
	approvedVersion, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(approvedVersion.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}

	// A reclassification that says something different about the same thing.
	e.model.calls = 0
	e.model.responses = []string{
		e.answer(t,
			profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production since 2021"}),
	}
	proposed, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("reclassifying: %v", err)
	}

	// The approved version still says 2019, and is still the one in use.
	inUse, err := e.profiles.InUse(id)
	if err != nil {
		t.Fatalf("in use: %v", err)
	}
	if inUse.ID != approvedVersion.ID {
		t.Fatalf("the version in use is %d, want the approved %d", inUse.ID, approvedVersion.ID)
	}
	if len(inUse.Aspects) != 1 || !strings.Contains(inUse.Aspects[0].Wording, "2019") {
		t.Fatalf("the approved aspect changed: %+v", inUse.Aspects)
	}

	// And the difference is a conflict, not an update.
	diff, err := e.profiles.DiffAgainstApproved(id, proposed.ID)
	if err != nil {
		t.Fatalf("diffing: %v", err)
	}
	if len(diff.Conflicts) != 1 {
		t.Fatalf("got %d conflicts, %d additions, %d removals — want one conflict",
			len(diff.Conflicts), len(diff.Additions), len(diff.Removals))
	}
	if !strings.Contains(diff.Conflicts[0].Approved.Wording, "2019") ||
		!strings.Contains(diff.Conflicts[0].Proposed.Wording, "2021") {
		t.Fatalf("the conflict does not show both sides: %+v", diff.Conflicts[0])
	}
}

func TestTheDiffSeparatesAdditionsRemovalsAndConflicts(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	quote := "Go and SQLite"
	chunk := e.chunkQuoting(t)
	cite := []profile.Citation{{ChunkID: chunk, Quote: quote}}

	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production", Citations: cite},
		{Type: profile.Seniority, Wording: "Senior platform engineer", Citations: cite},
	})}
	approved, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(approved.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}

	e.model.calls = 0
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		// Unchanged.
		{Type: profile.Skill, Wording: "Go and SQLite in production", Citations: cite},
		// Addition: a type the approved version has nothing of.
		{Type: profile.Location, Wording: "Melbourne", Citations: cite},
	})}
	proposed, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("reclassifying: %v", err)
	}

	diff, err := e.profiles.DiffAgainstApproved(id, proposed.ID)
	if err != nil {
		t.Fatalf("diffing: %v", err)
	}
	if len(diff.Additions) != 1 || diff.Additions[0].Type != string(profile.Location) {
		t.Errorf("additions: %+v", diff.Additions)
	}
	if len(diff.Removals) != 1 || diff.Removals[0].Type != string(profile.Seniority) {
		t.Errorf("removals: %+v", diff.Removals)
	}
	if len(diff.Conflicts) != 0 {
		t.Errorf("conflicts: %+v", diff.Conflicts)
	}

	// A diff is a pure comparison: same inputs, same answer, no model call.
	before := e.model.callCount()
	again, err := e.profiles.DiffAgainstApproved(id, proposed.ID)
	if err != nil {
		t.Fatalf("diffing again: %v", err)
	}
	if e.model.callCount() != before {
		t.Error("diffing called a model")
	}
	if len(again.Additions) != len(diff.Additions) ||
		len(again.Removals) != len(diff.Removals) ||
		len(again.Conflicts) != len(diff.Conflicts) {
		t.Error("two diffs of the same versions disagreed")
	}
}

func TestResolvingAConflictProducesAVersionAndModifiesNeitherSide(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	quote := "Go and SQLite"
	cite := []profile.Citation{{ChunkID: e.chunkQuoting(t), Quote: quote}}

	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production since 2019", Citations: cite},
	})}
	approved, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(approved.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}

	e.model.calls = 0
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production since 2021", Citations: cite},
	})}
	proposed, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("reclassifying: %v", err)
	}

	resolved, err := e.profiles.ResolveConflicts(id, proposed.ID, []int{0})
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if len(resolved.Aspects) != 1 || !strings.Contains(resolved.Aspects[0].Wording, "2021") {
		t.Fatalf("taking the proposed side gave %+v", resolved.Aspects)
	}
	if resolved.ID == approved.ID || resolved.ID == proposed.ID {
		t.Fatal("resolution reused one of the compared versions")
	}

	// Neither compared version moved.
	stillApproved, err := e.classify.Aspects(approved.ID)
	if err != nil {
		t.Fatalf("reading the approved version: %v", err)
	}
	if !strings.Contains(stillApproved[0].Wording, "2019") {
		t.Error("the approved version was modified by a resolution")
	}
	stillProposed, err := e.classify.Aspects(proposed.ID)
	if err != nil {
		t.Fatalf("reading the proposed version: %v", err)
	}
	if !strings.Contains(stillProposed[0].Wording, "2021") {
		t.Error("the proposed version was modified by a resolution")
	}
}

func TestKeepingTheApprovedSideOfAConflict(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	quote := "Go and SQLite"
	cite := []profile.Citation{{ChunkID: e.chunkQuoting(t), Quote: quote}}

	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production since 2019", Citations: cite},
	})}
	approved, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(approved.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}
	e.model.calls = 0
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production since 2021", Citations: cite},
	})}
	proposed, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("reclassifying: %v", err)
	}

	// Take nothing: the approved account survives.
	resolved, err := e.profiles.ResolveConflicts(id, proposed.ID, nil)
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if !strings.Contains(resolved.Aspects[0].Wording, "2019") {
		t.Fatalf("keeping the approved side gave %+v", resolved.Aspects)
	}
}

func TestEditingAndRemovingProduceVersionsWithRecruiterOrigin(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	quote := "Go and SQLite"
	cite := []profile.Citation{{ChunkID: e.chunkQuoting(t), Quote: quote}}
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production", Citations: cite},
		{Type: profile.Seniority, Wording: "Senior platform engineer", Citations: cite},
	})}
	original, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}

	edited, err := e.profiles.EditAspect(id, 0, "Go, SQLite, and PostgreSQL in production", nil)
	if err != nil {
		t.Fatalf("editing: %v", err)
	}
	if edited.ID == original.ID {
		t.Fatal("an edit mutated the version instead of making one")
	}
	if edited.Aspects[0].Origin != string(profile.RecruiterSupplied) {
		t.Errorf("an edited aspect has origin %q, want recruiter supplied — a person now asserts it",
			edited.Aspects[0].Origin)
	}
	if !strings.Contains(edited.Aspects[0].Wording, "PostgreSQL") {
		t.Errorf("the edit did not take: %q", edited.Aspects[0].Wording)
	}

	// The original is untouched.
	before, err := e.classify.Aspects(original.ID)
	if err != nil {
		t.Fatalf("reading the original: %v", err)
	}
	if strings.Contains(before[0].Wording, "PostgreSQL") {
		t.Error("the original version was mutated by an edit")
	}

	removed, err := e.profiles.RemoveAspect(id, 1)
	if err != nil {
		t.Fatalf("removing: %v", err)
	}
	if len(removed.Aspects) != 1 {
		t.Fatalf("after removing one of two, %d aspects remain", len(removed.Aspects))
	}
	stillThere, err := e.classify.Aspects(edited.ID)
	if err != nil {
		t.Fatalf("reading the edited version: %v", err)
	}
	if len(stillThere) != 2 {
		t.Error("removal mutated the previous version")
	}
}

// A scanned resume must not make a candidate unusable.
func TestAProfileCanBeBuiltByHandAfterExtractionFails(t *testing.T) {
	e := newProfileEnv(t)
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Tobias Fenn"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	// A resume that could not be extracted: no chunks at all.
	a, err := e.artifacts.create("scan", "scan.pdf", "test", []byte("%PDF-1.4 not really"),
		models.LinkCandidate, c.ID)
	if err != nil {
		t.Fatalf("attaching: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state": models.ExtractionFailed,
		"extraction_error": "unreadable",
	}).Error
	if err != nil {
		t.Fatalf("recording failure: %v", err)
	}

	// Nothing to classify from, and the recruiter builds it anyway.
	for _, aspect := range []profile.Aspect{
		{Type: profile.Skill, Wording: "Financial modelling in Excel"},
		{Type: profile.Seniority, Wording: "Senior analyst"},
	} {
		if _, err := e.profiles.AddAspect(c.ID, aspect); err != nil {
			t.Fatalf("adding an aspect by hand: %v", err)
		}
	}
	built, err := e.profiles.InUse(c.ID)
	if err != nil || built == nil {
		t.Fatalf("hand-built profile: %v %v", built, err)
	}
	if len(built.Aspects) != 2 {
		t.Fatalf("hand-built profile has %d aspects", len(built.Aspects))
	}
	for _, aspect := range built.Aspects {
		if aspect.Origin != string(profile.RecruiterSupplied) {
			t.Errorf("a hand-built aspect has origin %q", aspect.Origin)
		}
		if !strings.Contains(aspect.Citations, "candidate") {
			t.Errorf("a hand-built aspect does not cite its record: %s", aspect.Citations)
		}
	}

	// And it can be approved like any other.
	if _, err := e.profiles.Approve(built.ID); err != nil {
		t.Fatalf("approving a hand-built profile: %v", err)
	}
	r, err := e.profiles.Readiness(c.ID)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if !r.Ready {
		t.Fatalf("a hand-built approved profile is not ready: %q", r.Reason)
	}
}

func TestAFailedProfileIsVisibleRetryableAndBlocks(t *testing.T) {
	e := newProfileEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	bad := `{"aspects":[{"type":"culture_fit","wording":"vibes","citations":[]}]}`
	e.model.responses = []string{bad, bad}

	if _, err := e.profiles.Classify(id); err == nil {
		t.Fatal("an invalid classification was accepted")
	}
	r, err := e.profiles.Readiness(id)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if r.Ready {
		t.Fatal("a failed profile did not block matching")
	}
	if !strings.Contains(r.Reason, "could not be built") {
		t.Errorf("the block reason does not name the failure: %q", r.Reason)
	}
	// A failed version cannot be approved into use.
	current, err := e.classify.Current(profile.SubjectCandidate, id)
	if err != nil || current == nil {
		t.Fatalf("current: %v %v", current, err)
	}
	if _, err := e.profiles.Approve(current.ID); err == nil {
		t.Error("a failed profile was approved")
	}
}

func TestClassificationCombinesTheRecordAndTheArtifacts(t *testing.T) {
	e := newProfileEnv(t)
	c, err := e.records.CreateCandidate(models.Candidate{
		FullName:               "Kalinda Reyes",
		Location:               "Melbourne, VIC",
		WorkRights:             "Australian citizen",
		DesiredWorkArrangement: "hybrid",
	})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	// Record-only first: those aspects cite the record and are recruiter supplied.
	built, err := e.profiles.Classify(c.ID)
	if err != nil {
		t.Fatalf("classifying from the record alone: %v", err)
	}
	kinds := map[string]bool{}
	for _, a := range built.Aspects {
		kinds[a.Type] = true
		if a.Origin != string(profile.RecruiterSupplied) {
			t.Errorf("a record-derived aspect has origin %q", a.Origin)
		}
		if !strings.Contains(a.Citations, "candidate") {
			t.Errorf("a record-derived aspect does not cite the record: %s", a.Citations)
		}
	}
	for _, want := range []string{string(profile.Location), string(profile.WorkRights),
		string(profile.WorkArrangement)} {
		if !kinds[want] {
			t.Errorf("the record produced no %s aspect", want)
		}
	}
}

func TestDroppingAResumeCreatesOneCandidateAndOneArtifact(t *testing.T) {
	e := newProfileEnv(t)
	body := []byte("# Tobias Fenn\n\nFinancial analyst at Harbourline.\n")

	out, err := e.profiles.DropResume(ResumeDrop{
		InitiativeID:     e.initiative,
		FullName:         "Tobias Fenn",
		OriginalFilename: "fenn.md",
		Source:           "drag-and-drop",
		DataBase64:       base64.StdEncoding.EncodeToString(body),
	})
	if err != nil {
		t.Fatalf("dropping: %v", err)
	}
	if !out.Created || out.Candidate == nil || out.Artifact == nil {
		t.Fatalf("drop returned %+v", out)
	}

	var candidates, artifacts int64
	if err := e.db.Model(&models.Candidate{}).Count(&candidates).Error; err != nil {
		t.Fatalf("counting candidates: %v", err)
	}
	if err := e.db.Model(&models.Artifact{}).Count(&artifacts).Error; err != nil {
		t.Fatalf("counting artifacts: %v", err)
	}
	if candidates != 1 || artifacts != 1 {
		t.Fatalf("got %d candidates and %d artifacts, want exactly one of each", candidates, artifacts)
	}

	// Linked to both the person and the workspace it arrived in.
	links, err := e.artifacts.Links(out.Artifact.ID)
	if err != nil {
		t.Fatalf("listing links: %v", err)
	}
	seen := map[models.LinkTarget]bool{}
	for _, l := range links {
		seen[l.TargetType] = true
	}
	if !seen[models.LinkCandidate] || !seen[models.LinkInitiative] {
		t.Fatalf("the dropped resume is linked to %v", seen)
	}
}

func TestAFailedDropCreatesNeitherCandidateNorArtifact(t *testing.T) {
	e := newProfileEnv(t)
	body := base64.StdEncoding.EncodeToString([]byte("# Someone\n"))

	// An initiative that does not exist: the link fails, so the transaction
	// rolls back and the candidate created moments earlier goes with it.
	_, err := e.profiles.DropResume(ResumeDrop{
		InitiativeID:     99999,
		FullName:         "Tobias Fenn",
		OriginalFilename: "fenn.md",
		DataBase64:       body,
	})
	if err == nil {
		t.Fatal("a drop onto a missing initiative succeeded")
	}
	var candidates, artifacts int64
	if err := e.db.Model(&models.Candidate{}).Count(&candidates).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if err := e.db.Model(&models.Artifact{}).Count(&artifacts).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if candidates != 0 || artifacts != 0 {
		t.Fatalf("a failed drop left %d candidates and %d artifacts", candidates, artifacts)
	}
}

func TestDroppingOntoAnExistingCandidateCreatesNoDuplicate(t *testing.T) {
	e := newProfileEnv(t)
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Kalinda Reyes"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	out, err := e.profiles.DropResume(ResumeDrop{
		InitiativeID:     e.initiative,
		CandidateID:      c.ID,
		OriginalFilename: "reyes.md",
		DataBase64:       base64.StdEncoding.EncodeToString([]byte("# Kalinda Reyes\n")),
	})
	if err != nil {
		t.Fatalf("dropping: %v", err)
	}
	if out.Created {
		t.Error("dropping onto an existing candidate reported creating one")
	}
	var candidates int64
	if err := e.db.Model(&models.Candidate{}).Count(&candidates).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if candidates != 1 {
		t.Fatalf("got %d candidates, want the one that already existed", candidates)
	}
}
