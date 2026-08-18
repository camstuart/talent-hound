package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/vector"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// fakeEmbedder is an endpoint that returns a vector chosen by the test.
//
// The default is a deterministic hash of the text, which is enough for every
// property this suite asserts: identical text gives identical vectors, so ties
// are producible, and different text gives different ones, so ordering is
// producible. Nothing here pretends to be a language model.
type fakeEmbedder struct {
	mu sync.Mutex
	// dims is the length of the vectors returned.
	dims int
	// fixed, when set, is returned for any text whose key is present, so a test
	// can place a chunk at a chosen angle from a chosen query.
	fixed map[string][]float32
	// err, when set, is returned instead of a vector.
	err error
	// delay before answering, for the cancellation test.
	delay time.Duration
	// calls counts every request, and texts records what was asked for.
	calls atomic.Int64
	texts []string
}

func newFakeEmbedder(dims int) *fakeEmbedder {
	return &fakeEmbedder{dims: dims, fixed: map[string][]float32{}}
}

func (f *fakeEmbedder) Embed(ctx context.Context, _ string, text string) ([]float32, error) {
	f.calls.Add(1)
	f.mu.Lock()
	delay, err := f.delay, f.err
	if v, ok := f.fixed[text]; ok {
		out := append([]float32(nil), v...)
		f.texts = append(f.texts, text)
		f.mu.Unlock()
		return out, nil
	}
	f.texts = append(f.texts, text)
	dims := f.dims
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	return hashVector(text, dims), nil
}

func (f *fakeEmbedder) set(text string, v []float32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fixed[text] = v
}

func (f *fakeEmbedder) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// hashVector is a deterministic pseudo-embedding: same text, same vector,
// always, on any machine.
func hashVector(text string, dims int) []float32 {
	out := make([]float32, dims)
	for i := range out {
		h := fnv.New32a()
		_, _ = fmt.Fprintf(h, "%d:%s", i, text)
		// Spread over roughly [-1, 1) without ever landing on all-zero, using
		// only the low bits so nothing has to be reinterpreted as signed.
		out[i] = float32(h.Sum32()%2001)/1000 - 1 + 0.001
	}
	return out
}

// embedEnv is an indexEnv with the registry and embedding service wired in.
type embedEnv struct {
	*indexEnv
	registry *ModelService
	embed    *EmbedService
	endpoint *fakeEmbedder
}

func newEmbedEnv(t *testing.T, dims int) *embedEnv {
	t.Helper()
	base := newIndexEnv(t)
	registry := NewModelService(base.db, base.jobs, nil)
	endpoint := newFakeEmbedder(dims)
	return &embedEnv{
		indexEnv: base,
		registry: registry,
		embed:    NewEmbedService(base.db, base.jobs, registry, endpoint),
		endpoint: endpoint,
	}
}

// assignEmbed points the embed role at a model name. The endpoint stays the
// local one, because the registry refuses anything else.
func (e *embedEnv) assignEmbed(t *testing.T, model string) {
	t.Helper()
	if _, err := e.registry.Assign(AssignInput{Role: models.RoleEmbed, Model: model}); err != nil {
		t.Fatalf("assigning the embed role: %v", err)
	}
}

// embedAllAndWait indexes the initiative and waits for the job.
func (e *embedEnv) embedAllAndWait(t *testing.T) models.Job {
	t.Helper()
	job, err := e.embed.EmbedAll(e.initiative)
	if err != nil {
		t.Fatalf("queuing embedding: %v", err)
	}
	return waitForJob(t, e.jobs, job.ID)
}

// corpus chunks a set of documents so there is something to embed.
func (e *embedEnv) corpus(t *testing.T, docs map[string]string) {
	t.Helper()
	for name, md := range docs {
		a := e.extracted(t, name, md)
		e.chunkAndWait(t, a.ID)
	}
}

const platformCV = `# Kalinda Reyes

## Experience

Senior platform engineer at Northwind, working on billing and payments.
`

const analystCV = `# Tobias Fenn

## Experience

Financial analyst at Harbourline, reporting on quarterly revenue.
`

func TestEmbeddingCreatesItsSpaceAndReusesIt(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "synthetic-embed")
	e.corpus(t, map[string]string{"cv": platformCV})

	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding job %s (%s)", done.State, done.FailureReason)
	}

	spaces, err := e.embed.Spaces()
	if err != nil {
		t.Fatalf("listing spaces: %v", err)
	}
	if len(spaces) != 1 {
		t.Fatalf("got %d spaces, want exactly one", len(spaces))
	}
	space := spaces[0]
	if space.Dimensions != 8 {
		t.Errorf("space has %d dimensions, want the 8 the endpoint returned", space.Dimensions)
	}
	if space.Model != "synthetic-embed" || space.Metric != models.MetricCosine {
		t.Errorf("space records model %q metric %q", space.Model, space.Metric)
	}
	if space.Revision != 1 {
		t.Errorf("space names revision %d, want the assignment's 1", space.Revision)
	}

	// Embedding more evidence under an unchanged assignment must not make a
	// second space, or the two halves of one corpus stop being comparable.
	e.corpus(t, map[string]string{"second": analystCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("second embedding job %s (%s)", done.State, done.FailureReason)
	}
	spaces, err = e.embed.Spaces()
	if err != nil {
		t.Fatalf("listing spaces: %v", err)
	}
	if len(spaces) != 1 {
		t.Fatalf("got %d spaces after a second run, want one", len(spaces))
	}
}

func TestChangingTheModelCreatesANewSpaceAndStrandsTheOldVectors(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "first-embed")
	e.corpus(t, map[string]string{"cv": platformCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding job %s (%s)", done.State, done.FailureReason)
	}

	before, err := e.embed.Coverage(e.initiative)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if before.Total == 0 || before.Embedded != before.Total || before.Outstanding != 0 {
		t.Fatalf("before the change: %d of %d embedded, %d outstanding",
			before.Embedded, before.Total, before.Outstanding)
	}

	e.assignEmbed(t, "second-embed")

	after, err := e.embed.Coverage(e.initiative)
	if err != nil {
		t.Fatalf("coverage after the change: %v", err)
	}
	if after.Space != nil {
		t.Errorf("the new assignment already has a space before anything was embedded through it")
	}
	if after.Embedded != 0 || after.Outstanding != after.Total {
		t.Fatalf("after the change: %d of %d embedded, want none of them",
			after.Embedded, after.Total)
	}

	// The old vectors are retained: they were correct, and re-embedding is
	// compute nobody asked for.
	var kept int64
	if err := e.db.Model(&models.Embedding{}).Count(&kept).Error; err != nil {
		t.Fatalf("counting embeddings: %v", err)
	}
	if kept != before.Total {
		t.Errorf("kept %d vectors from the old space, want the %d that were there", kept, before.Total)
	}

	// And a search in the new space cannot see them.
	if _, err := e.embed.SemanticSearch(e.initiative, "platform engineer", 10); err == nil {
		t.Error("searched in a space with nothing embedded and got results")
	}

	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("re-embedding job %s (%s)", done.State, done.FailureReason)
	}
	spaces, err := e.embed.Spaces()
	if err != nil {
		t.Fatalf("listing spaces: %v", err)
	}
	if len(spaces) != 2 {
		t.Fatalf("got %d spaces after a model change, want two", len(spaces))
	}
}

// The failure this whole phase exists to prevent: two geometries, one number.
func TestVectorsFromDifferentSpacesAreNeverCompared(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "first-embed")
	e.corpus(t, map[string]string{"cv": platformCV, "other": analystCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("first embedding %s (%s)", done.State, done.FailureReason)
	}
	e.assignEmbed(t, "second-embed")
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("second embedding %s (%s)", done.State, done.FailureReason)
	}

	current, err := e.embed.CurrentSpace()
	if err != nil || current == nil {
		t.Fatalf("current space: %v %v", current, err)
	}
	hits, err := e.embed.SemanticSearch(e.initiative, "billing and payments", 50)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no results at all")
	}
	// Every hit names the current space, and the count cannot exceed what that
	// space holds even though twice as many vectors exist.
	var inSpace int64
	err = e.db.Model(&models.Embedding{}).Where("space_id = ?", current.ID).Count(&inSpace).Error
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if int64(len(hits)) > inSpace {
		t.Fatalf("got %d hits from a space holding %d vectors", len(hits), inSpace)
	}
	for _, h := range hits {
		if h.SpaceID != current.ID {
			t.Fatalf("a hit came from space %d, not the current %d", h.SpaceID, current.ID)
		}
	}
}

func TestTiedScoresOrderDeterministicallyAndRepeat(t *testing.T) {
	e := newEmbedEnv(t, 4)
	e.assignEmbed(t, "synthetic-embed")

	// Three documents whose bodies are word-for-word identical embed to
	// word-for-word identical vectors, which is exactly how ties happen in a
	// real corpus: boilerplate.
	same := "## Summary\n\nAvailable for contract work in Melbourne.\n"
	e.corpus(t, map[string]string{"a": same, "b": same, "c": same})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}

	first, err := e.embed.SemanticSearch(e.initiative, "contract work", 10)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(first) < 3 {
		t.Fatalf("got %d hits, want at least the three identical ones", len(first))
	}
	for i := 1; i < len(first); i++ {
		if first[i].Score > first[i-1].Score {
			t.Fatalf("hit %d scored %v above hit %d at %v", i, first[i].Score, i-1, first[i-1].Score)
		}
		if first[i].Score == first[i-1].Score && first[i].ChunkID < first[i-1].ChunkID {
			t.Fatalf("tied hits are out of identifier order: %d then %d",
				first[i-1].ChunkID, first[i].ChunkID)
		}
	}

	for run := range 5 {
		again, err := e.embed.SemanticSearch(e.initiative, "contract work", 10)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if len(again) != len(first) {
			t.Fatalf("run %d returned %d hits, first run returned %d", run, len(again), len(first))
		}
		for i := range again {
			if again[i].ChunkID != first[i].ChunkID || again[i].Score != first[i].Score {
				t.Fatalf("run %d differs at %d: chunk %d score %v, want chunk %d score %v",
					run, i, again[i].ChunkID, again[i].Score, first[i].ChunkID, first[i].Score)
			}
		}
	}
}

// Meaning, not words: the query shares no term with the text it must find.
func TestSemanticSearchRanksByAngleNotByWords(t *testing.T) {
	e := newEmbedEnv(t, 3)
	e.assignEmbed(t, "synthetic-embed")
	e.corpus(t, map[string]string{"platform": platformCV, "analyst": analystCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}

	chunks := []models.Chunk{}
	if err := e.db.Order("id asc").Find(&chunks).Error; err != nil {
		t.Fatalf("listing chunks: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("got %d chunks, want at least two", len(chunks))
	}

	// Place the two documents at chosen angles from a query that shares no word
	// with either — which is the whole point of embedding rather than matching.
	query := "someone who has run infrastructure at scale"
	var wanted uint
	for _, c := range chunks {
		if strings.Contains(c.Text, "platform engineer") {
			wanted = c.ID
			e.endpoint.set(c.Text, []float32{1, 0, 0})
		} else {
			e.endpoint.set(c.Text, []float32{0, 1, 0})
		}
	}
	if wanted == 0 {
		t.Fatal("no chunk carried the platform text")
	}
	e.endpoint.set(query, []float32{1, 0.05, 0})

	// Re-embed under a fresh assignment so the placed vectors are the stored
	// ones rather than the hashed ones.
	e.assignEmbed(t, "placed-embed")
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("re-embedding %s (%s)", done.State, done.FailureReason)
	}

	hits, err := e.embed.SemanticSearch(e.initiative, query, 10)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no results")
	}
	if hits[0].ChunkID != wanted {
		t.Fatalf("top hit is chunk %d (%q), want chunk %d",
			hits[0].ChunkID, hits[0].Text, wanted)
	}
	if hits[0].Score <= hits[len(hits)-1].Score && len(hits) > 1 {
		t.Errorf("the nearest and furthest hits scored %v and %v",
			hits[0].Score, hits[len(hits)-1].Score)
	}
}

func TestASearchIsScopedToTheInitiativeThatAsked(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "synthetic-embed")

	inits := NewInitiativeService(e.db)
	other, err := inits.Create("Other "+t.Name(), models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating the other initiative: %v", err)
	}
	distinctive := "## Note\n\nQuokkabeam telemetry rollout, Fremantle.\n"
	a, err := e.artifacts.create("other", "other.md", "test", []byte(distinctive),
		models.LinkInitiative, other.ID)
	if err != nil {
		t.Fatalf("ingesting into the other initiative: %v", err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state":  models.ExtractionExtracted,
		"extractor":         "native-text",
		"extractor_version": "1",
		"markdown":          distinctive,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	e.chunkAndWait(t, a.ID)

	e.corpus(t, map[string]string{"mine": platformCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}

	hits, err := e.embed.SemanticSearch(e.initiative, "Quokkabeam telemetry", 50)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	for _, h := range hits {
		if strings.Contains(h.Text, "Quokkabeam") {
			t.Fatalf("a search of one initiative returned the other's evidence: %q", h.Text)
		}
	}
}

func TestADegenerateVectorIsAFailureNotAVector(t *testing.T) {
	cases := []struct {
		name string
		v    []float32
	}{
		{"all zeroes", []float32{0, 0, 0, 0}},
		{"a NaN component", []float32{1, float32(math.NaN()), 1, 1}},
		{"an infinite component", []float32{1, float32(math.Inf(1)), 1, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEmbedEnv(t, 4)
			e.assignEmbed(t, "synthetic-embed")
			e.corpus(t, map[string]string{"cv": platformCV})

			chunks := []models.Chunk{}
			if err := e.db.Order("id asc").Find(&chunks).Error; err != nil {
				t.Fatalf("listing chunks: %v", err)
			}
			for _, ch := range chunks {
				e.endpoint.set(ch.Text, c.v)
			}

			done := e.embedAllAndWait(t)
			if done.State != models.JobFailed {
				t.Fatalf("embedding a %s vector ended %s, want a failure", c.name, done.State)
			}
			if done.FailureReason != models.ReasonBadVector {
				t.Errorf("failure reason %q, want %q", done.FailureReason, models.ReasonBadVector)
			}
			var stored int64
			if err := e.db.Model(&models.Embedding{}).Count(&stored).Error; err != nil {
				t.Fatalf("counting: %v", err)
			}
			if stored != 0 {
				t.Errorf("stored %d vectors from a %s response", stored, c.name)
			}
		})
	}
}

func TestAProviderFailureWritesNothingAndCarriesNoContent(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "synthetic-embed")
	e.corpus(t, map[string]string{"cv": platformCV})
	// The endpoint's own words quote the text it was given — which is exactly
	// what must not reach a stored job row.
	e.endpoint.fail(errors.New("failed to embed \"Senior platform engineer at Northwind\": out of memory"))

	done := e.embedAllAndWait(t)
	if done.State != models.JobFailed {
		t.Fatalf("job ended %s, want a failure", done.State)
	}
	if done.FailureReason != models.ReasonEndpointFailed {
		t.Errorf("failure reason %q, want %q", done.FailureReason, models.ReasonEndpointFailed)
	}
	if strings.Contains(done.FailureReason, "Northwind") || strings.Contains(done.Params, "Northwind") {
		t.Fatalf("the job row quotes the document: reason %q params %q", done.FailureReason, done.Params)
	}
	var stored int64
	if err := e.db.Model(&models.Embedding{}).Count(&stored).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if stored != 0 {
		t.Errorf("stored %d vectors despite the endpoint failing", stored)
	}
}

func TestCancellationLeavesNoPartialVector(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "synthetic-embed")
	// Enough chunks that a cancellation lands mid-run rather than after it.
	docs := map[string]string{}
	for i := range 8 {
		docs[fmt.Sprintf("doc%d", i)] = fmt.Sprintf(
			"# Document %d\n\nA paragraph of invented text for item %d.\n", i, i)
	}
	e.corpus(t, docs)
	e.endpoint.mu.Lock()
	e.endpoint.delay = 60 * time.Millisecond
	e.endpoint.mu.Unlock()

	job, err := e.embed.EmbedAll(e.initiative)
	if err != nil {
		t.Fatalf("queuing: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := e.jobs.Cancel(job.ID); err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	done := waitForJob(t, e.jobs, job.ID)
	if done.State != models.JobCancelled {
		t.Fatalf("job ended %s, want cancelled", done.State)
	}

	var stored int64
	if err := e.db.Model(&models.Embedding{}).Count(&stored).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	// Some completed, not all — and every one that exists is whole, which the
	// database's own length check enforces on the way in.
	if stored == 0 {
		t.Skip("the cancellation landed before the first item committed")
	}
	if stored >= int64(done.TotalItems) {
		t.Fatalf("stored %d vectors of %d items despite cancelling", stored, done.TotalItems)
	}
	space, err := e.embed.CurrentSpace()
	if err != nil || space == nil {
		t.Fatalf("current space: %v %v", space, err)
	}
	rows := []models.Embedding{}
	if err := e.db.Find(&rows).Error; err != nil {
		t.Fatalf("listing embeddings: %v", err)
	}
	for _, r := range rows {
		if len(r.Vector) != 4*space.Dimensions {
			t.Fatalf("embedding %d holds %d bytes, want %d", r.ID, len(r.Vector), 4*space.Dimensions)
		}
		if _, err := vector.Decode(r.Vector, space.Dimensions); err != nil {
			t.Fatalf("embedding %d does not decode: %v", r.ID, err)
		}
	}
}

// A vector whose chunk is gone is not stale data, it is a retrieval result that
// cannot be cited.
func TestVectorsDoNotOutliveTheirChunks(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "synthetic-embed")
	a := e.extracted(t, "cv", platformCV)
	e.chunkAndWait(t, a.ID)
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}

	var before int64
	if err := e.db.Model(&models.Embedding{}).Count(&before).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if before == 0 {
		t.Fatal("nothing was embedded")
	}

	// Re-chunking replaces the rows; the vectors of the replaced ones must go
	// in the same transaction.
	e.chunkAndWait(t, a.ID)

	var orphans int64
	err := e.db.Model(&models.Embedding{}).
		Where("owner_kind = ?", models.OwnerChunk).
		Where("owner_id NOT IN (SELECT id FROM chunks)").
		Count(&orphans).Error
	if err != nil {
		t.Fatalf("counting orphans: %v", err)
	}
	if orphans != 0 {
		t.Fatalf("%d vectors name chunks that no longer exist", orphans)
	}
}

func TestReEmbeddingReplacesRatherThanDuplicates(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "synthetic-embed")
	e.corpus(t, map[string]string{"cv": platformCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("first embedding %s (%s)", done.State, done.FailureReason)
	}
	var after1 int64
	if err := e.db.Model(&models.Embedding{}).Count(&after1).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}

	// Force every chunk back through the endpoint by asking for them by id.
	chunks := []models.Chunk{}
	if err := e.db.Order("id asc").Find(&chunks).Error; err != nil {
		t.Fatalf("listing chunks: %v", err)
	}
	ids := make([]uint, 0, len(chunks))
	for _, c := range chunks {
		ids = append(ids, c.ID)
	}
	job, err := e.embedIDs(ids)
	if err != nil {
		t.Fatalf("queuing: %v", err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("second embedding %s (%s)", done.State, done.FailureReason)
	}

	var after2 int64
	if err := e.db.Model(&models.Embedding{}).Count(&after2).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if after2 != after1 {
		t.Fatalf("re-embedding grew the table from %d to %d", after1, after2)
	}
}

// embedIDs queues a specific set of chunks, bypassing the already-embedded
// filter, so the upsert path can be exercised.
func (e *embedEnv) embedIDs(ids []uint) (*models.Job, error) {
	params, err := json.Marshal(embedParams{OwnerKind: models.OwnerChunk, OwnerIDs: ids})
	if err != nil {
		return nil, err
	}
	return e.jobs.Enqueue(JobInput{
		Kind:         "embed",
		InitiativeID: e.initiative,
		Params:       string(params),
		TotalItems:   len(ids),
	})
}

func TestEmbeddingWithoutAModelFails(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.corpus(t, map[string]string{"cv": platformCV})

	done := e.embedAllAndWait(t)
	if done.State != models.JobFailed {
		t.Fatalf("job ended %s, want a failure", done.State)
	}
	if done.FailureReason != models.ReasonNoEmbedModel {
		t.Errorf("failure reason %q, want %q", done.FailureReason, models.ReasonNoEmbedModel)
	}
	if _, err := e.embed.SemanticSearch(e.initiative, "anything", 10); err == nil {
		t.Error("searched with no embed model assigned and got results")
	}
}

// An assertion about what a call site does holds against the code that exists
// today. An assertion that a server received nothing holds against code nobody
// has written yet.
func TestACloudEndpointReceivesNoCandidateEmbeddingCalls(t *testing.T) {
	var cloudCalls atomic.Int64
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cloudCalls.Add(1)
		t.Errorf("a cloud endpoint received %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer cloud.Close()

	e := newEmbedEnv(t, 8)

	// There is no configuration in which the embed role names a remote host:
	// the registry refuses, so no path through the application can route
	// candidate content off the machine.
	//
	// The recorder above cannot itself stand in for that check — httptest
	// listens on 127.0.0.1, which is local and correctly accepted. It stands in
	// for the other half: a reachable endpoint nothing is supposed to talk to.
	for _, remote := range []string{
		"https://api.example-cloud.invalid/v1",
		"http://embeddings.internal.example:8080",
		"https://127.0.0.1.nip.io/v1",
	} {
		if _, err := e.registry.Assign(AssignInput{
			Role: models.RoleEmbed, Endpoint: remote, Model: "cloud-embed",
		}); err == nil {
			t.Fatalf("the embed role accepted the remote endpoint %q", remote)
		}
	}

	// And with the local assignment, the full path runs and the cloud server
	// still records nothing.
	e.assignEmbed(t, "synthetic-embed")
	e.corpus(t, map[string]string{"cv": platformCV, "other": analystCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}
	if _, err := e.embed.SemanticSearch(e.initiative, "billing and payments", 10); err != nil {
		t.Fatalf("searching: %v", err)
	}
	if got := cloudCalls.Load(); got != 0 {
		t.Fatalf("the cloud endpoint recorded %d requests", got)
	}
	if e.endpoint.calls.Load() == 0 {
		t.Fatal("the local endpoint recorded no requests either — the path did not run")
	}
}

func TestCoverageReportsWhatIsOutstanding(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.corpus(t, map[string]string{"cv": platformCV})

	none, err := e.embed.Coverage(e.initiative)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if none.Space != nil {
		t.Error("a space exists before any model is assigned")
	}
	if none.Total == 0 || none.Outstanding != none.Total {
		t.Fatalf("%d of %d embedded with no model assigned", none.Embedded, none.Total)
	}

	e.assignEmbed(t, "synthetic-embed")
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}
	full, err := e.embed.Coverage(e.initiative)
	if err != nil {
		t.Fatalf("coverage: %v", err)
	}
	if full.Space == nil {
		t.Fatal("no space after a successful run")
	}
	if full.Outstanding != 0 || full.Embedded != full.Total {
		t.Fatalf("%d of %d embedded, %d outstanding", full.Embedded, full.Total, full.Outstanding)
	}
}

// The database refuses a vector whose length does not match its dimensions,
// independently of the Go code that is supposed to never produce one.
func TestTheDatabaseRefusesAMisshapenVector(t *testing.T) {
	e := newEmbedEnv(t, 8)
	e.assignEmbed(t, "synthetic-embed")
	e.corpus(t, map[string]string{"cv": platformCV})
	if done := e.embedAllAndWait(t); done.State != models.JobCompleted {
		t.Fatalf("embedding %s (%s)", done.State, done.FailureReason)
	}
	space, err := e.embed.CurrentSpace()
	if err != nil || space == nil {
		t.Fatalf("current space: %v %v", space, err)
	}

	err = e.db.Exec(
		"INSERT INTO embeddings (space_id, owner_kind, owner_id, dimensions, vector) VALUES (?,?,?,?,?)",
		space.ID, models.OwnerChunk, 99999, space.Dimensions, make([]byte, 4*space.Dimensions-1)).Error
	if err == nil {
		t.Fatal("the database accepted a vector one byte short of its dimensions")
	}
	err = e.db.Exec(
		"INSERT INTO embeddings (space_id, owner_kind, owner_id, dimensions, vector) VALUES (?,?,?,?,?)",
		space.ID, models.OwnerChunk, 99998, space.Dimensions+1, make([]byte, 4*(space.Dimensions+1))).Error
	if err == nil {
		t.Fatal("the database accepted a row whose dimensions disagree with its space")
	}
}
