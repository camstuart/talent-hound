package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/vector"
)

// Embedder is the endpoint an embedding comes from. It is an interface for one
// reason: the tests need an endpoint that returns a chosen vector, a NaN, or a
// failure on demand, and standing up a real model to assert a byte length would
// be a slower way to learn less.
type Embedder interface {
	Embed(ctx context.Context, model, text string) ([]float32, error)
}

// EmbedService turns retrieval units into vectors and vectors into results.
//
// Everything hard here is bookkeeping. The arithmetic is in internal/vector and
// is eight lines; what this file does is make sure the numbers it compares were
// ever in the same geometry, which is a database question.
type EmbedService struct {
	db       *gorm.DB
	jobs     *JobService
	registry *ModelService
	endpoint Embedder
}

// embedTimeout bounds one unit's embedding. Generous, because a cold model
// loads on the first call, and bounded, because a job that never finishes is a
// job that cannot be cancelled honestly.
const embedTimeout = 2 * time.Minute

// NewEmbedService returns an embedding service and registers the embed worker.
func NewEmbedService(db *gorm.DB, jobs *JobService, registry *ModelService, endpoint Embedder) *EmbedService {
	s := &EmbedService{db: db, jobs: jobs, registry: registry, endpoint: endpoint}
	jobs.register("embed", s.work)
	return s
}

// embedParams is what an embedding job carries: a kind and identifiers. No
// content, ever — a job row is not a place text belongs.
type embedParams struct {
	OwnerKind models.OwnerKind `json:"ownerKind"`
	OwnerIDs  []uint           `json:"ownerIds"`
}

// EmbedAll queues every chunk of an initiative that the current space does not
// already have a vector for.
//
// Already-embedded units are skipped rather than re-embedded: the vector is
// deterministic for the same text and the same space, so redoing it is minutes
// of local compute to arrive at the bytes already stored.
func (s *EmbedService) EmbedAll(initiativeID uint) (*models.Job, error) {
	space, err := s.CurrentSpace()
	if err != nil {
		return nil, err
	}

	ids := []uint{}
	q := s.db.Model(&models.Chunk{}).
		Where("artifact_id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)",
			models.LinkInitiative, initiativeID)
	if space != nil {
		q = q.Where("id NOT IN (SELECT owner_id FROM embeddings WHERE space_id = ? AND owner_kind = ?)",
			space.ID, models.OwnerChunk)
	}
	if err := q.Order("id asc").Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("listing chunks to embed for initiative %d: %w", initiativeID, err)
	}

	params, err := json.Marshal(embedParams{OwnerKind: models.OwnerChunk, OwnerIDs: ids})
	if err != nil {
		return nil, fmt.Errorf("encoding embedding params: %w", err)
	}
	return s.jobs.Enqueue(JobInput{
		Kind:         "embed",
		InitiativeID: initiativeID,
		Params:       string(params),
		TotalItems:   len(ids),
	})
}

// Coverage reports how much of an initiative's evidence the current space has.
type Coverage struct {
	// Space is nil when no embed model is assigned, or when nothing has been
	// embedded under the current assignment yet — the space is created by the
	// first successful embedding, not by configuring one.
	Space       *models.EmbeddingSpace `json:"space"`
	Total       int64                  `json:"total"`
	Embedded    int64                  `json:"embedded"`
	Outstanding int64                  `json:"outstanding"`
}

// Coverage counts an initiative's chunks and how many the current space holds.
func (s *EmbedService) Coverage(initiativeID uint) (*Coverage, error) {
	out := &Coverage{}
	linked := "artifact_id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)"
	err := s.db.Model(&models.Chunk{}).
		Where(linked, models.LinkInitiative, initiativeID).Count(&out.Total).Error
	if err != nil {
		return nil, fmt.Errorf("counting chunks for initiative %d: %w", initiativeID, err)
	}

	space, err := s.CurrentSpace()
	if err != nil {
		return nil, err
	}
	out.Space = space
	if space != nil {
		err = s.db.Model(&models.Chunk{}).
			Where(linked, models.LinkInitiative, initiativeID).
			Where("id IN (SELECT owner_id FROM embeddings WHERE space_id = ? AND owner_kind = ?)",
				space.ID, models.OwnerChunk).
			Count(&out.Embedded).Error
		if err != nil {
			return nil, fmt.Errorf("counting embedded chunks for initiative %d: %w", initiativeID, err)
		}
	}
	out.Outstanding = out.Total - out.Embedded
	return out, nil
}

// CurrentSpace returns the embedding space of the current embed assignment, or
// nil when there is none yet.
//
// Nil is two different situations that need no distinguishing here: no embed
// model is assigned, or one is and nothing has been embedded through it. Both
// mean the same to a search — there is nothing to scan.
func (s *EmbedService) CurrentSpace() (*models.EmbeddingSpace, error) {
	res, err := s.registry.Resolve(models.RoleEmbed)
	if err != nil {
		return nil, err
	}
	if res.Assignment == nil {
		return nil, nil
	}
	var space models.EmbeddingSpace
	err = s.db.Where("endpoint = ? AND model = ? AND digest = ? AND revision = ? AND metric = ?",
		res.Assignment.Endpoint, res.Assignment.Model, res.Assignment.Digest,
		res.Assignment.Revision, models.MetricCosine).
		First(&space).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading the current embedding space: %w", err)
	}
	return &space, nil
}

// Spaces lists every space, newest first, so the settings view can say what a
// model change left behind.
func (s *EmbedService) Spaces() ([]models.EmbeddingSpace, error) {
	rows := []models.EmbeddingSpace{}
	if err := s.db.Order("id desc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("listing embedding spaces: %w", err)
	}
	return rows, nil
}

// ensureSpace returns the space for the current assignment, creating it with
// the dimensions the endpoint just demonstrated.
//
// Dimensions are learned rather than configured: nothing declares how many a
// model produces, and a configured count that is wrong fails silently — a
// shorter vector decodes fine, compares fine, and means nothing.
func (s *EmbedService) ensureSpace(tx *gorm.DB, a models.ModelAssignment, dims int) (*models.EmbeddingSpace, error) {
	space := models.EmbeddingSpace{
		Endpoint:   a.Endpoint,
		Model:      a.Model,
		Digest:     a.Digest,
		Revision:   a.Revision,
		Dimensions: dims,
		Metric:     models.MetricCosine,
	}
	// The unique index is over the whole identity, so a concurrent creator and
	// this one agree rather than race into two spaces whose vectors never meet.
	err := tx.Where(&models.EmbeddingSpace{
		Endpoint: a.Endpoint, Model: a.Model, Digest: a.Digest,
		Revision: a.Revision, Metric: models.MetricCosine,
	}).Attrs(models.EmbeddingSpace{Dimensions: dims}).FirstOrCreate(&space).Error
	if err != nil {
		return nil, fmt.Errorf("resolving the embedding space for %s: %w", a.Model, err)
	}
	if space.Dimensions != dims {
		// The space was defined by an earlier response and this one disagrees.
		// Loud, because the quiet version is a corpus of vectors that are not
		// the same geometry as each other.
		return nil, fmt.Errorf("the %s space has %d dimensions but the endpoint returned %d",
			a.Model, space.Dimensions, dims)
	}
	return &space, nil
}

// work embeds one retrieval unit. The endpoint call happens with no transaction
// open; only the vector's insertion is inside one, so a cancellation between
// the two writes nothing at all.
func (s *EmbedService) work(ctx context.Context, job models.Job, item int) (JobCommit, error) {
	var p embedParams
	if err := json.Unmarshal([]byte(job.Params), &p); err != nil {
		return nil, FailReason("bad_params")
	}
	if item < 0 || item >= len(p.OwnerIDs) {
		return nil, FailReason(models.ReasonEmbedFailed)
	}
	if p.OwnerKind != models.OwnerChunk {
		// Aspects arrive in Phase 10 and share this storage; nothing produces
		// one yet, so a job naming one is a bug rather than a state.
		return nil, FailReason(models.ReasonEmbedFailed)
	}
	ownerID := p.OwnerIDs[item]

	res, err := s.registry.Resolve(models.RoleEmbed)
	if err != nil || res.Assignment == nil {
		return nil, FailReason(models.ReasonNoEmbedModel)
	}
	assignment := *res.Assignment

	var row models.Chunk
	if err := s.db.Select("id", "text").First(&row, ownerID).Error; err != nil {
		return nil, FailReason(models.ReasonMissingOwner)
	}

	callCtx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()
	v, err := s.endpoint.Embed(callCtx, assignment.Model, row.Text)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// The endpoint's own words can quote the text it was given, so they stay
		// at the endpoint and the job stores a code.
		return nil, FailReason(models.ReasonEndpointFailed)
	}
	// A model that returns an all-zero or non-finite vector has failed. Storing
	// that as a legitimate vector means every future query silently scores it,
	// always identically and always meaninglessly.
	if err := vector.Check(v); err != nil {
		return nil, FailReason(models.ReasonBadVector)
	}

	blob := vector.Encode(v)
	dims := len(v)
	return func(tx *gorm.DB) error {
		space, err := s.ensureSpace(tx, assignment, dims)
		if err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "space_id"}, {Name: "owner_kind"}, {Name: "owner_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"vector", "dimensions", "updated_at"}),
		}).Create(&models.Embedding{
			SpaceID:    space.ID,
			OwnerKind:  models.OwnerChunk,
			OwnerID:    ownerID,
			Dimensions: dims,
			Vector:     blob,
		}).Error
	}, nil
}

// SemanticHit is one scored retrieval unit, with everything needed to cite it.
type SemanticHit struct {
	ChunkID      uint     `json:"chunkId"`
	ArtifactID   uint     `json:"artifactId"`
	ArtifactName string   `json:"artifactName"`
	Ordinal      int      `json:"ordinal"`
	HeadingPath  []string `json:"headingPath"`
	// Text a stranger wrote: displayed, never rendered, never executed.
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	// Which space answered, so a result is never a number without a geometry.
	SpaceID uint `json:"spaceId"`
}

// SemanticSearch embeds the query and scans within one space.
//
// It takes text rather than a vector on purpose. A method accepting two vectors
// is one convenient call site away from comparing last month's geometry with
// this month's, and the result of that call looks entirely normal.
func (s *EmbedService) SemanticSearch(initiativeID uint, query string, limit int) ([]SemanticHit, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	space, err := s.CurrentSpace()
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, fmt.Errorf("nothing is embedded yet for the current model — index this initiative first")
	}
	res, err := s.registry.Resolve(models.RoleEmbed)
	if err != nil {
		return nil, err
	}
	if res.Assignment == nil {
		return nil, fmt.Errorf("no embedding model is assigned")
	}

	ctx, cancel := context.WithTimeout(context.Background(), embedTimeout)
	defer cancel()
	q, err := s.endpoint.Embed(ctx, res.Assignment.Model, query)
	if err != nil {
		return nil, fmt.Errorf("embedding the query: %w", err)
	}
	if err := vector.Check(q); err != nil {
		return nil, fmt.Errorf("the endpoint returned an unusable vector for this query: %w", err)
	}
	if len(q) != space.Dimensions {
		return nil, fmt.Errorf("the query embedding has %d dimensions but the space has %d",
			len(q), space.Dimensions)
	}

	// Scoped in SQL before anything is scored: the scan runs over the rows that
	// could possibly match rather than over the table, which is what keeps an
	// exact scan cheap enough not to need an index.
	type candidate struct {
		ID           uint
		ChunkID      uint
		ArtifactID   uint
		ArtifactName string
		Ordinal      int
		HeadingPath  models.StringList
		Text         string
		Vector       []byte
		Dimensions   int
	}
	rows := []candidate{}
	err = s.db.Raw(`
		SELECT e.id AS id, c.id AS chunk_id, c.artifact_id AS artifact_id,
		       a.display_name AS artifact_name, c.ordinal AS ordinal,
		       c.heading_path AS heading_path, c.text AS text,
		       e.vector AS vector, e.dimensions AS dimensions
		FROM embeddings e
		JOIN chunks c ON c.id = e.owner_id
		JOIN artifacts a ON a.id = c.artifact_id
		WHERE e.space_id = ? AND e.owner_kind = ?
		  AND c.artifact_id IN (
		      SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?)
		ORDER BY e.id ASC`,
		space.ID, models.OwnerChunk, models.LinkInitiative, initiativeID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("scanning embeddings for initiative %d: %w", initiativeID, err)
	}

	hits := make([]SemanticHit, 0, len(rows))
	for _, r := range rows {
		v, err := vector.Decode(r.Vector, space.Dimensions)
		if err != nil {
			return nil, fmt.Errorf("reading the vector for chunk %d: %w", r.ChunkID, err)
		}
		score, err := vector.Cosine(q, v)
		if err != nil {
			return nil, fmt.Errorf("scoring chunk %d: %w", r.ChunkID, err)
		}
		hits = append(hits, SemanticHit{
			ChunkID: r.ChunkID, ArtifactID: r.ArtifactID, ArtifactName: r.ArtifactName,
			Ordinal: r.Ordinal, HeadingPath: r.HeadingPath, Text: r.Text,
			Score: score, SpaceID: space.ID,
		})
	}

	// Score descending, then identifier ascending. Ties are real — identical
	// text embeds identically — and "the shortlist changed and nothing changed"
	// is an expensive thing to debug three phases from now.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ChunkID < hits[j].ChunkID
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
