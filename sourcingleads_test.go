package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// leads runs one search with the given results and returns the lead ids.
func (e *sourcingEnv) leads(t *testing.T, roleID uint, results ...platform.SearchResult) []uint {
	t.Helper()
	e.exa.responses = []*platform.SearchResponse{{Results: results}}
	return e.send(t, roleID, "Go engineers").LeadIDs
}

func TestPromotingALeadMakesACandidateWithAnIdentityAndEvidence(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	ids := e.leads(t, roleID,
		person("p2", "https://github.com/WombatDev", "WombatDev", "Builds local-first tools in Go."))

	suggested, err := e.sourcing.Suggest(ids[0])
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if suggested.FullName != "" {
		t.Fatalf("a login was suggested as a name: %q", suggested.FullName)
	}
	var candidates int64
	e.db.Model(&models.Candidate{}).Count(&candidates)
	if candidates != 0 {
		t.Fatalf("suggest created %d candidates", candidates)
	}

	suggested.FullName = "Wombat Developer"
	suggested.Location = "Melbourne"
	c, err := e.sourcing.Promote(ids[0], *suggested)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if c.ID == 0 || c.FullName != "Wombat Developer" || !strings.Contains(c.SourceNote, "exa") {
		t.Fatalf("candidate = %+v", c)
	}

	var identity models.Identity
	if err := e.db.Where("candidate_id = ?", c.ID).First(&identity).Error; err != nil {
		t.Fatalf("no identity: %v", err)
	}
	if identity.Provider != models.IdentityGitHub || identity.Handle != "wombatdev" {
		t.Fatalf("identity = %+v", identity)
	}

	artifacts, err := e.artifacts.ListForTarget(models.LinkCandidate, c.ID)
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("artifacts = %+v, err = %v", artifacts, err)
	}
	if artifacts[0].Source != "exa:https://github.com/WombatDev" {
		t.Fatalf("artifact source = %q", artifacts[0].Source)
	}
	encoded, err := e.artifacts.Bytes(artifacts[0].ID)
	if err != nil {
		t.Fatalf("reading evidence: %v", err)
	}
	body, _ := base64.StdEncoding.DecodeString(encoded)
	if !strings.Contains(string(body), "local-first tools") {
		t.Fatalf("evidence = %q", body)
	}

	var lead models.Lead
	e.db.First(&lead, ids[0])
	if lead.State != models.LeadPromoted || lead.CandidateID == nil || *lead.CandidateID != c.ID {
		t.Fatalf("lead = %+v", lead)
	}

	// Twice is refused, and the pool is unchanged.
	if _, err := e.sourcing.Promote(ids[0], *suggested); err == nil {
		t.Fatal("a promoted lead was promoted again")
	}
	e.db.Model(&models.Candidate{}).Count(&candidates)
	if candidates != 1 {
		t.Fatalf("%d candidates after a refused promotion", candidates)
	}

	// The next search recognises the person.
	again := e.leads(t, roleID, person("p2", "https://github.com/wombatdev/", "wombatdev", ""))
	views, err := e.sourcing.Leads(e.initiative, "")
	if err != nil {
		t.Fatalf("leads: %v", err)
	}
	for _, v := range views {
		if v.ID == again[0] && (v.CandidateName != "Wombat Developer" || v.Host != "github.com") {
			t.Fatalf("recognised lead = %+v", v)
		}
	}
}

func TestSuggestTakesTheNameFromTheTitle(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	ids := e.leads(t, roleID,
		person("p1", "https://quokka.example.invalid/about", "Quokka Stack — platform engineer, Melbourne", ""))
	s, err := e.sourcing.Suggest(ids[0])
	if err != nil || s.FullName != "Quokka Stack" {
		t.Fatalf("suggested %+v, err = %v", s, err)
	}
	// A page with no handle gets a website identity on promotion.
	c, err := e.sourcing.Promote(ids[0], *s)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	var identity models.Identity
	e.db.Where("candidate_id = ?", c.ID).First(&identity)
	if identity.Provider != models.IdentityWebsite || identity.Handle != "quokka.example.invalid" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestAPromotionThatFailsLeavesNothingBehind(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	ids := e.leads(t, roleID, person("p1", "https://quokka.example.invalid/about", "Quokka", ""))
	if _, err := e.sourcing.Promote(ids[0], models.Candidate{FullName: "  "}); err == nil {
		t.Fatal("a nameless candidate was created")
	}
	var candidates, artifacts, identities int64
	e.db.Model(&models.Candidate{}).Count(&candidates)
	e.db.Model(&models.Artifact{}).Where("source LIKE 'exa:%'").Count(&artifacts)
	e.db.Model(&models.Identity{}).Count(&identities)
	if candidates+artifacts+identities != 0 {
		t.Fatalf("left behind: %d candidates, %d artifacts, %d identities", candidates, artifacts, identities)
	}
}

func TestDismissedLeadsLeaveTheNewList(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	ids := e.leads(t, roleID,
		person("p1", "https://quokka.example.invalid/about", "Quokka", ""),
		person("p2", "https://github.com/wombatdev", "wombatdev", ""))
	if err := e.sourcing.Dismiss(ids[0]); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	fresh, err := e.sourcing.Leads(e.initiative, models.LeadNew)
	if err != nil || len(fresh) != 1 || fresh[0].ID != ids[1] {
		t.Fatalf("new leads = %+v, err = %v", fresh, err)
	}
	all, _ := e.sourcing.Leads(e.initiative, "")
	if len(all) != 2 {
		t.Fatalf("%d leads in total", len(all))
	}
	// Dismissing is not deleting: promote refuses it.
	if _, err := e.sourcing.Promote(ids[0], models.Candidate{FullName: "Quokka"}); err == nil {
		t.Fatal("a dismissed lead was promoted")
	}
}

func TestDeletingACandidateRemovesIdentitiesAndKeepsLeads(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	ids := e.leads(t, roleID, person("p2", "https://github.com/wombatdev", "wombatdev", ""))
	c, err := e.sourcing.Promote(ids[0], models.Candidate{FullName: "Wombat Developer"})
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	deletion := NewDeletionService(e.db)
	preview, err := deletion.PreviewCandidate(c.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	found := false
	for _, r := range preview.Removes {
		if r.Kind == "identities" && r.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("the preview does not mention the identity: %+v", preview.Removes)
	}
	if err := deletion.DeleteCandidate(c.ID, ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var identities int64
	e.db.Model(&models.Identity{}).Count(&identities)
	if identities != 0 {
		t.Fatalf("%d identities survived", identities)
	}
	var lead models.Lead
	e.db.First(&lead, ids[0])
	if lead.CandidateID != nil {
		t.Fatalf("the lead still points at a deleted candidate: %+v", lead)
	}

	// The initiative's leads go with the initiative; a purged role leaves its
	// leads with no role.
	if err := deletion.PurgeRole(roleID); err != nil {
		t.Fatalf("purge role: %v", err)
	}
	e.db.First(&lead, ids[0])
	if lead.RoleID != nil {
		t.Fatalf("the lead still points at a purged role")
	}
	if err := deletion.DeleteInitiative(e.initiative); err != nil {
		t.Fatalf("delete initiative: %v", err)
	}
	var leads int64
	e.db.Model(&models.Lead{}).Count(&leads)
	if leads != 0 {
		t.Fatalf("%d leads survived the initiative", leads)
	}
}
