package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// deletionEnv is a cloudEnv with deletion wired in — deletion touches
// everything, so its environment is everything.
type deletionEnv struct {
	*cloudEnv
	deletion *DeletionService
}

func newDeletionEnv(t *testing.T) *deletionEnv {
	t.Helper()
	base := newCloudEnv(t)
	return &deletionEnv{cloudEnv: base, deletion: NewDeletionService(base.db)}
}

// count is the shorthand every invariant assertion uses.
func (e *deletionEnv) count(t *testing.T, model any, where string, args ...any) int64 {
	t.Helper()
	var n int64
	q := e.db.Model(model)
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	return n
}

// candidateWithEvidence makes a candidate with an artifact, chunks, a profile,
// and an embedding — one of everything a deletion has to reach.
func (e *deletionEnv) candidateWithEvidence(t *testing.T) (uint, uint) {
	t.Helper()
	// Invented, like every name in this repository's tests.
	const name = "Kalinda Reyes"
	c, err := e.records.CreateCandidate(models.Candidate{FullName: name})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	md := "# " + name + "\n\n## Experience\n\nquokkastack engineering at scale.\n"
	a, err := e.artifacts.create(name+" resume", "resume.md", "test", []byte(md),
		models.LinkCandidate, c.ID)
	if err != nil {
		t.Fatalf("attaching resume: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state": models.ExtractionExtracted, "extractor": "native-text",
		"extractor_version": "1", "markdown": md,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	// Linked to the workspace as well as the person, which is what a dropped
	// résumé does — and what puts it in scope for indexing.
	if err := e.artifacts.Link(a.ID, models.LinkInitiative, e.initiative); err != nil {
		t.Fatalf("linking to the initiative: %v", err)
	}
	e.chunkAndWait(t, a.ID)

	if _, err := e.profiles.AddAspect(c.ID, profile.Aspect{
		Type: profile.Skill, Wording: "quokkastack engineering",
	}); err != nil {
		t.Fatalf("adding an aspect: %v", err)
	}
	// One embedding, so the cascade has a vector to miss.
	e.assignEmbed(t)
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}
	return c.ID, a.ID
}

func (e *deletionEnv) assignEmbed(t *testing.T) {
	t.Helper()
	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: "synthetic-embed"}); err != nil {
		t.Fatalf("assigning embed: %v", err)
	}
}

func (e *deletionEnv) embedAllAndWait(t *testing.T) models.Job {
	t.Helper()
	job, err := e.embed.EmbedAll(e.initiative)
	if err != nil {
		t.Fatalf("queuing embedding: %v", err)
	}
	return waitForJob(t, e.jobs, job.ID)
}

// This is the rule most likely to be "improved" by someone tidying up.
func TestDeletingAnInitiativeKeepsSharedRecords(t *testing.T) {
	e := newDeletionEnv(t)
	candidateID, artifactID := e.candidateWithEvidence(t)
	e.add(t, "five years of quokkastack", models.CriterionMustHave)
	role, err := e.records.CreateRole(models.Role{
		Title: "Platform engineer", Origin: models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating role: %v", err)
	}
	if err := e.deletion.DeleteInitiative(e.initiative); err != nil {
		t.Fatalf("deleting the initiative: %v", err)
	}

	// What it owned is gone.
	if n := e.count(t, &models.SearchCriterion{}, "initiative_id = ?", e.initiative); n != 0 {
		t.Errorf("%d criteria survived", n)
	}
	if n := e.count(t, &models.Initiative{}, "id = ?", e.initiative); n != 0 {
		t.Error("the initiative survived")
	}
	// What it merely referenced is not.
	if n := e.count(t, &models.Candidate{}, "id = ?", candidateID); n != 1 {
		t.Error("deleting an initiative deleted a candidate")
	}
	if n := e.count(t, &models.Role{}, "id = ?", role.ID); n != 1 {
		t.Error("deleting an initiative deleted a role")
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", artifactID); n != 1 {
		t.Error("deleting an initiative deleted a recruiter-added artifact")
	}
	// And the artifact keeps its other link.
	if n := e.count(t, &models.ArtifactLink{},
		"artifact_id = ? AND target_type = ?", artifactID, models.LinkCandidate); n != 1 {
		t.Error("the artifact lost its candidate link")
	}
}

// An archive is a record of work that happened; deleting its subject leaves an
// account of a search for nobody.
func TestCandidateDeletionIsBlockedByActiveAndArchivedInitiatives(t *testing.T) {
	for _, status := range []models.InitiativeStatus{
		models.InitiativeActive, models.InitiativeArchived,
	} {
		t.Run(string(status), func(t *testing.T) {
			e := newDeletionEnv(t)
			c, err := e.records.CreateCandidate(models.Candidate{FullName: "Kalinda Reyes"})
			if err != nil {
				t.Fatalf("creating candidate: %v", err)
			}
			inits := NewInitiativeService(e.db)
			init, err := inits.Create("Job search "+t.Name(), models.InitiativeTypeJobSearch, []uint{c.ID})
			if err != nil {
				t.Fatalf("creating initiative: %v", err)
			}
			if status == models.InitiativeArchived {
				if err := e.db.Model(&models.Initiative{}).Where("id = ?", init.ID).
					Update("status", models.InitiativeArchived).Error; err != nil {
					t.Fatalf("archiving: %v", err)
				}
			}

			preview, err := e.deletion.PreviewCandidate(c.ID)
			if err != nil {
				t.Fatalf("previewing: %v", err)
			}
			if !preview.Blocked {
				t.Fatalf("a %s initiative did not block candidate deletion", status)
			}
			// The refusal is a to-do list, not a dead end.
			if len(preview.Blockers) == 0 || !strings.Contains(preview.Blockers[0], init.Name) {
				t.Fatalf("the blocker does not name the initiative: %+v", preview.Blockers)
			}
			if err := e.deletion.DeleteCandidate(c.ID, ""); err == nil {
				t.Fatal("the candidate was deleted despite a referencing initiative")
			}
			if n := e.count(t, &models.Candidate{}, "id = ?", c.ID); n != 1 {
				t.Fatal("a blocked deletion removed the candidate anyway")
			}

			// Once the reference is gone, deletion succeeds.
			if err := e.deletion.DeleteInitiative(init.ID); err != nil {
				t.Fatalf("deleting the initiative: %v", err)
			}
			if err := e.deletion.DeleteCandidate(c.ID, ""); err != nil {
				t.Fatalf("deleting the candidate after its references went: %v", err)
			}
		})
	}
}

func TestDeletingACandidateRemovesTheirDerivedData(t *testing.T) {
	e := newDeletionEnv(t)
	candidateID, artifactID := e.candidateWithEvidence(t)

	before := e.count(t, &models.Embedding{}, "")
	if before == 0 {
		t.Fatal("the fixture produced no embeddings, so the cascade has nothing to miss")
	}

	if err := e.deletion.DeleteCandidate(candidateID, ""); err != nil {
		t.Fatalf("deleting: %v", err)
	}

	checks := map[string]int64{
		"the candidate":  e.count(t, &models.Candidate{}, "id = ?", candidateID),
		"their artifact": e.count(t, &models.Artifact{}, "id = ?", artifactID),
		"its chunks":     e.count(t, &models.Chunk{}, "artifact_id = ?", artifactID),
		"its embeddings": e.count(t, &models.Embedding{}, ""),
		"their profiles": e.count(t, &models.Profile{},
			"subject_kind = ? AND subject_id = ?", profile.SubjectCandidate, candidateID),
	}
	for what, n := range checks {
		if n != 0 {
			t.Errorf("%s: %d survived", what, n)
		}
	}
	// And nothing derived answers a search any more.
	hits, err := e.search.Search(e.initiative, "quokkastack", 10)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("deleted evidence still answers searches: %+v", hits)
	}
}

// Neither default is safe, so the application refuses to choose.
func TestASharedCandidateArtifactRequiresAChoice(t *testing.T) {
	e := newDeletionEnv(t)
	candidateID, artifactID := e.candidateWithEvidence(t)
	role, err := e.records.CreateRole(models.Role{
		Title: "Platform engineer", Origin: models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating role: %v", err)
	}
	if err := e.artifacts.Link(artifactID, models.LinkRole, role.ID); err != nil {
		t.Fatalf("linking: %v", err)
	}

	preview, err := e.deletion.PreviewCandidate(candidateID)
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	if preview.Choice == "" {
		t.Fatal("a shared artifact did not require a choice")
	}
	if !strings.Contains(preview.Choice, "may contain candidate information") {
		t.Errorf("the warning is missing from the choice: %q", preview.Choice)
	}
	if err := e.deletion.DeleteCandidate(candidateID, ""); err == nil {
		t.Fatal("deletion proceeded without a choice")
	}

	// Retaining keeps it under its other link, without the candidate one.
	if err := e.deletion.DeleteCandidate(candidateID, RetainShared); err != nil {
		t.Fatalf("deleting with retention: %v", err)
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", artifactID); n != 1 {
		t.Fatal("retention deleted the artifact")
	}
	if n := e.count(t, &models.ArtifactLink{},
		"artifact_id = ? AND target_type = ?", artifactID, models.LinkCandidate); n != 0 {
		t.Error("retention kept the candidate link")
	}
	if n := e.count(t, &models.ArtifactLink{},
		"artifact_id = ? AND target_type = ?", artifactID, models.LinkRole); n != 1 {
		t.Error("retention lost the other link")
	}
}

func TestChoosingGlobalDeletionRemovesASharedArtifactEverywhere(t *testing.T) {
	e := newDeletionEnv(t)
	candidateID, artifactID := e.candidateWithEvidence(t)
	// Shared with another candidate rather than a role, so it is not a source
	// listing — those are read-only and tested separately.
	other, err := e.records.CreateCandidate(models.Candidate{FullName: "Tobias Fenn"})
	if err != nil {
		t.Fatalf("creating the other candidate: %v", err)
	}
	if err := e.artifacts.Link(artifactID, models.LinkCandidate, other.ID); err != nil {
		t.Fatalf("linking: %v", err)
	}

	if err := e.deletion.DeleteCandidate(candidateID, DeleteShared); err != nil {
		t.Fatalf("deleting globally: %v", err)
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", artifactID); n != 0 {
		t.Fatal("global deletion left the artifact")
	}
	if n := e.count(t, &models.ArtifactLink{}, "artifact_id = ?", artifactID); n != 0 {
		t.Fatal("global deletion left links behind")
	}
	// The other candidate survives — only the artifact was shared.
	if n := e.count(t, &models.Candidate{}, "id = ?", other.ID); n != 1 {
		t.Fatal("global artifact deletion deleted another candidate")
	}
}

func TestDetachRemovesOneLinkAndDeleteRemovesEverything(t *testing.T) {
	e := newDeletionEnv(t)
	_, artifactID := e.candidateWithEvidence(t)

	// Detach: one link, and nothing else.
	if err := e.deletion.Detach(artifactID, models.LinkInitiative, e.initiative); err != nil {
		t.Fatalf("detaching: %v", err)
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", artifactID); n != 1 {
		t.Fatal("detaching deleted the artifact")
	}
	if n := e.count(t, &models.ArtifactLink{},
		"artifact_id = ? AND target_type = ?", artifactID, models.LinkCandidate); n != 1 {
		t.Fatal("detaching one link removed another")
	}
	if n := e.count(t, &models.Chunk{}, "artifact_id = ?", artifactID); n == 0 {
		t.Fatal("detaching removed the chunks")
	}

	// Global deletion lists every link first, then removes everything.
	preview, err := e.deletion.PreviewArtifact(artifactID)
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	var listed bool
	for _, c := range preview.Removes {
		if c.Kind == "links" && len(c.Detail) > 0 {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("global deletion did not list the links first: %+v", preview.Removes)
	}
	if err := e.deletion.DeleteArtifact(artifactID); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	if n := e.count(t, &models.Chunk{}, "artifact_id = ?", artifactID); n != 0 {
		t.Error("global deletion left chunks")
	}
}

// An interaction's companion note is owned by the interaction, not by
// whatever record it is attached to: detaching it would sever the
// artifact_links row SearchService.People joins on while the interaction
// still points at it.
func TestDetachRefusesAnInteractionsArtifact(t *testing.T) {
	e := newCrmEnv(t)
	logged, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "note", Note: "Left a voicemail.", OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}

	deletion := NewDeletionService(e.db)
	if err := deletion.Detach(logged.ArtifactID, models.LinkCandidate, e.candidate); err == nil {
		t.Fatal("detached an interaction's companion artifact")
	}
	var n int64
	e.db.Model(&models.ArtifactLink{}).Where("artifact_id = ?", logged.ArtifactID).Count(&n)
	if n != 1 {
		t.Fatalf("refused detach removed the link anyway: %d links remain", n)
	}

	// An ordinary artifact on the same candidate still detaches normally.
	artifacts := NewArtifactService(e.db)
	ordinary, err := artifacts.create("Attached resume", "resume.pdf", "", []byte("resume bytes"),
		models.LinkCandidate, e.candidate)
	if err != nil {
		t.Fatalf("creating ordinary artifact: %v", err)
	}
	if err := deletion.Detach(ordinary.ID, models.LinkCandidate, e.candidate); err != nil {
		t.Fatalf("detaching an ordinary artifact was refused: %v", err)
	}
}

// A role's provenance is the sequence of listings it was seen as.
func TestARoleSourceArtifactCannotBeDetachedOrDeleted(t *testing.T) {
	e := newDeletionEnv(t)
	roleID := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")
	var artifactID uint
	err := e.db.Model(&models.ArtifactLink{}).Select("artifact_id").
		Where("target_type = ? AND target_id = ?", models.LinkRole, roleID).
		Limit(1).Scan(&artifactID).Error
	if err != nil {
		t.Fatalf("finding the listing: %v", err)
	}
	// Marked as discovery stores it. The rule this asserts is about an Exa
	// source artifact — "role-owned and read-only" — and the fixture used to
	// leave the source unset, so it passed under a rule broad enough to catch
	// the recruiter's own notes as well.
	if err := e.db.Model(&models.Artifact{}).Where("id = ?", artifactID).
		Update("source", models.ProviderExa).Error; err != nil {
		t.Fatalf("marking the listing as a source: %v", err)
	}

	if err := e.deletion.Detach(artifactID, models.LinkRole, roleID); err == nil {
		t.Fatal("a role's source listing was detached")
	}
	preview, err := e.deletion.PreviewArtifact(artifactID)
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	if !preview.Blocked {
		t.Fatal("a role's source listing was offered for global deletion")
	}
	if err := e.deletion.DeleteArtifact(artifactID); err == nil {
		t.Fatal("a role's source listing was deleted individually")
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", artifactID); n != 1 {
		t.Fatal("a refused deletion removed it anyway")
	}

	// Purging the role is the way, and it takes the listing with it.
	if err := e.deletion.PurgeRole(roleID); err != nil {
		t.Fatalf("purging: %v", err)
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", artifactID); n != 0 {
		t.Fatal("purging the role left its source listing")
	}
}

func TestPurgingARoleRemovesItsDerivedDataIncludingHistoricalSources(t *testing.T) {
	e := newDeletionEnv(t)
	roleID := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")

	// A superseded source, which purging must also take.
	old, err := e.artifacts.create("older listing", "old.md", "exa",
		[]byte("# Platform engineer\n\nAn earlier version.\n"), models.LinkRole, roleID)
	if err != nil {
		t.Fatalf("attaching an older listing: %v", err)
	}
	err = e.db.Model(&models.ArtifactLink{}).
		Where("artifact_id = ? AND target_type = ?", old.ID, models.LinkRole).
		Update("historical", true).Error
	if err != nil {
		t.Fatalf("historicizing: %v", err)
	}

	preview, err := e.deletion.PreviewRolePurge(roleID)
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	var sources int64
	for _, c := range preview.Removes {
		if strings.Contains(c.Kind, "source listings") {
			sources = c.Count
		}
	}
	if sources < 2 {
		t.Fatalf("the preview counted %d sources, want the current one and the historical one", sources)
	}

	if err := e.deletion.PurgeRole(roleID); err != nil {
		t.Fatalf("purging: %v", err)
	}
	if n := e.count(t, &models.Role{}, "id = ?", roleID); n != 0 {
		t.Error("the role survived its purge")
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", old.ID); n != 0 {
		t.Error("the historical source survived the purge")
	}
	if n := e.count(t, &models.Profile{},
		"subject_kind = ? AND subject_id = ?", profile.SubjectRole, roleID); n != 0 {
		t.Error("the role profile survived the purge")
	}
}

// The recruiter's own words, which nothing else authored.
func TestSurvivorsKeepTheirTextAndLoseTheirReferences(t *testing.T) {
	e := newDeletionEnv(t)
	roleID := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")

	// A copy event about a draft about this role.
	id := e.initiative
	role := roleID
	event := models.DisclosureEvent{
		OccurredAt: e.clock, Provider: "local", Task: models.TaskCopiedOut,
		Categories:   "draft text copied to the clipboard",
		InitiativeID: &id, RoleID: &role,
	}
	if err := e.db.Create(&event).Error; err != nil {
		t.Fatalf("recording a copy: %v", err)
	}

	if err := e.deletion.PurgeRole(roleID); err != nil {
		t.Fatalf("purging: %v", err)
	}

	var after models.DisclosureEvent
	if err := e.db.First(&after, event.ID).Error; err != nil {
		t.Fatalf("the copy event did not survive the purge: %v", err)
	}
	if after.RoleID != nil {
		t.Errorf("the surviving event still points at the purged role: %+v", after.RoleID)
	}
	if after.InitiativeID == nil || *after.InitiativeID != e.initiative {
		t.Error("the surviving event left its initiative")
	}
	if after.Task != models.TaskCopiedOut {
		t.Error("the surviving event changed what it recorded")
	}
}

func TestDeletingADraftClearsTheReferenceOnItsCopyEvents(t *testing.T) {
	e := newDeletionEnv(t)
	id := e.draftableCandidate(t)
	e.generateModel(t)
	e.model.respond = func(string) string {
		return drafted("Subject", "Some text.", Claim{Text: "text", Refs: []string{"profile-1"}})
	}
	draft, err := e.drafts.Generate(DraftInput{
		InitiativeID: e.initiative, CandidateID: id, Kind: models.DraftOutreach,
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	if err := e.drafts.Copy(draft.ID); err != nil {
		t.Fatalf("copying: %v", err)
	}

	if err := e.deletion.DeleteDraft(draft.ID); err != nil {
		t.Fatalf("deleting the draft: %v", err)
	}
	if n := e.count(t, &models.Draft{}, "id = ?", draft.ID); n != 0 {
		t.Error("the draft survived")
	}
	events := []models.DisclosureEvent{}
	if err := e.db.Where("task = ?", models.TaskCopiedOut).Find(&events).Error; err != nil {
		t.Fatalf("listing events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("the copy event did not survive: %d events", len(events))
	}
	if events[0].DraftID != nil {
		t.Error("the surviving event still points at the deleted draft")
	}
}

// This is the test that justifies the transaction.
func TestAFailureAtAnyCascadeStepChangesNothing(t *testing.T) {
	// Every table a candidate deletion touches. A rollback means every one of
	// these counts is identical afterwards — not most of them.
	tables := []struct {
		name  string
		model any
	}{
		{"candidates", &models.Candidate{}},
		{"artifacts", &models.Artifact{}},
		{"artifact links", &models.ArtifactLink{}},
		{"chunks", &models.Chunk{}},
		{"embeddings", &models.Embedding{}},
		{"profiles", &models.Profile{}},
		{"profile aspects", &models.ProfileAspect{}},
	}

	e := newDeletionEnv(t)
	candidateID, _ := e.candidateWithEvidence(t)

	before := map[string]int64{}
	for _, table := range tables {
		before[table.name] = e.count(t, table.model, "")
		// A count of zero makes the assertion below vacuous: nothing can be
		// left behind from a table that was empty. The fixture has to actually
		// populate every table the cascade touches, or this proves nothing
		// about that step.
		if before[table.name] == 0 {
			t.Fatalf("the fixture leaves %s empty, so a rollback of it asserts nothing", table.name)
		}
	}

	// The failure is injected into the real cascade, once per table it touches,
	// rather than into a transaction the test writes itself.
	//
	// This test used to hand-roll a transaction that deleted two tables and
	// returned an error. That asserts the database rolls back a transaction,
	// which it does; it says nothing about DeleteCandidate, which it never
	// called. A step accidentally outside the service's transaction would have
	// gone unnoticed — and "failure is injected at each cascade step in turn"
	// is what the requirement asks for.
	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			stop := e.failDeletesOf(t, e.tableNameOf(t, table.model))
			err := e.deletion.DeleteCandidate(candidateID, RetainShared)
			stop()
			if err == nil {
				t.Fatalf("a failure deleting %s did not surface", table.name)
			}
			for _, other := range tables {
				if after := e.count(t, other.model, ""); after != before[other.name] {
					t.Errorf("%s changed from %d to %d despite the rollback",
						other.name, before[other.name], after)
				}
			}
		})
	}
}

// failDeletesOf makes every DELETE against one table fail, and returns the
// function that stops it.
func (e *deletionEnv) failDeletesOf(t *testing.T, table string) func() {
	t.Helper()
	return e.failDeletes(t, table, false)
}

// failFirstDeleteOf makes only the first DELETE against one table fail, so a
// batch can be made to fail on one item rather than on all of them.
func (e *deletionEnv) failFirstDeleteOf(t *testing.T, table string) func() {
	t.Helper()
	return e.failDeletes(t, table, true)
}

// failDeletes registers the failure and returns the function that stops it.
func (e *deletionEnv) failDeletes(t *testing.T, table string, once bool) func() {
	t.Helper()
	const name = "test:fail_delete"
	fired := false
	err := e.db.Callback().Delete().Before("gorm:delete").Register(name, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != table {
			return
		}
		if once && fired {
			return
		}
		fired = true
		_ = tx.AddError(errInjected)
	})
	if err != nil {
		t.Fatalf("registering the failure: %v", err)
	}
	return func() {
		if err := e.db.Callback().Delete().Remove(name); err != nil {
			t.Fatalf("removing the failure: %v", err)
		}
	}
}

// tableNameOf is what the database calls this model.
func (e *deletionEnv) tableNameOf(t *testing.T, model any) string {
	t.Helper()
	stmt := &gorm.Statement{DB: e.db}
	if err := stmt.Parse(model); err != nil {
		t.Fatalf("resolving the table name: %v", err)
	}
	return stmt.Schema.Table
}

// errInjected is the mid-cascade failure the rollback test causes.
var errInjected = errors.New("injected failure")

// A chunk shared with something else is not a failure, and a check that counted
// it would make every correct deletion look broken.
func TestVerificationToleratesIntentionallySharedEvidence(t *testing.T) {
	e := newDeletionEnv(t)
	candidateID, artifactID := e.candidateWithEvidence(t)
	other, err := e.records.CreateCandidate(models.Candidate{FullName: "Tobias Fenn"})
	if err != nil {
		t.Fatalf("creating the other candidate: %v", err)
	}
	if err := e.artifacts.Link(artifactID, models.LinkCandidate, other.ID); err != nil {
		t.Fatalf("linking: %v", err)
	}

	// Retained under the other link: its chunks survive, and that is correct.
	if err := e.deletion.DeleteCandidate(candidateID, RetainShared); err != nil {
		t.Fatalf("a correct deletion was reported failed because shared evidence survived: %v", err)
	}
	if n := e.count(t, &models.Chunk{}, "artifact_id = ?", artifactID); n == 0 {
		t.Fatal("retention removed the shared evidence")
	}
}

func TestRepeatedDeletionIsSafe(t *testing.T) {
	e := newDeletionEnv(t)
	candidateID, _ := e.candidateWithEvidence(t)
	other, err := e.records.CreateCandidate(models.Candidate{FullName: "Tobias Fenn"})
	if err != nil {
		t.Fatalf("creating the other candidate: %v", err)
	}

	if err := e.deletion.DeleteCandidate(candidateID, ""); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	gone, err := e.deletion.Gone("candidate", candidateID)
	if err != nil {
		t.Fatalf("checking: %v", err)
	}
	if !gone {
		t.Fatal("the candidate is not reported gone")
	}
	// Again, harmlessly.
	if err := e.deletion.DeleteCandidate(candidateID, ""); err != nil {
		t.Fatalf("a repeated deletion failed: %v", err)
	}
	// And the unrelated record is untouched.
	if n := e.count(t, &models.Candidate{}, "id = ?", other.ID); n != 1 {
		t.Fatal("a repeated deletion damaged an unrelated record")
	}
}

func TestPurgingStaleRolesAppliesTheInvariantPerRole(t *testing.T) {
	e := newDeletionEnv(t)
	first := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")
	second := e.roleWithListing(t, "Data engineer", "dbt and Airflow at scale.")

	report := e.deletion.PurgeStale([]uint{first, second})
	if len(report.Purged) != 2 || len(report.Failed) != 0 {
		t.Fatalf("purged %v, failed %v", report.Purged, report.Failed)
	}
	for _, id := range []uint{first, second} {
		if n := e.count(t, &models.Role{}, "id = ?", id); n != 0 {
			t.Errorf("role %d survived", id)
		}
	}

	// A role that does not exist is reported rather than crashing the batch.
	again := e.deletion.PurgeStale([]uint{first, 999999})
	if len(again.Purged)+len(again.Failed) != 2 {
		t.Fatalf("the batch did not account for both roles: %+v", again)
	}
}

func TestPreviewingChangesNothing(t *testing.T) {
	e := newDeletionEnv(t)
	candidateID, artifactID := e.candidateWithEvidence(t)
	roleID := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")

	before := map[string]int64{
		"candidates": e.count(t, &models.Candidate{}, ""),
		"artifacts":  e.count(t, &models.Artifact{}, ""),
		"chunks":     e.count(t, &models.Chunk{}, ""),
		"roles":      e.count(t, &models.Role{}, ""),
	}
	if _, err := e.deletion.PreviewCandidate(candidateID); err != nil {
		t.Fatalf("previewing a candidate: %v", err)
	}
	if _, err := e.deletion.PreviewArtifact(artifactID); err != nil {
		t.Fatalf("previewing an artifact: %v", err)
	}
	if _, err := e.deletion.PreviewRolePurge(roleID); err != nil {
		t.Fatalf("previewing a purge: %v", err)
	}
	if _, err := e.deletion.PreviewInitiative(e.initiative); err != nil {
		t.Fatalf("previewing an initiative: %v", err)
	}
	for what, n := range before {
		var after int64
		switch what {
		case "candidates":
			after = e.count(t, &models.Candidate{}, "")
		case "artifacts":
			after = e.count(t, &models.Artifact{}, "")
		case "chunks":
			after = e.count(t, &models.Chunk{}, "")
		case "roles":
			after = e.count(t, &models.Role{}, "")
		}
		if after != n {
			t.Errorf("previewing changed %s from %d to %d", what, n, after)
		}
	}
}

// Every deletion proves itself, and draft deletion was the one that did not.
//
// The PRD asks it of all of them: "a scoped verification query proves the
// deleted entity and exclusively owned evidence no longer appear in retrieval
// or matching". Initiative, candidate, artifact and role each had one. A draft
// is deleted inside a transaction, which makes a partial write unlikely rather
// than impossible — and what a transaction cannot help with is a table gaining
// a reference to a draft that nobody remembers to clear, which is the shape the
// copy events already have.
func TestDeletingADraftProvesTheDraftAndItsReferencesAreGone(t *testing.T) {
	e := newDeletionEnv(t)
	draft := models.Draft{
		InitiativeID: e.initiative,
		Kind:         models.DraftPitch, Subject: "A pitch", Body: "Five years of Go.",
		State: models.DraftActive,
	}
	if err := e.db.Create(&draft).Error; err != nil {
		t.Fatalf("creating the draft: %v", err)
	}
	id := e.initiative
	event := models.DisclosureEvent{
		OccurredAt: time.Now().UTC(), Provider: "local", Task: models.TaskCopiedOut,
		Categories: "a draft", InitiativeID: &id, DraftID: &draft.ID,
	}
	if err := e.db.Create(&event).Error; err != nil {
		t.Fatalf("creating the copy event: %v", err)
	}

	if err := e.deletion.DeleteDraft(draft.ID); err != nil {
		t.Fatalf("deleting the draft: %v", err)
	}

	// The draft is gone.
	var drafts int64
	if err := e.db.Model(&models.Draft{}).Where("id = ?", draft.ID).Count(&drafts).Error; err != nil {
		t.Fatalf("counting drafts: %v", err)
	}
	if drafts != 0 {
		t.Fatal("the draft survived its own deletion")
	}
	// The copy event survives, as the PRD requires, with its reference cleared.
	var events []models.DisclosureEvent
	if err := e.db.Find(&events).Error; err != nil {
		t.Fatalf("reading events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("%d copy events survived, want the one", len(events))
	}
	if events[0].DraftID != nil {
		t.Fatalf("a copy event still points at the deleted draft: %v", *events[0].DraftID)
	}
}

// discoveredRole is a role as discovery would have created it.
func (e *deletionEnv) discoveredRole(t *testing.T) uint {
	t.Helper()
	role, err := e.records.CreateRole(models.Role{
		Title: "Platform engineer", Origin: models.RoleOriginDiscovered,
		LifecycleState: models.RoleActive,
	})
	if err != nil {
		t.Fatalf("creating the role: %v", err)
	}
	return role.ID
}

// attachTo links a new artifact to a role, marked with the source that decides
// whether it is the role's own listing or something the recruiter added.
func (e *deletionEnv) attachTo(t *testing.T, roleID uint, name, source, body string) uint {
	t.Helper()
	a, err := e.artifacts.create(name, name+".md", source, []byte(body), models.LinkRole, roleID)
	if err != nil {
		t.Fatalf("attaching %s: %v", name, err)
	}
	return a.ID
}

// Purging a role destroys its source listing and keeps what the recruiter
// wrote.
//
// The invariants distinguish the two. An Exa source artifact is "role-owned and
// read-only… it cannot be independently detached or globally deleted; purge the
// role instead". A recruiter-added artifact survives deletions elsewhere, and a
// purge leaves "recruiter-authored notes… with an unavailable role reference".
//
// Nothing distinguished them. Every artifact linked to the role was deleted,
// bytes and all, so a recruiter who attached their own notes to a discovered
// role lost them by purging it — and could not detach them first, because
// detaching refuses anything linked to a role and tells them to purge.
func TestPurgingARoleKeepsWhatTheRecruiterAttached(t *testing.T) {
	e := newDeletionEnv(t)
	roleID := e.discoveredRole(t)
	// The listing as discovery stores it, and the recruiter's own notes.
	listing := e.attachTo(t, roleID, "listing", models.ProviderExa,
		"# Platform engineer\n\nMust have Go.\n")
	notes := e.attachTo(t, roleID, "my notes", "recruiter",
		"# Notes\n\nSpoke to the hiring manager on Tuesday.\n")

	if err := e.deletion.PurgeRole(roleID); err != nil {
		t.Fatalf("purging: %v", err)
	}

	if n := e.count(t, &models.Artifact{}, "id = ?", listing); n != 0 {
		t.Fatal("the role's source listing survived the purge")
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", notes); n != 1 {
		t.Fatal("the recruiter's own notes were deleted by purging a role")
	}
	// Surviving with an unavailable role reference: the link is gone, the
	// bytes are not.
	if n := e.count(t, &models.ArtifactLink{},
		"artifact_id = ? AND target_type = ?", notes, models.LinkRole); n != 0 {
		t.Fatal("the notes still point at a role that no longer exists")
	}
}

// A recruiter can detach their own notes from a role.
//
// Detaching refused anything linked to a role, on the rule that a role's source
// listing is read-only. Their notes are linked to a role and are not its source,
// so they were told to purge the role instead — which deleted the notes.
func TestARecruitersOwnNotesCanBeDetachedFromARole(t *testing.T) {
	e := newDeletionEnv(t)
	roleID := e.discoveredRole(t)
	notes := e.attachTo(t, roleID, "my notes", "recruiter",
		"# Notes\n\nSpoke to the hiring manager on Tuesday.\n")

	if err := e.deletion.Detach(notes, models.LinkRole, roleID); err != nil {
		t.Fatalf("detaching the recruiter's own notes: %v", err)
	}
	// The link goes and the bytes stay: detaching "removes one link only".
	if n := e.count(t, &models.ArtifactLink{},
		"artifact_id = ? AND target_type = ?", notes, models.LinkRole); n != 0 {
		t.Fatal("the link survived the detach")
	}
	if n := e.count(t, &models.Artifact{}, "id = ?", notes); n != 1 {
		t.Fatal("detaching deleted the artifact")
	}
}

// One role failing leaves that role whole and purges the others.
//
// "Purging all stale roles SHALL apply the same rules independently to each,
// and SHALL report any role that could not be purged rather than partially
// deleting it."
//
// The existing test covers both succeeding and a role that does not exist,
// which fails with nothing to undo. The case the clause is about is a purge
// that fails partway: the danger is a role left half deleted — its profile and
// matches gone, the role still listed — and a report that says it worked.
func TestOneFailedPurgeLeavesThatRoleWholeAndPurgesTheRest(t *testing.T) {
	e := newDeletionEnv(t)
	first := e.roleWithListing(t, "Platform engineer", "quokkastack engineering at scale.")
	second := e.roleWithListing(t, "Data engineer", "dbt and Airflow at scale.")

	profilesBefore := e.count(t, &models.Profile{}, "subject_kind = ? AND subject_id = ?",
		profile.SubjectRole, first)
	if profilesBefore == 0 {
		t.Fatal("the first role has no profile, so a partial deletion would leave nothing to see")
	}

	// The first role's purge fails at its last step; the second is untouched by
	// the failure.
	stop := e.failFirstDeleteOf(t, e.tableNameOf(t, &models.Role{}))
	report := e.deletion.PurgeStale([]uint{first, second})
	stop()

	if len(report.Failed) != 1 || report.Failed[first] == "" {
		t.Fatalf("the report does not name the role that failed: %+v", report)
	}
	if len(report.Purged) != 1 || report.Purged[0] != second {
		t.Fatalf("the second role was not purged: %+v", report)
	}

	// The failed one is whole, not half gone.
	if n := e.count(t, &models.Role{}, "id = ?", first); n != 1 {
		t.Fatal("the role that failed to purge is gone anyway")
	}
	if n := e.count(t, &models.Profile{}, "subject_kind = ? AND subject_id = ?",
		profile.SubjectRole, first); n != profilesBefore {
		t.Fatal("the role survived and its profile did not — it is half purged")
	}
	// And the second really went.
	if n := e.count(t, &models.Role{}, "id = ?", second); n != 0 {
		t.Fatal("the second role survived")
	}
}
