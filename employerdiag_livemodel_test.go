//go:build livemodel

package main

import (
	"os"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/platform"
)

// Whether the words that connect a candidate to a role survive into the aspects
// the shortlist searches with.
//
// embedded-c-perth works at Redgum Mining Tech in Perth. staff-engineer-perth is
// a role at Redgum Mining Tech in Perth whose nice-to-have is mining domain
// experience. The recruiter calls it plausible on four facts both documents
// state, and the shipped ranking does not surface it at all.
//
//	go test -tags livemodel -run TestWhatSurvivesIntoTheQueries -v -timeout 30m .
func TestWhatSurvivesIntoTheQueries(t *testing.T) {
	model := os.Getenv("TH_CLASSIFY_MODEL")
	if model == "" {
		t.Skip("set TH_CLASSIFY_MODEL")
	}
	corpus, err := bench.Load()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	var scenario bench.Scenario
	for _, s := range corpus.Scenarios {
		if s.ID == "embedded-c-perth" {
			scenario = s
		}
	}

	e := newShortlistEnv(t)
	live := NewClassifyService(e.db, e.registry, platform.NewOllama())
	e.classify = live
	e.profiles = NewCandidateProfileService(e.db, live, e.records)
	e.roles = NewRoleProfileService(e.db, live)
	e.shortlist = NewShortlistService(e.db, e.search, e.embed, e.criteria, e.profiles, e.roles)
	e.assignClassify(t, model)

	candidateID, note := scenarioCandidate(t, e, scenario)
	if note != "" {
		t.Fatalf("no profile: %s", note)
	}
	approved, err := e.profiles.Approved(candidateID)
	if err != nil || approved == nil {
		t.Fatalf("reading the approved profile: %v", err)
	}
	for _, a := range approved.Aspects {
		t.Logf("aspect %-16s %q", a.Type, a.Wording)
	}

	queries, err := e.shortlist.queries(e.initiative, candidateID)
	if err != nil {
		t.Fatalf("building queries: %v", err)
	}
	all := strings.ToLower(scenario.Resume)
	searched := strings.Builder{}
	for _, q := range queries {
		t.Logf("query %q", q.text)
		searched.WriteString(strings.ToLower(q.text) + " ")
	}

	// The specific words that connect this candidate to the role the recruiter
	// called plausible and the ranking missed.
	for _, word := range []string{"redgum", "mining", "perth", "conveyor", "can", "safety"} {
		t.Logf("%-9s in resume=%v in what the shortlist searches with=%v",
			word, strings.Contains(all, word), strings.Contains(searched.String(), word))
	}
}
