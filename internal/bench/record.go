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

// ClassifierTotals is the benchmark's four conditions over the whole corpus.
//
// The PRD states them over the corpus, not per listing: "at least 80% of
// recruiter-labeled material aspects are captured" is one number about the
// held-out set. Requiring every listing to clear 80% by itself is a stricter
// bar than the one the product was accepted against, and inventing a harder
// test is as wrong as inventing an easier one.
type ClassifierTotals struct {
	Listings    int     `json:"listings"`
	Material    int     `json:"material"`
	Captured    int     `json:"captured"`
	CaptureRate float64 `json:"captureRate"`
	Uncited     int     `json:"uncited"`
	Unsupported int     `json:"unsupported"`
	Misreported int     `json:"misreported"`

	AllCited         bool `json:"allCited"`
	NoUnsupported    bool `json:"noUnsupported"`
	CaptureMet       bool `json:"captureMet"`
	ConstraintsExact bool `json:"constraintsExact"`
	Pass             bool `json:"pass"`
}

// ClassifierTotals sums the per-listing scores. The per-listing detail stays,
// because a corpus-level failure with no way to see which listing caused it is
// a number nobody can act on.
func (r *Record) ClassifierTotals() ClassifierTotals {
	out := ClassifierTotals{Listings: len(r.Classifier)}
	for _, score := range r.Classifier {
		out.Material += score.Material
		out.Captured += score.Captured
		out.Uncited += len(score.Uncited)
		out.Unsupported += len(score.Unsupported)
		out.Misreported += len(score.Misreported)
	}
	if out.Material > 0 {
		out.CaptureRate = float64(out.Captured) / float64(out.Material)
	}
	out.AllCited = out.Uncited == 0
	out.NoUnsupported = out.Unsupported == 0
	out.CaptureMet = out.Material > 0 && out.CaptureRate >= CaptureThreshold
	out.ConstraintsExact = out.Misreported == 0
	out.Pass = out.Listings > 0 && out.AllCited && out.NoUnsupported &&
		out.CaptureMet && out.ConstraintsExact
	return out
}

// ClassifierPass reports whether the corpus as a whole met the four conditions.
func (r *Record) ClassifierPass() bool { return r.ClassifierTotals().Pass }

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

func passWord(pass bool) string {
	if pass {
		return "pass"
	}
	return "fail"
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

	totals := r.ClassifierTotals()
	fmt.Fprintf(&b, "\nClassifier over the corpus: capture %.0f%% (%d/%d), "+
		"uncited %d, unsupported %d, misreported %d — %s\n",
		totals.CaptureRate*100, totals.Captured, totals.Material,
		totals.Uncited, totals.Unsupported, totals.Misreported, passWord(totals.Pass))
	fmt.Fprintf(&b, "\nClassifier: %d listings\n", len(r.Classifier))
	for _, score := range r.Classifier {
		fmt.Fprintf(&b, "  %s: capture %.0f%% (%d/%d), cited %v, no unsupported %v, constraints exact %v\n",
			score.Listing, score.CaptureRate*100, score.Captured, score.Material,
			score.AllCited, score.NoUnsupported, score.ConstraintsExact)
		for _, why := range append(append([]string{}, score.Unsupported...), score.Misreported...) {
			fmt.Fprintf(&b, "    %s\n", why)
		}
		for _, missed := range score.Missed {
			fmt.Fprintf(&b, "    missed %s\n", missed)
		}
	}

	fmt.Fprintf(&b, "\nMatching: %d of %d scenarios reached three plausible\n",
		r.Matching.MetCount, len(r.Matching.Scenarios))
	for _, s := range r.Matching.Scenarios {
		fmt.Fprintf(&b, "  %s: %d plausible of %d distinct", s.ScenarioID, s.Plausible, len(s.Distinct))
		if s.Note != "" {
			fmt.Fprintf(&b, " — %s", s.Note)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "\nEligible roles: %d\n", r.EligibleRoles)
	for _, m := range r.Measurements {
		// A row the PRD sets no target for reads as a failed one if it is
		// printed with a target of zero and met false.
		against := fmt.Sprintf("target %.2f, met %v", m.Target, m.Met)
		if m.Target == 0 {
			against = "no target"
		}
		fmt.Fprintf(&b, "  %s: %.2f %s (%s) — %s\n",
			m.Name, m.Value, m.Unit, against, m.Conditions)
	}
	fmt.Fprintf(&b, "\nOutcome: %s\n", r.Outcome)
	if r.Outcome == OutcomeInconclusive {
		fmt.Fprintf(&b, "Fewer than %d eligible roles: source coverage, not the matcher.\n",
			MinimumEligibleRoles)
	}
	return b.String()
}
