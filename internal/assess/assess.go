// Package assess holds the parts of matching that must be identical on every
// machine and every run: what an assessment's identity is, how structured
// facts are compared, and how matches are ordered.
//
// No database, no model, no clock. Everything here is a function of its
// arguments, which is what lets the hash be canonical and the ranking be a
// table test.
package assess

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Versions of the rules themselves. They are inputs to the hash, so changing a
// comparison rule or a ranking rule invalidates every stored result — which is
// correct, because those results were about different rules.
const (
	ComparisonVersion = "1"
	RankingVersion    = "1"
	PromptVersion     = "1"
	SchemaVersion     = "1"
)

// Result is one requirement's outcome.
type Result string

// The three states, and nothing else.
const (
	Met     Result = "met"
	NotMet  Result = "not_met"
	Unknown Result = "unknown"
)

// Valid reports whether r is one of the three.
func (r Result) Valid() bool { return r == Met || r == NotMet || r == Unknown }

// Direction is which way a comparison runs.
type Direction string

const (
	// RoleFitsCandidate assesses the Role Profile against the initiative's
	// Search Criteria and the candidate's preferences.
	RoleFitsCandidate Direction = "role_fits_candidate"
	// CandidateFitsRole assesses the approved Candidate Profile against the
	// Role Profile's requirements.
	CandidateFitsRole Direction = "candidate_fits_role"
)

// Valid reports whether d is one of the two.
func (d Direction) Valid() bool {
	return d == RoleFitsCandidate || d == CandidateFitsRole
}

// Inputs is everything that could change an assessment's answer.
//
// The PRD lists these, and the list is the contract: a missing field is a
// result that gets reused when it should not have been, which is the failure
// this whole mechanism exists to prevent.
type Inputs struct {
	CandidateProfileVersion int
	CandidateProfileState   string
	RoleProfileVersion      int
	RoleProfileState        string
	CriteriaVersion         int
	// EvidenceHashes are the content hashes of every chunk and aspect shown to
	// the model, in whatever order the caller has them.
	EvidenceHashes    []string
	ComparisonVersion string
	RankingVersion    string
	// EndpointRevision is the generate assignment revision; ModelDigest is what
	// the endpoint reported, which may be empty.
	EndpointRevision int
	ModelDigest      string
	ModelName        string
	PromptVersion    string
	SchemaVersion    string
	// GenerationParams is the role's normalized parameter JSON.
	GenerationParams string
	RoleStale        bool
}

// Hash is the assessment's identity.
//
// The serialization is explicit and ordered because Go randomizes map
// iteration, and a hash that varied with it would change between runs of the
// same binary against the same data. Strings are length-prefixed so that
// ("ab","c") and ("a","bc") cannot collide.
func (in Inputs) Hash() string {
	h := sha256.New()
	write := func(label, value string) {
		_, _ = fmt.Fprintf(h, "%s=%d:%s\n", label, len(value), value)
	}
	writeInt := func(label string, value int) {
		_, _ = fmt.Fprintf(h, "%s=%d\n", label, value)
	}

	writeInt("candidate_profile_version", in.CandidateProfileVersion)
	write("candidate_profile_state", in.CandidateProfileState)
	writeInt("role_profile_version", in.RoleProfileVersion)
	write("role_profile_state", in.RoleProfileState)
	writeInt("criteria_version", in.CriteriaVersion)

	// Sorted, so the same evidence in a different retrieval order is the same
	// assessment. Which it is: the model sees the same text.
	hashes := append([]string(nil), in.EvidenceHashes...)
	sort.Strings(hashes)
	writeInt("evidence_count", len(hashes))
	for _, e := range hashes {
		write("evidence", e)
	}

	write("comparison_version", in.ComparisonVersion)
	write("ranking_version", in.RankingVersion)
	writeInt("endpoint_revision", in.EndpointRevision)
	write("model_digest", in.ModelDigest)
	write("model_name", in.ModelName)
	write("prompt_version", in.PromptVersion)
	write("schema_version", in.SchemaVersion)
	write("generation_params", in.GenerationParams)
	write("role_stale", fmt.Sprintf("%t", in.RoleStale))

	return hex.EncodeToString(h.Sum(nil))
}

// Value is one side of a structured comparison.
//
// Empty and "unknown" mean the same thing and both produce Unknown: a source
// that does not say is a fact, and guessing in its place is the failure the
// whole taxonomy exists to prevent.
type Value struct {
	Text string
	// Min and Max carry a compensation range. Zero means unstated.
	Min, Max int
	Currency string
}

// Stated reports whether this side says anything.
func (v Value) Stated() bool {
	return strings.TrimSpace(v.Text) != "" && !strings.EqualFold(strings.TrimSpace(v.Text), "unknown")
}

// Compare is the deterministic structured comparison.
//
// A model asked whether "Melbourne" satisfies "Melbourne, VIC" will usually say
// yes and occasionally say no, and the occasional no is unexplainable. Code
// comparing two normalized values is boring, right every time, and identical on
// every machine.
func Compare(wanted, found Value) Result {
	if !wanted.Stated() || !found.Stated() {
		return Unknown
	}
	if strings.EqualFold(strings.TrimSpace(wanted.Text), strings.TrimSpace(found.Text)) {
		return Met
	}
	return NotMet
}

// CompareCompensation compares two ranges.
//
// Overlap is met, disjoint is not met, and anything unstated — or two different
// currencies — is unknown rather than a conversion nobody asked for.
func CompareCompensation(wanted, found Value) Result {
	if wanted.Min == 0 && wanted.Max == 0 {
		return Unknown
	}
	if found.Min == 0 && found.Max == 0 {
		return Unknown
	}
	if wanted.Currency != "" && found.Currency != "" &&
		!strings.EqualFold(wanted.Currency, found.Currency) {
		return Unknown
	}

	wantedMax := wanted.Max
	if wantedMax == 0 {
		wantedMax = int(^uint(0) >> 1)
	}
	foundMax := found.Max
	if foundMax == 0 {
		foundMax = int(^uint(0) >> 1)
	}
	if wanted.Min <= foundMax && found.Min <= wantedMax {
		return Met
	}
	return NotMet
}

// Tally is what ranking counts, across both directions of one match.
type Tally struct {
	RoleID uint
	// UnmetMustHaves and UnknownMustHaves are summed over both directions.
	UnmetMustHaves   int
	UnknownMustHaves int
	MetNiceToHaves   int
	// RetrievalPosition is the shortlist position, one-based; lower is better.
	// Zero means it was not shortlisted.
	RetrievalPosition int
}

// Less reports whether a should be ordered before b.
//
// The PRD's six steps, lexicographically. A comparator rather than a score
// because no number of met nice-to-haves compensates for an unmet must-have,
// and expressing that as a weighted sum requires weights that are lies.
func Less(a, b Tally) bool {
	// 1. No unmet must-haves on either direction.
	aClean, bClean := a.UnmetMustHaves == 0, b.UnmetMustHaves == 0
	if aClean != bClean {
		return aClean
	}
	// 2. Fewer total unmet must-haves.
	if a.UnmetMustHaves != b.UnmetMustHaves {
		return a.UnmetMustHaves < b.UnmetMustHaves
	}
	// 3. Fewer total unknown must-haves.
	if a.UnknownMustHaves != b.UnknownMustHaves {
		return a.UnknownMustHaves < b.UnknownMustHaves
	}
	// 4. More total met nice-to-haves.
	if a.MetNiceToHaves != b.MetNiceToHaves {
		return a.MetNiceToHaves > b.MetNiceToHaves
	}
	// 5. Higher retrieval position — lower number, with zero meaning "not
	// shortlisted" and therefore last.
	aPos, bPos := a.RetrievalPosition, b.RetrievalPosition
	if aPos == 0 {
		aPos = int(^uint(0) >> 1)
	}
	if bPos == 0 {
		bPos = int(^uint(0) >> 1)
	}
	if aPos != bPos {
		return aPos < bPos
	}
	// 6. Role identifier, which makes the order total.
	return a.RoleID < b.RoleID
}

// Rank orders matches, stably.
func Rank(tallies []Tally) []Tally {
	out := append([]Tally(nil), tallies...)
	sort.SliceStable(out, func(i, j int) bool { return Less(out[i], out[j]) })
	return out
}
