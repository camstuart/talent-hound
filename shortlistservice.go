package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/fusion"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// ShortlistService chooses which roles are worth the expensive stage.
//
// Assessment is a local model reading a role and a candidate and reasoning
// about fit, per requirement, with citations. Running it against every
// discovered role would take minutes and produce a wall nobody reads. So
// something cheap picks twenty, and its job is to be cheap, deterministic, and
// explainable — three properties easy to have separately and easy to lose
// together.
type ShortlistService struct {
	// Depth and FusionK are the retrieval constants, settable only so the
	// tuning corpus can sweep them. Zero means the value the product ships.
	Depth   int
	FusionK int

	db       *gorm.DB
	search   *SearchService
	embed    *EmbedService
	criteria *CriteriaService
	profiles *CandidateProfileService
	roles    *RoleProfileService
}

// NewShortlistService composes the retrieval built in Phases 7 and 9.
func NewShortlistService(
	db *gorm.DB, search *SearchService, embed *EmbedService,
	criteria *CriteriaService, profiles *CandidateProfileService, roles *RoleProfileService,
) *ShortlistService {
	return &ShortlistService{db: db, search: search, embed: embed,
		criteria: criteria, profiles: profiles, roles: roles}
}

// ShortlistSize is the PRD's twenty. Not configurable: a recruiter who wants
// more runs a narrower search.
const ShortlistSize = 20

// perQueryDepth is how deep each individual retrieval goes before fusion.
// Deeper than the shortlist, because fusion's whole value is combining the
// tails of several lists.
const perQueryDepth = 30

// ScopeConflict is a structured must-have the role appears to fail.
//
// Named apart from the profile diff's Conflict, which is a different thing: one
// is two versions disagreeing about a person, this is a role disagreeing with
// what the search asked for.
//
// It is carried, never applied. "No results" and "results you would have
// rejected" look identical on screen, and only one of them is true — a
// recruiter who sees zero roles concludes the market is empty.
type ScopeConflict struct {
	Field string `json:"field"`
	// Wanted is what the must-have criterion or candidate fact says.
	Wanted string `json:"wanted"`
	// Found is what the role says.
	Found string `json:"found"`
}

// Entry is one shortlisted role and why it is there.
type Entry struct {
	RoleID   uint    `json:"roleId"`
	Title    string  `json:"title"`
	Position int     `json:"position"`
	Score    float64 `json:"score"`
	// Why names what retrieved it, by which method, at which rank.
	Why []fusion.Contribution `json:"why"`
	// Conflicts are structured must-haves it appears to fail. They do not
	// remove it from the list.
	Conflicts []ScopeConflict `json:"conflicts"`
}

// Shortlist is the top twenty and the intent it was computed under.
type Shortlist struct {
	InitiativeID uint    `json:"initiativeId"`
	CandidateID  uint    `json:"candidateId"`
	Entries      []Entry `json:"entries"`
	// CriteriaVersion and SpaceID say what this list reflects, so a later
	// reader can tell whether it is still about the current intent.
	CriteriaVersion int  `json:"criteriaVersion"`
	SpaceID         uint `json:"spaceId"`
	// Eligible is how many roles were in scope before ranking.
	Eligible int `json:"eligible"`
}

// Build computes the shortlist for an initiative.
func (s *ShortlistService) Build(initiativeID, candidateID uint) (*Shortlist, error) {
	out := &Shortlist{InitiativeID: initiativeID, CandidateID: candidateID, Entries: []Entry{}}

	version, err := s.criteria.Version(initiativeID)
	if err != nil {
		return nil, err
	}
	out.CriteriaVersion = version
	if space, err := s.embed.CurrentSpace(); err == nil && space != nil {
		out.SpaceID = space.ID
	}

	// Scope first, and it is the only filtering there is. A top twenty computed
	// over everything and then filtered returns fewer than twenty with the
	// missing slots invisible; filtering first means twenty eligible roles are
	// twenty eligible roles.
	eligible, err := s.eligibleRoles(initiativeID)
	if err != nil {
		return nil, err
	}
	out.Eligible = len(eligible)
	if len(eligible) == 0 {
		return out, nil
	}
	allowed := make(map[uint]bool, len(eligible))
	for _, r := range eligible {
		allowed[r.ID] = true
	}

	queries, err := s.queries(initiativeID, candidateID)
	if err != nil {
		return nil, err
	}
	if len(queries) == 0 {
		return out, nil
	}

	lists := []fusion.Ranked{}
	for _, q := range queries {
		lexical, err := s.lexical(initiativeID, q.text, q.anyTerms, allowed)
		if err != nil {
			return nil, err
		}
		lists = append(lists, fusion.Ranked{Source: q.source, Method: "lexical", Keys: lexical})

		if q.lexicalOnly {
			continue
		}
		lists = append(lists, fusion.Ranked{
			Source: q.source, Method: "semantic",
			Keys: s.semantic(initiativeID, q.text, allowed),
		})
	}

	byID := make(map[uint]models.Role, len(eligible))
	for _, r := range eligible {
		byID[r.ID] = r
	}
	conflicts, err := s.conflicts(initiativeID, candidateID, eligible)
	if err != nil {
		return nil, err
	}

	for i, f := range fusion.Top(fusion.FuseWith(s.FusionK, lists), ShortlistSize) {
		role := byID[f.Key]
		out.Entries = append(out.Entries, Entry{
			RoleID:    f.Key,
			Title:     role.Title,
			Position:  i + 1,
			Score:     f.Score,
			Why:       f.Why,
			Conflicts: conflicts[f.Key],
		})
	}
	return out, nil
}

// eligibleRoles selects what may be retrieved against.
//
// Out of scope, deleted, and Stale — and nothing else. In particular a role
// that obviously fails a must-have is eligible, because the recruiter needs to
// see and reject it.
func (s *ShortlistService) eligibleRoles(initiativeID uint) ([]models.Role, error) {
	rows := []models.Role{}
	err := s.db.
		Where("lifecycle_state <> ?", models.RolePurged).
		Where("id IN (SELECT target_id FROM artifact_links WHERE target_type = ? "+
			"AND artifact_id IN (SELECT artifact_id FROM artifact_links WHERE target_type = ? AND target_id = ?))",
			models.LinkRole, models.LinkInitiative, initiativeID).
		Order("id asc").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing eligible roles: %w", err)
	}

	kept := make([]models.Role, 0, len(rows))
	for _, r := range rows {
		// Phase 12 already decided a stale role is not assessed; asking it
		// rather than re-deriving staleness keeps one answer to the question.
		eligible, err := s.roles.Eligibility(r.ID)
		if err != nil {
			return nil, err
		}
		if eligible.Eligible {
			kept = append(kept, r)
		}
	}
	return kept, nil
}

// query is one thing to search for, and what to call it in the provenance.
type query struct {
	source string
	text   string
	// anyTerms ORs the query's words instead of ANDing them.
	//
	// A criterion is wording the recruiter typed on purpose: two words mean
	// both, and AND is right. A profile aspect is a sentence lifted out of a
	// document — "Ran the platform team's shared services in Go" — and ANDing
	// it demands a role listing containing all nine words, which no listing
	// does. It is the same distinction Phase 17 drew for questions, arriving
	// here for the same reason.
	anyTerms bool
	// lexicalOnly keeps a term out of the similarity half.
	//
	// A place is the case this exists for. "Melbourne" and "Sydney" are close
	// in an embedding space and opposite in fact, so a place must never reach
	// the similarity half — but the full-text half matches "Perth" against
	// "Perth" exactly and cannot confuse the two, and a candidate in the city a
	// role is in is evidence a recruiter would use.
	//
	// Measured: embedded-c-perth lives in Perth and works at Redgum Mining
	// Tech; staff-engineer-perth is a role at Redgum Mining Tech in Perth. The
	// recruiter calls it plausible and the shortlist never surfaced it, because
	// the word Perth reached neither half of the retrieval.
	lexicalOnly bool
}

// queries builds the search terms: every approved criterion, and every
// candidate aspect whose type takes part in similarity retrieval.
func (s *ShortlistService) queries(initiativeID, candidateID uint) ([]query, error) {
	out := []query{}

	criteria, err := s.criteria.List(initiativeID)
	if err != nil {
		return nil, err
	}
	for _, c := range criteria {
		out = append(out, query{source: c.Text, text: c.Text})
	}

	if candidateID == 0 {
		return out, nil
	}
	ready, err := s.profiles.Readiness(candidateID)
	if err != nil {
		return nil, err
	}
	if !ready.Ready {
		// Unapproved evidence does not drive retrieval, the same way it does
		// not drive a query in Phase 14.
		return out, nil
	}
	approved, err := s.profiles.Approved(candidateID)
	if err != nil || approved == nil {
		return out, err
	}
	for _, a := range approved.Aspects {
		typ := profile.AspectType(a.Type)
		// Structured types are compared, not searched: "Melbourne" and "Sydney"
		// are close in an embedding space and opposite in fact.
		if !fusion.Searchable(typ) {
			continue
		}
		// And the compatibility map decides whether this candidate aspect can
		// reach any role aspect at all.
		if len(fusion.RoleAspectsFor(typ)) == 0 {
			continue
		}
		out = append(out, query{source: a.Wording, text: a.Wording, anyTerms: true})
	}

	// And the places, by their normalized value rather than their wording: the
	// aspect reads "Perth, onsite, permanent, around AUD 160,000" and only the
	// city is the place.
	for _, a := range approved.Aspects {
		if profile.AspectType(a.Type) != profile.Location {
			continue
		}
		values := map[string]any{}
		if err := json.Unmarshal([]byte(a.Structured), &values); err != nil {
			continue
		}
		for _, field := range []string{"city", "region", "country"} {
			place, ok := values[field].(string)
			if !ok || strings.TrimSpace(place) == "" {
				continue
			}
			out = append(out, query{source: place, text: place, lexicalOnly: true})
		}
	}
	return out, nil
}

// depth is how deep each retrieval goes, defaulting to what the product ships.
func (s *ShortlistService) depth() int {
	if s.Depth > 0 {
		return s.Depth
	}
	return perQueryDepth
}

// lexical retrieves role chunks by word, grouped to roles in rank order.
func (s *ShortlistService) lexical(
	initiativeID uint, text string, anyTerms bool, allowed map[uint]bool,
) ([]uint, error) {
	find := s.search.Search
	if anyTerms {
		find = s.search.SearchAny
	}
	hits, err := find(initiativeID, text, s.depth())
	if err != nil {
		return nil, err
	}
	return s.rolesOf(artifactIDs(hits), allowed)
}

// semantic retrieves role chunks by meaning, grouped to roles in rank order.
func (s *ShortlistService) semantic(initiativeID uint, text string, allowed map[uint]bool) []uint {
	// Aspects, not chunks. The PRD asks for exact-cosine aspect KNN, and a
	// chunk carries a listing's blurb along with its requirements — so a
	// similarity query matched the sentences every listing shares. An aspect is
	// one statement, which is what a candidate's aspect should be compared
	// against.
	// Roles, not aspects: the depth is a number of roles to consider, and
	// limiting aspects instead hides a role whose best aspect fell outside the
	// page because other listings wrote more of them.
	hits, err := s.embed.SearchRoles(initiativeID, text, s.depth())
	if err != nil {
		// Deliberately swallowed: no embedding space yet is the ordinary state
		// before anything is embedded, and it is not a failure of the
		// shortlist.
		//
		return nil
	}
	seen := map[uint]bool{}
	ids := make([]uint, 0, len(hits))
	for _, h := range hits {
		if !allowed[h.RoleID] || seen[h.RoleID] {
			continue
		}
		seen[h.RoleID] = true
		ids = append(ids, h.RoleID)
	}
	return ids
}

// artifactIDs is the artifacts a lexical result names, in rank order.
func artifactIDs(hits []Hit) []uint {
	ids := make([]uint, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ArtifactID)
	}
	return ids
}

// rolesOf maps retrieved artifacts to the roles that own them, preserving rank
// order and keeping each role once.
//
// This is the grouping the PRD asks for: five matching chunks of one role are
// one shortlist position, not five.
func (s *ShortlistService) rolesOf(artifactIDs []uint, allowed map[uint]bool) ([]uint, error) {
	if len(artifactIDs) == 0 {
		return nil, nil
	}
	links := []models.ArtifactLink{}
	err := s.db.Where("target_type = ? AND artifact_id IN ?", models.LinkRole, artifactIDs).
		Where("historical = ?", false).
		Find(&links).Error
	if err != nil {
		return nil, fmt.Errorf("mapping artifacts to roles: %w", err)
	}
	roleOf := make(map[uint][]uint, len(links))
	for _, l := range links {
		roleOf[l.ArtifactID] = append(roleOf[l.ArtifactID], l.TargetID)
	}

	out := []uint{}
	seen := map[uint]bool{}
	for _, artifactID := range artifactIDs {
		for _, roleID := range roleOf[artifactID] {
			if seen[roleID] || !allowed[roleID] {
				continue
			}
			seen[roleID] = true
			out = append(out, roleID)
		}
	}
	return out, nil
}

// conflicts computes the structured must-haves each role appears to fail.
//
// Structured equality on the normalizable fields, computed here and attached.
// Anything cleverer is Phase 16's assessment, which is the stage allowed to be
// expensive.
func (s *ShortlistService) conflicts(
	initiativeID, candidateID uint, roles []models.Role,
) (map[uint][]ScopeConflict, error) {
	out := map[uint][]ScopeConflict{}

	wanted, err := s.wantedValues(initiativeID, candidateID)
	if err != nil {
		return nil, err
	}
	if len(wanted) == 0 {
		return out, nil
	}

	for _, role := range roles {
		found, err := s.roleValues(role)
		if err != nil {
			return nil, err
		}
		for field, want := range wanted {
			got, ok := found[field]
			if !ok || got == "" || got == "unknown" || want == "unknown" {
				// Silence is not a conflict: "the listing does not say" is a
				// fact, and Phase 16 reports it as missing information.
				continue
			}
			if !strings.EqualFold(got, want) {
				out[role.ID] = append(out[role.ID], ScopeConflict{Field: field, Wanted: want, Found: got})
			}
		}
	}
	return out, nil
}

// wantedValues are the structured values the must-have criteria and the
// candidate's approved structured aspects assert.
func (s *ShortlistService) wantedValues(initiativeID, candidateID uint) (map[string]string, error) {
	wanted := map[string]string{}

	// Criteria are free text, so the structured half comes from the candidate's
	// approved aspects; a criterion contributes its wording to retrieval and
	// its priority to Phase 16.
	if candidateID != 0 {
		approved, err := s.profiles.Approved(candidateID)
		if err != nil {
			return nil, err
		}
		if approved != nil {
			for _, a := range approved.Aspects {
				if !fusion.IsStructured(profile.AspectType(a.Type)) {
					continue
				}
				values := map[string]any{}
				if err := json.Unmarshal([]byte(a.Structured), &values); err != nil {
					continue
				}
				for field, v := range values {
					if text, ok := v.(string); ok && text != "" {
						wanted[field] = text
					}
				}
			}
		}
	}
	_ = initiativeID
	return wanted, nil
}

// roleValues are the structured values a role's current profile asserts.
func (s *ShortlistService) roleValues(role models.Role) (map[string]string, error) {
	found := map[string]string{}
	status, err := s.roles.Status(role.ID)
	if err != nil {
		return nil, err
	}
	for _, a := range status.Aspects {
		if !fusion.IsStructured(profile.AspectType(a.Type)) {
			continue
		}
		values := map[string]any{}
		if err := json.Unmarshal([]byte(a.Structured), &values); err != nil {
			continue
		}
		for field, v := range values {
			if text, ok := v.(string); ok && text != "" {
				found[field] = text
			}
		}
	}
	return found, nil
}
