//go:build livemodel

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// Choosing the retrieval constants, on data the frozen corpus never sees.
//
// The PRD says to tune only on separate non-held-out data. This sweeps the
// fusion constant and the per-query depth over the tuning corpus and reports
// what each scores, so the choice is made where it is allowed to be made and
// then measured, once, by `just bench`.
//
//	just tune    (TH_CLASSIFY_MODEL=<name> TH_EMBED_MODEL=<name>)
func TestTuneRetrieval(t *testing.T) {
	classifyModel := os.Getenv("TH_CLASSIFY_MODEL")
	embedModel := os.Getenv("TH_EMBED_MODEL")
	if classifyModel == "" || embedModel == "" {
		t.Skip("set TH_CLASSIFY_MODEL and TH_EMBED_MODEL")
	}
	corpus, err := bench.LoadTuning()
	if err != nil {
		t.Fatalf("loading the tuning corpus: %v", err)
	}

	// K is the lever the provenance pointed at. Reciprocal rank fusion divides
	// by K plus the rank, so against twenty roles a K of sixty scores rank one
	// at 1/61 and rank twenty at 1/80 — a spread of under a third across the
	// whole corpus, which is less than one lexical hit is worth. Small values
	// are in the sweep because that arithmetic says they should be.
	depths := []int{3, 5, 10, 20, 30}
	ks := []int{1, 2, 5, 10, 20, 60}

	// One workspace holding every role, and a candidate per scenario in it.
	// Decomposing twenty listings once per scenario is sixty classifications
	// for a set of constants that does not depend on which candidate is asking.
	type prepared struct {
		scenario    bench.Scenario
		candidateID uint
	}
	e := newShortlistEnv(t)
	live := NewClassifyService(e.db, e.registry, platform.NewOllama())
	e.classify = live
	e.profiles = NewCandidateProfileService(e.db, live, e.records)
	e.roles = NewRoleProfileService(e.db, live)
	e.shortlist = NewShortlistService(e.db, e.search, e.embed, e.criteria, e.profiles, e.roles)
	e.assignClassify(t, classifyModel)
	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: embedModel}); err != nil {
		t.Fatalf("assigning the embedding model: %v", err)
	}

	byRole := map[uint]string{}
	for _, listing := range corpus.Listings {
		// Decomposed by the model, not a hand-built aspect holding the title: a
		// constant swept against twenty one-aspect profiles is a constant
		// chosen for a corpus the product never has.
		byRole[classifiedRole(t, e, listing)] = listing.ID
	}

	ready := []prepared{}
	for _, scenario := range corpus.Scenarios {
		candidateID, note := scenarioCandidate(t, e, scenario)
		if note != "" {
			t.Logf("%s: %s", scenario.ID, note)
			continue
		}
		ready = append(ready, prepared{scenario: scenario, candidateID: candidateID})
	}

	// The aspects have to be embedded before the similarity half can retrieve
	// anything, and every candidate is classified before that call so the
	// classify model is not evicted and reloaded between them.
	if job, err := e.embed.EmbedAspects(e.initiative); err != nil {
		t.Fatalf("embedding aspects: %v", err)
	} else if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("aspect embedding is %s (%q)", done.State, done.FailureReason)
	}

	if len(ready) == 0 {
		t.Skip("no tuning scenario produced a profile")
	}

	type result struct {
		depth, k  int
		plausible int
		met       int
	}
	results := []result{}
	for _, depth := range depths {
		for _, k := range ks {
			tops := []bench.TopFive{}
			for _, p := range ready {
				e.shortlist.Depth = depth
				e.shortlist.FusionK = k
				list, err := e.shortlist.Build(e.initiative, p.candidateID)
				if err != nil {
					t.Fatalf("%s: building: %v", p.scenario.ID, err)
				}
				top := bench.TopFive{ScenarioID: p.scenario.ID, RoleIDs: []string{}}
				for _, entry := range list.Entries {
					if len(top.RoleIDs) == 5 {
						break
					}
					top.RoleIDs = append(top.RoleIDs, byRole[entry.RoleID])
				}
				tops = append(tops, top)
			}
			score := bench.ScoreMatching(corpus, tops)
			total := 0
			for _, s := range score.Scenarios {
				total += s.Plausible
			}
			results = append(results, result{depth: depth, k: k, plausible: total, met: score.MetCount})
			t.Logf("EVIDENCE tuning depth=%d k=%d plausible=%d met=%d/%d",
				depth, k, total, score.MetCount, len(ready))
		}
	}

	// Most plausible overall, then most scenarios meeting the bar, then the
	// larger constants — a tie goes to what the product already ships.
	sort.Slice(results, func(a, b int) bool {
		if results[a].plausible != results[b].plausible {
			return results[a].plausible > results[b].plausible
		}
		if results[a].met != results[b].met {
			return results[a].met > results[b].met
		}
		return results[a].k > results[b].k
	})
	best := results[0]
	t.Logf("EVIDENCE tuning best depth=%d k=%d plausible=%d met=%d",
		best.depth, best.k, best.plausible, best.met)

	var b strings.Builder
	fmt.Fprintf(&b, "Retrieval tuning on the tuning corpus, %d scenarios, %d roles\n",
		len(ready), len(corpus.Listings))
	fmt.Fprintf(&b, "classify %s, embed %s\n\n", classifyModel, embedModel)
	for _, r := range results {
		fmt.Fprintf(&b, "  depth %2d  k %2d  plausible %2d  met %d\n", r.depth, r.k, r.plausible, r.met)
	}
	fmt.Fprintf(&b, "\nbest: depth %d, k %d\n", best.depth, best.k)
	path := filepath.Join("docs", "product", "benchmarks",
		fmt.Sprintf("tuning-%s.txt", strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339), ":", "-")))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("creating the folder: %v", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	t.Logf("recorded %s", path)
}
