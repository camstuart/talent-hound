package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
	"camstuart/talent-hound/internal/scrub"
)

// PeopleSearcher is the provider that finds pages about people.
type PeopleSearcher interface {
	SearchPeople(ctx context.Context, query string, limit int, cursor string) (*platform.SearchResponse, error)
}

// SourcingService finds people for a role.
//
// It is the mirror of DiscoveryService: that one takes a candidate and looks
// for roles, this one takes a role and looks for people. What leaves the
// machine is the role's requirements, scrubbed of the client's name and its
// contacts, and shown to the recruiter byte for byte before it goes. What
// comes back is a lead — a URL and a snippet — and never a candidate until the
// recruiter says so.
type SourcingService struct {
	db *gorm.DB
	// exa, when set, is used instead of a client built from the stored
	// credential. Only tests set it.
	exa       PeopleSearcher
	out       *outbound
	roles     *RoleProfileService
	records   *RecordService
	artifacts *ArtifactService
	now       Clock
	// Guard refuses personal data in demo scope and on an unencrypted volume.
	// A promoted lead is personal data: the refusal is at the write, as it is
	// for a candidate typed in.
	Guard DataGuard
}

// NewSourcingService wires sourcing to the role profile gate and the records.
func NewSourcingService(
	db *gorm.DB, exa PeopleSearcher, roles *RoleProfileService,
	records *RecordService, artifacts *ArtifactService, credentials *CredentialService,
) *SourcingService {
	return &SourcingService{
		db: db, exa: exa, roles: roles, records: records, artifacts: artifacts,
		out: &outbound{db: db, credentials: credentials},
		now: func() time.Time { return time.Now().UTC() },
	}
}

// sourcingCategories are the kinds of thing a generated people query carries.
var sourcingCategories = []string{"professional requirements"}

// aspectTypesForPeopleQuery are the role aspects a people query draws on: the
// professional ones discovery uses, plus what the recruiter typed by hand —
// which for a role is a requirement in their own words, the most exact
// statement of what they are looking for.
var aspectTypesForPeopleQuery = append(append([]profile.AspectType(nil), aspectTypesForQuery...), profile.Other)

// Preview builds a query from the role's ready profile, and writes nothing.
func (s *SourcingService) Preview(roleID uint) (*QueryPreview, error) {
	status, err := s.roles.Status(roleID)
	if err != nil {
		return nil, err
	}
	if status.State != RoleProfileReady {
		return nil, fmt.Errorf("a query is built from a ready role profile: %s", status.Reason)
	}
	parts := []string{}
	for _, a := range status.Aspects {
		if containsType(aspectTypesForPeopleQuery, profile.AspectType(a.Type)) {
			parts = append(parts, a.Wording)
		}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("the role profile has no requirements to search with yet")
	}
	ids, err := s.identifiers(roleID)
	if err != nil {
		return nil, err
	}
	query := scrub.Generalize(scrub.Text(strings.Join(parts, ", "), ids))
	return s.out.describe(query, ids), nil
}

// Inspect reports what is worrying about a query the recruiter has edited.
func (s *SourcingService) Inspect(roleID uint, query string) (*QueryPreview, error) {
	ids, err := s.identifiers(roleID)
	if err != nil {
		return nil, err
	}
	return s.out.describe(query, ids), nil
}

// identifiers gathers what would name the client: the company and the people
// at it. They go in as names — the scrubber has no separate slot for an
// organization, and a name is removed the same way and warned about harder,
// which is the safe direction to be wrong in.
func (s *SourcingService) identifiers(roleID uint) (scrub.Identifiers, error) {
	role, err := s.records.GetRole(roleID)
	if err != nil {
		return scrub.Identifiers{}, err
	}
	ids := scrub.Identifiers{}
	if role.CompanyName != "" {
		ids.Names = append(ids.Names, role.CompanyName)
	}
	if role.CompanyID != nil {
		company, err := s.records.GetCompany(*role.CompanyID)
		if err != nil {
			return scrub.Identifiers{}, err
		}
		ids.Names = append(ids.Names, company.Name)
		at, err := s.records.ContactsAtCompany(company.ID)
		if err != nil {
			return scrub.Identifiers{}, err
		}
		for _, c := range at.Contacts {
			ids.Names = append(ids.Names, c.FullName)
			if c.Email != "" {
				ids.Emails = append(ids.Emails, c.Email)
			}
			if c.Phone != "" {
				ids.Phones = append(ids.Phones, c.Phone)
			}
		}
	}
	return ids, nil
}

// SourcingSendInput is a confirmed people search.
type SourcingSendInput struct {
	InitiativeID uint `json:"initiativeId"`
	RoleID       uint `json:"roleId"`
	// Query is exactly what the recruiter confirmed. It is transmitted
	// unchanged.
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// SourcingOutcome is what a sent people search produced.
type SourcingOutcome struct {
	SearchID uint `json:"searchId"`
	// LeadIDs are the leads this search produced, in result order.
	LeadIDs []uint `json:"leadIds"`
	Created int    `json:"created"`
	// AlreadyInPool counts results recognised as an existing candidate.
	AlreadyInPool int  `json:"alreadyInPool"`
	Skipped       int  `json:"skipped"`
	Partial       bool `json:"partial"`
}

// Send transmits the confirmed query and keeps what comes back as leads.
//
// The refusal and disclosure rules are DiscoveryService.Send's, for the same
// reasons: a refused search is recorded but not disclosed, and a transmitted
// one is disclosed whether or not the provider answered.
func (s *SourcingService) Send(in SourcingSendInput) (*SourcingOutcome, error) {
	query := in.Query
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("there is no query to send")
	}
	if in.InitiativeID == 0 {
		return nil, fmt.Errorf("a search belongs to an initiative")
	}
	if in.RoleID == 0 {
		return nil, fmt.Errorf("a people search is for a role")
	}

	ctx, cancel := context.WithTimeout(context.Background(), searchTimeout)
	defer cancel()

	sentAt := s.now()
	record := models.Search{
		InitiativeID: in.InitiativeID,
		Provider:     models.ProviderExa,
		Task:         models.TaskCandidateSourcing,
		Query:        query,
		SentAt:       sentAt,
	}
	searcher, err := s.searcher()
	if err != nil {
		record.FailureReason = models.ReasonUnauthorized
		if writeErr := s.db.Create(&record).Error; writeErr != nil {
			return nil, fmt.Errorf("recording the search: %w", writeErr)
		}
		return nil, err
	}
	resp, sendErr := searcher.SearchPeople(ctx, query, in.Limit, "")

	if err := s.recordDisclosure(sentAt, in); err != nil {
		return nil, err
	}
	if sendErr != nil {
		record.FailureReason = searchReason(sendErr)
		if err := s.db.Create(&record).Error; err != nil {
			return nil, fmt.Errorf("recording the search: %w", err)
		}
		return nil, sendErr
	}
	record.ResultCount = len(resp.Results)
	record.SkippedCount = resp.Skipped
	record.Partial = resp.Skipped > 0
	if err := s.db.Create(&record).Error; err != nil {
		return nil, fmt.Errorf("recording the search: %w", err)
	}

	out := &SourcingOutcome{SearchID: record.ID, Skipped: resp.Skipped, Partial: record.Partial}
	roleID := in.RoleID
	seen := map[uint]bool{}
	for _, r := range resp.Results {
		lead := models.Lead{
			SearchID: record.ID, InitiativeID: in.InitiativeID, RoleID: &roleID,
			Provider: models.ProviderExa, SourceID: r.SourceID,
			URL: r.URL, Title: r.Title, Snippet: r.Text, State: models.LeadNew,
		}
		if err := lead.Validate(); err != nil {
			out.Skipped++
			out.Partial = true
			continue
		}
		known, err := s.knownCandidate(lead.URL)
		if err != nil {
			return nil, err
		}
		if known != 0 {
			lead.CandidateID = &known
			out.AlreadyInPool++
		}
		// One lead per URL per search: the same page twice in one response
		// is one lead, and a resend replaces rather than duplicates.
		err = s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "search_id"}, {Name: "url"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "snippet", "source_id", "updated_at"}),
		}).Create(&lead).Error
		if err != nil {
			return nil, fmt.Errorf("keeping a lead: %w", err)
		}
		if lead.ID == 0 {
			if err := s.db.Where("search_id = ? AND url = ?", record.ID, lead.URL).First(&lead).Error; err != nil {
				return nil, fmt.Errorf("finding a kept lead: %w", err)
			}
		}
		if seen[lead.ID] {
			// The same page twice in one response is one lead.
			continue
		}
		seen[lead.ID] = true
		out.LeadIDs = append(out.LeadIDs, lead.ID)
		out.Created++
	}
	return out, nil
}

// knownCandidate reports the candidate an identity already names this URL as,
// or zero.
func (s *SourcingService) knownCandidate(rawURL string) (uint, error) {
	provider, handle := identityFromURL(rawURL)
	q := s.db.Model(&models.Identity{}).Where("url = ?", rawURL)
	if provider != "" {
		q = q.Or("provider = ? AND handle = ?", provider, handle)
	}
	ids := []uint{}
	if err := q.Limit(1).Pluck("candidate_id", &ids).Error; err != nil {
		return 0, fmt.Errorf("checking the pool: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return ids[0], nil
}

// identityFromURL names the provider and handle a profile URL carries, or
// nothing when the host is not one that has handles.
func identityFromURL(rawURL string) (provider, handle string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case host == "github.com" && len(segments) == 1 && segments[0] != "":
		return models.IdentityGitHub, strings.ToLower(segments[0])
	case host == "linkedin.com" && len(segments) == 2 && segments[0] == "in":
		return models.IdentityLinkedIn, segments[1]
	case host == "news.ycombinator.com" && u.Query().Get("id") != "":
		return models.IdentityHN, u.Query().Get("id")
	}
	return "", ""
}

// recordDisclosure writes the audit event: that a role's requirements left,
// never which.
func (s *SourcingService) recordDisclosure(at time.Time, in SourcingSendInput) error {
	initiativeID, roleID := in.InitiativeID, in.RoleID
	event := models.DisclosureEvent{
		OccurredAt:   at,
		Provider:     models.ProviderExa,
		Task:         models.TaskCandidateSourcing,
		InitiativeID: &initiativeID,
		RoleID:       &roleID,
	}
	ids, err := s.identifiers(in.RoleID)
	if err != nil {
		// The record is still made: a disclosure that happened is not made
		// not to have happened by a lookup failing afterwards.
		event.Categories = strings.Join(append(append([]string(nil), sourcingCategories...), "unverified content"), ", ")
	} else {
		event.Categories = s.out.categories(sourcingCategories, in.Query, ids)
	}
	return s.out.record(event)
}

// searcher is the client this search will use, built per request from the
// stored credential — see DiscoveryService.searcher for why.
func (s *SourcingService) searcher() (PeopleSearcher, error) {
	if s.exa != nil {
		return s.exa, nil
	}
	return s.out.exa()
}

// Searches lists the people searches sent under an initiative, newest first.
func (s *SourcingService) Searches(initiativeID uint) ([]models.Search, error) {
	rows := []models.Search{}
	err := s.db.Where("initiative_id = ? AND task = ?", initiativeID, models.TaskCandidateSourcing).
		Order("sent_at desc, id desc").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing people searches: %w", err)
	}
	return rows, nil
}
