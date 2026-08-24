package main

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

type crmEnv struct {
	*indexEnv
	interactions *InteractionService
	candidate    uint
}

func newCrmEnv(t *testing.T) *crmEnv {
	t.Helper()
	e := newIndexEnv(t)
	cand := models.Candidate{FullName: "Quinn Sample"}
	if err := e.db.Create(&cand).Error; err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	return &crmEnv{
		indexEnv:     e,
		interactions: NewInteractionService(e.db, e.chunks),
		candidate:    cand.ID,
	}
}

func TestLoggingAnInteractionCreatesSearchableEvidence(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate,
		TargetID:   e.candidate,
		Kind:       "call",
		Note:       "Wants to move into fintech. Available from October.",
		OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if made.ArtifactID == 0 {
		t.Fatalf("interaction has no companion artifact")
	}

	// The companion artifact is linked to the candidate and already extracted:
	// the note is Markdown this application wrote, no sidecar involved.
	var artifact models.Artifact
	if err := e.db.First(&artifact, made.ArtifactID).Error; err != nil {
		t.Fatalf("loading artifact: %v", err)
	}
	if artifact.Source != "interaction" || artifact.ExtractionState != models.ExtractionExtracted {
		t.Fatalf("artifact source=%q state=%q", artifact.Source, artifact.ExtractionState)
	}
	if !strings.Contains(artifact.Markdown, "fintech") || !strings.Contains(artifact.Markdown, "Quinn Sample") {
		t.Fatalf("markdown lacks note or subject name: %q", artifact.Markdown)
	}

	// The chunk job was enqueued; once it runs, the note is findable by FTS.
	waitForLatestJob(t, e.jobs)
	hits := []models.Chunk{}
	if err := e.db.Where("artifact_id = ?", made.ArtifactID).Find(&hits).Error; err != nil || len(hits) == 0 {
		t.Fatalf("no chunks for the note (err %v)", err)
	}
}

// waitForLatestJob waits for the most recently enqueued job to finish.
func waitForLatestJob(t *testing.T, jobs *JobService) {
	t.Helper()
	all, err := jobs.List()
	if err != nil || len(all) == 0 {
		t.Fatalf("listing jobs: %v (%d jobs)", err, len(all))
	}
	newest := all[0]
	for _, j := range all {
		if j.ID > newest.ID {
			newest = j
		}
	}
	if done := waitForJob(t, jobs, newest.ID); done.State != models.JobCompleted {
		t.Fatalf("job %d finished %s: %s", newest.ID, done.State, done.FailureReason)
	}
}

func TestAnOutcomeNeedsItsRoleAndTheTimelineNamesIt(t *testing.T) {
	e := newCrmEnv(t)
	if _, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "placement", Note: "Placed.", OccurredAt: "2026-08-01",
	}); err == nil || !strings.Contains(err.Error(), "role") {
		t.Fatalf("placement without role accepted: %v", err)
	}

	role := models.Role{
		Title:          "Staff Engineer",
		CanonicalURL:   "https://example.test/r1",
		Origin:         models.RoleOriginDiscovered,
		LifecycleState: models.RoleActive,
	}
	if err := e.db.Create(&role).Error; err != nil {
		t.Fatalf("creating role: %v", err)
	}
	if _, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "placement", Note: "Placed after two rounds.", OccurredAt: "2026-08-01",
		RoleID: role.ID,
	}); err != nil {
		t.Fatalf("logging placement: %v", err)
	}
	if _, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "call", Note: "Follow-up call.", OccurredAt: "2026-08-10",
	}); err != nil {
		t.Fatalf("logging call: %v", err)
	}

	timeline, err := e.interactions.Timeline(models.LinkCandidate, e.candidate)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if len(timeline) != 2 {
		t.Fatalf("want 2 entries, got %d", len(timeline))
	}
	// Newest first by occurred date.
	if timeline[0].Kind != "call" || timeline[1].Kind != "placement" {
		t.Fatalf("order wrong: %s then %s", timeline[0].Kind, timeline[1].Kind)
	}
	if timeline[1].RoleTitle != "Staff Engineer" {
		t.Fatalf("placement entry lacks role title: %q", timeline[1].RoleTitle)
	}
}

func TestAnInteractionDefaultsItsDateToToday(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "note", Note: "No date supplied.",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if made.OccurredAt == "" {
		t.Fatalf("occurred date not defaulted")
	}
}

func TestAnInteractionOnAMissingTargetIsRefused(t *testing.T) {
	e := newCrmEnv(t)
	_, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: 999999,
		Kind: "note", Note: "Ghost.", OccurredAt: "2026-08-24",
	})
	if err == nil {
		t.Fatalf("interaction on missing candidate accepted")
	}
	// Nothing half-created: no interaction row, no orphaned artifact.
	var n int64
	if e.db.Model(&models.Interaction{}).Count(&n); n != 0 {
		t.Fatalf("%d interaction rows left behind", n)
	}
	var a int64
	if e.db.Model(&models.Artifact{}).Where("source = ?", "interaction").Count(&a); a != 0 {
		t.Fatalf("%d interaction artifacts left behind", a)
	}
}

func TestEditingAnInteractionReplacesItsEvidence(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "call", Note: "Interested in quokkastack roles.", OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	waitForLatestJob(t, e.jobs)
	oldArtifact := made.ArtifactID

	edited, err := e.interactions.Update(InteractionInput{
		ID: made.ID, TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "call", Note: "Interested in wombatscale roles.", OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("updating: %v", err)
	}
	waitForLatestJob(t, e.jobs)

	if edited.ArtifactID == oldArtifact {
		t.Fatalf("edit kept the old artifact")
	}
	var gone int64
	e.db.Model(&models.Artifact{}).Where("id = ?", oldArtifact).Count(&gone)
	if gone != 0 {
		t.Fatalf("old artifact survived the edit")
	}
	var stale int64
	e.db.Model(&models.Chunk{}).Where("artifact_id = ?", oldArtifact).Count(&stale)
	if stale != 0 {
		t.Fatalf("old chunks survived the edit")
	}
	// The new wording is what search finds.
	var fresh []models.Chunk
	e.db.Where("artifact_id = ?", edited.ArtifactID).Find(&fresh)
	if len(fresh) == 0 || !strings.Contains(fresh[0].Text, "wombatscale") {
		t.Fatalf("new chunks missing or stale: %+v", fresh)
	}
}

func TestDeletingAnInteractionRemovesItsEvidence(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "note", Note: "Short-lived note.", OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	waitForLatestJob(t, e.jobs)

	if err := e.interactions.Delete(made.ID); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	var rows, artifacts, chunks int64
	e.db.Model(&models.Interaction{}).Count(&rows)
	e.db.Model(&models.Artifact{}).Where("id = ?", made.ArtifactID).Count(&artifacts)
	e.db.Model(&models.Chunk{}).Where("artifact_id = ?", made.ArtifactID).Count(&chunks)
	if rows != 0 || artifacts != 0 || chunks != 0 {
		t.Fatalf("leftovers: %d rows, %d artifacts, %d chunks", rows, artifacts, chunks)
	}
}

var _ = gorm.ErrRecordNotFound // keep the import if unused after edits
