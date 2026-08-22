//go:build perf

package main

// The PRD states performance budgets, and the acceptance record carried eight
// NOT RUN rows against them with nothing to run. This is the thing to run.
//
// It measures the two budgets that need no model at all — hybrid retrieval P95
// at the PRD's corpus size, and the database size at that corpus — so the same
// command produces a number on this machine and on the target laptop. The
// budgets that need a live model stay in the livemodel harness.
//
// Every fixture here is generated. No real candidate information appears in
// this repository's tests, fixtures, or output.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
	"camstuart/talent-hound/internal/vector"
)

// The PRD's representative corpus: "approximately 1,000 candidates and 1,000
// Active roles".
const (
	perfRoles      = 1000
	perfCandidates = 1000
	// Aspects per role profile. The frozen corpus decomposes a listing into
	// roughly this many, and the aspect count is what the KNN half scans.
	perfAspects = 9
	// A realistic embedding width. nomic-embed-text is 768; this is the wider
	// common case, so the number is not flattered by a narrow vector.
	perfDims = 1024
	// How many shortlists to time. Each runs against a different candidate, so
	// the query set differs between them rather than measuring one cached path.
	perfQueries = 20
	// Distinct candidates that are Ready and therefore matchable.
	perfReady = 5
)

// PRD budget: "hybrid retrieval P95 below 2 seconds on a representative derived
// corpus from approximately 1,000 candidates and 1,000 Active roles".
const retrievalBudget = 2 * time.Second

// PRD budget: "database below 5 GB at the representative corpus".
const databaseBudget = 5 << 30

func TestRetrievalAtTheRepresentativeCorpus(t *testing.T) {
	e := newShortlistEnvWithDims(t, perfDims)
	e.assignEmbedForPerf(t)
	space := e.spaceForPerf(t)

	start := time.Now()
	e.seedRoles(t, space, perfRoles)
	// The candidates are what make the database and the full-text index
	// representative. Only a few need to be matchable: a shortlist is built for
	// one candidate at a time, and a thousand approved profiles would measure
	// the seed rather than the query.
	ready := e.seedCandidates(t, space, perfCandidates, perfReady)
	// The full-text index is maintained by the search service, and a corpus it
	// never saw would make the lexical half return nothing. Once, at the end:
	// rebuilding per document would time the indexer two thousand times.
	if err := e.search.Rebuild(); err != nil {
		t.Fatalf("rebuilding the index: %v", err)
	}
	fmt.Printf("EVIDENCE perf-seed roles=%d candidates=%d aspects_per_role=%d dims=%d took=%s\n",
		perfRoles, perfCandidates, perfAspects, perfDims, time.Since(start).Round(time.Millisecond))

	// One criterion, so the lexical half has a recruiter-typed query as well as
	// the candidate's own aspects.
	e.add(t, "quokkastack in production", models.CriterionMustHave)

	timings := make([]time.Duration, 0, perfQueries)
	for i := 0; i < perfQueries; i++ {
		candidate := ready[i%len(ready)]
		began := time.Now()
		out, err := e.shortlist.Build(e.initiative, candidate)
		took := time.Since(began)
		if err != nil {
			t.Fatalf("building the shortlist for candidate %d: %v", candidate, err)
		}
		if out.Eligible != perfRoles {
			t.Fatalf("the shortlist considered %d roles, want the %d seeded — the corpus is not what is being measured",
				out.Eligible, perfRoles)
		}
		if len(out.Entries) == 0 {
			t.Fatal("the shortlist returned nothing, so this measures a refusal rather than retrieval")
		}
		// Both halves have to have run. A semantic half that quietly returned
		// nothing would leave every assertion above satisfied and this timing a
		// measurement of full-text search alone — which is the cheap half, and
		// so would report a comfortable pass for the wrong reason.
		assertBothHalvesRan(t, out)
		timings = append(timings, took)
	}

	sort.Slice(timings, func(a, b int) bool { return timings[a] < timings[b] })
	p95 := timings[percentileIndex(len(timings), 95)]
	fmt.Printf("EVIDENCE perf-retrieval queries=%d min=%s median=%s p95=%s max=%s budget=%s\n",
		len(timings), timings[0].Round(time.Millisecond),
		timings[len(timings)/2].Round(time.Millisecond), p95.Round(time.Millisecond),
		timings[len(timings)-1].Round(time.Millisecond), retrievalBudget)

	size := e.databaseBytes(t)
	fmt.Printf("EVIDENCE perf-database bytes=%d mib=%.1f budget_gib=%.1f\n",
		size, float64(size)/(1<<20), float64(databaseBudget)/(1<<30))

	// Recorded as measured either way. A miss is a number with a decision
	// attached, not a hidden failure — but it is still a failure here, because
	// a budget nobody fails is not a budget.
	if p95 > retrievalBudget {
		t.Errorf("hybrid retrieval P95 is %s over %d roles and %d candidates, above the PRD's %s budget",
			p95.Round(time.Millisecond), perfRoles, perfCandidates, retrievalBudget)
	}
	if size > databaseBudget {
		t.Errorf("the database is %d bytes at the representative corpus, above the PRD's %d byte budget",
			size, databaseBudget)
	}
}

// assertBothHalvesRan checks the fused provenance names lexical and semantic
// retrieval, so what was timed is the hybrid path the budget is about.
func assertBothHalvesRan(t *testing.T, out *Shortlist) {
	t.Helper()
	methods := map[string]bool{}
	for _, entry := range out.Entries {
		for _, why := range entry.Why {
			methods[why.Method] = true
		}
	}
	for _, want := range []string{"lexical", "semantic"} {
		if !methods[want] {
			t.Fatalf("no entry was retrieved by the %s half (methods seen: %v) — this timing is not the hybrid path",
				want, methods)
		}
	}
}

// percentileIndex is the index into a sorted slice holding the pth percentile,
// by the nearest-rank definition.
func percentileIndex(n, p int) int {
	i := (n*p + 99) / 100
	if i > 0 {
		i--
	}
	return i
}

// databaseBytes is what the database occupies, asked of SQLite rather than of
// the filesystem: the test database has no file, and page count times page size
// is the same number the file would hold.
func (e *shortlistEnv) databaseBytes(t *testing.T) int64 {
	t.Helper()
	var size int64
	err := e.db.Raw("SELECT (SELECT * FROM pragma_page_count()) * (SELECT * FROM pragma_page_size())").
		Scan(&size).Error
	if err != nil {
		t.Fatalf("measuring the database: %v", err)
	}
	return size
}

func (e *shortlistEnv) assignEmbedForPerf(t *testing.T) {
	t.Helper()
	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: "synthetic-embed"}); err != nil {
		t.Fatalf("assigning the embed role: %v", err)
	}
}

// spaceForPerf creates the embedding space the seeded vectors belong to.
//
// The real one is created by the first successful embedding, which is the right
// design and the wrong cost here: seeding through the job path would time the
// indexer rather than the query. The vectors are generated instead, because
// what a scan costs depends on how many there are and how wide they are, not on
// what they mean — and what they mean is what the frozen benchmark measures.
func (e *shortlistEnv) spaceForPerf(t *testing.T) *models.EmbeddingSpace {
	t.Helper()
	res, err := e.registry.Resolve(models.RoleEmbed)
	if err != nil || res.Assignment == nil {
		t.Fatalf("resolving the embed assignment: %v", err)
	}
	a := res.Assignment
	space := &models.EmbeddingSpace{
		Endpoint: a.Endpoint, Model: a.Model, Digest: a.Digest, Revision: a.Revision,
		Dimensions: perfDims, Metric: models.MetricCosine,
	}
	if err := e.db.Create(space).Error; err != nil {
		t.Fatalf("creating the embedding space: %v", err)
	}
	// The service must find this space, or the semantic half is silently absent
	// and the measurement is of the lexical half alone.
	got, err := e.embed.CurrentSpace()
	if err != nil || got == nil || got.ID != space.ID {
		t.Fatalf("the seeded space is not the current one (%v, %v)", got, err)
	}
	return space
}

// spread generates the corpus vectors: a plain multiplicative generator, so the
// numbers are varied, identical on every machine, and free of a dependency.
type spread struct{ state uint64 }

func (s *spread) next() float32 {
	s.state = s.state*6364136223846793005 + 1442695040888963407
	return float32(s.state>>40)/(1<<23) - 1
}

func (s *spread) vector(dims int) []float32 {
	v := make([]float32, dims)
	for i := range v {
		v[i] = s.next()
	}
	return v
}

// perfWords are assembled into listing and resume text. Real wording matters
// here in one way only: the full-text half has to match something, and a corpus
// of identical documents would make every lexical query return everything.
var perfWords = []string{
	"quokkastack", "platform", "kubernetes", "embedded", "conveyor", "ledger",
	"reconciliation", "telemetry", "provisioning", "observability", "mining",
	"logistics", "payments", "scheduling", "firmware", "migration",
}

func (s *spread) sentence(n int) string {
	out := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			out += " "
		}
		idx := int(s.state>>17) % len(perfWords)
		s.next()
		out += perfWords[idx]
	}
	return out
}

// statements generates the sentences a document is written from and its profile
// is decomposed into, so every aspect's wording is verbatim in its source.
func (s *spread) statements(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s.sentence(9)
	}
	return out
}

// seedRoles writes the roles, their listings, chunks, Ready profiles, aspects,
// and aspect vectors straight to the database.
func (e *shortlistEnv) seedRoles(t *testing.T, space *models.EmbeddingSpace, n int) {
	t.Helper()
	rng := &spread{state: 11}
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("Role %d %s", i, perfWords[i%len(perfWords)])
		role := &models.Role{
			Title:  title,
			Origin: models.RoleOriginRecruiterEntered,
			// Active, which is what makes it eligible to be matched.
			LifecycleState: models.RoleOpen,
		}
		if err := e.db.Create(role).Error; err != nil {
			t.Fatalf("creating role %d: %v", i, err)
		}
		statements := rng.statements(perfAspects)
		markdown := "# " + title + "\n\n## Requirements\n\n" + strings.Join(statements, "\n\n") + "\n"
		chunkID := e.seedDocument(t, markdown, models.LinkRole, role.ID)
		e.seedProfile(t, space, rng, profile.SubjectRole, role.ID, chunkID, markdown, statements)
	}
}

// seedCandidates writes the candidates and their resumes. The first `ready` of
// them also get an approved profile, so they can be matched on.
func (e *shortlistEnv) seedCandidates(t *testing.T, space *models.EmbeddingSpace, n, ready int) []uint {
	t.Helper()
	rng := &spread{state: 23}
	matchable := []uint{}
	for i := 0; i < n; i++ {
		// Generated names. The rule is that no real person appears here, and a
		// numbered placeholder cannot accidentally be one.
		c := &models.Candidate{FullName: fmt.Sprintf("Candidate %d", i)}
		if err := e.db.Create(c).Error; err != nil {
			t.Fatalf("creating candidate %d: %v", i, err)
		}
		statements := rng.statements(perfAspects)
		markdown := fmt.Sprintf("# Candidate %d\n\n## Experience\n\n%s\n", i,
			strings.Join(statements, "\n\n"))
		chunkID := e.seedDocument(t, markdown, models.LinkCandidate, c.ID)
		if i < ready {
			e.seedProfile(t, space, rng, profile.SubjectCandidate, c.ID, chunkID, markdown, statements)
			matchable = append(matchable, c.ID)
		}
	}
	if len(matchable) == 0 {
		t.Fatal("no candidate is matchable, so nothing can be measured")
	}
	return matchable
}

// seedDocument writes one extracted artifact, links it to its subject and to
// the initiative, chunks it whole, and returns the chunk's identifier.
func (e *shortlistEnv) seedDocument(t *testing.T, markdown string, kind models.LinkTarget, subject uint) uint {
	t.Helper()
	a := &models.Artifact{
		DisplayName: fmt.Sprintf("doc-%s-%d", kind, subject),
		MediaType:   "text/markdown",
		ByteLength:  int64(len(markdown)),
		SHA256:      fmt.Sprintf("%064x", subject),
		CapturedAt:  time.Now().UTC(),
		Bytes:       []byte(markdown),
		// Already extracted: the extractor is not what this measures.
		ExtractionState:  models.ExtractionExtracted,
		Extractor:        "perf-seed",
		ExtractorVersion: "1",
		Markdown:         markdown,
	}
	if err := e.db.Create(a).Error; err != nil {
		t.Fatalf("creating artifact: %v", err)
	}
	for _, link := range []models.ArtifactLink{
		{ArtifactID: a.ID, TargetType: kind, TargetID: subject},
		{ArtifactID: a.ID, TargetType: models.LinkInitiative, TargetID: e.initiative},
	} {
		if err := e.db.Create(&link).Error; err != nil {
			t.Fatalf("linking artifact: %v", err)
		}
	}
	chunk := &models.Chunk{
		ArtifactID: a.ID, Ordinal: 0, Text: markdown,
		StartOffset: 0, EndOffset: len(markdown),
		TokenCount: len(markdown) / 4, Hash: fmt.Sprintf("%064x", a.ID),
		Chunker: "perf-seed", ChunkerVersion: "1",
	}
	if err := e.db.Create(chunk).Error; err != nil {
		t.Fatalf("creating chunk: %v", err)
	}
	return chunk.ID
}

// seedProfile writes an approved profile whose source hash matches the chunk it
// cites, so the readiness check finds it Ready rather than Stale.
func (e *shortlistEnv) seedProfile(t *testing.T, space *models.EmbeddingSpace, rng *spread,
	kind profile.SubjectKind, subject, chunkID uint, text string, statements []string) {
	t.Helper()
	hash := profile.HashSources([]profile.Source{{ChunkID: chunkID, Text: text}})
	now := time.Now().UTC()
	p := &models.Profile{
		SubjectKind: string(kind), SubjectID: subject, Version: 1,
		State:         string(models.ProfileApproved),
		SchemaVersion: profile.SchemaVersion, PromptVersion: profile.PromptVersion,
		ModelRevision: 1, ModelName: "synthetic-classify",
		SourceHash: hash, Identity: fmt.Sprintf("%s-%d", kind, subject),
		ApprovedAt: &now, ApprovedSourceHash: hash,
	}
	if err := e.db.Create(p).Error; err != nil {
		t.Fatalf("creating profile: %v", err)
	}
	for i, wording := range statements {
		// The aspect cites the chunk its wording came from, quoting it exactly.
		// The schema refuses an uncited aspect, which is the invariant the whole
		// contract rests on — so the corpus has to be citable, not merely large.
		cite, err := json.Marshal([]profile.Citation{{ChunkID: chunkID, Quote: wording}})
		if err != nil {
			t.Fatalf("encoding the citation: %v", err)
		}
		a := &models.ProfileAspect{
			ProfileID: p.ID, Ordinal: i, Type: string(profile.Other),
			Wording: wording, Origin: string(profile.Extracted),
			Citations: string(cite),
		}
		if err := e.db.Create(a).Error; err != nil {
			t.Fatalf("creating aspect: %v", err)
		}
		vec := &models.Embedding{
			SpaceID: space.ID, OwnerKind: models.OwnerAspect, OwnerID: a.ID,
			Dimensions: space.Dimensions, Vector: vector.Encode(rng.vector(space.Dimensions)),
		}
		if err := e.db.Create(vec).Error; err != nil {
			t.Fatalf("creating aspect embedding: %v", err)
		}
	}
}
