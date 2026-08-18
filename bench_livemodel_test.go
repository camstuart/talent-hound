//go:build livemodel

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
)

// The two benchmarks, run against the selected local models on the target
// laptop:
//
//	just bench    (TH_CLASSIFY_MODEL=<name> TH_EMBED_MODEL=<name>)
//
// They live behind the live-model build tag with the other gates, because
// nothing about a model on a laptop belongs in a suite that has to pass on a
// machine with no models installed.
//
// The corpus is the frozen one in internal/bench/testdata, and every word of it
// is invented. The record says so, so no run of this is mistaken for a run
// against the recruiter's real placements.

func TestBenchmark(t *testing.T) {
	classifyModel := os.Getenv("TH_CLASSIFY_MODEL")
	embedModel := os.Getenv("TH_EMBED_MODEL")
	if classifyModel == "" || embedModel == "" {
		t.Skip("set TH_CLASSIFY_MODEL and TH_EMBED_MODEL to the selected local models")
	}

	corpus, err := bench.Load()
	if err != nil {
		t.Fatalf("loading the corpus: %v", err)
	}
	hash, err := bench.Hash()
	if err != nil {
		t.Fatalf("hashing the corpus: %v", err)
	}

	record := bench.NewRecord(
		time.Now().UTC().Format(time.RFC3339), Version, runtime.GOOS+"/"+runtime.GOARCH,
		corpus, hash, map[string]bench.Assignment{
			"classify": {Model: classifyModel, Digest: digestOf(t, classifyModel)},
			"embed":    {Model: embedModel, Digest: digestOf(t, embedModel)},
			// Generation takes no part in either benchmark, and saying so beats
			// leaving the reader to infer it from an absent line.
			"generate": {Model: "not used by this benchmark", Digest: "n/a"},
		})

	record.Classifier = runClassifier(t, corpus, classifyModel)
	record.Matching, record.EligibleRoles = runMatching(t, corpus, classifyModel, embedModel)
	record.Conclude()

	write(t, record)
	t.Log("\n" + record.Summary())

	// The outcome is evidence either way: a failing benchmark is a
	// model-selection decision, not a bug in the scorer.
	if record.Outcome != bench.OutcomePass {
		t.Logf("benchmark outcome: %s", record.Outcome)
	}
}

// runClassifier classifies every frozen listing with the live model and scores
// each one against its labels.
func runClassifier(t *testing.T, corpus *bench.Corpus, model string) []bench.ClassifierScore {
	t.Helper()
	scores := []bench.ClassifierScore{}
	for _, listing := range corpus.Listings {
		e := newClassifyEnv(t)
		e.classify = NewClassifyService(e.db, e.registry, platform.NewOllama())
		e.assignClassify(t, model)

		ids := e.withSource(t, listing.ID, listing.Markdown)
		sources := map[uint]string{}
		for _, chunk := range e.chunks2 {
			sources[chunk.ID] = chunk.Text
		}

		p, err := e.classify.Classify(ClassifyInput{
			SubjectKind: profile.SubjectRole, SubjectID: 1, ChunkIDs: ids,
		})
		if err != nil {
			// A refusal is a score of nothing captured, which is what it is: the
			// model did not produce a usable profile for this listing.
			t.Logf("%s: the model produced no usable profile: %v", listing.ID, err)
			scores = append(scores, bench.ScoreClassifier(listing, nil, sources))
			continue
		}
		scores = append(scores, bench.ScoreClassifier(listing, aspectsOf(t, e, p.ID), sources))
	}
	return scores
}

// runMatching builds a shortlist per scenario over the whole frozen listing
// corpus and scores the top five against the recruiter's ratings.
//
// Each scenario's own resume goes through the live classifier: a benchmark that
// gave every scenario the same candidate would score the same shortlist five
// times and call it five scenarios.
func runMatching(t *testing.T, corpus *bench.Corpus, classifyModel, embedModel string) (bench.MatchingScore, int) {
	t.Helper()
	results := []bench.TopFive{}
	eligible := 0

	for _, scenario := range corpus.Scenarios {
		e := newShortlistEnv(t)
		// Every service that decomposes anything is rebuilt against the live
		// model, so nothing in this run is answered by a scripted fake.
		live := NewClassifyService(e.db, e.registry, platform.NewOllama())
		e.classify = live
		e.profiles = NewCandidateProfileService(e.db, live, e.records)
		e.roles = NewRoleProfileService(e.db, live)
		e.shortlist = NewShortlistService(e.db, e.search, e.embed, e.criteria, e.profiles, e.roles)

		e.assignClassify(t, classifyModel)
		if _, err := e.registry.Assign(AssignInput{
			Role: models.RoleEmbed, Model: embedModel,
		}); err != nil {
			t.Fatalf("assigning the embedding model: %v", err)
		}

		// The candidate is profiled before the listings are brought in. Setting
		// up twenty listings runs the embedding model, which evicts the classify
		// model from memory on a laptop this size, and the resume call then pays
		// a full reload inside its own timeout. Recruiters work this way round
		// too: the candidate first, then the roles.
		candidateID, note := scenarioCandidate(t, e, scenario)

		// Every listing is in scope for every scenario: the benchmark measures
		// ranking, not filtering.
		byRole := map[uint]string{}
		for _, listing := range corpus.Listings {
			roleID := e.roleWithListing(t, listing.Title, listing.Markdown)
			byRole[roleID] = listing.ID
		}

		shortlist, err := e.shortlist.Build(e.initiative, candidateID)
		if err != nil {
			// A refusal is the scenario's outcome, recorded as itself rather
			// than as an empty top five that reads like a ranking failure.
			t.Logf("%s: the shortlist was refused: %v", scenario.ID, err)
			results = append(results, bench.TopFive{
				ScenarioID: scenario.ID, RoleIDs: []string{},
				Note: "no shortlist: " + err.Error(),
			})
			continue
		}
		eligible = shortlist.Eligible

		top := bench.TopFive{ScenarioID: scenario.ID, RoleIDs: []string{}, Note: note}
		for _, entry := range shortlist.Entries {
			if len(top.RoleIDs) == 5 {
				break
			}
			top.RoleIDs = append(top.RoleIDs, byRole[entry.RoleID])
		}
		t.Logf("%s: top five %v of %d eligible", scenario.ID, top.RoleIDs, eligible)
		results = append(results, top)
	}
	return bench.ScoreMatching(corpus, results), eligible
}

// scenarioCandidate creates the scenario's candidate, attaches its resume, and
// classifies and approves a profile with the live model.
//
// The name is invented and belongs to this repository, not to the scenario: the
// corpus carries a resume, and a benchmark needs a record to hang it on.
func scenarioCandidate(t *testing.T, e *shortlistEnv, scenario bench.Scenario) (uint, string) {
	t.Helper()
	c, err := e.records.CreateCandidate(models.Candidate{FullName: "Benchmark subject " + scenario.ID})
	if err != nil {
		t.Fatalf("creating the candidate: %v", err)
	}
	a, err := e.artifacts.create("resume", "resume.md", "test", []byte(scenario.Resume),
		models.LinkCandidate, c.ID)
	if err != nil {
		t.Fatalf("attaching the resume: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state":  models.ExtractionExtracted,
		"extractor":         "native-text",
		"extractor_version": "1",
		"markdown":          scenario.Resume,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	e.chunks2 = e.chunkAndWait(t, a.ID)

	p, err := e.profiles.Classify(c.ID)
	if err != nil {
		// A model that cannot decompose the resume cannot be matched with. The
		// scenario still runs, and the reason travels into the record so an
		// empty top five is not read as the matcher failing.
		t.Logf("%s: the model produced no usable candidate profile: %v", scenario.ID, err)
		return c.ID, "no candidate profile: " + err.Error()
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving the profile: %v", err)
	}
	return c.ID, ""
}

// aspectsOf reads back what the classifier stored, so the benchmark scores what
// the product kept rather than what the model said before validation.
func aspectsOf(t *testing.T, e *classifyEnv, profileID uint) []profile.Aspect {
	t.Helper()
	aspects, err := e.classify.Aspects(profileID)
	if err != nil {
		t.Fatalf("reading aspects: %v", err)
	}
	return aspects
}

// digestOf asks the endpoint what it is actually running, so the record names
// the model that answered rather than the name it was asked for.
func digestOf(t *testing.T, model string) string {
	t.Helper()
	info, err := platform.NewOllama().Show(t.Context(), model)
	if err != nil {
		return "unknown: " + strings.SplitN(err.Error(), "\n", 2)[0]
	}
	return info.Digest
}

// write stores the record beside the other product evidence.
func write(t *testing.T, record *bench.Record) {
	t.Helper()
	raw, err := record.JSON()
	if err != nil {
		t.Fatalf("writing the record: %v", err)
	}
	name := fmt.Sprintf("benchmark-%s.json", strings.ReplaceAll(record.Taken, ":", "-"))
	path := filepath.Join("docs", "product", "benchmarks", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("creating the benchmark folder: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	summary := strings.TrimSuffix(path, ".json") + ".txt"
	if err := os.WriteFile(summary, []byte(record.Summary()), 0o600); err != nil {
		t.Fatalf("writing %s: %v", summary, err)
	}
	t.Logf("recorded %s", path)
}
