package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// shortlistEnv is a discoveryEnv with role profiling, embedding, and the
// shortlist wired in — every stage the shortlist composes.
type shortlistEnv struct {
	*discoveryEnv
	roles     *RoleProfileService
	embed     *EmbedService
	endpoint  *fakeEmbedder
	shortlist *ShortlistService
}

func newShortlistEnv(t *testing.T) *shortlistEnv {
	t.Helper()
	base := newDiscoveryEnv(t)
	roles := NewRoleProfileService(base.db, base.classify)
	// Small deterministic vectors: the semantic half has to run, and what it
	// returns matters less than that fusion combines two real lists.
	endpoint := newFakeEmbedder(8)
	embed := NewEmbedService(base.db, base.jobs, base.registry, endpoint)
	return &shortlistEnv{
		discoveryEnv: base,
		roles:        roles,
		embed:        embed,
		endpoint:     endpoint,
		shortlist: NewShortlistService(base.db, base.search, embed,
			base.criteria, base.profiles, roles),
	}
}

// roleWithListing creates a role, attaches an extracted listing linked to both
// the role and the initiative, chunks it, and profiles it Ready.
func (e *shortlistEnv) roleWithListing(t *testing.T, title, body string, aspects ...profile.Aspect) uint {
	t.Helper()
	role, err := e.records.CreateRole(models.Role{
		Title:          title,
		Origin:         models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating role: %v", err)
	}
	markdown := "# " + title + "\n\n## Requirements\n\n" + body + "\n"
	a, err := e.artifacts.create(title, title+".md", "test", []byte(markdown),
		models.LinkRole, role.ID)
	if err != nil {
		t.Fatalf("attaching listing: %v", err)
	}
	// Also in the workspace, which is what puts it in scope.
	if err := e.artifacts.Link(a.ID, models.LinkInitiative, e.initiative); err != nil {
		t.Fatalf("linking to the initiative: %v", err)
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
	if len(chunks) == 0 {
		t.Fatalf("%s produced no chunks", title)
	}

	// A Ready profile, so the role is eligible. Hand-built when no aspects were
	// asked for, so no model is needed.
	e.assignClassify(t, "synthetic-classify")
	if len(aspects) == 0 {
		aspects = []profile.Aspect{{Type: profile.Other, Wording: title}}
	}
	for _, aspect := range aspects {
		version, err := e.roles.AddAspect(role.ID, aspect)
		if err != nil {
			t.Fatalf("adding a role aspect: %v", err)
		}
		if version == nil {
			t.Fatalf("adding a role aspect returned no version")
		}
	}
	status, err := e.roles.Status(role.ID)
	if err != nil {
		t.Fatalf("role status: %v", err)
	}
	if status.State != RoleProfileReady {
		t.Fatalf("%s is %q (%s), want ready", title, status.State, status.Reason)
	}
	return role.ID
}

func (e *shortlistEnv) build(t *testing.T, candidateID uint) *Shortlist {
	t.Helper()
	out, err := e.shortlist.Build(e.initiative, candidateID)
	if err != nil {
		t.Fatalf("building the shortlist: %v", err)
	}
	return out
}

func TestTheShortlistFindsRolesByCriteriaAndSaysWhy(t *testing.T) {
	e := newShortlistEnv(t)
	wanted := e.roleWithListing(t, "Platform engineer",
		"We need someone with strong quokkastack experience in production.")
	e.roleWithListing(t, "Financial analyst",
		"Quarterly reporting and reconciliation for a mid-market lender.")

	e.add(t, "strong quokkastack experience", models.CriterionMustHave)

	out := e.build(t, 0)
	if len(out.Entries) == 0 {
		t.Fatal("nothing was shortlisted")
	}
	if out.Entries[0].RoleID != wanted {
		t.Fatalf("the matching role ranked %d, the top entry is %d",
			wanted, out.Entries[0].RoleID)
	}
	// And it explains itself without re-running anything.
	why := out.Entries[0].Why
	if len(why) == 0 {
		t.Fatal("the top entry does not say why it is there")
	}
	if !strings.Contains(why[0].Source, "quokkastack") {
		t.Errorf("the provenance does not name the criterion: %+v", why)
	}
	if why[0].Method != "lexical" && why[0].Method != "semantic" {
		t.Errorf("the provenance does not name the method: %+v", why)
	}
	if why[0].Rank < 1 {
		t.Errorf("the provenance records rank %d", why[0].Rank)
	}
}

// Five matching chunks of one role are one shortlist position, not five.
func TestManyMatchingChunksOccupyOneSlot(t *testing.T) {
	e := newShortlistEnv(t)
	// A listing long enough to chunk several times, saying the same thing in
	// each section.
	body := ""
	for i := range 6 {
		body += fmt.Sprintf("\n\n## Section %d\n\nquokkastack engineering at scale, again.\n", i)
	}
	roleID := e.roleWithListing(t, "Platform engineer", body)
	e.add(t, "quokkastack engineering", models.CriterionMustHave)

	out := e.build(t, 0)
	seen := 0
	for _, entry := range out.Entries {
		if entry.RoleID == roleID {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("one role occupies %d shortlist positions", seen)
	}
}

func TestRolesOutsideTheInitiativeAreNeverShortlisted(t *testing.T) {
	e := newShortlistEnv(t)
	mine := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")

	// A role in another workspace, with the same words.
	inits := NewInitiativeService(e.db)
	other, err := inits.Create("Other "+t.Name(), models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating the other initiative: %v", err)
	}
	theirs, err := e.records.CreateRole(models.Role{
		Title: "Their platform engineer", Origin: models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating their role: %v", err)
	}
	md := "# Their platform engineer\n\n## Requirements\n\nquokkastack engineering at scale.\n"
	a, err := e.artifacts.create("theirs", "theirs.md", "test", []byte(md), models.LinkRole, theirs.ID)
	if err != nil {
		t.Fatalf("attaching: %v", err)
	}
	if err := e.artifacts.Link(a.ID, models.LinkInitiative, other.ID); err != nil {
		t.Fatalf("linking: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state": models.ExtractionExtracted, "extractor": "native-text",
		"extractor_version": "1", "markdown": md,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	e.chunkAndWait(t, a.ID)

	e.add(t, "quokkastack engineering", models.CriterionMustHave)
	out := e.build(t, 0)
	for _, entry := range out.Entries {
		if entry.RoleID == theirs.ID {
			t.Fatal("another initiative's role reached the shortlist")
		}
	}
	if len(out.Entries) == 0 || out.Entries[0].RoleID != mine {
		t.Fatalf("the workspace's own role is not on the list: %+v", out.Entries)
	}
}

func TestAStaleRoleIsExcludedBeforeRetrieval(t *testing.T) {
	e := newShortlistEnv(t)
	stale := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")
	fresh := e.roleWithListing(t, "Staff engineer", "quokkastack engineering at scale.")
	e.add(t, "quokkastack engineering", models.CriterionMustHave)

	before := e.build(t, 0)
	if len(before.Entries) != 2 || before.Eligible != 2 {
		t.Fatalf("setup shortlisted %d of %d eligible", len(before.Entries), before.Eligible)
	}

	// The listing changes underneath the profile.
	var artifactID uint
	err := e.db.Model(&models.ArtifactLink{}).Select("artifact_id").
		Where("target_type = ? AND target_id = ?", models.LinkRole, stale).
		Limit(1).Scan(&artifactID).Error
	if err != nil {
		t.Fatalf("finding the artifact: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", artifactID).
		Update("markdown", "# Platform engineer\n\n## Requirements\n\nNow something else entirely.\n").Error
	if err != nil {
		t.Fatalf("changing the listing: %v", err)
	}
	e.chunkAndWait(t, artifactID)

	after := e.build(t, 0)
	if after.Eligible != 1 {
		t.Fatalf("%d roles are eligible after one went stale", after.Eligible)
	}
	for _, entry := range after.Entries {
		if entry.RoleID == stale {
			t.Fatal("a stale role reached the shortlist")
		}
	}
	if len(after.Entries) != 1 || after.Entries[0].RoleID != fresh {
		t.Fatalf("the fresh role is not the shortlist: %+v", after.Entries)
	}
}

// "No results" and "results you would have rejected" look identical on screen,
// and only one of them is true.
func TestAConflictingRoleStaysOnTheListWithItsConflict(t *testing.T) {
	e := newShortlistEnv(t)
	roleID := e.roleWithListing(t, "Platform engineer",
		"quokkastack engineering at scale, onsite in Sydney.",
		profile.Aspect{
			Type: profile.WorkArrangement, Wording: "Onsite in Sydney",
			Structured: map[string]any{"arrangement": "onsite"},
			Priority:   profile.MustHave,
		},
	)
	e.add(t, "quokkastack engineering", models.CriterionMustHave)

	// The candidate wants remote work, which the role is not.
	id := e.candidateWantingRemote(t)

	out := e.build(t, id)
	if len(out.Entries) == 0 {
		t.Fatal("a conflicting role was filtered out — it should be shown and rejected")
	}
	var entry *Entry
	for i := range out.Entries {
		if out.Entries[i].RoleID == roleID {
			entry = &out.Entries[i]
		}
	}
	if entry == nil {
		t.Fatal("the conflicting role is not on the shortlist")
	}
	if len(entry.Conflicts) == 0 {
		t.Fatal("the conflict is not recorded against the entry")
	}
	found := false
	for _, c := range entry.Conflicts {
		if c.Field == "arrangement" && c.Wanted == "remote" && c.Found == "onsite" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the arrangement conflict is not described: %+v", entry.Conflicts)
	}
}

// candidateWantingRemote makes an approved candidate whose structured
// arrangement is remote.
func (e *shortlistEnv) candidateWantingRemote(t *testing.T) uint {
	t.Helper()
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Tobias Fenn"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	p, err := e.profiles.AddAspect(c.ID, profile.Aspect{
		Type: profile.WorkArrangement, Wording: "Wants fully remote work",
		Structured: map[string]any{"arrangement": "remote"},
	})
	if err != nil {
		t.Fatalf("adding the arrangement: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}
	return c.ID
}

func TestSilenceIsNotAConflict(t *testing.T) {
	e := newShortlistEnv(t)
	// The role says nothing about arrangement.
	roleID := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")
	e.add(t, "quokkastack engineering", models.CriterionMustHave)
	id := e.candidateWantingRemote(t)

	out := e.build(t, id)
	for _, entry := range out.Entries {
		if entry.RoleID == roleID && len(entry.Conflicts) > 0 {
			t.Fatalf("a listing that says nothing produced a conflict: %+v", entry.Conflicts)
		}
	}
}

func TestMoreThanTwentyReturnsExactlyTwentyAndFewerReturnsAll(t *testing.T) {
	e := newShortlistEnv(t)
	for i := range 25 {
		e.roleWithListing(t, fmt.Sprintf("Platform engineer %02d", i),
			"quokkastack engineering at scale.")
	}
	e.add(t, "quokkastack engineering", models.CriterionMustHave)

	out := e.build(t, 0)
	if out.Eligible != 25 {
		t.Fatalf("%d roles are eligible", out.Eligible)
	}
	if len(out.Entries) != ShortlistSize {
		t.Fatalf("got %d entries from 25 eligible roles, want %d", len(out.Entries), ShortlistSize)
	}
	// Positions are one-based and consecutive.
	for i, entry := range out.Entries {
		if entry.Position != i+1 {
			t.Fatalf("entry %d has position %d", i, entry.Position)
		}
	}

	fewer := newShortlistEnv(t)
	for i := range 7 {
		fewer.roleWithListing(t, fmt.Sprintf("Data engineer %02d", i),
			"quokkastack engineering at scale.")
	}
	fewer.add(t, "quokkastack engineering", models.CriterionMustHave)
	small := fewer.build(t, 0)
	if len(small.Entries) != 7 {
		t.Fatalf("got %d entries from 7 eligible roles", len(small.Entries))
	}
}

func TestRepeatedRunsReturnIdenticalOrdering(t *testing.T) {
	e := newShortlistEnv(t)
	for i := range 12 {
		e.roleWithListing(t, fmt.Sprintf("Platform engineer %02d", i),
			"quokkastack engineering at scale, identical wording.")
	}
	e.add(t, "quokkastack engineering", models.CriterionMustHave)

	first := e.build(t, 0)
	if len(first.Entries) == 0 {
		t.Fatal("nothing was shortlisted")
	}
	for run := range 5 {
		again := e.build(t, 0)
		if len(again.Entries) != len(first.Entries) {
			t.Fatalf("run %d returned %d entries, the first returned %d",
				run, len(again.Entries), len(first.Entries))
		}
		for i := range again.Entries {
			if again.Entries[i].RoleID != first.Entries[i].RoleID ||
				again.Entries[i].Score != first.Entries[i].Score {
				t.Fatalf("run %d differs at position %d: %+v vs %+v",
					run, i, again.Entries[i], first.Entries[i])
			}
		}
	}
	// Identical listings tie, so the order is purely by identifier — which is
	// what makes the run-to-run stability a rule rather than luck.
	for i := 1; i < len(first.Entries); i++ {
		if first.Entries[i].Score == first.Entries[i-1].Score &&
			first.Entries[i].RoleID < first.Entries[i-1].RoleID {
			t.Fatalf("tied entries are out of identifier order: %d before %d",
				first.Entries[i-1].RoleID, first.Entries[i].RoleID)
		}
	}
}

func TestAShortlistRecordsTheIntentItWasComputedUnder(t *testing.T) {
	e := newShortlistEnv(t)
	e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")
	e.add(t, "quokkastack engineering", models.CriterionMustHave)

	first := e.build(t, 0)
	if first.CriteriaVersion == 0 {
		t.Fatal("the shortlist records no criteria version")
	}

	// Changing the criteria changes the intent, and the recorded version says so.
	e.add(t, "has led a platform team", models.CriterionNiceToHave)
	after := e.build(t, 0)
	if after.CriteriaVersion == first.CriteriaVersion {
		t.Fatal("changing the criteria did not change the recorded version")
	}
}

func TestNothingMatchingIsAnEmptyShortlistNotAnError(t *testing.T) {
	e := newShortlistEnv(t)
	e.roleWithListing(t, "Financial analyst", "Quarterly reporting and reconciliation.")
	e.add(t, "quokkastack engineering", models.CriterionMustHave)

	out := e.build(t, 0)
	if out.Eligible != 1 {
		t.Fatalf("%d roles are eligible", out.Eligible)
	}
	if len(out.Entries) != 0 {
		t.Fatalf("a non-matching corpus shortlisted %+v", out.Entries)
	}
}

func TestNoCriteriaAndNoCandidateIsAnEmptyShortlist(t *testing.T) {
	e := newShortlistEnv(t)
	e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")

	out := e.build(t, 0)
	if len(out.Entries) != 0 {
		t.Fatalf("with nothing to search for, %d entries were returned", len(out.Entries))
	}
}

// Representative timings, printed for the gate evidence beside Phase 9's.
func TestShortlistTimingAtRepresentativeSize(t *testing.T) {
	if testing.Short() {
		t.Skip("timing measurement")
	}
	e := newShortlistEnv(t)
	const roles = 40
	for i := range roles {
		e.roleWithListing(t, fmt.Sprintf("Engineer %02d", i),
			"quokkastack engineering at scale, with reporting and reconciliation.")
	}
	for _, text := range []string{
		"quokkastack engineering", "has led a platform team", "reporting and reconciliation",
	} {
		e.add(t, text, models.CriterionNiceToHave)
	}

	// Warm, then measured: the first call pays for whatever SQLite caches.
	e.build(t, 0)
	start := time.Now()
	const runs = 5
	for range runs {
		e.build(t, 0)
	}
	per := time.Since(start) / runs
	fmt.Printf("EVIDENCE shortlist roles=%d criteria=3 per_build=%s\n", roles, per.Round(time.Millisecond))
	if per > 5*time.Second {
		t.Errorf("building a shortlist over %d roles took %s, which is too slow to precede assessment",
			roles, per)
	}
}

// The matching benchmark's shape: a candidate profile, a corpus of roles, and
// no search criteria at all. The recruiter has not stated any intent yet — the
// approved profile is the intent — and a shortlist that needs criteria to
// return anything would make that benchmark unrunnable.
func TestAnApprovedProfileAloneDrivesTheShortlist(t *testing.T) {
	e := newShortlistEnv(t)
	wanted := e.roleWithListing(t, "Platform engineer",
		"Must have Go and SQLite in production. Melbourne, hybrid.")
	e.roleWithListing(t, "Pastry chef",
		"Must have patisserie experience and early starts. Hobart, onsite.")

	candidateID := e.approvedCandidate(t)
	// Deliberately no criteria.
	if criteria, err := e.criteria.List(e.initiative); err != nil {
		t.Fatalf("listing criteria: %v", err)
	} else if len(criteria) != 0 {
		t.Fatalf("%d criteria exist, want none", len(criteria))
	}

	out := e.build(t, candidateID)
	if len(out.Entries) == 0 {
		t.Fatal("an approved profile returned no roles at all")
	}
	if out.Entries[0].RoleID != wanted {
		t.Fatalf("the top role is %d, want the platform role %d", out.Entries[0].RoleID, wanted)
	}
}

// A profile aspect is a sentence lifted out of a document, not a phrase the
// recruiter typed. ANDing its words demands a listing containing every one of
// them, which is why five live benchmark scenarios ranked nothing at all while
// carrying nine searchable aspects each.
func TestAnAspectSentenceStillFindsItsRole(t *testing.T) {
	e := newShortlistEnv(t)
	wanted := e.roleWithListing(t, "Platform engineer",
		"Must have Go and production SQLite experience. Melbourne, hybrid.")
	e.roleWithListing(t, "Pastry chef", "Must have patisserie experience. Hobart, onsite.")

	candidateID := e.candidateWithAspect(t,
		"Ran the platform team's shared services in Go, including an embedded SQLite cache")

	out := e.build(t, candidateID)
	if len(out.Entries) == 0 {
		t.Fatal("a sentence-shaped aspect ranked nothing at all")
	}
	if out.Entries[0].RoleID != wanted {
		t.Fatalf("top role is %d, want %d", out.Entries[0].RoleID, wanted)
	}
}

// candidateWithAspect approves a candidate profile holding one aspect with the
// given wording, cited to the resume it came from.
func (e *shortlistEnv) candidateWithAspect(t *testing.T, wording string, extra ...profile.Aspect) uint {
	t.Helper()
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Nadia Frost"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	md := "# Nadia Frost\n\n## Experience\n\n" + wording + ".\n\n## Location\n\nPerth, onsite.\n"
	a, err := e.artifacts.create("resume", "resume.md", "test", []byte(md),
		models.LinkCandidate, c.ID)
	if err != nil {
		t.Fatalf("attaching resume: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state":  models.ExtractionExtracted,
		"extractor":         "native-text",
		"extractor_version": "1",
		"markdown":          md,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	e.chunks2 = e.chunkAndWait(t, a.ID)

	e.assignClassify(t, "synthetic-classify")
	// The chunker splits at headings, so the citing chunk is the one that
	// actually holds the wording rather than whichever came first.
	var chunkID uint
	for _, ch := range e.chunks2 {
		if strings.Contains(ch.Text, wording) {
			chunkID = ch.ID
		}
	}
	if chunkID == 0 {
		t.Fatal("no chunk holds the aspect wording")
	}
	cite := []profile.Citation{{ChunkID: chunkID, Quote: wording}}
	aspects := []profile.Aspect{{Type: profile.Experience, Wording: wording, Citations: cite}}
	for _, a := range extra {
		if len(a.Citations) == 0 {
			// Cited to the line the resume actually carries, so the aspect is
			// as answerable as any other.
			for _, ch := range e.chunks2 {
				if strings.Contains(ch.Text, "Perth, onsite") {
					a.Citations = []profile.Citation{{ChunkID: ch.ID, Quote: "Perth, onsite"}}
				}
			}
		}
		aspects = append(aspects, a)
	}
	e.model.responses = []string{jsonProposal(t, aspects)}
	p, err := e.profiles.Classify(c.ID)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}
	return c.ID
}

// Similarity retrieval runs over Profile Aspects, which is what the PRD asks
// for: "structured scope filters, FTS, and exact-cosine aspect KNN produce a
// 20-role assessment shortlist". It had been running over source chunks, so a
// query met the blurb every listing shares rather than the statement it should
// be compared against.
//
// The wording here shares no word with the role's listing, so the lexical half
// cannot find it and only the aspect vectors can.
func TestSimilarityRetrievesOverAspects(t *testing.T) {
	e := newShortlistEnv(t)
	wanted := e.roleWithListing(t, "Platform engineer",
		"Must have Go and production SQLite experience.",
		profile.Aspect{Type: profile.Skill, Wording: "distributed storage engineering",
			Citations: []profile.Citation{{Record: "recruiter"}}})
	e.roleWithListing(t, "Pastry chef", "Must have patisserie experience.",
		profile.Aspect{Type: profile.Skill, Wording: "laminated dough",
			Citations: []profile.Citation{{Record: "recruiter"}}})

	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"}); err != nil {
		t.Fatalf("assigning the embed role: %v", err)
	}
	// The embedder is deterministic per text, so the aspect closest to the
	// query is the one that shares its wording.
	job, err := e.embed.EmbedAspects(e.initiative)
	if err != nil {
		t.Fatalf("embedding aspects: %v", err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("aspect embedding is %s (%q)", done.State, done.FailureReason)
	}

	hits, err := e.embed.SearchAspects(e.initiative, "distributed storage engineering", 10)
	if err != nil {
		t.Fatalf("searching aspects: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no aspect was retrieved")
	}
	if hits[0].RoleID != wanted {
		t.Fatalf("the closest aspect belongs to role %d, want %d", hits[0].RoleID, wanted)
	}
	if hits[0].Wording != "distributed storage engineering" {
		t.Fatalf("retrieved %q", hits[0].Wording)
	}
}

// Aspects of a role outside the initiative are never retrieved, the same way
// its chunks are not.
func TestAspectRetrievalRespectsScope(t *testing.T) {
	e := newShortlistEnv(t)
	e.roleWithListing(t, "Platform engineer", "Must have Go.",
		profile.Aspect{Type: profile.Skill, Wording: "distributed storage engineering",
			Citations: []profile.Citation{{Record: "recruiter"}}})

	outside, err := e.records.CreateRole(models.Role{
		Title: "Outside role", Origin: models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating the outside role: %v", err)
	}
	version, err := e.roles.AddAspect(outside.ID, profile.Aspect{
		Type: profile.Skill, Wording: "distributed storage engineering",
		Citations: []profile.Citation{{Record: "recruiter"}},
	})
	if err != nil || version == nil {
		t.Fatalf("adding an aspect to the outside role: %v", err)
	}

	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"}); err != nil {
		t.Fatalf("assigning the embed role: %v", err)
	}
	job, err := e.embed.EmbedAspects(e.initiative)
	if err != nil {
		t.Fatalf("embedding: %v", err)
	}
	waitForJob(t, e.jobs, job.ID)

	hits, err := e.embed.SearchAspects(e.initiative, "distributed storage engineering", 10)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	for _, h := range hits {
		if h.RoleID == outside.ID {
			t.Fatal("an aspect from outside the initiative was retrieved")
		}
	}
}

// A role is as relevant as its best statement, not as prolific as its listing.
//
// Aspect KNN limited to a page of aspects hides a role whose best aspect fell
// outside the page because other listings wrote more of them. With one aspect
// per role the whole corpus fits under the limit and the bug is invisible; it
// appears the moment listings are decomposed properly. Measured on the frozen
// corpus, the obviously right role went from first to fifth.
//
// The vectors are fixed rather than hashed, so this asserts the ranking rule
// and not the embedder: one prolific listing owns the six closest aspects, and
// asking for three roles still has to return three roles.
func TestARoleIsRankedByItsBestAspectNotByHowManyItHas(t *testing.T) {
	e := newShortlistEnv(t)

	const query = "distributed storage engineering"
	near := func(t *testing.T, text string, closeness float32) {
		t.Helper()
		v := make([]float32, 8)
		v[0], v[1] = closeness, 1-closeness
		e.endpoint.set(text, v)
	}
	near(t, query, 1)

	// Six aspects on one role, every one of them closer than anything else.
	crowding := []profile.Aspect{}
	for i, wording := range []string{"storage replication", "storage sharding",
		"storage compaction", "storage tiering", "storage durability", "storage indexing"} {
		near(t, wording, 0.99-float32(i)/1000)
		crowding = append(crowding, profile.Aspect{Type: profile.Skill, Wording: wording,
			Citations: []profile.Citation{{Record: "recruiter"}}})
	}
	prolific := e.roleWithListing(t, "Prolific listing", "Writes everything down.", crowding...)

	others := map[uint]bool{}
	for i, wording := range []string{"storage systems", "storage platforms", "storage tooling"} {
		near(t, wording, 0.90)
		id := e.roleWithListing(t, fmt.Sprintf("Terse listing %d", i), "Says one thing.",
			profile.Aspect{Type: profile.Skill, Wording: wording,
				Citations: []profile.Citation{{Record: "recruiter"}}})
		others[id] = true
	}

	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"}); err != nil {
		t.Fatalf("assigning the embed role: %v", err)
	}
	job, err := e.embed.EmbedAspects(e.initiative)
	if err != nil {
		t.Fatalf("embedding aspects: %v", err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("aspect embedding is %s (%q)", done.State, done.FailureReason)
	}

	roles, err := e.embed.SearchRoles(e.initiative, query, 3)
	if err != nil {
		t.Fatalf("searching roles: %v", err)
	}
	if len(roles) != 3 {
		t.Fatalf("asked for three roles and got %d — one listing filled the page", len(roles))
	}
	if roles[0].RoleID != prolific {
		t.Fatalf("the closest role is %d, want %d", roles[0].RoleID, prolific)
	}
	seen, terse := map[uint]bool{}, 0
	for _, r := range roles {
		if seen[r.RoleID] {
			t.Fatalf("role %d appears twice", r.RoleID)
		}
		seen[r.RoleID] = true
		if others[r.RoleID] {
			terse++
		}
	}
	if terse != 2 {
		t.Fatalf("%d terse roles survived the truncation, want 2", terse)
	}
}

// A candidate's city is evidence, and the full-text half can use it safely.
//
// Places are kept out of the similarity half because "Melbourne" and "Sydney"
// are close in an embedding space and opposite in fact. That reasoning is about
// embeddings, and it had been applied to both halves, so the word never reached
// retrieval at all. Measured: a candidate living in Perth and working at Redgum
// Mining Tech never surfaced a role at Redgum Mining Tech in Perth.
func TestACandidatesCityReachesTheFullTextHalfOnly(t *testing.T) {
	e := newShortlistEnv(t)
	// The place as the classifier records it: a wording carrying the
	// arrangement with it, and a normalized value holding only the city.
	candidateID := e.candidateWithAspect(t, "Embedded C for conveyor control units",
		profile.Aspect{Type: profile.Location, Wording: "Perth, onsite",
			Structured: map[string]any{"city": "Perth"}})

	queries, err := e.shortlist.queries(e.initiative, candidateID)
	if err != nil {
		t.Fatalf("building queries: %v", err)
	}
	var place *query
	for i := range queries {
		if queries[i].text == "Perth" {
			place = &queries[i]
		}
	}
	if place == nil {
		t.Fatalf("the candidate's city never became a query: %+v", queries)
	}
	if !place.lexicalOnly {
		t.Fatal("the city reaches the similarity half, where Perth and Sydney are neighbours")
	}
	// By its normalized value, not the aspect's wording, which carries the
	// arrangement and the salary with it.
	for _, q := range queries {
		if strings.Contains(q.text, "Perth,") {
			t.Fatalf("the whole location wording became a query: %q", q.text)
		}
	}
}

// A shared city is evidence, not a trump card.
//
// Letting the place reach the full-text half risks the opposite error: a role
// down the road in an unrelated field outranking the right role in another
// city. It is one query among many for exactly that reason.
func TestASharedCityDoesNotOutrankTheWork(t *testing.T) {
	e := newShortlistEnv(t)
	candidateID := e.candidateWithAspect(t, "Embedded C for conveyor control units",
		profile.Aspect{Type: profile.Location, Wording: "Perth, onsite",
			Structured: map[string]any{"city": "Perth"}})

	// The right work, in the wrong city.
	right := e.roleWithListing(t, "Firmware engineer",
		"Must have embedded C for conveyor control units in Hobart.",
		profile.Aspect{Type: profile.Skill, Wording: "Embedded C for conveyor control units",
			Citations: []profile.Citation{{Record: "recruiter"}}})
	// The wrong work, in the candidate's own city, naming it repeatedly.
	e.roleWithListing(t, "Pastry chef",
		"Must have laminated dough. Perth kitchen, Perth hours, Perth team.",
		profile.Aspect{Type: profile.Skill, Wording: "laminated dough",
			Citations: []profile.Citation{{Record: "recruiter"}}})

	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"}); err != nil {
		t.Fatalf("assigning the embed role: %v", err)
	}
	job, err := e.embed.EmbedAspects(e.initiative)
	if err != nil {
		t.Fatalf("embedding aspects: %v", err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("aspect embedding is %s (%q)", done.State, done.FailureReason)
	}

	list := e.build(t, candidateID)
	if len(list.Entries) == 0 {
		t.Fatal("nothing ranked")
	}
	if list.Entries[0].RoleID != right {
		t.Fatalf("a shared city outranked the work: first is %q",
			list.Entries[0].Title)
	}
}
