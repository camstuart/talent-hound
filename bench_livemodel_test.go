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
	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/fusion"
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

	// Timings are taken as the run goes and turned into measurements below. A
	// record that scores the answers and says nothing about what they cost is
	// half a record: the PRD sets targets for both.
	roleProfiles, candidateProfiles, retrievals := []time.Duration{}, []time.Duration{}, []time.Duration{}

	record.Classifier = runClassifier(t, corpus, classifyModel, &roleProfiles)
	record.Matching, record.EligibleRoles = runMatching(t, corpus, classifyModel, embedModel,
		&candidateProfiles, &retrievals)
	record.Measurements = measure(roleProfiles, candidateProfiles, retrievals, len(corpus.Listings))
	record.Measurements = append(record.Measurements, coldStart(t))
	record.Conclude()

	write(t, record, corpus.Synthetic)
	t.Log("\n" + record.Summary())

	// The outcome is evidence either way: a failing benchmark is a
	// model-selection decision, not a bug in the scorer.
	if record.Outcome != bench.OutcomePass {
		t.Logf("benchmark outcome: %s", record.Outcome)
	}
}

// runClassifier classifies every frozen listing with the live model and scores
// each one against its labels.
func runClassifier(t *testing.T, corpus *bench.Corpus, model string, elapsed *[]time.Duration) []bench.ClassifierScore {
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

		started := time.Now()
		p, err := e.classify.Classify(ClassifyInput{
			SubjectKind: profile.SubjectRole, SubjectID: 1, ChunkIDs: ids,
		})
		*elapsed = append(*elapsed, time.Since(started))
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
func runMatching(t *testing.T, corpus *bench.Corpus, classifyModel, embedModel string,
	candidate, retrieval *[]time.Duration) (bench.MatchingScore, int) {
	t.Helper()
	results := []bench.TopFive{}
	eligible := 0

	// One workspace holding every role, rather than one per scenario.
	//
	// The roles used to be built with a hand-written aspect holding the role
	// title, because decomposing twenty listings inside each of five scenarios
	// is a hundred classifications. The consequence was that the half of the
	// retrieval this benchmark exists to measure — aspect-level KNN — ranked
	// titles, and the structured constraints that decide eligibility were not
	// in the profiles at all. Building the roles once and shortlisting five
	// candidates against them costs twenty classifications, not a hundred, and
	// the roles are the ones the product would actually hold.
	//
	// Candidates do not interact: a shortlist is built for one candidate over
	// the roles in scope, and another candidate in the same workspace is not in
	// scope for it.
	e := newShortlistEnv(t)
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

	// Every listing is in scope for every scenario: the benchmark measures
	// ranking, not filtering.
	byRole := map[uint]string{}
	for _, listing := range corpus.Listings {
		roleID := classifiedRole(t, e, listing)
		byRole[roleID] = listing.ID
	}

	// Then the candidates, before the embedding model is called. Running the
	// embedding model evicts the classify model from memory on a laptop this
	// size, and every classify call after it pays a full reload inside its own
	// timeout — so all of them happen first.
	candidateIDs := map[string]uint{}
	notes := map[string]string{}
	for _, scenario := range corpus.Scenarios {
		startedCandidate := time.Now()
		candidateID, note := scenarioCandidate(t, e, scenario)
		*candidate = append(*candidate, time.Since(startedCandidate))
		candidateIDs[scenario.ID], notes[scenario.ID] = candidateID, note
	}

	// The similarity half retrieves over aspects, so they have to be embedded —
	// the same step the interface runs when indexing.
	startedEmbedding := time.Now()
	if job, err := e.embed.EmbedAspects(e.initiative); err != nil {
		t.Fatalf("embedding aspects: %v", err)
	} else if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Logf("aspect embedding is %s (%q)", done.State, done.FailureReason)
	} else {
		embedding = append(embedding, time.Since(startedEmbedding))
	}

	for _, scenario := range corpus.Scenarios {
		startedShortlist := time.Now()
		shortlist, err := e.shortlist.Build(e.initiative, candidateIDs[scenario.ID])
		*retrieval = append(*retrieval, time.Since(startedShortlist))
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

		top := bench.TopFive{ScenarioID: scenario.ID, RoleIDs: []string{}, Note: notes[scenario.ID]}
		if len(shortlist.Entries) == 0 && top.Note == "" {
			// An empty list with a profile in place is a different failure from
			// an empty list with no profile, and the record has to be able to
			// tell them apart.
			top.Note = whyNothingRanked(t, e, candidateIDs[scenario.ID])
		}
		for _, entry := range shortlist.Entries {
			if len(top.RoleIDs) == 5 {
				break
			}
			top.RoleIDs = append(top.RoleIDs, byRole[entry.RoleID])
		}
		t.Logf("%s: top five %v of %d eligible", scenario.ID, top.RoleIDs, eligible)
		explain(t, scenario, shortlist, byRole, e, candidateIDs[scenario.ID])
		results = append(results, top)
	}
	return bench.ScoreMatching(corpus, results), eligible
}

// classifiedRole creates a role from a listing and decomposes it with the live
// model, which is what makes its profile the one the product would hold.
//
// Not roleWithListing: that assigns a synthetic classify model and adds aspects
// through the recruiter-authored path, which stamps them recruiter origin and
// re-cites them to the role record. A role is Ready when it has a current
// profile whose source hash still matches its listing, so nothing here has to
// approve anything.
func classifiedRole(t *testing.T, e *shortlistEnv, listing bench.Listing) uint {
	t.Helper()
	role, err := e.records.CreateRole(models.Role{
		Title:          listing.Title,
		Origin:         models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("creating the role: %v", err)
	}
	a, err := e.artifacts.create(listing.Title, listing.ID+".md", "test",
		[]byte(listing.Markdown), models.LinkRole, role.ID)
	if err != nil {
		t.Fatalf("attaching the listing: %v", err)
	}
	if err := e.artifacts.Link(a.ID, models.LinkInitiative, e.initiative); err != nil {
		t.Fatalf("linking to the initiative: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state":  models.ExtractionExtracted,
		"extractor":         "native-text",
		"extractor_version": "1",
		"markdown":          listing.Markdown,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	chunks := e.chunkAndWait(t, a.ID)
	if len(chunks) == 0 {
		t.Fatalf("%s produced no chunks", listing.ID)
	}
	ids := []uint{}
	for _, chunk := range chunks {
		ids = append(ids, chunk.ID)
	}

	if _, err := e.classify.Classify(ClassifyInput{
		SubjectKind: profile.SubjectRole, SubjectID: role.ID, ChunkIDs: ids,
	}); err != nil {
		// A listing the model cannot decompose is a role with no profile, which
		// the shortlist will find ineligible. That is the product's own
		// behaviour and the record should show it, not a fabricated profile.
		t.Logf("%s: the model produced no usable role profile: %v", listing.ID, err)
		return role.ID
	}
	status, err := e.roles.Status(role.ID)
	if err != nil {
		t.Fatalf("reading role status: %v", err)
	}
	if status.State != RoleProfileReady {
		t.Logf("%s is %q (%s)", listing.ID, status.State, status.Reason)
	}
	return role.ID
}

// whyNothingRanked reports what the shortlist had to work with, so an empty
// list is diagnosable from the record alone.
func whyNothingRanked(t *testing.T, e *shortlistEnv, candidateID uint) string {
	t.Helper()
	ready, err := e.profiles.Readiness(candidateID)
	if err != nil {
		return "readiness could not be read: " + err.Error()
	}
	if !ready.Ready {
		return "no roles ranked: the candidate profile is not ready"
	}
	approved, err := e.profiles.Approved(candidateID)
	if err != nil || approved == nil {
		return "no roles ranked: no approved profile could be read"
	}
	searchable := 0
	for _, a := range approved.Aspects {
		typ := profile.AspectType(a.Type)
		if fusion.Searchable(typ) && len(fusion.RoleAspectsFor(typ)) > 0 {
			searchable++
		}
	}
	return fmt.Sprintf("no roles ranked: the approved profile has %d aspects, %d of them searchable",
		len(approved.Aspects), searchable)
}

// scenarioCandidate creates the scenario's candidate, attaches its resume, and
// classifies and approves a profile with the live model.
//
// The name is invented and belongs to this repository, not to the scenario: the
// corpus carries a resume, and a benchmark needs a record to hang it on.
// indexing collects how long one resume took from attached to chunked and
// searchable, which is the PRD's indexing measurement.
var indexing []time.Duration

func scenarioCandidate(t *testing.T, e *shortlistEnv, scenario bench.Scenario) (uint, string) {
	t.Helper()
	startedIndexing := time.Now()
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
	indexing = append(indexing, time.Since(startedIndexing))

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
func write(t *testing.T, record *bench.Record, synthetic bool) {
	t.Helper()
	raw, err := record.JSON()
	if err != nil {
		t.Fatalf("writing the record: %v", err)
	}
	name := fmt.Sprintf("benchmark-%s.json", strings.ReplaceAll(record.Taken, ":", "-"))
	path := filepath.Join(bench.RecordDir(synthetic), name)
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

// measure turns the timings a run took into the record's measurements, against
// the PRD's provisional targets.
//
// The conditions are stated with every figure because they are what makes it
// readable: a decomposition time from a corpus of twenty short listings on a
// development machine is not the same measurement as one from the recruiter's
// documents on the target laptop, and a record that omitted the difference
// would invite exactly that comparison.
func measure(roleProfiles, candidateProfiles, retrievals []time.Duration, listings int) []bench.Measurement {
	out := []bench.Measurement{}
	add := func(name string, value float64, unit string, target float64, conditions string) {
		out = append(out, bench.Measurement{
			Name: name, Value: value, Unit: unit, Target: target,
			Met: target > 0 && value <= target, Conditions: conditions,
		})
	}
	where := runtime.GOOS + "/" + runtime.GOARCH + ", models resident, the synthetic corpus"

	if len(roleProfiles) > 0 {
		add("one role profile, mean", seconds(mean(roleProfiles)), "s", 30,
			fmt.Sprintf("%d listings of roughly 450 characters, two model passes each, %s", len(roleProfiles), where))
		add("one role profile, slowest", seconds(slowest(roleProfiles)), "s", 30,
			"the slowest single listing in the run, "+where)
		add(fmt.Sprintf("%d role profiles, total", listings), seconds(total(roleProfiles)), "s", 600,
			"sequential, no concurrency, "+where)
	}
	if len(candidateProfiles) > 0 {
		add("one candidate profile, mean", seconds(mean(candidateProfiles)), "s", 180,
			fmt.Sprintf("%d scenario resumes, %s", len(candidateProfiles), where))
	}
	if len(indexing) > 0 {
		add("one resume ingested, chunked and indexed", seconds(slowest(indexing)), "s", 60,
			fmt.Sprintf("the slowest of %d scenario resumes, already extracted — the extraction "+
				"sidecar is not in this path, so a real PDF costs more", len(indexing)))
	}
	if len(embedding) > 0 {
		add("every aspect of twenty roles embedded", seconds(slowest(embedding)), "s", 0,
			fmt.Sprintf("the slowest of %d runs; the PRD sets no target at this size", len(embedding)))
	}
	if len(retrievals) > 0 {
		// P95 over five scenarios is the slowest of the five, and saying so
		// beats printing a percentile the sample cannot support.
		add("hybrid retrieval, slowest of the run", seconds(slowest(retrievals)), "s", 2,
			fmt.Sprintf("%d scenarios over 20 eligible roles — the PRD's target is set at "+
				"approximately 1,000 candidates and 1,000 roles, which this corpus is not", len(retrievals)))
	}
	return out
}

// embedding collects how long every aspect of twenty roles took to embed.
var embedding []time.Duration

// coldStart measures what the application does before it can show anything:
// open the database and run every migration. It excludes Ollama, as the PRD's
// target does, and excludes the WebView — which is the larger half on Windows
// and cannot be measured from here.
func coldStart(t *testing.T) bench.Measurement {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cold.db")
	started := time.Now()
	opened, err := db.Open(path)
	if err != nil {
		t.Fatalf("opening a database: %v", err)
	}
	closeOnCleanup(t, opened)
	elapsed := time.Since(started)
	if raw, err := opened.DB(); err == nil {
		_ = raw.Close()
	}
	return bench.Measurement{
		Name: "cold start: open an empty database and migrate", Value: elapsed.Seconds(),
		Unit: "s", Target: 5, Met: elapsed.Seconds() <= 5,
		Conditions: "an empty database on " + runtime.GOOS + "/" + runtime.GOARCH +
			", excluding Ollama and excluding the WebView, which is the larger half on Windows",
	}
}

func seconds(d time.Duration) float64 { return d.Seconds() }

func total(in []time.Duration) time.Duration {
	sum := time.Duration(0)
	for _, d := range in {
		sum += d
	}
	return sum
}

func mean(in []time.Duration) time.Duration {
	if len(in) == 0 {
		return 0
	}
	return total(in) / time.Duration(len(in))
}

func slowest(in []time.Duration) time.Duration {
	out := time.Duration(0)
	for _, d := range in {
		if d > out {
			out = d
		}
	}
	return out
}

// explain says where every role the recruiter called plausible actually landed,
// and what retrieved the ones that beat them.
//
// A plausible role at rank seven is a ranking problem; a plausible role that is
// nowhere in the list at all is a retrieval or eligibility problem, and the two
// have nothing to do with each other. Guessing which one is happening is how
// three hypotheses in a row were wrong.
func explain(t *testing.T, scenario bench.Scenario, shortlist *Shortlist,
	byRole map[uint]string, e2 *shortlistEnv, candidateID uint) {
	t.Helper()
	// What the shortlist actually searched with. Ranking by raw word overlap
	// against the whole resume beats the shipped ranking on one frozen
	// scenario, and the queries are the obvious difference: the resume is a
	// page, and the queries are whatever the classifier lifted out of it.
	for _, q := range queriesOf(t, e2, candidateID) {
		t.Logf("  %s: query %q anyTerms=%v", scenario.ID, q.text, q.anyTerms)
	}

	rank := map[string]int{}
	entry := map[string]Entry{}
	for i, e := range shortlist.Entries {
		rank[byRole[e.RoleID]] = i + 1
		entry[byRole[e.RoleID]] = e
	}

	for _, rating := range scenario.Ratings {
		if !rating.Plausible {
			continue
		}
		at, listed := rank[rating.RoleID]
		if !listed {
			t.Logf("  %s: plausible %s is NOWHERE in %d ranked",
				scenario.ID, rating.RoleID, len(shortlist.Entries))
			continue
		}
		t.Logf("  %s: plausible %s at rank %d, score %.5f, %s",
			scenario.ID, rating.RoleID, at, entry[rating.RoleID].Score,
			why(entry[rating.RoleID]))
	}

	// And what the top five were retrieved by, so a wrong pick can be traced to
	// the half that produced it.
	for i, e := range shortlist.Entries {
		if i == 5 {
			break
		}
		t.Logf("  %s: rank %d %s score %.5f, %s",
			scenario.ID, i+1, byRole[e.RoleID], e.Score, why(e))
	}
}

// why summarises one entry's provenance: how many lists of each method found
// it, and the best rank it reached in each.
func why(e Entry) string {
	best := map[string]int{}
	count := map[string]int{}
	for _, c := range e.Why {
		count[c.Method]++
		if at, seen := best[c.Method]; !seen || c.Rank < at {
			best[c.Method] = c.Rank
		}
	}
	out := []string{}
	for _, method := range []string{"lexical", "semantic"} {
		if count[method] == 0 {
			out = append(out, method+" none")
			continue
		}
		out = append(out, fmt.Sprintf("%s %d lists best rank %d", method, count[method], best[method]))
	}
	return strings.Join(out, ", ")
}

// queriesOf reads the search terms the shortlist would build for this candidate.
func queriesOf(t *testing.T, e *shortlistEnv, candidateID uint) []query {
	t.Helper()
	out, err := e.shortlist.queries(e.initiative, candidateID)
	if err != nil {
		t.Logf("reading the queries: %v", err)
		return nil
	}
	return out
}
