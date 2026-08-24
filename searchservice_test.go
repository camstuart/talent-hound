package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/extract"
	"camstuart/talent-hound/internal/models"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

const briefMarkdown = `# Platform engineer

## Requirements

Five years building distributed systems in Go. Melbourne based, hybrid.

## Nice to have

Experience with SQLite and payments.
`

// indexed ingests, chunks, and returns the artifact, so a search test starts
// from a searchable workspace.
func (e *indexEnv) indexed(t *testing.T, name, markdown string) *models.Artifact {
	t.Helper()
	a := e.extracted(t, name, markdown)
	e.chunkAndWait(t, a.ID)
	return a
}

func (e *indexEnv) find(t *testing.T, query string) []Hit {
	t.Helper()
	hits, err := e.search.Search(e.initiative, query, 0)
	if err != nil {
		t.Fatalf("searching %q: %v", query, err)
	}
	return hits
}

func TestSearchFindsAChunkAndCitesIt(t *testing.T) {
	e := newIndexEnv(t)
	a := e.indexed(t, "Role brief", briefMarkdown)

	hits := e.find(t, "distributed systems")
	if len(hits) == 0 {
		t.Fatal("no hits for a phrase that is in the document")
	}
	hit := hits[0]
	if hit.ArtifactID != a.ID || hit.ArtifactName != "Role brief" {
		t.Fatalf("hit names artifact %d %q", hit.ArtifactID, hit.ArtifactName)
	}
	if !strings.Contains(hit.Text, "distributed systems") {
		t.Fatalf("hit text does not contain the terms: %q", hit.Text)
	}

	cite, err := e.search.Cite(hit.ChunkID)
	if err != nil {
		t.Fatalf("citing: %v", err)
	}
	if cite.Text != hit.Text {
		t.Error("the citation's text differs from the hit's")
	}
	want := models.StringList{"Platform engineer", "Requirements"}
	if !slices.Equal(cite.HeadingPath, want) {
		t.Errorf("heading path %v, want %v", cite.HeadingPath, want)
	}
	for _, part := range []string{"Role brief", "Requirements", "section"} {
		if !strings.Contains(cite.Location, part) {
			t.Errorf("location %q does not name %q", cite.Location, part)
		}
	}
}

func TestACitationResolvesAgainstTheSource(t *testing.T) {
	e := newIndexEnv(t)
	a := e.indexed(t, "Role brief", briefMarkdown)
	chunks, err := e.chunks.List(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chunks {
		cite, err := e.search.Cite(c.ID)
		if err != nil {
			t.Fatalf("citing chunk %d: %v", c.ID, err)
		}
		if briefMarkdown[cite.StartOffset:cite.EndOffset] != cite.Text {
			t.Errorf("chunk %d offsets do not select its text", c.Ordinal)
		}
	}
}

func TestAStaleCitationFailsRatherThanMisleading(t *testing.T) {
	e := newIndexEnv(t)
	a := e.indexed(t, "Role brief", briefMarkdown)
	chunks, _ := e.chunks.List(a.ID)
	target := chunks[len(chunks)-1]

	// The Markdown moves under the chunk without the chunks being discarded —
	// the situation the invalidation rule exists to prevent. The citation must
	// fail, not quietly quote a different part of the document.
	shifted := "An inserted paragraph nobody asked for.\n\n" + briefMarkdown
	if err := e.db.Model(&models.Artifact{}).Where("id = ?", a.ID).
		UpdateColumn("markdown", shifted).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := e.search.Cite(target.ID); err == nil {
		t.Fatal("a stale citation resolved")
	}
}

func TestSearchIsScopedToTheInitiativeThatAsked(t *testing.T) {
	e := newIndexEnv(t)
	e.indexed(t, "Role brief", briefMarkdown)

	// A second workspace with its own evidence.
	other, err := NewInitiativeService(e.db).Create("Other", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A term that appears nowhere else, so the assertion is about scope only.
	otherWorkspaceText := "# Private\n\nAn unmistakable term: quokkasandwich.\n"
	b, err := e.artifacts.create("Other CV", "other.md", "test", []byte(otherWorkspaceText), models.LinkInitiative, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.db.Model(&models.Artifact{}).Where("id = ?", b.ID).Updates(map[string]any{
		"extraction_state": models.ExtractionExtracted, "markdown": otherWorkspaceText,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if job, err := e.chunks.Chunk(b.ID, other.ID); err != nil {
		t.Fatal(err)
	} else if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("chunking the other workspace: %s", done.State)
	}

	if hits := e.find(t, "quokkasandwich"); len(hits) != 0 {
		t.Fatalf("a search in one workspace returned %d hits from another", len(hits))
	}
	hits, err := e.search.Search(other.ID, "quokkasandwich", 0)
	if err != nil || len(hits) != 1 {
		t.Fatalf("the owning workspace got %d hits (err %v)", len(hits), err)
	}
}

func TestQueriesAreTextRatherThanSyntax(t *testing.T) {
	e := newIndexEnv(t)
	e.indexed(t, "Role brief", briefMarkdown)

	// No input is a syntax error, and none of it is syntax. Where a query
	// finds nothing, that is because one of its words is genuinely absent —
	// which is itself the proof that the word was searched for rather than
	// obeyed: `text:Melbourne` as a column filter would have matched.
	cases := []struct {
		name  string
		query string
		want  bool
		why   string
	}{
		{"plain", "melbourne", true, "both the term and the document have it"},
		{"punctuation", "Melbourne, hybrid.", true, "punctuation separates terms"},
		{"unbalanced parenthesis", "systems (Go", true, "a bracket is not an expression"},
		{"quotes", `"distributed systems"`, true, "quotes separate terms rather than phrasing them"},
		{"hyphen", "Melbourne-based", true, "a hyphen separates, as it does in the index"},
		{"prefix operator", "Melbourne*", true, "the asterisk is a separator, leaving one term"},
		{"apostrophe", "Melbourne's", false, "the stray s is a term of its own, and absent"},
		{"operator words", "Melbourne NOT hybrid", false, "NOT is a word, and this document has none"},
		{"fts column filter", "text:Melbourne", false, "as syntax it would match; as words it does not"},
		{"near function", "NEAR(Melbourne hybrid)", false, "NEAR is a word here"},
		{"sql injection shaped", "'; DROP TABLE chunks; --", false, "words that are not in the document"},
		{"empty", "", false, "nothing was asked"},
		{"whitespace", "   \t\n", false, "nothing was asked"},
		{"punctuation only", "()*\"", false, "no terms at all"},
		{"absent term", "kangaroo", false, "not in the document"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := e.search.Search(e.initiative, tc.query, 0)
			if err != nil {
				t.Fatalf("query %q failed: %v", tc.query, err)
			}
			if got := len(hits) > 0; got != tc.want {
				t.Errorf("query %q returned %d hits, want any=%v (%s)", tc.query, len(hits), tc.want, tc.why)
			}
		})
	}

	// The injection-shaped query is a query, not a statement.
	var n int64
	if err := e.db.Model(&models.Chunk{}).Count(&n).Error; err != nil || n == 0 {
		t.Fatalf("the chunks table is gone: %d rows, err %v", n, err)
	}
}

func TestUnicodeTermsAreSearchable(t *testing.T) {
	e := newIndexEnv(t)
	e.indexed(t, "Notes", "# 経歴\n\nWorked in 東京 on a résumé parser. Работал в Москве.\n")
	for _, q := range []string{"東京", "résumé", "Москве"} {
		if hits := e.find(t, q); len(hits) == 0 {
			t.Errorf("no hits for %q", q)
		}
	}
}

func TestACommonTermReturnsUpToTheLimit(t *testing.T) {
	e := newIndexEnv(t)
	for i := 0; i < 8; i++ {
		e.indexed(t, "Doc", "# Section\n\nthe common word appears here too.\n")
	}
	hits, err := e.search.Search(e.initiative, "the", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 {
		t.Fatalf("got %d hits, want the requested limit of 3", len(hits))
	}
}

func TestIndexFollowsChunkWrites(t *testing.T) {
	e := newIndexEnv(t)
	a := e.indexed(t, "Role brief", briefMarkdown)
	chunks, _ := e.chunks.List(a.ID)
	target := chunks[0]

	// Update: the new words match, the old ones do not.
	if err := e.db.Model(&models.Chunk{}).Where("id = ?", target.ID).
		UpdateColumn("text", "replaced with wombatnotation").Error; err != nil {
		t.Fatal(err)
	}
	if hits := e.find(t, "wombatnotation"); len(hits) != 1 {
		t.Fatalf("the updated text is not searchable: %d hits", len(hits))
	}
	if hits := e.find(t, "Platform"); len(hits) != 0 {
		t.Fatalf("the replaced text is still searchable: %d hits", len(hits))
	}

	// Delete: nothing left to find.
	if err := e.db.Delete(&models.Chunk{}, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if hits := e.find(t, "wombatnotation"); len(hits) != 0 {
		t.Fatalf("a deleted chunk is still searchable: %d hits", len(hits))
	}
}

func TestARollbackLeavesNoIndexEntries(t *testing.T) {
	e := newIndexEnv(t)
	a := e.extracted(t, "Role brief", briefMarkdown)

	rollback := e.db.Transaction(func(tx *gorm.DB) error {
		row := models.Chunk{
			ArtifactID: a.ID, Ordinal: 0, Text: "rolled back numbatphrase",
			StartOffset: 0, EndOffset: 0, TokenCount: 3, Hash: "x",
			Chunker: "test", ChunkerVersion: "1",
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return errRollbackOnPurpose
	})
	if rollback == nil {
		t.Fatal("the transaction did not roll back")
	}
	if hits := e.find(t, "numbatphrase"); len(hits) != 0 {
		t.Fatalf("a rolled-back chunk is searchable: %d hits", len(hits))
	}
}

var errRollbackOnPurpose = errTest("rolled back on purpose")

type errTest string

func (e errTest) Error() string { return string(e) }

func TestReExtractionRemovesChunksFromTheIndex(t *testing.T) {
	e := newIndexEnv(t)
	a := e.indexed(t, "Role brief", briefMarkdown)
	if hits := e.find(t, "distributed"); len(hits) == 0 {
		t.Fatal("the document was not indexed to begin with")
	}

	// Extraction is recorded again, as a retry does before its attempt runs.
	extraction := NewExtractService(e.db, e.jobs, t.TempDir())
	if err := extraction.setState(e.db, a.ID, models.ExtractionPending, "", extract.Result{}); err != nil {
		t.Fatal(err)
	}
	if hits := e.find(t, "distributed"); len(hits) != 0 {
		t.Fatalf("chunks outlived the markdown they came from: %d hits", len(hits))
	}
	if n := chunkCount(t, e, a.ID); n != 0 {
		t.Fatalf("%d chunk rows survived re-extraction", n)
	}
}

func TestRebuildLeavesResultsUnchangedOnDisk(t *testing.T) {
	// Disk-backed on purpose: an external-content index and its rebuild are
	// about what is written, not about what an in-memory database remembers.
	path := filepath.Join(t.TempDir(), "rebuild.db")
	e := newIndexEnvAt(t, path)
	e.indexed(t, "Role brief", briefMarkdown)
	e.indexed(t, "CV", cvMarkdown)

	before := e.find(t, "systems")
	if len(before) == 0 {
		t.Fatal("nothing to compare")
	}
	if err := e.search.Rebuild(); err != nil {
		t.Fatalf("rebuilding: %v", err)
	}
	after := e.find(t, "systems")
	if len(after) != len(before) {
		t.Fatalf("got %d hits after the rebuild, %d before", len(after), len(before))
	}
	for i := range before {
		if before[i].ChunkID != after[i].ChunkID {
			t.Fatalf("hit %d is chunk %d after the rebuild, %d before", i, after[i].ChunkID, before[i].ChunkID)
		}
	}
}

func TestRebuildRepairsADamagedIndex(t *testing.T) {
	e := newIndexEnvAt(t, filepath.Join(t.TempDir(), "repair.db"))
	e.indexed(t, "Role brief", briefMarkdown)
	before := e.find(t, "systems")
	if len(before) == 0 {
		t.Fatal("nothing to damage")
	}

	// Emptying the index without touching the rows is exactly the drift an
	// external-content table can suffer.
	if err := e.db.Exec(`INSERT INTO chunks_fts(chunks_fts) VALUES ('delete-all')`).Error; err != nil {
		t.Fatal(err)
	}
	if hits := e.find(t, "systems"); len(hits) != 0 {
		t.Fatalf("the index was not emptied: %d hits", len(hits))
	}
	if err := e.search.Rebuild(); err != nil {
		t.Fatal(err)
	}
	if hits := e.find(t, "systems"); len(hits) != len(before) {
		t.Fatalf("the rebuild restored %d of %d hits", len(hits), len(before))
	}
}

func TestPeopleSearchGroupsHitsByCandidate(t *testing.T) {
	e := newIndexEnv(t)
	mkCandidate := func(name string) uint {
		t.Helper()
		c := models.Candidate{FullName: name}
		if err := e.db.Create(&c).Error; err != nil {
			t.Fatalf("candidate: %v", err)
		}
		return c.ID
	}
	attach := func(candidate uint, name, markdown string) {
		t.Helper()
		a := e.extracted(t, name, markdown)
		if err := e.db.Create(&models.ArtifactLink{
			ArtifactID: a.ID, TargetType: models.LinkCandidate, TargetID: candidate,
		}).Error; err != nil {
			t.Fatalf("linking: %v", err)
		}
		e.chunkAndWait(t, a.ID)
	}

	alice := mkCandidate("Alice Amber")
	bob := mkCandidate("Bob Blue")
	attach(alice, "alice-resume", "# Resume\n\nDeep quokkastack experience across two startups.\n\nAlso quokkastack platform work.")
	attach(bob, "bob-note", "# Note\n\nSome quokkastack exposure, mostly wombatscale.")
	// An initiative-only artifact must not appear: it belongs to no candidate.
	e.chunkAndWait(t, e.extracted(t, "brief", "# Brief\n\nquokkastack quokkastack quokkastack").ID)

	hits, err := e.search.People("quokkastack", 10)
	if err != nil {
		t.Fatalf("people search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 people, got %d: %+v", len(hits), hits)
	}
	// One entry per candidate, each with a snippet and a citable chunk.
	seen := map[uint]bool{}
	for _, h := range hits {
		if seen[h.Candidate.ID] {
			t.Fatalf("candidate %d appears twice", h.Candidate.ID)
		}
		seen[h.Candidate.ID] = true
		if h.Snippet == "" || h.ChunkID == 0 || h.Candidate.FullName == "" {
			t.Fatalf("incomplete hit: %+v", h)
		}
		if _, err := e.search.Cite(h.ChunkID); err != nil {
			t.Fatalf("citing %d: %v", h.ChunkID, err)
		}
	}
	if !seen[alice] || !seen[bob] {
		t.Fatalf("missing a candidate: %v", seen)
	}
}

func TestPeopleSearchWithNoMatchesIsEmptyNotAnError(t *testing.T) {
	e := newIndexEnv(t)
	hits, err := e.search.People("zyzzyva", 10)
	if err != nil || len(hits) != 0 {
		t.Fatalf("want empty, got %v / %v", hits, err)
	}
}
