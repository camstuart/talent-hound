package main

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// Every fixture here is invented. No real person appears in these tests.

// fakeGitHub answers per endpoint and counts what it was asked.
type fakeGitHub struct {
	profile *platform.GitHubProfile
	repos   []platform.GitHubRepo
	events  []platform.GitHubEvent
	fail    map[string]error
	asked   []string
}

func (f *fakeGitHub) Profile(_ context.Context, login string) (*platform.GitHubProfile, error) {
	f.asked = append(f.asked, "profile:"+login)
	if err := f.fail["profile"]; err != nil {
		return nil, err
	}
	return f.profile, nil
}

func (f *fakeGitHub) Repos(_ context.Context, login string) ([]platform.GitHubRepo, error) {
	f.asked = append(f.asked, "repos:"+login)
	if err := f.fail["repos"]; err != nil {
		return nil, err
	}
	return f.repos, nil
}

func (f *fakeGitHub) Events(_ context.Context, login string) ([]platform.GitHubEvent, error) {
	f.asked = append(f.asked, "events:"+login)
	if err := f.fail["events"]; err != nil {
		return nil, err
	}
	return f.events, nil
}

type enrichEnv struct {
	*profileEnv
	github    *fakeGitHub
	enrich    *EnrichService
	candidate uint
}

func newEnrichEnv(t *testing.T) *enrichEnv {
	t.Helper()
	base := newProfileEnv(t)
	c, err := base.records.CreateCandidate(models.Candidate{FullName: "Wombat Developer"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	gh := &fakeGitHub{
		profile: &platform.GitHubProfile{Login: "wombatdev", Name: "Wombat Developer", Location: "Melbourne", Bio: "Local-first tools."},
		repos: []platform.GitHubRepo{
			{Name: "burrow", Language: "Go", Stars: 10, PushedOn: "2026-08-01", URL: "https://github.com/wombatdev/burrow", Description: "A local-first sync engine."},
		},
		events: []platform.GitHubEvent{{Type: "PushEvent", Repo: "wombatdev/burrow", CreatedOn: "2026-08-20"}},
		fail:   map[string]error{},
	}
	e := &enrichEnv{profileEnv: base, github: gh, candidate: c.ID}
	e.enrich = NewEnrichService(base.db, gh, base.records, base.artifacts, nil)
	return e
}

func (e *enrichEnv) withGitHub(t *testing.T) {
	t.Helper()
	if _, err := e.enrich.AddIdentity(e.candidate, "", "https://github.com/WombatDev"); err != nil {
		t.Fatalf("adding identity: %v", err)
	}
}

func (e *enrichEnv) disclosures(t *testing.T) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := e.db.Raw("SELECT * FROM disclosure_events").Scan(&rows).Error; err != nil {
		t.Fatalf("reading disclosures: %v", err)
	}
	return rows
}

func TestAnIdentityIsParsedFromItsURL(t *testing.T) {
	e := newEnrichEnv(t)
	id, err := e.enrich.AddIdentity(e.candidate, "", "https://github.com/@Octocat/")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if id.Provider != models.IdentityGitHub || id.Handle != "octocat" {
		t.Fatalf("identity = %+v", id)
	}
	if _, err := e.enrich.AddIdentity(e.candidate, "", "https://github.com/octocat"); err == nil ||
		!strings.Contains(err.Error(), "already belongs") {
		t.Fatalf("a duplicate handle was accepted: %v", err)
	}
	if _, err := e.enrich.AddIdentity(e.candidate, models.IdentityGitHub, "https://wombat.example.invalid/"); err == nil {
		t.Fatal("a website was accepted as a GitHub identity")
	}
	site, err := e.enrich.AddIdentity(e.candidate, "", "https://wombat.example.invalid/about")
	if err != nil || site.Provider != models.IdentityWebsite || site.Handle != "wombat.example.invalid" {
		t.Fatalf("website identity = %+v, err = %v", site, err)
	}
	all, _ := e.enrich.Identities(e.candidate)
	if len(all) != 2 {
		t.Fatalf("%d identities", len(all))
	}
	if err := e.enrich.RemoveIdentity(site.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	all, _ = e.enrich.Identities(e.candidate)
	if len(all) != 1 {
		t.Fatalf("%d identities after removal", len(all))
	}
}

func TestARunIsRefusedBeforeAnythingLeavesWithoutAHandleOrAToken(t *testing.T) {
	e := newEnrichEnv(t)
	preview, err := e.enrich.Preview(e.candidate)
	if err != nil || !strings.Contains(preview.Reason, "no GitHub identity") {
		t.Fatalf("preview = %+v, err = %v", preview, err)
	}
	if _, err := e.enrich.Run(e.candidate); err == nil {
		t.Fatal("a run with no identity proceeded")
	}

	e.withGitHub(t)
	e.enrich.github = nil
	e.enrich.out.credentials = &CredentialService{store: newMemoryStore()}
	preview, _ = e.enrich.Preview(e.candidate)
	if preview.TokenStored || !strings.Contains(preview.Reason, "no GitHub token") {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := e.enrich.Run(e.candidate); err == nil {
		t.Fatal("a run with no token proceeded")
	}
	if len(e.github.asked) != 0 || len(e.disclosures(t)) != 0 {
		t.Fatalf("asked %v, disclosed %d", e.github.asked, len(e.disclosures(t)))
	}
}

func TestARunKeepsThreeArtifactsAndDisclosesAHandleNotWhich(t *testing.T) {
	e := newEnrichEnv(t)
	e.withGitHub(t)

	out, err := e.enrich.Run(e.candidate)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(out.ArtifactIDs) != 3 || out.Unchanged != 0 || out.Partial {
		t.Fatalf("outcome = %+v", out)
	}
	artifacts, err := e.artifacts.ListForTarget(models.LinkCandidate, e.candidate)
	if err != nil || len(artifacts) != 3 {
		t.Fatalf("artifacts = %+v, err = %v", artifacts, err)
	}
	sources := map[string]bool{}
	for _, a := range artifacts {
		sources[a.Source] = true
		encoded, _ := e.artifacts.Bytes(a.ID)
		body, _ := base64.StdEncoding.DecodeString(encoded)
		if strings.Contains(string(body), "@") {
			t.Fatalf("evidence carries an address: %s", body)
		}
	}
	for _, want := range []string{"github:wombatdev/profile", "github:wombatdev/repos", "github:wombatdev/activity"} {
		if !sources[want] {
			t.Fatalf("missing %s in %v", want, sources)
		}
	}

	rows := e.disclosures(t)
	if len(rows) != 1 {
		t.Fatalf("%d disclosures", len(rows))
	}
	flat := ""
	for _, v := range rows[0] {
		if s, ok := v.(string); ok {
			flat += " " + s
		}
	}
	if strings.Contains(strings.ToLower(flat), "wombat") {
		t.Fatalf("the disclosure names the handle: %s", flat)
	}
	if rows[0]["task"] != models.TaskCandidateEnrich || rows[0]["categories"] != "public handle" {
		t.Fatalf("task/categories = %v / %v", rows[0]["task"], rows[0]["categories"])
	}

	var identity models.Identity
	e.db.Where("candidate_id = ?", e.candidate).First(&identity)
	if identity.VerifiedAt == "" {
		t.Fatal("the identity was not marked verified")
	}

	// Unchanged answers add nothing.
	again, err := e.enrich.Run(e.candidate)
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if len(again.ArtifactIDs) != 0 || again.Unchanged != 3 {
		t.Fatalf("rerun outcome = %+v", again)
	}
	artifacts, _ = e.artifacts.ListForTarget(models.LinkCandidate, e.candidate)
	if len(artifacts) != 3 {
		t.Fatalf("%d artifacts after a rerun", len(artifacts))
	}
}

func TestAFailureMidRunIsPartialAndKeepsWhatLanded(t *testing.T) {
	e := newEnrichEnv(t)
	e.withGitHub(t)
	e.github.fail["repos"] = platform.ErrSearchRateLimited

	out, err := e.enrich.Run(e.candidate)
	if err != nil {
		t.Fatalf("a partial run errored outright: %v", err)
	}
	if !out.Partial || len(out.ArtifactIDs) != 1 || !strings.Contains(out.FailureReason, "rate limiting") {
		t.Fatalf("outcome = %+v", out)
	}
	if len(e.disclosures(t)) != 1 {
		t.Fatalf("%d disclosures for a run that sent something", len(e.disclosures(t)))
	}

	// Failing on the very first request is an error, and no disclosure: the
	// request was attempted, which is the disclosure — so it is recorded.
	e2 := newEnrichEnv(t)
	e2.withGitHub(t)
	e2.github.fail["profile"] = platform.ErrSearchOffline
	if _, err := e2.enrich.Run(e2.candidate); err == nil {
		t.Fatal("a run that got nothing succeeded")
	}
	if len(e2.disclosures(t)) != 1 {
		t.Fatalf("an attempted request was not recorded: %d disclosures", len(e2.disclosures(t)))
	}
}
