//go:build livemodel

package main

import (
	"os"
	"testing"
	"time"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// One match assessed end to end with the live models, against the PRD's 60
// second target.
//
// The benchmark proper does not call the generate model at all, so this target
// had no measurement. It is separate because assessment is a different model
// role with a different cost, and folding it into a two-hour run would hide it.
//
//	TH_CLASSIFY_MODEL=... TH_GENERATE_MODEL=... go test -tags livemodel -run TestAssessOneMatch -v -timeout 60m .
func TestAssessOneMatch(t *testing.T) {
	classifyModel := os.Getenv("TH_CLASSIFY_MODEL")
	generateModel := os.Getenv("TH_GENERATE_MODEL")
	if classifyModel == "" || generateModel == "" {
		t.Skip("set TH_CLASSIFY_MODEL and TH_GENERATE_MODEL")
	}
	corpus, err := bench.Load()
	if err != nil {
		t.Fatalf("loading the corpus: %v", err)
	}

	base := newShortlistEnv(t)
	live := NewClassifyService(base.db, base.registry, platform.NewOllama())
	base.classify = live
	base.profiles = NewCandidateProfileService(base.db, live, base.records)
	base.roles = NewRoleProfileService(base.db, live)
	base.shortlist = NewShortlistService(base.db, base.search, base.embed, base.criteria,
		base.profiles, base.roles)
	e := &assessEnv{shortlistEnv: base, assess: NewAssessService(base.db, base.jobs,
		base.registry, platform.NewOllama(), base.embed, base.criteria, base.profiles,
		base.roles, base.shortlist)}

	e.assignClassify(t, classifyModel)
	if _, err := e.registry.Assign(AssignInput{
		Role: models.RoleGenerate, Model: generateModel,
	}); err != nil {
		t.Fatalf("assigning the generate model: %v", err)
	}

	// A real listing and a real resume, decomposed by the model, so the
	// assessment reasons over the evidence the product would give it.
	listing := corpus.Listings[0]
	roleID := classifiedRole(t, e.shortlistEnv, listing)
	candidateID, note := scenarioCandidate(t, e.shortlistEnv, corpus.Scenarios[0])
	if note != "" {
		t.Logf("candidate: %s", note)
	}

	started := time.Now()
	match, err := e.assess.Assess(e.initiative, candidateID, roleID)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("assessing after %.1f s: %v", elapsed.Seconds(), err)
	}

	t.Logf("one match assessed: %.2f s (target 60.00, met %v) — %d results over "+
		"%s against %s, one generate call per result, darwin/arm64, models resident",
		elapsed.Seconds(), elapsed.Seconds() <= 60, len(match.Results), listing.ID,
		corpus.Scenarios[0].ID)
	for _, r := range match.Results {
		t.Logf("  %s %s: %s", r.Direction, r.Result, r.Requirement)
	}
}
