package bench

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"camstuart/talent-hound/internal/profile"
)

// Assignment is one model role as it was configured for a run. A role the run
// used with nothing assigned is recorded as unassigned rather than omitted: a
// missing line reads as "not used", which is the wrong conclusion.
type Assignment struct {
	Role   string `json:"role"`
	Model  string `json:"model"`
	Digest string `json:"digest"`
}

// Measurement is one timing or volume figure with the conditions it was taken
// under. A number without its conditions is not a measurement.
type Measurement struct {
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Conditions string  `json:"conditions"`
	// Target is the provisional target, and Met says whether it was reached.
	// Neither changes the recorded value.
	Target float64 `json:"target"`
	Met    bool    `json:"met"`
}

// Record is one benchmark run: what it ran with, what it measured, and what it
// concluded.
type Record struct {
	// Taken is supplied by the caller, so nothing here reads a clock and
	// nothing here is unreproducible.
	Taken         string `json:"taken"`
	Version       string `json:"version"`
	Platform      string `json:"platform"`
	SchemaVersion string `json:"schemaVersion"`
	PromptVersion string `json:"promptVersion"`
	CorpusHash    string `json:"corpusHash"`
	CorpusIs      string `json:"corpusIs"`
	CloudAssisted bool   `json:"cloudAssisted"`

	Assignments   []Assignment      `json:"assignments"`
	Classifier    []ClassifierScore `json:"classifier"`
	Matching      MatchingScore     `json:"matching"`
	EligibleRoles int               `json:"eligibleRoles"`
	Measurements  []Measurement     `json:"measurements"`
	Outcome       Outcome           `json:"outcome"`
}

// NewRecord assembles a record. Roles the run used are always present, so an
// unassigned role is visible rather than absent.
func NewRecord(taken, version, platform string, corpus *Corpus, hash string, roles map[string]Assignment) *Record {
	corpusIs := "the recruiter's frozen corpus"
	if corpus.Synthetic {
		corpusIs = "the synthetic corpus in this repository — invented, not real placements"
	}
	assignments := []Assignment{}
	for _, role := range sortedKeys(roles) {
		assignment := roles[role]
		assignment.Role = role
		if assignment.Model == "" {
			assignment.Model = "unassigned"
		}
		if assignment.Digest == "" {
			assignment.Digest = "unknown"
		}
		assignments = append(assignments, assignment)
	}
	return &Record{
		Taken: taken, Version: version, Platform: platform,
		SchemaVersion: profile.SchemaVersion, PromptVersion: profile.PromptVersion,
		CorpusHash: hash, CorpusIs: corpusIs,
		Assignments: assignments, Classifier: []ClassifierScore{},
		Measurements: []Measurement{},
	}
}

func sortedKeys(in map[string]Assignment) []string {
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ClassifierPass reports whether every scored listing passed.
func (r *Record) ClassifierPass() bool {
	if len(r.Classifier) == 0 {
		return false
	}
	for _, score := range r.Classifier {
		if !score.Pass {
			return false
		}
	}
	return true
}

// Conclude sets the outcome from what the run found.
func (r *Record) Conclude() {
	r.Outcome = Conclude(r.EligibleRoles, r.CloudAssisted, r.ClassifierPass() && r.Matching.Pass)
}

// JSON is the record as stored.
func (r *Record) JSON() ([]byte, error) {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("writing the record: %w", err)
	}
	return raw, nil
}

// Summary is the record as read. It leads with what it ran against, because a
// result read without its corpus and its models is a result read wrongly.
func (r *Record) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Talent Hound %s on %s, %s\n", r.Version, r.Platform, r.Taken)
	fmt.Fprintf(&b, "Corpus: %s\n  hash %s\n", r.CorpusIs, r.CorpusHash)
	fmt.Fprintf(&b, "Schema v%s, prompt v%s\n", r.SchemaVersion, r.PromptVersion)
	for _, a := range r.Assignments {
		fmt.Fprintf(&b, "  %s: %s (%s)\n", a.Role, a.Model, a.Digest)
	}
	if r.CloudAssisted {
		b.WriteString("Cloud-assisted: this run cannot pass the PoC.\n")
	}

	fmt.Fprintf(&b, "\nClassifier: %d listings\n", len(r.Classifier))
	for _, score := range r.Classifier {
		fmt.Fprintf(&b, "  %s: capture %.0f%% (%d/%d), cited %v, no unsupported %v, constraints exact %v\n",
			score.Listing, score.CaptureRate*100, score.Captured, score.Material,
			score.AllCited, score.NoUnsupported, score.ConstraintsExact)
		for _, why := range append(append([]string{}, score.Unsupported...), score.Misreported...) {
			fmt.Fprintf(&b, "    %s\n", why)
		}
	}

	fmt.Fprintf(&b, "\nMatching: %d of %d scenarios reached three plausible\n",
		r.Matching.MetCount, len(r.Matching.Scenarios))
	for _, s := range r.Matching.Scenarios {
		fmt.Fprintf(&b, "  %s: %d plausible of %d distinct\n", s.ScenarioID, s.Plausible, len(s.Distinct))
	}

	fmt.Fprintf(&b, "\nEligible roles: %d\n", r.EligibleRoles)
	for _, m := range r.Measurements {
		fmt.Fprintf(&b, "  %s: %.2f %s (target %.2f, met %v) — %s\n",
			m.Name, m.Value, m.Unit, m.Target, m.Met, m.Conditions)
	}
	fmt.Fprintf(&b, "\nOutcome: %s\n", r.Outcome)
	if r.Outcome == OutcomeInconclusive {
		fmt.Fprintf(&b, "Fewer than %d eligible roles: source coverage, not the matcher.\n",
			MinimumEligibleRoles)
	}
	return b.String()
}
