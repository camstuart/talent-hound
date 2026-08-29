package main

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/chunk"
	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// indexEnv is a database with the retrieval services wired to it and one
// initiative to hang artifacts off.
type indexEnv struct {
	db         *gorm.DB
	jobs       *JobService
	artifacts  *ArtifactService
	chunks     *ChunkService
	search     *SearchService
	initiative uint
}

func newIndexEnv(t *testing.T) *indexEnv {
	t.Helper()
	return newIndexEnvAt(t, filepath.Join(t.TempDir(), "index.db"))
}

// newIndexEnvAt is newIndexEnv on a chosen path, so a test can reopen the same
// file on disk.
func newIndexEnvAt(t *testing.T, path string) *indexEnv {
	t.Helper()
	gdb, err := db.Open(path)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	closeOnCleanup(t, gdb)
	jobs := NewJobService(gdb)
	inits := NewInitiativeService(gdb)
	init, err := inits.Create("Retrieval "+t.Name(), models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating initiative: %v", err)
	}
	return &indexEnv{
		db:         gdb,
		jobs:       jobs,
		artifacts:  NewArtifactService(gdb),
		chunks:     NewChunkService(gdb, jobs),
		search:     NewSearchService(gdb),
		initiative: init.ID,
	}
}

// extracted ingests a text artifact, links it to the initiative, and records it
// as already extracted — this suite is about what happens to Markdown, not
// about producing it.
var ingestCounter atomic.Int64

func (e *indexEnv) extracted(t *testing.T, name, markdown string) *models.Artifact {
	t.Helper()
	// The raw bytes get a unique tail so identical markdown fixtures do not
	// trip the same-target duplicate refusal; the markdown below is verbatim.
	raw := fmt.Sprintf("%s\n<!-- ingest %d -->", markdown, ingestCounter.Add(1))
	a, err := e.artifacts.create(name, name+".md", "test", []byte(raw),
		models.LinkInitiative, e.initiative)
	if err != nil {
		t.Fatalf("ingesting %s: %v", name, err)
	}
	err = e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).Updates(map[string]any{
		"extraction_state":  models.ExtractionExtracted,
		"extractor":         "native-text",
		"extractor_version": "1",
		"markdown":          markdown,
	}).Error
	if err != nil {
		t.Fatalf("recording extraction: %v", err)
	}
	return a
}

// waitForJob polls until a job reaches a final state.
func waitForJob(t *testing.T, jobs *JobService, id uint) models.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobs.Get(id)
		if err != nil {
			t.Fatalf("loading job %d: %v", id, err)
		}
		if job.State.Final() {
			return *job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %d never finished", id)
	return models.Job{}
}

func (e *indexEnv) chunkAndWait(t *testing.T, artifactID uint) []models.Chunk {
	t.Helper()
	job, err := e.chunks.Chunk(artifactID, e.initiative)
	if err != nil {
		t.Fatalf("queuing chunking: %v", err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("chunking job %s (%s)", done.State, done.FailureReason)
	}
	chunks, err := e.chunks.List(artifactID)
	if err != nil {
		t.Fatalf("listing chunks: %v", err)
	}
	return chunks
}

const cvMarkdown = `# Kalinda Reyes

## Experience

Senior platform engineer at Northwind, working on billing and payments.

## Skills

- Go and SQLite
- Distributed systems
`

func TestChunkingStoresProvenance(t *testing.T) {
	e := newIndexEnv(t)
	a := e.extracted(t, "CV", cvMarkdown)
	chunks := e.chunkAndWait(t, a.ID)

	if len(chunks) == 0 {
		t.Fatal("no chunks were stored")
	}
	for i, c := range chunks {
		if c.Ordinal != i {
			t.Errorf("chunk at index %d has ordinal %d", i, c.Ordinal)
		}
		if c.Chunker != chunk.Name || c.ChunkerVersion != chunk.Version || c.ChunkerParams != chunk.Params {
			t.Errorf("chunk %d records %s/%s %s", i, c.Chunker, c.ChunkerVersion, c.ChunkerParams)
		}
		if c.TokenCount == 0 || c.Hash == "" {
			t.Errorf("chunk %d has no token count or hash", i)
		}
		if c.Hash != chunk.Hash(c.Text) {
			t.Errorf("chunk %d hash does not match its text", i)
		}
		if cvMarkdown[c.StartOffset:c.EndOffset] != c.Text {
			t.Errorf("chunk %d offsets do not select its text", i)
		}
	}
}

func TestEveryHeadingPathResolves(t *testing.T) {
	e := newIndexEnv(t)
	a := e.extracted(t, "CV", cvMarkdown)
	for _, c := range e.chunkAndWait(t, a.ID) {
		for _, heading := range c.HeadingPath {
			// Each heading in the path must be a heading that really appears
			// above this chunk in the source.
			if !containsHeading(cvMarkdown[:c.EndOffset], heading) {
				t.Errorf("chunk %d cites the heading %q, which is not above it", c.Ordinal, heading)
			}
		}
	}
}

func containsHeading(md, heading string) bool {
	for _, prefix := range []string{"# ", "## ", "### ", "#### "} {
		if idx := indexOfLine(md, prefix+heading); idx >= 0 {
			return true
		}
	}
	return false
}

func indexOfLine(md, line string) int {
	for i, l := range splitLinesForTest(md) {
		if l == line {
			return i
		}
	}
	return -1
}

func splitLinesForTest(md string) []string {
	var out []string
	start := 0
	for i := 0; i < len(md); i++ {
		if md[i] == '\n' {
			out = append(out, md[start:i])
			start = i + 1
		}
	}
	return append(out, md[start:])
}

func TestRechunkingReplacesEveryChunk(t *testing.T) {
	e := newIndexEnv(t)
	a := e.extracted(t, "CV", cvMarkdown)
	first := e.chunkAndWait(t, a.ID)
	second := e.chunkAndWait(t, a.ID)

	if len(first) != len(second) {
		t.Fatalf("re-chunking produced %d chunks, was %d", len(second), len(first))
	}
	for i := range first {
		if first[i].Text != second[i].Text || first[i].Hash != second[i].Hash {
			t.Errorf("chunk %d changed between runs", i)
		}
		if first[i].ID == second[i].ID {
			t.Errorf("chunk %d was reused rather than replaced", i)
		}
	}
	var n int64
	if err := e.db.Model(&models.Chunk{}).Where("artifact_id = ?", a.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if int(n) != len(second) {
		t.Fatalf("%d chunk rows remain, want %d", n, len(second))
	}
}

func TestAnUnextractedArtifactCannotBeChunked(t *testing.T) {
	e := newIndexEnv(t)
	a, err := e.artifacts.create("Not read", "notes.txt", "test", []byte("some bytes"),
		models.LinkInitiative, e.initiative)
	if err != nil {
		t.Fatal(err)
	}
	job, err := e.chunks.Chunk(a.ID, e.initiative)
	if err != nil {
		t.Fatal(err)
	}
	done := waitForJob(t, e.jobs, job.ID)
	if done.State != models.JobFailed || done.FailureReason != models.ReasonNotExtracted {
		t.Fatalf("job is %s (%q), want failed/not_extracted", done.State, done.FailureReason)
	}
	if chunks, err := e.chunks.List(a.ID); err != nil || len(chunks) != 0 {
		t.Fatalf("got %d chunks (err %v), want none", len(chunks), err)
	}
}

func TestChunkAllCoversTheWorkspace(t *testing.T) {
	e := newIndexEnv(t)
	a := e.extracted(t, "CV", cvMarkdown)
	b := e.extracted(t, "Brief", "# Role brief\n\nWe need a platform engineer in Melbourne.\n")

	job, err := e.chunks.ChunkAll(e.initiative)
	if err != nil {
		t.Fatal(err)
	}
	if job.TotalItems != 2 {
		t.Fatalf("job has %d items, want 2", job.TotalItems)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("job is %s (%q)", done.State, done.FailureReason)
	}
	for _, a := range []*models.Artifact{a, b} {
		if chunks, _ := e.chunks.List(a.ID); len(chunks) == 0 {
			t.Errorf("artifact %d was not chunked", a.ID)
		}
	}
	n, err := e.chunks.CountForInitiative(e.initiative)
	if err != nil || n == 0 {
		t.Fatalf("count %d (err %v)", n, err)
	}
}

func TestChunkAllWithNothingExtractedCompletesEmpty(t *testing.T) {
	e := newIndexEnv(t)
	job, err := e.chunks.ChunkAll(e.initiative)
	if err != nil {
		t.Fatal(err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("an empty batch is a batch that finished, got %s", done.State)
	}
}

func TestCancellingChunkingKeepsCommittedArtifactsOnly(t *testing.T) {
	e := newIndexEnv(t)
	// Enough artifacts that cancellation lands part way through.
	var ids []uint
	for i := 0; i < 12; i++ {
		a := e.extracted(t, "Doc", cvMarkdown)
		ids = append(ids, a.ID)
	}
	job, err := e.chunks.ChunkAll(e.initiative)
	if err != nil {
		t.Fatal(err)
	}
	// Cancel as soon as at least one item has committed, so the assertion is
	// about a partial run rather than an empty one.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		cur, err := e.jobs.Get(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if cur.CompletedItems > 0 && cur.CompletedItems < len(ids) {
			break
		}
		if cur.State.Final() {
			t.Skip("chunking finished before it could be cancelled")
		}
	}
	if err := e.jobs.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	done := waitForJob(t, e.jobs, job.ID)
	if done.State != models.JobCancelled {
		t.Fatalf("job is %s, want cancelled", done.State)
	}

	// Every artifact either has all of its chunks or none of them: an item
	// commits its whole artifact or nothing.
	whole := chunkCount(t, e, ids[0])
	chunked := 0
	for _, id := range ids {
		n := chunkCount(t, e, id)
		if n != 0 && n != whole {
			t.Fatalf("artifact %d has %d chunks, want 0 or %d", id, n, whole)
		}
		if n > 0 {
			chunked++
		}
	}
	if chunked != done.CompletedItems {
		t.Fatalf("%d artifacts have chunks but the job completed %d items", chunked, done.CompletedItems)
	}
}

func chunkCount(t *testing.T, e *indexEnv, artifactID uint) int {
	t.Helper()
	var n int64
	if err := e.db.Model(&models.Chunk{}).Where("artifact_id = ?", artifactID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return int(n)
}

func TestChunkFailureReasonsAreCodes(t *testing.T) {
	e := newIndexEnv(t)
	for _, reason := range []string{models.ReasonNotExtracted, models.ReasonChunkFailed} {
		if !models.ValidReason(reason) {
			t.Errorf("%q is not a storable reason code", reason)
		}
	}
	// And the database refuses anything that is not one.
	err := e.db.Exec(
		"INSERT INTO `jobs` (`kind`,`state`,`params`,`failure_reason`) VALUES ('chunk','failed','{}',?)",
		"could not chunk Kalinda's résumé").Error
	if err == nil {
		t.Fatal("the database stored a sentence as a failure reason")
	}
}
