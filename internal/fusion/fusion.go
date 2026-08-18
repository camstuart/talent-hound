// Package fusion combines ranked lists that do not share a scale, and says
// which aspect types may be compared with which.
//
// It has no database, no model, and no network: every rule here is arithmetic
// over ranks or a lookup in one table, which is what lets the compatibility map
// and every fusion case be exhaustive table tests.
package fusion

import (
	"sort"

	"camstuart/talent-hound/internal/profile"
)

// K is the reciprocal-rank fusion constant.
//
// 60, from the original paper, written down rather than tuned. Tuning it
// against this corpus would produce a number that is right for today's fixtures
// and unjustifiable tomorrow. Its effect is to flatten the gap between rank one
// and rank two relative to the gap between appearing and not appearing — which
// is the property a shortlist wants, since a role found by three criteria is
// more interesting than a role found first by one.
const K = 60

// Ranked is one ranked list from one system, best first.
//
// Only the order is used. The scores are deliberately absent: bm25 and cosine
// do not share a scale, averaging them is meaningless, and normalizing them
// would make a role's rank depend on how the corpus has grown.
type Ranked struct {
	// Source names what produced this list — a criterion, an aspect — for
	// provenance.
	Source string
	// Method is how it was retrieved: lexical or semantic.
	Method string
	// Keys are the grouped identifiers in rank order, best first.
	Keys []uint
}

// Contribution is one list's reason for a key being in the result.
// The JSON tags matter: this type crosses the binding boundary as a shortlist
// entry's provenance, and the rest of the wire shape is lowerCamel.
type Contribution struct {
	Source string `json:"source"`
	Method string `json:"method"`
	// Rank is one-based, and is the best rank this key reached in that list.
	Rank int `json:"rank"`
}

// Fused is one key's place in the combined ranking.
type Fused struct {
	Key   uint    `json:"key"`
	Score float64 `json:"score"`
	// Why is every list that contributed, in the order the lists were given.
	Why []Contribution `json:"why"`
}

// Fuse combines ranked lists by reciprocal rank.
//
// A key at rank r in a list contributes 1/(K+r); a key's score is the sum over
// the lists it appears in. A key appearing twice in one list contributes its
// best rank once — repetition within a list is a retrieval artifact, not
// evidence.
func Fuse(lists []Ranked) []Fused {
	scores := map[uint]float64{}
	why := map[uint][]Contribution{}

	for _, list := range lists {
		best := map[uint]int{}
		for i, key := range list.Keys {
			rank := i + 1
			if seen, ok := best[key]; !ok || rank < seen {
				best[key] = rank
			}
		}
		for key, rank := range best {
			scores[key] += 1 / float64(K+rank)
		}
		// Recorded in list order rather than map order, so provenance is stable.
		for _, key := range list.Keys {
			if best[key] == 0 {
				continue
			}
			why[key] = append(why[key], Contribution{
				Source: list.Source, Method: list.Method, Rank: best[key],
			})
			// Only the first occurrence records a contribution for this list.
			best[key] = 0
		}
	}

	out := make([]Fused, 0, len(scores))
	for key, score := range scores {
		out = append(out, Fused{Key: key, Score: score, Why: why[key]})
	}
	// Score descending, then identifier ascending. Ties are constant — two keys
	// found at the same rank by the same one list have identical scores — so
	// "repeated runs return identical ordering" has to be a rule rather than a
	// hope about map iteration.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Top returns the first n, or all of them when there are fewer.
func Top(fused []Fused, n int) []Fused {
	if n <= 0 || len(fused) <= n {
		return fused
	}
	return fused[:n]
}

// compatible is the PRD's aspect compatibility map, transcribed exactly:
// role aspect → the candidate aspects it searches.
//
// The absences matter as much as the entries. A qualification searches nothing
// but a qualification, and a test asserts that it does not reach a skill.
var compatible = map[profile.AspectType][]profile.AspectType{
	profile.Skill:          {profile.Skill, profile.Experience, profile.Responsibility},
	profile.Responsibility: {profile.Responsibility, profile.Experience, profile.Skill},
	profile.Experience:     {profile.Experience, profile.Responsibility},
	profile.Qualification:  {profile.Qualification},
	profile.Seniority:      {profile.Seniority, profile.Experience},
	profile.Other:          {profile.Other},
}

// CandidateAspectsFor returns the candidate aspect types a role aspect searches.
func CandidateAspectsFor(role profile.AspectType) []profile.AspectType {
	return append([]profile.AspectType(nil), compatible[role]...)
}

// RoleAspectsFor returns the role aspect types a candidate aspect can be found
// by — the inverse of the map.
//
// Derived rather than written, because a hand-written inverse drifts the first
// time someone adds an edge, and the drift is invisible: a missing edge is a
// role that quietly never appears.
func RoleAspectsFor(candidate profile.AspectType) []profile.AspectType {
	out := []profile.AspectType{}
	// Iterated in the taxonomy's order so the result is stable.
	for _, role := range profile.AspectTypes {
		for _, searched := range compatible[role] {
			if searched == candidate {
				out = append(out, role)
				break
			}
		}
	}
	return out
}

// Map returns the whole map, for a test that wants to check every edge and for
// a screen that wants to explain the rule.
func Map() map[profile.AspectType][]profile.AspectType {
	out := make(map[profile.AspectType][]profile.AspectType, len(compatible))
	for role, searched := range compatible {
		out[role] = append([]profile.AspectType(nil), searched...)
	}
	return out
}

// structuredTypes are compared by their normalized values rather than retrieved
// by similarity: "Melbourne" and "Sydney" are close in an embedding space and
// opposite in fact.
var structuredTypes = map[profile.AspectType]bool{
	profile.Location:        true,
	profile.WorkArrangement: true,
	profile.WorkRights:      true,
	profile.EmploymentType:  true,
	profile.Compensation:    true,
}

// IsStructured reports whether a type is compared rather than searched.
func IsStructured(t profile.AspectType) bool { return structuredTypes[t] }

// Searchable reports whether a type takes part in similarity retrieval at all.
func Searchable(t profile.AspectType) bool {
	_, known := compatible[t]
	return known && !structuredTypes[t]
}
