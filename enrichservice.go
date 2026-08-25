package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"camstuart/talent-hound/internal/enrich"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// Enricher reads a public footprint by login. An interface so a fake answers
// the tests; *platform.GitHub satisfies it.
type Enricher interface {
	Profile(ctx context.Context, login string) (*platform.GitHubProfile, error)
	Repos(ctx context.Context, login string) ([]platform.GitHubRepo, error)
	Events(ctx context.Context, login string) ([]platform.GitHubEvent, error)
}

// EnrichService reads what a candidate's public handles say and keeps it as
// evidence.
//
// It writes artifacts, never candidate fields: what someone's repositories
// say about their skills is a claim with a source, which is what a profile
// aspect is and a column is not. Every run discloses one thing — a handle —
// and the audit row says that a handle went, never which.
type EnrichService struct {
	db *gorm.DB
	// github, when set, is used instead of a client built from the stored
	// credential. Only tests set it.
	github    Enricher
	out       *outbound
	records   *RecordService
	artifacts *ArtifactService
	now       Clock
	// Guard refuses personal data in demo scope and on an unencrypted volume.
	// An identity names a person; the evidence a run keeps is guarded by the
	// artifact store itself.
	Guard DataGuard
}

// NewEnrichService wires enrichment to the records and the evidence store.
func NewEnrichService(
	db *gorm.DB, github Enricher, records *RecordService,
	artifacts *ArtifactService, credentials *CredentialService,
) *EnrichService {
	return &EnrichService{
		db: db, github: github, records: records, artifacts: artifacts,
		out: &outbound{db: db, credentials: credentials},
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Identities lists a candidate's public handles.
func (s *EnrichService) Identities(candidateID uint) ([]models.Identity, error) {
	rows := []models.Identity{}
	err := s.db.Where("candidate_id = ?", candidateID).Order("provider asc, handle asc").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing identities: %w", err)
	}
	return rows, nil
}

// AddIdentity records a handle from its URL. The handle is parsed from the
// URL where the host has handles; elsewhere it is the host.
func (s *EnrichService) AddIdentity(candidateID uint, provider, rawURL string) (*models.Identity, error) {
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
	if _, err := s.records.GetCandidate(candidateID); err != nil {
		return nil, err
	}
	detected, handle := identityFromURL(rawURL)
	if provider == "" {
		provider = detected
	}
	if provider == "" {
		provider = models.IdentityWebsite
	}
	if handle == "" || detected != provider {
		handle = hostOf(rawURL)
		if provider == models.IdentityGitHub {
			return nil, fmt.Errorf("a GitHub identity is a profile address: github.com/<login>")
		}
	}
	identity := models.Identity{CandidateID: candidateID, Provider: provider, Handle: handle, URL: rawURL}
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	if err := s.db.Create(&identity).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("that %s handle already belongs to a candidate", provider)
		}
		return nil, fmt.Errorf("recording the identity: %w", err)
	}
	return &identity, nil
}

// RemoveIdentity forgets a handle. The evidence already gathered from it
// stays: it was true when it was read.
func (s *EnrichService) RemoveIdentity(id uint) error {
	if err := s.db.Delete(&models.Identity{}, id).Error; err != nil {
		return fmt.Errorf("removing the identity: %w", err)
	}
	return nil
}

// EnrichPreview is what a run would disclose and where.
type EnrichPreview struct {
	// Handles are the GitHub logins that would be sent.
	Handles []string `json:"handles"`
	// Endpoints are what would be called, per handle.
	Endpoints   []string `json:"endpoints"`
	TokenStored bool     `json:"tokenStored"`
	// Reason says why a run would be refused. Empty when it would proceed.
	Reason string `json:"reason"`
}

// githubEndpoints are the calls a run makes, in order.
var githubEndpoints = []string{"/users/{login}", "/users/{login}/repos", "/users/{login}/events/public"}

// Preview says what Run would do, and writes nothing.
func (s *EnrichService) Preview(candidateID uint) (*EnrichPreview, error) {
	out := &EnrichPreview{Handles: []string{}, Endpoints: append([]string(nil), githubEndpoints...)}
	logins, err := s.logins(candidateID)
	if err != nil {
		return nil, err
	}
	out.Handles = logins
	if s.github != nil {
		out.TokenStored = true
	} else if _, err := s.out.key(models.ProviderGitHub); err == nil {
		out.TokenStored = true
	}
	switch {
	case len(logins) == 0:
		out.Reason = "this candidate has no GitHub identity to read"
	case !out.TokenStored:
		out.Reason = "no GitHub token is stored — add one in Settings"
	}
	return out, nil
}

// EnrichOutcome is what a run produced.
type EnrichOutcome struct {
	ArtifactIDs []uint `json:"artifactIds"`
	// Unchanged counts answers identical to evidence already held.
	Unchanged int `json:"unchanged"`
	// Partial says a later request failed after an earlier one landed.
	Partial       bool   `json:"partial"`
	FailureReason string `json:"failureReason"`
}

// Run reads every GitHub identity and keeps what it says as artifacts.
//
// Refused before anything leaves when there is no handle or no token. One
// disclosure per run, written after the first request is made; a run that
// fails on its first request discloses nothing because nothing was sent.
func (s *EnrichService) Run(candidateID uint) (*EnrichOutcome, error) {
	preview, err := s.Preview(candidateID)
	if err != nil {
		return nil, err
	}
	if preview.Reason != "" {
		return nil, errors.New(preview.Reason)
	}
	client, err := s.client()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*searchTimeout)
	defer cancel()

	out := &EnrichOutcome{ArtifactIDs: []uint{}}
	startedAt := s.now()
	disclosed := false
	disclose := func() error {
		if disclosed {
			return nil
		}
		disclosed = true
		id := candidateID
		return s.out.record(models.DisclosureEvent{
			OccurredAt: startedAt, Provider: models.ProviderGitHub, Task: models.TaskCandidateEnrich,
			Categories: "public handle", CandidateID: &id,
		})
	}

	for _, login := range preview.Handles {
		profile, err := client.Profile(ctx, login)
		if derr := disclose(); derr != nil {
			return nil, derr
		}
		if err != nil {
			return s.partial(out, err)
		}
		if err := s.keep(candidateID, out, "GitHub profile: "+login, "github:"+login+"/profile", enrich.Profile(profile)); err != nil {
			return nil, err
		}
		s.verified(login)

		repos, err := client.Repos(ctx, login)
		if err != nil {
			return s.partial(out, err)
		}
		if err := s.keep(candidateID, out, "GitHub repositories: "+login, "github:"+login+"/repos", enrich.Repos(login, repos)); err != nil {
			return nil, err
		}

		events, err := client.Events(ctx, login)
		if err != nil {
			return s.partial(out, err)
		}
		if err := s.keep(candidateID, out, "GitHub activity: "+login, "github:"+login+"/activity", enrich.Events(login, events)); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// keep ingests one rendering as evidence. The same bytes already under this
// candidate is a repeat, counted rather than refused.
func (s *EnrichService) keep(candidateID uint, out *EnrichOutcome, name, source, markdown string) error {
	a, err := s.artifacts.create(name, "", source, []byte(markdown), models.LinkCandidate, candidateID)
	if err != nil {
		if strings.Contains(err.Error(), "already") {
			out.Unchanged++
			return nil
		}
		return err
	}
	out.ArtifactIDs = append(out.ArtifactIDs, a.ID)
	return nil
}

// partial reports a run that stopped after something landed.
func (s *EnrichService) partial(out *EnrichOutcome, err error) (*EnrichOutcome, error) {
	out.Partial = len(out.ArtifactIDs)+out.Unchanged > 0
	out.FailureReason = err.Error()
	if !out.Partial {
		return nil, err
	}
	return out, nil
}

// verified stamps the identity with today: the provider confirmed the login.
func (s *EnrichService) verified(login string) {
	_ = s.db.Model(&models.Identity{}).
		Where("provider = ? AND handle = ?", models.IdentityGitHub, login).
		Update("verified_at", s.now().Format("2006-01-02")).Error
}

func (s *EnrichService) logins(candidateID uint) ([]string, error) {
	logins := []string{}
	err := s.db.Model(&models.Identity{}).
		Where("candidate_id = ? AND provider = ?", candidateID, models.IdentityGitHub).
		Order("handle asc").Pluck("handle", &logins).Error
	if err != nil {
		return nil, fmt.Errorf("listing GitHub identities: %w", err)
	}
	return logins, nil
}

// client is the provider this run will use, built per request from the
// stored credential — see DiscoveryService.searcher for why.
func (s *EnrichService) client() (Enricher, error) {
	if s.github != nil {
		return s.github, nil
	}
	token, err := s.out.key(models.ProviderGitHub)
	if err != nil {
		return nil, err
	}
	return platform.NewGitHub(token, &dbETagCache{db: s.db, now: s.now}), nil
}

// dbETagCache keeps provider validators in the database.
type dbETagCache struct {
	db  *gorm.DB
	now Clock
}

func (c *dbETagCache) Get(url string) (string, []byte, bool) {
	var row models.HTTPCache
	if err := c.db.First(&row, "url = ?", url).Error; err != nil {
		return "", nil, false
	}
	return row.ETag, row.Body, true
}

func (c *dbETagCache) Put(url, etag string, body []byte) error {
	return c.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "url"}},
		DoUpdates: clause.AssignmentColumns([]string{"etag", "body", "fetched_at"}),
	}).Create(&models.HTTPCache{URL: url, ETag: etag, Body: body, FetchedAt: c.now()}).Error
}
