package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// fakeExa records exactly what it was asked and answers however the test says.
//
// The interesting assertion in this suite is not what comes back — it is what
// went out, byte for byte, and whether anything went out at all.
type fakeExa struct {
	mu sync.Mutex
	// queries is every query actually received, in order. A cancelled preview
	// must leave this empty.
	queries   []string
	responses []*platform.SearchResponse
	err       error
	calls     int
}

func (f *fakeExa) Search(_ context.Context, query string, _ int, _ string) (*platform.SearchResponse, error) {
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
	i := min(f.calls-1, len(f.responses)-1)
	return f.responses[i], nil
}

func (f *fakeExa) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.queries...)
}

// discoveryEnv is a criteriaEnv with discovery wired in.
type discoveryEnv struct {
	*criteriaEnv
	exa       *fakeExa
	discovery *DiscoveryService
	clock     time.Time
}

func newDiscoveryEnv(t *testing.T) *discoveryEnv {
	t.Helper()
	base := newCriteriaEnv(t)
	exa := &fakeExa{}
	e := &discoveryEnv{
		criteriaEnv: base,
		exa:         exa,
		clock:       time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
	}
	e.discovery = NewDiscoveryService(base.db, exa, base.profiles, base.criteria,
		base.records, base.artifacts, nil)
	// A clock the test moves, so the thirty-day boundary is an assertion rather
	// than a wait.
	e.discovery.now = func() time.Time { return e.clock }
	return e
}

// result builds one listing the fake provider returns.
func result(sourceID, url, title, text string) platform.SearchResult {
	return platform.SearchResult{SourceID: sourceID, URL: url, Title: title, Text: text}
}

func (e *discoveryEnv) answer(results ...platform.SearchResult) {
	e.exa.responses = []*platform.SearchResponse{{Results: results}}
}

func (e *discoveryEnv) send(t *testing.T, query string) *SearchOutcome {
	t.Helper()
	out, err := e.discovery.Send(SendInput{
		InitiativeID: e.initiative, Query: query, Limit: 10,
	})
	if err != nil {
		t.Fatalf("sending %q: %v", query, err)
	}
	return out
}

// approvedCandidate creates a candidate with identifiers and an approved
// profile naming an employer and a school.
func (e *discoveryEnv) approvedCandidate(t *testing.T) uint {
	t.Helper()
	c, err := e.records.CreateCandidate(models.Candidate{
		FullName: "Kalinda Reyes",
		Emails:   models.StringList{"kalinda.reyes@example.invalid"},
		Phones:   models.StringList{"+61 400 123 456"},
		Location: "12 Wattle Street, Fitzroy",
	})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	md := "# Kalinda Reyes\n\n## Experience\n\n" +
		"Senior platform engineer at Northwind Pty Ltd. Go and SQLite in production.\n"
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
	var chunkID uint
	for _, ch := range e.chunks2 {
		if strings.Contains(ch.Text, "Northwind") {
			chunkID = ch.ID
		}
	}
	if chunkID == 0 {
		t.Fatal("no chunk names the employer")
	}
	cite := []profile.Citation{{ChunkID: chunkID, Quote: "Senior platform engineer at Northwind"}}
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Seniority, Wording: "Senior platform engineer at Northwind Pty Ltd", Citations: cite},
		{Type: profile.Skill, Wording: "Go and SQLite in production", Citations: cite},
	})}
	p, err := e.profiles.Classify(c.ID)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}
	return c.ID
}

func TestAGeneratedQueryCarriesNoIdentifierAndNoEmployer(t *testing.T) {
	e := newDiscoveryEnv(t)
	id := e.approvedCandidate(t)

	preview, err := e.discovery.Preview(e.initiative, id)
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	for _, secret := range []string{
		"Kalinda", "Reyes", "kalinda.reyes@example.invalid", "400 123 456",
		"Wattle Street", "Northwind",
	} {
		if strings.Contains(preview.Query, secret) {
			t.Errorf("the generated query contains %q: %q", secret, preview.Query)
		}
	}
	// The professional shape survives, which is what makes it a search.
	if !strings.Contains(preview.Query, "Senior platform engineer") {
		t.Errorf("the query lost the professional description: %q", preview.Query)
	}
	// And the default warns about neither, because the default is already safe.
	if preview.OrganizationWarning != "" || preview.IdentifierWarning != "" {
		t.Errorf("a generated query warned: %q / %q",
			preview.OrganizationWarning, preview.IdentifierWarning)
	}
}

func TestPreviewingSendsNothing(t *testing.T) {
	e := newDiscoveryEnv(t)
	id := e.approvedCandidate(t)

	if _, err := e.discovery.Preview(e.initiative, id); err != nil {
		t.Fatalf("previewing: %v", err)
	}
	if got := e.exa.sent(); len(got) != 0 {
		t.Fatalf("previewing sent %d requests: %v", len(got), got)
	}
	// A cancelled preview is the absence of the operation: no search record,
	// and — the important one — no disclosure event for a disclosure that did
	// not happen.
	var searches, events int64
	if err := e.db.Model(&models.Search{}).Count(&searches).Error; err != nil {
		t.Fatalf("counting searches: %v", err)
	}
	if err := e.db.Model(&models.DisclosureEvent{}).Count(&events).Error; err != nil {
		t.Fatalf("counting events: %v", err)
	}
	if searches != 0 || events != 0 {
		t.Fatalf("previewing recorded %d searches and %d disclosure events", searches, events)
	}
}

// A preview generated by one code path and a request built by another will
// diverge, usually the day someone adds a default filter.
func TestThePreviewedQueryIsTheSentQueryByteForByte(t *testing.T) {
	e := newDiscoveryEnv(t)
	id := e.approvedCandidate(t)
	preview, err := e.discovery.Preview(e.initiative, id)
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go and SQLite."))
	e.send(t, preview.Query)

	sent := e.exa.sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d requests", len(sent))
	}
	if sent[0] != preview.Query {
		t.Fatalf("the provider received %q, the recruiter confirmed %q", sent[0], preview.Query)
	}
}

func TestAnEditedQueryIsSentExactlyAsEdited(t *testing.T) {
	e := newDiscoveryEnv(t)
	edited := "platform engineer roles in Melbourne with Go, hybrid"
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go."))
	e.send(t, edited)

	sent := e.exa.sent()
	if len(sent) != 1 || sent[0] != edited {
		t.Fatalf("the provider received %v, the recruiter wrote %q", sent, edited)
	}
	// And the search record reproduces it.
	searches, err := e.discovery.Searches(e.initiative)
	if err != nil {
		t.Fatalf("listing searches: %v", err)
	}
	if len(searches) != 1 || searches[0].Query != edited {
		t.Fatalf("the search record holds %+v", searches)
	}
}

func TestDeliberatelyReAddedSpecificsWarnDistinctly(t *testing.T) {
	e := newDiscoveryEnv(t)
	id := e.approvedCandidate(t)

	// An organization is a legitimate thing to search for: warn, do not refuse.
	org, err := e.discovery.Inspect(id, "platform engineer roles at Northwind Pty Ltd")
	if err != nil {
		t.Fatalf("inspecting: %v", err)
	}
	if org.OrganizationWarning == "" {
		t.Error("a re-added organization produced no warning")
	}
	if org.IdentifierWarning != "" {
		t.Errorf("an organization produced an identifier warning: %q", org.IdentifierWarning)
	}

	// A direct identifier is the serious case, and warns additionally.
	ident, err := e.discovery.Inspect(id, "platform engineer roles for Kalinda Reyes")
	if err != nil {
		t.Fatalf("inspecting: %v", err)
	}
	if ident.IdentifierWarning == "" {
		t.Fatal("a re-added direct identifier produced no warning")
	}
	if ident.IdentifierWarning == org.OrganizationWarning {
		t.Error("the two warnings are the same message")
	}

	// Warned or not, a deliberate human choice can still be sent.
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Role", "text"))
	e.send(t, "platform engineer roles at Northwind Pty Ltd")
	if sent := e.exa.sent(); len(sent) != 1 {
		t.Fatalf("a warned query was not sent: %v", sent)
	}
}

func TestASentSearchStoresItsQueryAndItsEventStoresNoContent(t *testing.T) {
	e := newDiscoveryEnv(t)
	const query = "platform engineer roles in Melbourne with Go"
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer",
		"We need someone who has run multi-region systems."))
	e.send(t, query)

	searches, err := e.discovery.Searches(e.initiative)
	if err != nil {
		t.Fatalf("listing searches: %v", err)
	}
	if len(searches) != 1 || searches[0].Query != query {
		t.Fatalf("the search record does not reproduce the query: %+v", searches)
	}

	events, err := e.discovery.Disclosures()
	if err != nil {
		t.Fatalf("listing events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d disclosure events for one request", len(events))
	}
	event := events[0]
	if event.Provider != models.ProviderExa || event.Task != models.TaskRoleSearch {
		t.Errorf("the event does not name the provider and task: %+v", event)
	}
	if event.InitiativeID == nil || *event.InitiativeID != e.initiative {
		t.Errorf("the event does not name the initiative: %+v", event)
	}

	// The whole point of the table: it records that, not what. Scanned as a
	// whole row rather than field by field, so a column added later is caught.
	blob := fmt.Sprintf("%+v", event)
	for _, content := range []string{query, "multi-region", "Melbourne"} {
		if strings.Contains(blob, content) {
			t.Fatalf("the disclosure event contains %q: %s", content, blob)
		}
	}
}

func TestEveryProviderFailureIsReportedAsItself(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"a rate limit", platform.ErrSearchRateLimited, models.ReasonRateLimited},
		{"a timeout", platform.ErrSearchTimeout, models.ReasonSearchTimeout},
		{"being offline", platform.ErrSearchOffline, models.ReasonOffline},
		{"a rejected key", platform.ErrSearchUnauthorized, models.ReasonUnauthorized},
		{"nonsense", platform.ErrSearchMalformed, models.ReasonMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newDiscoveryEnv(t)
			e.exa.err = c.err

			_, err := e.discovery.Send(SendInput{
				InitiativeID: e.initiative, Query: "platform engineer", Limit: 10,
			})
			if err == nil {
				t.Fatalf("%s was reported as success", c.name)
			}
			searches, listErr := e.discovery.Searches(e.initiative)
			if listErr != nil {
				t.Fatalf("listing: %v", listErr)
			}
			if len(searches) != 1 {
				t.Fatalf("got %d search records", len(searches))
			}
			if searches[0].FailureReason != c.want {
				t.Errorf("recorded reason %q, want %q", searches[0].FailureReason, c.want)
			}
			// A failure is not an empty result: no roles were invented.
			if searches[0].ResultCount != 0 {
				t.Errorf("a failed search recorded %d results", searches[0].ResultCount)
			}
			// The request was transmitted, so the disclosure happened.
			var events int64
			if err := e.db.Model(&models.DisclosureEvent{}).Count(&events).Error; err != nil {
				t.Fatalf("counting: %v", err)
			}
			if events != 1 {
				t.Errorf("a transmitted request that then failed recorded %d events", events)
			}
		})
	}
}

func TestDuplicatesInOneResponseCollapseToOneRole(t *testing.T) {
	e := newDiscoveryEnv(t)
	listing := result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go and SQLite.")
	e.answer(listing, listing, listing)

	out := e.send(t, "platform engineer")
	if len(out.RoleIDs) != 1 {
		t.Fatalf("three copies of one listing produced %d roles", len(out.RoleIDs))
	}
	var roles int64
	if err := e.db.Model(&models.Role{}).Count(&roles).Error; err != nil {
		t.Fatalf("counting roles: %v", err)
	}
	if roles != 1 {
		t.Fatalf("%d roles exist", roles)
	}
}

func TestAnUnusableRecordMakesTheResponsePartialWithoutDiscardingTheRest(t *testing.T) {
	e := newDiscoveryEnv(t)
	e.exa.responses = []*platform.SearchResponse{{
		Results: []platform.SearchResult{
			result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go."),
			result("ex-2", "https://example.invalid/roles/2", "Data engineer", "dbt."),
		},
		// The client could not read one record at all.
		Skipped: 1,
	}}

	out := e.send(t, "engineer")
	if len(out.RoleIDs) != 2 {
		t.Fatalf("the usable records produced %d roles", len(out.RoleIDs))
	}
	if !out.Partial || out.Skipped != 1 {
		t.Fatalf("an incomplete response was presented as complete: %+v", out)
	}
	searches, err := e.discovery.Searches(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if !searches[0].Partial || searches[0].SkippedCount != 1 {
		t.Errorf("the search record does not say it was partial: %+v", searches[0])
	}
}

// Precedence rather than scoring: a scored match fails as two roles that are
// the same, or one role that is two.
func TestRoleIdentityFollowsTheFixedPrecedence(t *testing.T) {
	t.Run("source id matches even when the url differs", func(t *testing.T) {
		e := newDiscoveryEnv(t)
		e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go."))
		first := e.send(t, "engineer")

		e.exa.calls = 0
		e.answer(result("ex-1", "https://example.invalid/roles/1-moved", "Platform engineer", "Go."))
		second := e.send(t, "engineer")

		if second.RoleIDs[0] != first.RoleIDs[0] {
			t.Fatal("the same source id produced two roles")
		}
	})

	t.Run("canonical url matches when there is no source id", func(t *testing.T) {
		e := newDiscoveryEnv(t)
		e.answer(result("", "https://example.invalid/roles/2", "Data engineer", "dbt."))
		first := e.send(t, "engineer")

		e.exa.calls = 0
		e.answer(result("", "https://example.invalid/roles/2", "Data engineer (updated)", "dbt and Airflow."))
		second := e.send(t, "engineer")

		if second.RoleIDs[0] != first.RoleIDs[0] {
			t.Fatal("the same canonical url produced two roles")
		}
	})

	t.Run("content fingerprint matches last", func(t *testing.T) {
		e := newDiscoveryEnv(t)
		const body = "We are hiring a staff engineer for our billing platform."
		e.answer(result("", "https://example.invalid/roles/3", "Staff engineer", body))
		first := e.send(t, "engineer")

		// Different id, different url, identical content.
		e.exa.calls = 0
		e.answer(result("", "https://elsewhere.invalid/jobs/99", "Staff engineer", body))
		second := e.send(t, "engineer")

		if second.RoleIDs[0] != first.RoleIDs[0] {
			t.Fatal("identical content produced two roles")
		}
	})

	t.Run("nothing matching creates a role", func(t *testing.T) {
		e := newDiscoveryEnv(t)
		e.answer(result("ex-1", "https://example.invalid/roles/1", "One", "first body"))
		first := e.send(t, "engineer")
		e.exa.calls = 0
		e.answer(result("ex-2", "https://example.invalid/roles/2", "Two", "second body"))
		second := e.send(t, "engineer")

		if second.RoleIDs[0] == first.RoleIDs[0] {
			t.Fatal("two different listings became one role")
		}
	})

	t.Run("source id wins when the signals disagree", func(t *testing.T) {
		e := newDiscoveryEnv(t)
		e.answer(
			result("ex-A", "https://example.invalid/a", "A", "body a"),
			result("ex-B", "https://example.invalid/b", "B", "body b"),
		)
		out := e.send(t, "engineer")
		if len(out.RoleIDs) != 2 {
			t.Fatalf("setup produced %d roles", len(out.RoleIDs))
		}
		byID := out.RoleIDs[0]
		byURL := out.RoleIDs[1]

		// Source ID of the first, canonical URL of the second.
		e.exa.calls = 0
		e.answer(result("ex-A", "https://example.invalid/b", "Ambiguous", "body c"))
		again := e.send(t, "engineer")

		if again.RoleIDs[0] != byID {
			t.Fatalf("the ambiguous result resolved to %d, want the source-id match %d (url match was %d)",
				again.RoleIDs[0], byID, byURL)
		}
	})
}

func TestUnchangedContentUpdatesRetrievalAndCreatesNoArtifact(t *testing.T) {
	e := newDiscoveryEnv(t)
	listing := result("ex-1", "https://example.invalid/roles/1", "Platform engineer",
		"We need someone who has run multi-region systems.")
	e.answer(listing)
	out := e.send(t, "engineer")
	roleID := out.RoleIDs[0]

	var before int64
	if err := e.db.Model(&models.Artifact{}).Count(&before).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}

	// Seen again a week later, unchanged.
	e.clock = e.clock.Add(7 * 24 * time.Hour)
	e.exa.calls = 0
	e.answer(listing)
	e.send(t, "engineer")

	var after int64
	if err := e.db.Model(&models.Artifact{}).Count(&after).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if after != before {
		t.Fatalf("unchanged content created %d new artifacts", after-before)
	}
	var role models.Role
	if err := e.db.First(&role, roleID).Error; err != nil {
		t.Fatalf("loading role: %v", err)
	}
	if role.RetrievedAt == nil || !role.RetrievedAt.Equal(e.clock) {
		t.Fatalf("the retrieval time did not move: %v", role.RetrievedAt)
	}
}

func TestChangedContentCreatesACurrentSourceAndHistoricizesTheOld(t *testing.T) {
	e := newDiscoveryEnv(t)
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer",
		"We need someone who has run multi-region systems."))
	out := e.send(t, "engineer")
	roleID := out.RoleIDs[0]

	e.clock = e.clock.Add(24 * time.Hour)
	e.exa.calls = 0
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer",
		"Updated: we now also need Kubernetes."))
	e.send(t, "engineer")

	sources, err := e.discovery.CurrentSources(roleID)
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("got %d sources, want the current one and the historical one", len(sources))
	}
	var current, historical int
	for _, s := range sources {
		if s.Historical {
			historical++
		} else {
			current++
		}
	}
	if current != 1 || historical != 1 {
		t.Fatalf("got %d current and %d historical sources", current, historical)
	}

	// The historical one is still there — a match made against the earlier
	// listing has to still resolve its citation.
	if historical == 0 {
		t.Fatal("the previous source was deleted rather than historicized")
	}
}

func TestStalenessUsesTheSuppliedClock(t *testing.T) {
	e := newDiscoveryEnv(t)
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go."))
	out := e.send(t, "engineer")
	roleID := out.RoleIDs[0]

	// Exactly thirty days: not yet stale.
	e.clock = e.clock.Add(30 * 24 * time.Hour)
	life, err := e.discovery.Lifecycle(roleID)
	if err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if life.State != string(models.RoleActive) {
		t.Fatalf("at exactly thirty days the role is %q", life.State)
	}

	// One moment later: stale.
	e.clock = e.clock.Add(time.Second)
	life, err = e.discovery.Lifecycle(roleID)
	if err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if life.State != string(models.RoleStale) {
		t.Fatalf("past thirty days the role is %q", life.State)
	}

	// Rediscovery reactivates: the listing is demonstrably still there.
	e.exa.calls = 0
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go."))
	e.send(t, "engineer")
	life, err = e.discovery.Lifecycle(roleID)
	if err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if life.State != string(models.RoleActive) {
		t.Fatalf("after rediscovery the role is %q (%s)", life.State, life.Reason)
	}
}

func TestAPassedClosingDateMakesARoleStale(t *testing.T) {
	e := newDiscoveryEnv(t)
	listing := result("ex-1", "https://example.invalid/roles/1", "Platform engineer", "Go.")
	listing.ClosingOn = "2026-03-05"
	e.answer(listing)
	out := e.send(t, "engineer")
	roleID := out.RoleIDs[0]

	// Retrieved today, closes in four days: still active.
	life, err := e.discovery.Lifecycle(roleID)
	if err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if life.State != string(models.RoleActive) {
		t.Fatalf("before its closing date the role is %q", life.State)
	}

	// Past the closing date, and recently seen: stale anyway.
	e.clock = time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	life, err = e.discovery.Lifecycle(roleID)
	if err != nil {
		t.Fatalf("lifecycle: %v", err)
	}
	if life.State != string(models.RoleStale) {
		t.Fatalf("past its closing date the role is %q", life.State)
	}
	if !strings.Contains(life.Reason, "closing") {
		t.Errorf("the reason does not mention the closing date: %q", life.Reason)
	}
}

// Deny-by-default with an empty allowlist, and no parameter changes the answer.
func TestDirectFetchingIsRefusedForEverything(t *testing.T) {
	if allow := platform.FetchAllowlist(); len(allow) != 0 {
		t.Fatalf("the shipped allowlist is not empty: %v", allow)
	}
	e := newDiscoveryEnv(t)
	refused := []string{
		"https://www.seek.com.au/job/123",
		"https://www.linkedin.com/jobs/view/123",
		"https://careers.example.invalid/roles/1",
		"https://en.wikipedia.org/wiki/Recruitment",
		"http://localhost:8080/anything",
		"ftp://example.invalid/file",
		"not a url",
	}
	for _, target := range refused {
		t.Run(target, func(t *testing.T) {
			if err := e.discovery.Fetch(target); err == nil {
				t.Fatalf("fetching %q was permitted", target)
			}
		})
	}
}

func TestNamedSourcesAreRefusedByName(t *testing.T) {
	for _, target := range []string{
		"https://www.seek.com.au/job/1",
		"https://seek.co.nz/job/1",
		"https://linkedin.com/jobs/1",
		"https://www.linkedin.com/jobs/1",
	} {
		err := platform.FetchAllowed(target)
		if err == nil {
			t.Fatalf("%q was permitted", target)
		}
		if !strings.Contains(err.Error(), "never fetched automatically") {
			t.Errorf("%q was refused for the wrong reason: %v", target, err)
		}
	}
}

func TestPastedContentDoesNotClaimAutomatedProvenance(t *testing.T) {
	e := newDiscoveryEnv(t)
	// A listing the provider gave almost nothing for.
	e.answer(result("ex-1", "https://example.invalid/roles/1", "Platform engineer", ""))
	out := e.send(t, "engineer")
	roleID := out.RoleIDs[0]

	// Nothing to store means no artifact, rather than an empty one pretending.
	sources, err := e.discovery.CurrentSources(roleID)
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}
	if len(sources) != 0 {
		t.Fatalf("an empty listing produced %d artifacts", len(sources))
	}

	pasted, err := e.discovery.Paste(PasteInput{
		RoleID: roleID,
		Text:   "# Platform engineer\n\nWe need Go, SQLite, and multi-region experience.\n",
	})
	if err != nil {
		t.Fatalf("pasting: %v", err)
	}
	if pasted.Source != "recruiter paste" {
		t.Errorf("pasted content claims source %q, want recruiter paste", pasted.Source)
	}
	if strings.Contains(strings.ToLower(pasted.Source), "exa") {
		t.Error("pasted content claims automated provenance")
	}

	sources, err = e.discovery.CurrentSources(roleID)
	if err != nil {
		t.Fatalf("listing sources: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("pasting produced %d sources", len(sources))
	}
}

func TestPreviewingNeedsSomethingToSearchFor(t *testing.T) {
	e := newDiscoveryEnv(t)
	if _, err := e.discovery.Preview(e.initiative, 0); err == nil {
		t.Fatal("previewing with no criteria and no candidate produced a query")
	}
}

func TestPreviewingNeedsAnApprovedProfile(t *testing.T) {
	e := newDiscoveryEnv(t)
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Tobias Fenn"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	_, err = e.discovery.Preview(e.initiative, c.ID)
	if err == nil {
		t.Fatal("a query was built from an unapproved profile")
	}
	if !strings.Contains(err.Error(), "approved") {
		t.Errorf("the refusal does not name approval: %v", err)
	}
}

func TestAQueryCanBeBuiltFromCriteriaAlone(t *testing.T) {
	e := newDiscoveryEnv(t)
	e.add(t, "five years of production Go", models.CriterionMustHave)
	e.add(t, "hybrid work in Melbourne", models.CriterionNiceToHave)

	preview, err := e.discovery.Preview(e.initiative, 0)
	if err != nil {
		t.Fatalf("previewing: %v", err)
	}
	if !strings.Contains(preview.Query, "production Go") {
		t.Errorf("the query does not carry the criteria: %q", preview.Query)
	}
}

// The disclosure record says what the query actually disclosed.
//
// A generated query carries professional requirements and search criteria, and
// the record used to say exactly that for every search regardless. The recruiter
// may edit a query before sending — an organization name is allowed outright,
// and a direct identifier is permitted after a warning saying it discloses who
// the candidate is — and both were recorded as though neither had happened.
//
// This is the evidence that every non-local request was what it was meant to be.
// A record that understates what left the machine is worse than none: it is what
// somebody checks instead of looking.
func TestTheDisclosureRecordNamesWhatWasActuallySent(t *testing.T) {
	e := newDiscoveryEnv(t)
	candidateID := e.approvedCandidate(t)

	latest := func(t *testing.T) string {
		t.Helper()
		events, err := e.discovery.Disclosures()
		if err != nil {
			t.Fatalf("reading disclosures: %v", err)
		}
		if len(events) == 0 {
			t.Fatal("nothing was recorded")
		}
		return events[0].Categories
	}

	t.Run("a generated query names the two ordinary kinds and no more", func(t *testing.T) {
		if _, err := e.discovery.Send(SendInput{
			InitiativeID: e.initiative, CandidateID: candidateID,
			Query: "senior platform engineer Go SQLite Melbourne", Limit: 10,
		}); err != nil {
			t.Fatalf("sending: %v", err)
		}
		got := latest(t)
		if !strings.Contains(got, "professional requirements") || !strings.Contains(got, "search criteria") {
			t.Fatalf("categories = %q", got)
		}
		if strings.Contains(got, "identifier") || strings.Contains(got, "organization") {
			t.Fatalf("categories = %q, which claims more than was sent", got)
		}
	})

	t.Run("an edited query carrying a name says an identifier was sent", func(t *testing.T) {
		if _, err := e.discovery.Send(SendInput{
			InitiativeID: e.initiative, CandidateID: candidateID,
			Query: "Kalinda Reyes senior platform engineer", Limit: 10,
		}); err != nil {
			t.Fatalf("sending: %v", err)
		}
		got := latest(t)
		if !strings.Contains(got, "a direct identifier") {
			t.Fatalf("categories = %q, and a candidate's name was in the query", got)
		}
		// The kind, never the thing itself.
		if strings.Contains(got, "Kalinda") || strings.Contains(got, "Reyes") {
			t.Fatalf("the disclosure record quotes the identifier: %q", got)
		}
	})
}

// A search uses the credential the recruiter stored, read when the search is
// made.
//
// The shipped build constructed the search client once at start-up, with an
// empty key, and never touched it again. Search refuses a blank key before it
// makes any request — so every search this application could ever make was
// refused for a missing credential, whatever the recruiter had since entered.
// A key read at start-up is a key read before there is one.
//
// Asserted on the store rather than on the failure. A request with a stored key
// and a request with none both fail from a test machine, and they can fail the
// same way: an unreachable provider and a provider rejecting the key are one
// error apart. What separates the defect from the fix is whether the credential
// was read at all.
func TestASearchReadsTheStoredCredentialWhenItSearches(t *testing.T) {
	e := newDiscoveryEnv(t)
	store := &countingStore{memoryStore: newMemoryStore()}
	e.discovery.exa = nil
	e.discovery.credentials = &CredentialService{store: store}

	// Nothing stored: refused by name, and the store was asked.
	if _, err := e.discovery.Send(SendInput{
		InitiativeID: e.initiative, Query: "platform engineer", Limit: 10,
	}); err == nil {
		t.Fatal("a search was attempted with no stored credential")
	}
	if store.loads == 0 {
		t.Fatal("the search never asked the credential store for anything")
	}

	if err := store.Store("exa", []byte("not-a-real-key-EXA-4c19f7")); err != nil {
		t.Fatalf("storing: %v", err)
	}
	before := store.loads
	// This one goes to the real provider address and will not arrive from a
	// test. What matters is that it read the key first.
	_, _ = e.discovery.Send(SendInput{
		InitiativeID: e.initiative, Query: "platform engineer", Limit: 10,
	})
	if store.loads == before {
		t.Fatal("the search did not read the stored credential — it is using a client built without one")
	}
}

// countingStore records how often a credential was read.
type countingStore struct {
	*memoryStore
	loads int
}

func (c *countingStore) Load(purpose string) ([]byte, error) {
	c.loads++
	return c.memoryStore.Load(purpose)
}

// A search that never left the machine is recorded as an attempt and not as a
// disclosure.
//
// The recruiter pressed send, so the search history has to show what happened —
// a search that vanishes is worse than one that failed. Nothing was sent, so
// the disclosure record must not claim otherwise: it is the evidence that every
// non-local request was what it was meant to be, and an event for a request
// that never happened is the same lie as a missing one.
func TestARefusedSearchIsRecordedWithoutADisclosure(t *testing.T) {
	e := newDiscoveryEnv(t)
	e.discovery.exa = nil
	e.discovery.credentials = &CredentialService{store: newMemoryStore()}

	before, err := e.discovery.Disclosures()
	if err != nil {
		t.Fatalf("reading disclosures: %v", err)
	}
	if _, err := e.discovery.Send(SendInput{
		InitiativeID: e.initiative, Query: "platform engineer", Limit: 10,
	}); err == nil {
		t.Fatal("a search with no stored credential was accepted")
	}

	searches, err := e.discovery.Searches(e.initiative)
	if err != nil {
		t.Fatalf("reading searches: %v", err)
	}
	if len(searches) != 1 {
		t.Fatalf("%d searches recorded, want the attempt", len(searches))
	}
	if searches[0].FailureReason == "" {
		t.Fatal("the attempt was recorded without saying why it failed")
	}
	if searches[0].ResultCount != 0 {
		t.Fatalf("a search that was never sent reports %d results", searches[0].ResultCount)
	}

	after, err := e.discovery.Disclosures()
	if err != nil {
		t.Fatalf("reading disclosures: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("a disclosure was recorded for a request that was never made (%d then %d)",
			len(before), len(after))
	}
}
