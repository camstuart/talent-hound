package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real person appears in these tests.

// fakePeopleExa records exactly what it was asked and answers however the test
// says. The interesting assertion is what went out, not what came back.
type fakePeopleExa struct {
	mu        sync.Mutex
	queries   []string
	responses []*platform.SearchResponse
	err       error
	calls     int
}

func (f *fakePeopleExa) SearchPeople(_ context.Context, query string, _ int, _ string) (*platform.SearchResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.queries = append(f.queries, query)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.responses) == 0 {
		return &platform.SearchResponse{}, nil
	}
	return f.responses[min(f.calls-1, len(f.responses)-1)], nil
}

type sourcingEnv struct {
	*roleEnv
	exa        *fakePeopleExa
	sourcing   *SourcingService
	initiative uint
	clock      time.Time
}

func newSourcingEnv(t *testing.T) *sourcingEnv {
	t.Helper()
	base := newRoleEnv(t)
	initiative, err := NewInitiativeService(base.db).Create("Sourcing", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating initiative: %v", err)
	}
	e := &sourcingEnv{
		roleEnv: base, exa: &fakePeopleExa{}, initiative: initiative.ID,
		clock: time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC),
	}
	e.sourcing = NewSourcingService(base.db, e.exa, base.roles, base.records, base.artifacts, nil)
	e.sourcing.now = func() time.Time { return e.clock }
	return e
}

// readyRole creates a role at a named client with a contact, and a ready
// profile whose wording names both — the things that must not leave.
func (e *sourcingEnv) readyRole(t *testing.T) uint {
	t.Helper()
	company, err := e.records.CreateCompany(models.Company{Name: "Northwind Pty Ltd"})
	if err != nil {
		t.Fatalf("creating company: %v", err)
	}
	if _, err := e.records.CreateContact(models.Contact{
		CompanyID: company.ID, FullName: "Priya Okonkwo", Email: "priya@northwind.invalid",
	}); err != nil {
		t.Fatalf("creating contact: %v", err)
	}
	roleID, _ := e.withListing(t)
	role, err := e.records.GetRole(roleID)
	if err != nil {
		t.Fatalf("reading role: %v", err)
	}
	role.CompanyID = &company.ID
	role.CompanyName = "Northwind Pty Ltd"
	if _, err := e.records.UpdateRole(*role); err != nil {
		t.Fatalf("attaching company: %v", err)
	}
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "strong Go and production SQLite experience",
			Priority: profile.MustHave, Citations: e.citing(t, "Go and production SQLite")},
		{Type: profile.Experience, Wording: "report to Priya Okonkwo at Northwind Pty Ltd",
			Priority: profile.NiceToHave, Citations: e.citing(t, "head of engineering")},
		{Type: profile.Other, Wording: "experience operating multi-region systems", Priority: profile.NiceToHave,
			Citations: e.citing(t, "operating multi-region systems")},
		{Type: profile.Compensation, Wording: "AUD 180,000 base", Citations: e.citing(t, "AUD 180,000 base")},
	})}
	if _, err := e.roles.Profile(roleID); err != nil {
		t.Fatalf("profiling: %v", err)
	}
	return roleID
}

func person(id, url, title, text string) platform.SearchResult {
	return platform.SearchResult{SourceID: id, URL: url, Title: title, Text: text}
}

func (e *sourcingEnv) send(t *testing.T, roleID uint, query string) *SourcingOutcome {
	t.Helper()
	out, err := e.sourcing.Send(SourcingSendInput{
		InitiativeID: e.initiative, RoleID: roleID, Query: query, Limit: 10,
	})
	if err != nil {
		t.Fatalf("sending %q: %v", query, err)
	}
	return out
}

func TestAPeopleQueryIsBuiltFromTheRoleProfileWithoutTheClient(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)

	preview, err := e.sourcing.Preview(roleID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !strings.Contains(preview.Query, "Go") || !strings.Contains(preview.Query, "SQLite") || !strings.Contains(preview.Query, "multi-region") {
		t.Fatalf("the requirements are missing from %q", preview.Query)
	}
	if strings.Contains(preview.Query, "180,000") {
		t.Fatalf("compensation is not a requirement to search with: %q", preview.Query)
	}
	for _, secret := range []string{"Northwind", "Priya", "Okonkwo", "northwind.invalid"} {
		if strings.Contains(preview.Query, secret) {
			t.Fatalf("%q names the client: %q", preview.Query, secret)
		}
	}
	if preview.IdentifierWarning != "" || preview.OrganizationWarning != "" {
		t.Fatalf("a generated query warns: %q / %q", preview.IdentifierWarning, preview.OrganizationWarning)
	}
	if e.exa.calls != 0 {
		t.Fatalf("a preview made %d requests", e.exa.calls)
	}

	// Typing the client back in is allowed, and warned about.
	edited, err := e.sourcing.Inspect(roleID, preview.Query+" who worked with Priya Okonkwo")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if edited.IdentifierWarning == "" {
		t.Fatalf("no warning for a contact's name in %q", edited.Query)
	}
}

func TestAPeopleQueryNeedsAReadyRoleProfile(t *testing.T) {
	e := newSourcingEnv(t)
	roleID, _ := e.withListing(t)
	if _, err := e.sourcing.Preview(roleID); err == nil || !strings.Contains(err.Error(), "ready role profile") {
		t.Fatalf("err = %v", err)
	}
}

func TestARefusedPeopleSearchIsRecordedWithoutADisclosure(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	e.sourcing.exa = nil
	e.sourcing.out.credentials = &CredentialService{store: newMemoryStore()}

	_, err := e.sourcing.Send(SourcingSendInput{InitiativeID: e.initiative, RoleID: roleID, Query: "Go engineers"})
	if err == nil {
		t.Fatal("a search with no credential was sent")
	}
	searches, err := e.sourcing.Searches(e.initiative)
	if err != nil || len(searches) != 1 || searches[0].FailureReason != models.ReasonUnauthorized {
		t.Fatalf("searches = %+v, err = %v", searches, err)
	}
	var disclosures int64
	e.db.Model(&models.DisclosureEvent{}).Count(&disclosures)
	if disclosures != 0 {
		t.Fatalf("%d disclosures recorded for a request that never left", disclosures)
	}
}

func TestASentPeopleSearchIsDisclosedAsAKindAndKeptAsLeads(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	e.exa.responses = []*platform.SearchResponse{{Results: []platform.SearchResult{
		person("p1", "https://quokka.example.invalid/about", "Quokka Stack — platform engineer", "Go, SQLite, desktop apps."),
		person("p2", "https://github.com/wombatdev", "wombatdev", "Builds local-first tools."),
		person("p1", "https://quokka.example.invalid/about", "Quokka Stack — platform engineer", "same page twice"),
		person("", "", "nothing to identify", ""),
	}}}
	const query = "Go and production SQLite experience, desktop applications"

	out := e.send(t, roleID, query)
	if e.exa.queries[0] != query {
		t.Fatalf("sent %q, confirmed %q", e.exa.queries[0], query)
	}
	if out.Created != 2 || len(out.LeadIDs) != 2 || out.Skipped != 1 || !out.Partial {
		t.Fatalf("outcome = %+v", out)
	}
	var leads int64
	e.db.Model(&models.Lead{}).Where("search_id = ?", out.SearchID).Count(&leads)
	if leads != 2 {
		t.Fatalf("%d leads kept for two distinct pages", leads)
	}

	// One disclosure, and the row holds kinds only — never the query, the
	// client, or a result.
	var rows []map[string]any
	e.db.Raw("SELECT * FROM disclosure_events").Scan(&rows)
	if len(rows) != 1 {
		t.Fatalf("%d disclosure rows", len(rows))
	}
	flat := ""
	for _, v := range rows[0] {
		if s, ok := v.(string); ok {
			flat += " " + s
		}
	}
	for _, secret := range []string{"SQLite", "Northwind", "Priya", "quokka", "wombat"} {
		if strings.Contains(flat, secret) {
			t.Fatalf("the disclosure row carries %q: %s", secret, flat)
		}
	}
	if rows[0]["task"] != models.TaskCandidateSourcing || rows[0]["categories"] != "professional requirements" {
		t.Fatalf("task/categories = %v / %v", rows[0]["task"], rows[0]["categories"])
	}
	if fmt.Sprint(rows[0]["role_id"]) != fmt.Sprint(roleID) {
		t.Fatalf("role_id = %v", rows[0]["role_id"])
	}

	// Sending again replaces rather than duplicates.
	e.clock = e.clock.Add(time.Hour)
	again := e.send(t, roleID, query)
	e.db.Model(&models.Lead{}).Where("search_id = ?", again.SearchID).Count(&leads)
	if leads != 2 {
		t.Fatalf("%d leads on the second search", leads)
	}
}

func TestAnEditedQueryNamingTheClientIsDisclosedAsSuch(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	e.send(t, roleID, "Go engineers who report to Priya Okonkwo")
	var categories sql.NullString
	e.db.Raw("SELECT categories FROM disclosure_events").Scan(&categories)
	if !strings.Contains(categories.String, "a direct identifier") {
		t.Fatalf("categories = %q", categories.String)
	}
}

func TestALeadForSomeoneAlreadyInThePoolSaysSo(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Wombat Dev"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	if err := e.db.Create(&models.Identity{
		CandidateID: c.ID, Provider: models.IdentityGitHub, Handle: "wombatdev", URL: "https://github.com/wombatdev",
	}).Error; err != nil {
		t.Fatalf("creating identity: %v", err)
	}
	e.exa.responses = []*platform.SearchResponse{{Results: []platform.SearchResult{
		// A different spelling of the same login, and a stranger.
		person("p2", "https://github.com/WombatDev/", "WombatDev", ""),
		person("p3", "https://quokka.example.invalid/about", "Quokka", ""),
	}}}
	out := e.send(t, roleID, "Go engineers")
	if out.AlreadyInPool != 1 || out.Created != 2 {
		t.Fatalf("outcome = %+v", out)
	}
	var lead models.Lead
	if err := e.db.Where("url LIKE ?", "https://github.com/WombatDev%").First(&lead).Error; err != nil {
		t.Fatalf("finding lead: %v", err)
	}
	if lead.CandidateID == nil || *lead.CandidateID != c.ID {
		t.Fatalf("lead not linked to the candidate: %+v", lead)
	}
}

func TestPeopleSearchesAndRoleSearchesAreListedApart(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	e.send(t, roleID, "Go engineers")
	if err := e.db.Create(&models.Search{
		InitiativeID: e.initiative, Provider: models.ProviderExa, Task: models.TaskRoleSearch,
		Query: "platform engineer roles", SentAt: e.clock,
	}).Error; err != nil {
		t.Fatalf("creating role search: %v", err)
	}
	people, err := e.sourcing.Searches(e.initiative)
	if err != nil || len(people) != 1 || people[0].Task != models.TaskCandidateSourcing {
		t.Fatalf("people searches = %+v, err = %v", people, err)
	}
	roles, err := NewDiscoveryService(e.db, nil, nil, nil, e.records, e.artifacts, nil).Searches(e.initiative)
	if err != nil || len(roles) != 1 || roles[0].Task != models.TaskRoleSearch {
		t.Fatalf("role searches = %+v, err = %v", roles, err)
	}
}
