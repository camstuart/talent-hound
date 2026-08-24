package main

import (
	"fmt"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/chunk"
	"camstuart/talent-hound/internal/models"
)

// SearchService is the lexical half of retrieval: FTS5 over chunks, and the
// resolution of a hit back to a place a person can look.
//
// The semantic half — embeddings, exact cosine, and the fusion of the two
// rankings — belongs to Phases 9 and 15. Nothing here anticipates them.
type SearchService struct {
	db *gorm.DB
}

// NewSearchService returns a SearchService backed by db.
func NewSearchService(db *gorm.DB) *SearchService { return &SearchService{db: db} }

// defaultSearchLimit bounds a result set the recruiter is reading by eye.
const defaultSearchLimit = 20

// Hit is one search result: enough to show, and the identity needed to cite it.
// Text is a slice of a document a stranger wrote, so it is displayed and
// nothing else.
type Hit struct {
	ChunkID      uint              `json:"chunkId"`
	ArtifactID   uint              `json:"artifactId"`
	ArtifactName string            `json:"artifactName"`
	Ordinal      int               `json:"ordinal"`
	HeadingPath  models.StringList `json:"headingPath"`
	Text         string            `json:"text"`
}

// Search returns the chunks matching every term of query, among the artifacts
// linked to one initiative.
//
// The scope is deliberate: searching every chunk in the database would put one
// candidate's CV into another engagement's results, which in a recruiting tool
// is not a complaint about relevance.
//
// ponytail: the scope is the initiative's own links; widen the subquery to its
// candidate and roles when those pipelines exist and their evidence needs
// finding from here.
func (s *SearchService) Search(initiativeID uint, query string, limit int) ([]Hit, error) {
	return s.search(initiativeID, ftsQuery(query), limit)
}

// SearchAny finds sections containing any of the query's words, best first.
//
// The AND of every term is right for a keyword search — a recruiter typing two
// words wants both — and wrong for a question, where "how many years of
// quokkastack do they have" would require the document to contain "how". So a
// question ORs its terms and lets bm25 decide which sections are worth reading.
func (s *SearchService) SearchAny(initiativeID uint, query string, limit int) ([]Hit, error) {
	return s.search(initiativeID, ftsAnyQuery(query), limit)
}

func (s *SearchService) search(initiativeID uint, match string, limit int) ([]Hit, error) {
	hits := []Hit{}
	if match == "" {
		return hits, nil
	}
	if limit <= 0 || limit > 200 {
		limit = defaultSearchLimit
	}
	err := s.db.Raw(`
		SELECT c.id AS chunk_id, c.artifact_id, a.display_name AS artifact_name,
		       c.ordinal, c.heading_path, c.text
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		JOIN artifacts a ON a.id = c.artifact_id
		WHERE chunks_fts MATCH ?
		  AND c.artifact_id IN (
		      SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)
		ORDER BY bm25(chunks_fts), c.id
		LIMIT ?`,
		match, models.LinkInitiative, initiativeID, limit).Scan(&hits).Error
	if err != nil {
		return nil, fmt.Errorf("searching: %w", err)
	}
	return hits, nil
}

// Citation is a chunk resolved back to its source: what it says, which artifact
// it came from, and where in that artifact to look.
type Citation struct {
	ChunkID     uint              `json:"chunkId"`
	ArtifactID  uint              `json:"artifactId"`
	Artifact    string            `json:"artifact"`
	Filename    string            `json:"filename"`
	Ordinal     int               `json:"ordinal"`
	HeadingPath models.StringList `json:"headingPath"`
	StartOffset int               `json:"startOffset"`
	EndOffset   int               `json:"endOffset"`
	Text        string            `json:"text"`
	// Location is the same information as one line a person can read.
	Location string `json:"location"`
}

// Cite resolves one chunk against the artifact's Markdown before returning it.
// The offsets are checked rather than trusted: a citation that quietly points
// at the wrong sentence is worse than one that fails.
func (s *SearchService) Cite(chunkID uint) (*Citation, error) {
	var c models.Chunk
	if err := s.db.First(&c, chunkID).Error; err != nil {
		return nil, fmt.Errorf("loading chunk %d: %w", chunkID, err)
	}
	var artifact models.Artifact
	err := s.db.Select("id", "display_name", "original_filename", "markdown").
		First(&artifact, c.ArtifactID).Error
	if err != nil {
		return nil, fmt.Errorf("loading artifact %d: %w", c.ArtifactID, err)
	}

	stored := chunk.Chunk{
		Ordinal: c.Ordinal, Text: c.Text,
		Start: c.StartOffset, End: c.EndOffset, Hash: c.Hash,
	}
	if err := chunk.Verify(artifact.Markdown, stored); err != nil {
		return nil, fmt.Errorf("citation for chunk %d is stale: %w", chunkID, err)
	}

	return &Citation{
		ChunkID:     c.ID,
		ArtifactID:  artifact.ID,
		Artifact:    artifact.DisplayName,
		Filename:    artifact.OriginalFilename,
		Ordinal:     c.Ordinal,
		HeadingPath: c.HeadingPath,
		StartOffset: c.StartOffset,
		EndOffset:   c.EndOffset,
		Text:        c.Text,
		Location:    location(artifact.DisplayName, c.HeadingPath, c.Ordinal),
	}, nil
}

// Rebuild restores the lexical index from the chunk rows. It is exposed rather
// than hidden because a rebuild path nobody can run is not a rebuild path.
func (s *SearchService) Rebuild() error {
	if err := s.db.Exec(`INSERT INTO chunks_fts(chunks_fts) VALUES ('rebuild')`).Error; err != nil {
		return fmt.Errorf("rebuilding the search index: %w", err)
	}
	return nil
}

// location renders a citation as one readable line.
func location(artifact string, path models.StringList, ordinal int) string {
	if len(path) == 0 {
		return fmt.Sprintf("%s — section %d", artifact, ordinal+1)
	}
	return fmt.Sprintf("%s — %s (section %d)", artifact, strings.Join(path, " › "), ordinal+1)
}

// ftsAnyQuery is ftsQuery with OR between the terms, for questions and for the
// sentences a profile aspect is made of.
//
// The common words come out first. ORing every word of "Ran the platform team's
// shared services in Go" asks for any document containing "the", which is every
// document — and measured against the frozen corpus one security listing
// reached the top five of four unrelated candidates that way. What is left is
// the words that carry the meaning.
func ftsAnyQuery(query string) string {
	terms := strings.Fields(ftsQuery(query))
	kept := make([]string, 0, len(terms))
	for _, t := range terms {
		if commonWords[strings.ToLower(strings.Trim(t, `"`))] {
			continue
		}
		kept = append(kept, t)
	}
	// A query of nothing but common words is still that query: better a weak
	// match than none at all.
	if len(kept) == 0 {
		kept = terms
	}
	return strings.Join(kept, " OR ")
}

// commonWords are the words that carry no retrieval signal in a listing or a
// resume. Deliberately short and English-only: this is a stop list, not a
// linguistics project.
var commonWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "been": true, "both": true, "but": true, "by": true, "for": true,
	"from": true, "had": true, "has": true, "have": true, "in": true, "into": true,
	"is": true, "it": true, "its": true, "of": true, "on": true, "or": true,
	"our": true, "out": true, "over": true, "that": true, "the": true, "their": true,
	"them": true, "then": true, "there": true, "these": true, "they": true,
	"this": true, "to": true, "up": true, "was": true, "were": true, "which": true,
	"with": true, "within": true, "would": true, "you": true, "your": true,
	// Words every listing and every resume contains, which is the same problem.
	"experience": true, "role": true, "work": true, "working": true, "team": true,
	"teams": true, "years": true, "year": true, "including": true, "across": true,
}

// ftsQuery turns whatever the recruiter typed into an FTS5 expression that
// searches for their words.
//
// MATCH takes an expression language with operators, prefixes, column filters,
// and NEAR. A recruiter typing `senior engineer (contract)` is not writing an
// expression: unmodified, an unbalanced parenthesis is a database error and a
// stray NOT silently changes what they searched for. So every term becomes a
// quoted string literal and they are ANDed. Every possible input is a search.
func ftsQuery(query string) string {
	terms := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	quoted := make([]string, 0, len(terms))
	for _, t := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(t, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " ")
}

// PersonHit is one candidate found through their evidence: who, and the best
// piece of why. Snippet is a slice of a document a stranger may have written,
// so it is displayed and nothing else.
type PersonHit struct {
	Candidate    models.Candidate `json:"candidate"`
	ChunkID      uint             `json:"chunkId"`
	ArtifactName string           `json:"artifactName"`
	Snippet      string           `json:"snippet"`
}

// People searches the whole talent pool: every chunk whose artifact is linked
// to a candidate, in any initiative or none. One entry per candidate, ranked by
// their best chunk, that chunk carried as the "why" and citable through Cite.
//
// ponytail: FTS only, best-chunk-wins per person. If ranking disappoints, the
// upgrade is a per-person index behind this same signature.
func (s *SearchService) People(query string, limit int) ([]PersonHit, error) {
	match := ftsAnyQuery(query)
	if match == "" {
		return []PersonHit{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = defaultSearchLimit
	}
	type row struct {
		CandidateID  uint
		ChunkID      uint
		ArtifactName string
		Text         string
	}
	rows := []row{}
	// Over-fetch chunks, then keep each candidate's best: a popular candidate
	// with many matching chunks must not crowd everyone else out.
	err := s.db.Raw(`
		SELECT l.target_id AS candidate_id, c.id AS chunk_id,
		       a.display_name AS artifact_name, c.text
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		JOIN artifacts a ON a.id = c.artifact_id
		JOIN artifact_links l ON l.artifact_id = c.artifact_id AND l.target_type = ?
		WHERE chunks_fts MATCH ?
		ORDER BY bm25(chunks_fts), c.id
		LIMIT ?`,
		models.LinkCandidate, match, limit*10).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("searching people: %w", err)
	}

	hits := []PersonHit{}
	seen := map[uint]bool{}
	ids := []uint{}
	for _, r := range rows {
		if seen[r.CandidateID] {
			continue
		}
		seen[r.CandidateID] = true
		ids = append(ids, r.CandidateID)
		hits = append(hits, PersonHit{
			Candidate:    models.Candidate{ID: r.CandidateID},
			ChunkID:      r.ChunkID,
			ArtifactName: r.ArtifactName,
			Snippet:      r.Text,
		})
		if len(hits) == limit {
			break
		}
	}
	if len(ids) == 0 {
		return hits, nil
	}
	people := []models.Candidate{}
	if err := s.db.Where("id IN ?", ids).Find(&people).Error; err != nil {
		return nil, fmt.Errorf("loading matched candidates: %w", err)
	}
	byID := map[uint]models.Candidate{}
	for _, p := range people {
		byID[p.ID] = p
	}
	for i := range hits {
		hits[i].Candidate = byID[hits[i].Candidate.ID]
	}
	return hits, nil
}
