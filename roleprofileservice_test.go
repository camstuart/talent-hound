package main

import (
	"strings"
	"testing"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// roleEnv is a classifyEnv with role profiling wired in.
type roleEnv struct {
	*classifyEnv
	records *RecordService
	roles   *RoleProfileService
}

func newRoleEnv(t *testing.T) *roleEnv {
	t.Helper()
	base := newClassifyEnv(t)
	return &roleEnv{
		classifyEnv: base,
		records:     NewRecordService(base.db),
		roles:       NewRoleProfileService(base.db, base.classify),
	}
}

// A listing written so every aspect type has wording to point at, and so that
// exactly one requirement is stated as essential, one as desirable, and one
// with no indication at all.
const listingMarkdown = `# Senior platform engineer — Northwind

## About

Northwind is hiring a senior platform engineer in Melbourne. Hybrid, three days
onsite. Permanent, AUD 180,000 base. You will lead a team of four and report to
the head of engineering.

## Requirements

You must have strong Go and production SQLite experience. Experience operating
multi-region systems would be desirable. We also use Terraform. Existing
Australian work rights are required; we do not sponsor. A postgraduate
qualification is nice to have.
`

// withListing creates a role, attaches an extracted listing, and chunks it.
func (e *roleEnv) withListing(t *testing.T) (uint, uint) {
	t.Helper()
	const markdown = listingMarkdown
	role, err := e.records.CreateRole(models.Role{
		Title:          "Senior platform engineer",
		Origin:         models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating role: %v", err)
	}
	a, err := e.artifacts.create("listing", "listing.md", "test", []byte(markdown),
		models.LinkRole, role.ID)
	if err != nil {
		t.Fatalf("attaching listing: %v", err)
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
	e.chunks2 = e.chunkAndWait(t, a.ID)
	return role.ID, a.ID
}

// citing builds a citation to whichever chunk holds a phrase.
//
// The comparison collapses whitespace because the fixture is wrapped prose: a
// phrase can straddle a line break in the source and still be one phrase, which
// is the same allowance the validator makes.
func (e *roleEnv) citing(t *testing.T, quote string) []profile.Citation {
	t.Helper()
	flat := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	for _, c := range e.chunks2 {
		if strings.Contains(flat(c.Text), flat(quote)) {
			return []profile.Citation{{ChunkID: c.ID, Quote: quote}}
		}
	}
	t.Fatalf("no chunk contains %q", quote)
	return nil
}

func TestARoleProfileIsReadyWithoutAnyApproval(t *testing.T) {
	e := newRoleEnv(t)
	roleID, _ := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "strong Go and production SQLite experience",
			Priority: profile.MustHave, Citations: e.citing(t, "Go and production SQLite")},
	})}

	if _, err := e.roles.Profile(roleID); err != nil {
		t.Fatalf("profiling: %v", err)
	}
	status, err := e.roles.Status(roleID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != RoleProfileReady {
		t.Fatalf("a freshly profiled role is %q (%s), want ready", status.State, status.Reason)
	}
	// Nothing approved it, and nothing needed to.
	eligible, err := e.roles.Eligibility(roleID)
	if err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	if !eligible.Eligible {
		t.Fatalf("a Ready role is not assessable: %q", eligible.Reason)
	}
	var approvals int64
	err = e.db.Model(&models.Profile{}).Where("subject_kind = ? AND approved_at IS NOT NULL",
		profile.SubjectRole).Count(&approvals).Error
	if err != nil {
		t.Fatalf("counting approvals: %v", err)
	}
	if approvals != 0 {
		t.Errorf("%d role profiles carry an approval — roles are not approved", approvals)
	}
}

// The listing states one requirement as essential, one as desirable, and
// mentions a third with no indication. Only the first two may carry a priority.
func TestPriorityIsAssignedOnlyWhereTheSourceSupportsIt(t *testing.T) {
	e := newRoleEnv(t)
	roleID, _ := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "strong Go and production SQLite experience",
			Priority: profile.MustHave, Citations: e.citing(t, "must have strong Go")},
		{Type: profile.Experience, Wording: "operating multi-region systems",
			Priority: profile.NiceToHave, Citations: e.citing(t, "would be desirable")},
		// Terraform is mentioned with no indication either way.
		{Type: profile.Skill, Wording: "Terraform",
			Citations: e.citing(t, "We also use Terraform")},
	})}

	p, err := e.roles.Profile(roleID)
	if err != nil {
		t.Fatalf("profiling: %v", err)
	}
	got := map[string]string{}
	for _, a := range p.Aspects {
		got[a.Wording] = a.Priority
	}
	if got["strong Go and production SQLite experience"] != string(profile.MustHave) {
		t.Errorf("the essential requirement is %q", got["strong Go and production SQLite experience"])
	}
	if got["operating multi-region systems"] != string(profile.NiceToHave) {
		t.Errorf("the desirable requirement is %q", got["operating multi-region systems"])
	}
	// The one the listing does not weight stays unspecified, terminally.
	if got["Terraform"] != string(profile.Unspecified) {
		t.Errorf("an unweighted mention became %q, want unspecified", got["Terraform"])
	}
}

// Normalized constraints sit beside the wording rather than replacing it.
func TestNormalizedConstraintsPreserveTheSourceWording(t *testing.T) {
	e := newRoleEnv(t)
	roleID, _ := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.WorkArrangement, Wording: "Hybrid, three days onsite",
			Structured: map[string]any{"arrangement": "hybrid", "days_onsite": 3},
			Citations:  e.citing(t, "Hybrid, three days")},
		{Type: profile.Compensation, Wording: "AUD 180,000 base",
			Structured: map[string]any{"currency": "AUD", "minimum": 180000, "period": "year", "basis": "base"},
			Citations:  e.citing(t, "AUD 180,000 base")},
		{Type: profile.EmploymentType, Wording: "Permanent",
			Structured: map[string]any{"employment_type": "permanent"},
			Citations:  e.citing(t, "Permanent")},
	})}

	p, err := e.roles.Profile(roleID)
	if err != nil {
		t.Fatalf("profiling: %v", err)
	}
	for _, a := range p.Aspects {
		if strings.TrimSpace(a.Wording) == "" {
			t.Errorf("a normalized aspect lost its source wording: %+v", a)
		}
		if a.Structured == "{}" {
			t.Errorf("aspect %q kept no structured value", a.Wording)
		}
	}
	// And the wording is the listing's, not a rendering of the structured value.
	var arrangement string
	for _, a := range p.Aspects {
		if a.Type == string(profile.WorkArrangement) {
			arrangement = a.Wording
		}
	}
	if !strings.Contains(arrangement, "three days onsite") {
		t.Errorf("the work arrangement wording is %q", arrangement)
	}
}

func TestEveryAspectTypeCanComeFromARoleListing(t *testing.T) {
	e := newRoleEnv(t)
	roleID, _ := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")

	quotes := map[profile.AspectType]string{
		profile.Skill:           "Go and production SQLite",
		profile.Responsibility:  "lead a team of four",
		profile.Experience:      "operating multi-region systems",
		profile.Qualification:   "postgraduate qualification",
		profile.Seniority:       "senior platform engineer",
		profile.Location:        "Melbourne",
		profile.WorkArrangement: "Hybrid",
		profile.WorkRights:      "Australian work rights",
		profile.EmploymentType:  "Permanent",
		profile.Compensation:    "AUD 180,000 base",
		profile.Other:           "report to",
	}
	if len(quotes) != len(profile.AspectTypes) {
		t.Fatalf("the fixture covers %d types, the taxonomy has %d", len(quotes), len(profile.AspectTypes))
	}
	aspects := make([]profile.Aspect, 0, len(quotes))
	for _, typ := range profile.AspectTypes {
		quote := quotes[typ]
		aspects = append(aspects, profile.Aspect{
			Type: typ, Wording: quote, Citations: e.citing(t, quote),
		})
	}
	e.model.responses = []string{jsonProposal(t, aspects)}

	p, err := e.roles.Profile(roleID)
	if err != nil {
		t.Fatalf("profiling a listing covering every type: %v", err)
	}
	if len(p.Aspects) != len(profile.AspectTypes) {
		t.Fatalf("stored %d aspects of %d types", len(p.Aspects), len(profile.AspectTypes))
	}
}

func TestAFailedRoleProfileStaysVisibleAndIsNeverAssessed(t *testing.T) {
	e := newRoleEnv(t)
	roleID, _ := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	bad := `{"aspects":[{"type":"culture_fit","wording":"vibes","citations":[]}]}`
	e.model.responses = []string{bad, bad}

	if _, err := e.roles.Profile(roleID); err == nil {
		t.Fatal("an invalid role decomposition was accepted")
	}
	status, err := e.roles.Status(roleID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != RoleProfileFailed {
		t.Fatalf("a failed decomposition reports %q", status.State)
	}
	if !strings.Contains(status.Reason, "retry") || !strings.Contains(status.Reason, "hand") {
		t.Errorf("the failure does not offer retry and manual entry: %q", status.Reason)
	}
	eligible, err := e.roles.Eligibility(roleID)
	if err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	if eligible.Eligible {
		t.Fatal("a failed role was assessable")
	}

	// And it is in the listing rather than absent from it — an absence is
	// indistinguishable from a role that was never discovered.
	list, err := e.roles.List()
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	found := false
	for _, s := range list {
		if s.RoleID == roleID {
			found = true
			if s.State != RoleProfileFailed {
				t.Errorf("the listing shows the failed role as %q", s.State)
			}
		}
	}
	if !found {
		t.Fatal("a failed role vanished from the listing")
	}
}

func TestARoleWithNoProfileIsItsOwnStateNotAnAbsence(t *testing.T) {
	e := newRoleEnv(t)
	role, err := e.records.CreateRole(models.Role{
		Title:          "Never profiled",
		Origin:         models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating role: %v", err)
	}
	list, err := e.roles.List()
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	var found *RoleStatus
	for i := range list {
		if list[i].RoleID == role.ID {
			found = &list[i]
		}
	}
	if found == nil {
		t.Fatal("a role with no profile is missing from the listing")
	}
	if found.State != RoleProfileMissing || found.Reason == "" {
		t.Fatalf("an unprofiled role reports %q (%s)", found.State, found.Reason)
	}
	eligible, err := e.roles.Eligibility(role.ID)
	if err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	if eligible.Eligible {
		t.Fatal("a role with no profile was assessable")
	}
}

func TestAChangedListingMakesTheProfileStaleAndIneligible(t *testing.T) {
	e := newRoleEnv(t)
	roleID, artifactID := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	valid := jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "strong Go and production SQLite experience",
			Priority: profile.MustHave, Citations: e.citing(t, "Go and production SQLite")},
	})
	e.model.responses = []string{valid, valid}

	if _, err := e.roles.Profile(roleID); err != nil {
		t.Fatalf("profiling: %v", err)
	}

	// The listing is republished with different content.
	changed := listingMarkdown + "\nUpdate: the role is now fully remote.\n"
	err := e.db.Model(&models.Artifact{}).Where("id = ?", artifactID).
		Update("markdown", changed).Error
	if err != nil {
		t.Fatalf("replacing the listing: %v", err)
	}
	e.chunks2 = e.chunkAndWait(t, artifactID)

	status, err := e.roles.Status(roleID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != RoleProfileStale {
		t.Fatalf("a changed listing reports %q", status.State)
	}
	eligible, err := e.roles.Eligibility(roleID)
	if err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	// Unlike a stale candidate, a stale role is not assessed at all: nobody has
	// independent knowledge of a listing that changed.
	if eligible.Eligible {
		t.Fatal("a stale role was assessed")
	}
	if !strings.Contains(eligible.Reason, "changed") {
		t.Errorf("the reason does not say the listing changed: %q", eligible.Reason)
	}

	// Profiling again from the current content restores Ready.
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "strong Go and production SQLite experience",
			Priority: profile.MustHave, Citations: e.citing(t, "Go and production SQLite")},
	})}
	if _, err := e.roles.Profile(roleID); err != nil {
		t.Fatalf("reprofiling: %v", err)
	}
	status, err = e.roles.Status(roleID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != RoleProfileReady {
		t.Fatalf("after reprofiling the role is %q (%s)", status.State, status.Reason)
	}
}

func TestARecruiterEditVersionsTheProfileAndLeavesTheListingAlone(t *testing.T) {
	e := newRoleEnv(t)
	roleID, artifactID := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Experience, Wording: "five years operating multi-region systems",
			Priority: profile.MustHave, Citations: e.citing(t, "operating multi-region systems")},
	})}
	original, err := e.roles.Profile(roleID)
	if err != nil {
		t.Fatalf("profiling: %v", err)
	}

	var before models.Artifact
	if err := e.db.First(&before, artifactID).Error; err != nil {
		t.Fatalf("reading the listing: %v", err)
	}

	// The recruiter knows the five years is negotiable.
	edited, err := e.roles.EditAspect(roleID, 0,
		"multi-region experience — five years stated, negotiable", profile.NiceToHave)
	if err != nil {
		t.Fatalf("editing: %v", err)
	}
	if edited.ID == original.ID {
		t.Fatal("an edit mutated the version instead of making one")
	}
	if edited.Aspects[0].Priority != string(profile.NiceToHave) {
		t.Errorf("the edited priority is %q", edited.Aspects[0].Priority)
	}
	if edited.Aspects[0].Origin != string(profile.RecruiterSupplied) {
		t.Errorf("an edited aspect has origin %q", edited.Aspects[0].Origin)
	}

	// The listing is untouched: artifacts are immutable, and the profile is
	// allowed to disagree with its source.
	var after models.Artifact
	if err := e.db.First(&after, artifactID).Error; err != nil {
		t.Fatalf("re-reading the listing: %v", err)
	}
	if after.Markdown != before.Markdown || after.SHA256 != before.SHA256 {
		t.Fatal("editing a role profile changed the source artifact")
	}

	// And the earlier version's citation still resolves to what the listing said.
	cites, err := e.roles.Citations(original.ID)
	if err != nil {
		t.Fatalf("resolving the original citations: %v", err)
	}
	if len(cites) == 0 || !strings.Contains(cites[0].Text, "multi-region") {
		t.Fatalf("the original citation no longer resolves: %+v", cites)
	}

	// An edit does not make the profile stale — the evidence did not move.
	status, err := e.roles.Status(roleID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != RoleProfileReady {
		t.Fatalf("after an edit the role is %q (%s)", status.State, status.Reason)
	}
}

func TestAnUndecomposableListingCanBeCompletedByHand(t *testing.T) {
	e := newRoleEnv(t)
	roleID, _ := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	bad := `{"aspects":[{"type":"nonsense","wording":"x","citations":[]}]}`
	e.model.responses = []string{bad, bad}
	if _, err := e.roles.Profile(roleID); err == nil {
		t.Fatal("an invalid decomposition was accepted")
	}

	// Typed by hand, with priorities — which role aspects may carry and
	// candidate aspects may not.
	for _, a := range []profile.Aspect{
		{Type: profile.Skill, Wording: "Go", Priority: profile.MustHave},
		{Type: profile.Location, Wording: "Melbourne", Priority: profile.Unspecified},
	} {
		if _, err := e.roles.AddAspect(roleID, a); err != nil {
			t.Fatalf("adding %q by hand: %v", a.Wording, err)
		}
	}
	status, err := e.roles.Status(roleID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != RoleProfileReady {
		t.Fatalf("a hand-completed role is %q (%s)", status.State, status.Reason)
	}
	if len(status.Aspects) != 2 {
		t.Fatalf("the hand-built profile has %d aspects", len(status.Aspects))
	}
	for _, a := range status.Aspects {
		if a.Origin != string(profile.RecruiterSupplied) {
			t.Errorf("a hand-typed requirement has origin %q", a.Origin)
		}
	}
	eligible, err := e.roles.Eligibility(roleID)
	if err != nil {
		t.Fatalf("eligibility: %v", err)
	}
	if !eligible.Eligible {
		t.Fatalf("a hand-completed role is not assessable: %q", eligible.Reason)
	}
}

func TestRemovingARequirementVersionsTheProfile(t *testing.T) {
	e := newRoleEnv(t)
	roleID, _ := e.withListing(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go", Citations: e.citing(t, "Go and production SQLite")},
		{Type: profile.Skill, Wording: "Terraform", Citations: e.citing(t, "We also use Terraform")},
	})}
	original, err := e.roles.Profile(roleID)
	if err != nil {
		t.Fatalf("profiling: %v", err)
	}

	after, err := e.roles.RemoveAspect(roleID, 1)
	if err != nil {
		t.Fatalf("removing: %v", err)
	}
	if len(after.Aspects) != 1 {
		t.Fatalf("after removing one of two, %d remain", len(after.Aspects))
	}
	still, err := e.classify.Aspects(original.ID)
	if err != nil {
		t.Fatalf("reading the original: %v", err)
	}
	if len(still) != 2 {
		t.Error("removal mutated the previous version")
	}
}
