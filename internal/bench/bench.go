// Package bench holds the frozen benchmark corpus, the two scorers, and the
// record a run produces.
//
// Nothing here runs a model. The scoring rules are the PRD's, expressed as pure
// functions, so they are table tests rather than something only observable on a
// laptop with 6 GB of models installed.
package bench

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"camstuart/talent-hound/internal/profile"
)

//go:embed testdata
var corpusFS embed.FS

// CaptureThreshold is the PRD's bar: at least 80% of the material aspects a
// recruiter labelled must be captured.
const CaptureThreshold = 0.80

// MinimumEligibleRoles is the PRD's floor for a live acceptance run. Below it
// the run says nothing about the matcher, because a thin source corpus and a
// bad matcher produce the same small number.
const MinimumEligibleRoles = 10

// Listing is one frozen role listing with the labels applied before any run.
type Listing struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
	// Material is what a recruiter marked as mattering in this listing.
	Material []profile.Aspect `json:"material"`
	// Structured is the subset of Material whose value must be reproduced
	// exactly: location, work rights, employment type, compensation.
	Structured []profile.Aspect `json:"structured"`
}

// Scenario is one past-placement scenario: a candidate, and the roles the
// recruiter rated.
type Scenario struct {
	ID      string   `json:"id"`
	Resume  string   `json:"resume"`
	Ratings []Rating `json:"ratings"`
}

// Rating is the recruiter's judgement of one role for one scenario. Plausibility
// is an input, never something a scorer decides: a scorer that judged it would
// be measuring the same model twice.
type Rating struct {
	RoleID    string `json:"roleId"`
	Plausible bool   `json:"plausible"`
}

// Corpus is the whole frozen set.
type Corpus struct {
	// Synthetic says these are invented stand-ins rather than the recruiter's
	// real placements. It travels into every record.
	Synthetic bool       `json:"synthetic"`
	Note      string     `json:"note"`
	Scenarios []Scenario `json:"scenarios"`
	Listings  []Listing  `json:"listings"`
}

// Load reads the frozen corpus and refuses one that is missing its labels: an
// unlabelled corpus scores everything as captured, which is the most flattering
// possible bug.
func Load() (*Corpus, error) {
	raw, err := corpusFS.ReadFile("testdata/corpus.json")
	if err != nil {
		return nil, fmt.Errorf("reading the corpus: %w", err)
	}
	var c Corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("reading the corpus: %w", err)
	}
	for _, listing := range c.Listings {
		if len(listing.Material) == 0 {
			return nil, fmt.Errorf("listing %q has no labelled material aspects", listing.ID)
		}
	}
	for _, scenario := range c.Scenarios {
		if len(scenario.Ratings) == 0 {
			return nil, fmt.Errorf("scenario %q has no ratings", scenario.ID)
		}
	}
	return &c, nil
}

// Hash identifies the corpus by its bytes: one SHA-256 over every corpus file
// in name order, each name length-prefixed.
//
// Length prefixes and sorting are the same rule Phase 16 applies to assessment
// inputs, for the same reason — a hash that depends on the order files came
// back in is a hash that changes when nothing did.
func Hash() (string, error) {
	names := []string{}
	err := fs.WalkDir(corpusFS, "testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking the corpus: %w", err)
	}
	sort.Strings(names)

	files := map[string][]byte{}
	for _, name := range names {
		body, err := corpusFS.ReadFile(name)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", name, err)
		}
		files[name] = body
	}
	return hashOf(files), nil
}

// hashOf is the hash construction itself, over files given by name, so the
// rule can be checked against an altered copy without altering the corpus.
func hashOf(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	sum := sha256.New()
	for _, name := range names {
		// A hash.Hash never fails a write, so the errors are deliberately
		// dropped rather than threaded through a function that cannot fail.
		_, _ = fmt.Fprintf(sum, "%d:%s", len(name), name)
		_, _ = fmt.Fprintf(sum, "%d:", len(files[name]))
		_, _ = sum.Write(files[name])
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// criticalTypes are the aspect types the PRD singles out: a value invented here
// is a value the recruiter would act on.
var criticalTypes = map[profile.AspectType]bool{
	profile.Location:       true,
	profile.WorkRights:     true,
	profile.EmploymentType: true,
	profile.Compensation:   true,
}

// ClassifierScore is the four conditions, reported separately. A single boolean
// would answer "did it pass" and lose the only thing a failing run is good for.
type ClassifierScore struct {
	// Listing names which one this is: twenty scores with no names is a record
	// that says something failed without saying what.
	Listing     string  `json:"listing"`
	Extracted   int     `json:"extracted"`
	Material    int     `json:"material"`
	Captured    int     `json:"captured"`
	CaptureRate float64 `json:"captureRate"`
	// Missed names the labels no extracted aspect covered, so a capture number
	// can be argued with rather than only reported.
	Missed      []string `json:"missed"`
	Uncited     []string `json:"uncited"`
	Unsupported []string `json:"unsupported"`
	Misreported []string `json:"misreported"`

	AllCited         bool `json:"allCited"`
	NoUnsupported    bool `json:"noUnsupported"`
	CaptureMet       bool `json:"captureMet"`
	ConstraintsExact bool `json:"constraintsExact"`
	Pass             bool `json:"pass"`
}

// ScoreClassifier scores one listing's extraction against its labels.
//
// A citation is valid when it names one of the chunks the classifier was given
// and quotes wording that appears in it — the same rule the contract enforces,
// applied again here so the benchmark measures the product's rule rather than a
// looser one of its own.
func ScoreClassifier(listing Listing, extracted []profile.Aspect, sources map[uint]string) ClassifierScore {
	score := ClassifierScore{
		Listing:   listing.ID,
		Extracted: len(extracted), Material: len(listing.Material),
		Missed: []string{}, Uncited: []string{}, Unsupported: []string{}, Misreported: []string{},
	}

	for _, aspect := range extracted {
		if !cited(aspect, sources) {
			score.Uncited = append(score.Uncited, aspect.Wording)
		}
		if criticalTypes[aspect.Type] && !supported(aspect, sources) {
			score.Unsupported = append(score.Unsupported, string(aspect.Type)+": "+aspect.Wording)
		}
	}

	// Capture asks whether the substance the recruiter marked is present, not
	// whether the model chose the same words for it.
	//
	// Meaning-key equality alone was the first rule here, and the first live run
	// showed why it is wrong: a label reading "Go" never equals an extracted
	// aspect reading "Must have strong Go and production SQLite experience",
	// though a recruiter would obviously count it as captured. So a label counts
	// when an extracted aspect of the same type either means the same thing by
	// the product's own duplicate rule, or contains the labelled wording.
	found := map[string]profile.Aspect{}
	for _, aspect := range extracted {
		found[profile.MeaningKey(aspect)] = aspect
	}
	for _, want := range listing.Material {
		if captured(want, extracted, found) {
			score.Captured++
			continue
		}
		score.Missed = append(score.Missed, string(want.Type)+": "+want.Wording)
	}
	if score.Material > 0 {
		score.CaptureRate = float64(score.Captured) / float64(score.Material)
	}

	// A structured constraint is reproduced correctly or it is not. There is no
	// partial credit for a compensation figure that is nearly right.
	//
	// Matching is by type rather than by meaning key: a wrong value produces a
	// different key, and a lookup by key would report a misreported constraint
	// as a missing one — which sends the reader looking for the wrong problem.
	byType := map[profile.AspectType][]profile.Aspect{}
	for _, aspect := range extracted {
		byType[aspect.Type] = append(byType[aspect.Type], aspect)
	}
	for _, want := range listing.Structured {
		candidates := byType[want.Type]
		if len(candidates) == 0 {
			score.Misreported = append(score.Misreported, string(want.Type)+": missing")
			continue
		}
		matched := false
		for _, got := range candidates {
			if reproduces(want.Structured, got.Structured) {
				matched = true
				// A field the source never states is a value introduced, which
				// is the other half of the PRD's rule: "no unsupported
				// must-have, location, work-rights, employment-type, or
				// compensation value is introduced".
				for _, extra := range extraFields(want.Structured, got.Structured) {
					score.Unsupported = append(score.Unsupported,
						fmt.Sprintf("%s: %s=%v, which the source does not state",
							want.Type, extra, got.Structured[extra]))
				}
				break
			}
		}
		if !matched {
			score.Misreported = append(score.Misreported,
				fmt.Sprintf("%s: %v, want %v", want.Type, candidates[0].Structured, want.Structured))
		}
	}

	score.AllCited = len(score.Uncited) == 0
	score.NoUnsupported = len(score.Unsupported) == 0
	score.CaptureMet = score.Material > 0 && score.CaptureRate >= CaptureThreshold
	score.ConstraintsExact = len(score.Misreported) == 0
	score.Pass = score.AllCited && score.NoUnsupported && score.CaptureMet && score.ConstraintsExact
	return score
}

// captured reports whether one labelled aspect is present in what was
// extracted.
//
// Type has to matter, but not equally. The five constraint types drive
// comparisons — "Melbourne" and "Sydney" are opposite facts — so a labelled
// constraint is captured only by an aspect of that same type, and whether its
// value is right is scored separately. The descriptive types are not sharply
// separable: whether "multi-region systems" is a skill, an experience, or a
// responsibility is a judgement call the taxonomy does not settle, and a
// recruiter checking whether the requirement was captured would accept any of
// them. Measuring against my own choice among them would be measuring my
// labelling, not the model.
//
// Containment is one-directional on purpose: the label is the terser statement,
// and finding it inside a fuller one is the case this rule exists for. The
// reverse — a one-word extraction "matching" a detailed label — is not capture.
func captured(want profile.Aspect, extracted []profile.Aspect, byKey map[string]profile.Aspect) bool {
	if _, ok := byKey[profile.MeaningKey(want)]; ok {
		return true
	}
	if _, constrained := profile.StructuredFields(want.Type); constrained {
		for _, got := range extracted {
			if got.Type == want.Type {
				return true
			}
		}
		return false
	}
	needle := normalize(want.Wording)
	if needle == "" {
		return false
	}
	for _, got := range extracted {
		if _, constrained := profile.StructuredFields(got.Type); constrained {
			continue
		}
		if strings.Contains(normalize(got.Wording), needle) {
			return true
		}
	}
	return false
}

// cited reports whether an aspect carries at least one citation that resolves.
func cited(a profile.Aspect, sources map[uint]string) bool {
	for _, c := range a.Citations {
		if c.Record != "" {
			return true
		}
		if text, ok := sources[c.ChunkID]; ok && quoted(text, c.Quote) {
			return true
		}
	}
	return false
}

// supported is cited, for the aspect types where an invented value is the
// failure the PRD names.
func supported(a profile.Aspect, sources map[uint]string) bool { return cited(a, sources) }

// quoted is the contract's quote rule: the wording appears in the chunk, with
// whitespace normalized so a line break is not a mismatch.
func quoted(text, quote string) bool {
	if strings.TrimSpace(quote) == "" {
		return false
	}
	return strings.Contains(normalize(text), normalize(quote))
}

func normalize(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// reproduces reports whether every value the source states was reproduced.
//
// It is a subset check, not equality. The label says what the listing states;
// an extraction that also carries a field the listing is silent about has not
// misreported the constraint — it has introduced a value, which is scored
// separately and under its own name.
func reproduces(want, got map[string]any) bool {
	w, g := canonical(want), canonical(got)
	for field, value := range w {
		other, ok := g[field]
		if !ok {
			return false
		}
		a, err := json.Marshal(value)
		if err != nil {
			return false
		}
		b, err := json.Marshal(other)
		if err != nil || string(a) != string(b) {
			return false
		}
	}
	return true
}

// extraFields names the structured fields an extraction carries that the label
// does not, in sorted order so a record does not change between identical runs.
func extraFields(want, got map[string]any) []string {
	w, g := canonical(want), canonical(got)
	out := []string{}
	for field := range g {
		if _, ok := w[field]; !ok {
			out = append(out, field)
		}
	}
	sort.Strings(out)
	return out
}

// canonical lowercases string values and drops empty ones, so "Melbourne" and
// "melbourne" are the same constraint and an absent field is absent either way.
func canonical(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		switch typed := v.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
			out[k] = normalize(typed)
		case nil:
			continue
		default:
			out[k] = v
		}
	}
	return out
}

// TopFive is one scenario's result: the roles the shortlist put in the top five,
// in order, with the recruiter's rating attached.
type TopFive struct {
	ScenarioID string   `json:"scenarioId"`
	RoleIDs    []string `json:"roleIds"`
	// Note records why a scenario produced nothing, when it produced nothing.
	// An empty top five because the candidate profile could not be built is not
	// the matcher failing, and a record that shows both as "0 plausible" sends
	// the reader after the wrong thing.
	Note string `json:"note,omitempty"`
}

// ScenarioScore is one scenario's outcome.
type ScenarioScore struct {
	ScenarioID string `json:"scenarioId"`
	// Distinct is the top five after duplicates collapse.
	Distinct  []string `json:"distinct"`
	Plausible int      `json:"plausible"`
	Met       bool     `json:"met"`
	// Note carries the reason a scenario produced nothing, verbatim.
	Note string `json:"note,omitempty"`
}

// MatchingScore is the benchmark's outcome across the five scenarios.
type MatchingScore struct {
	Scenarios []ScenarioScore `json:"scenarios"`
	MetCount  int             `json:"metCount"`
	Pass      bool            `json:"pass"`
}

// ScoreMatching applies the PRD's rule: duplicates collapse first, absent slots
// count as not plausible, three of the top five must be plausible, in at least
// four of the five scenarios.
func ScoreMatching(corpus *Corpus, results []TopFive) MatchingScore {
	ratings := map[string]map[string]bool{}
	for _, s := range corpus.Scenarios {
		ratings[s.ID] = map[string]bool{}
		for _, r := range s.Ratings {
			ratings[s.ID][r.RoleID] = r.Plausible
		}
	}

	out := MatchingScore{Scenarios: []ScenarioScore{}}
	for _, result := range results {
		seen := map[string]bool{}
		score := ScenarioScore{ScenarioID: result.ScenarioID, Distinct: []string{}, Note: result.Note}
		for _, id := range result.RoleIDs {
			if seen[id] {
				continue
			}
			seen[id] = true
			if len(score.Distinct) == 5 {
				break
			}
			score.Distinct = append(score.Distinct, id)
			// An unrated role is not plausible: the recruiter did not say it was.
			if ratings[result.ScenarioID][id] {
				score.Plausible++
			}
		}
		score.Met = score.Plausible >= 3
		if score.Met {
			out.MetCount++
		}
		out.Scenarios = append(out.Scenarios, score)
	}
	// A scenario that was not run did not pass.
	out.Pass = out.MetCount >= 4 && len(results) >= 5
	return out
}

// Outcome is what a run concluded.
type Outcome string

const (
	// OutcomePass is a local-only run that met every condition.
	OutcomePass Outcome = "pass"
	// OutcomeFail is a run that met the conditions for a result and missed one.
	OutcomeFail Outcome = "fail"
	// OutcomeInconclusive is a run that could not say anything: too few eligible
	// roles for the count to mean something about the matcher.
	OutcomeInconclusive Outcome = "inconclusive"
	// OutcomeCloudAssisted is a run that used a cloud override. It is recorded
	// separately and cannot pass the PoC.
	OutcomeCloudAssisted Outcome = "cloud-assisted"
)

// Conclude turns a run's facts into its outcome, applying the two rules that
// are easy to get wrong in the moment: a thin source corpus says nothing, and a
// cloud-assisted run cannot pass.
func Conclude(eligibleRoles int, cloudAssisted, met bool) Outcome {
	if cloudAssisted {
		return OutcomeCloudAssisted
	}
	if eligibleRoles < MinimumEligibleRoles {
		return OutcomeInconclusive
	}
	if met {
		return OutcomePass
	}
	return OutcomeFail
}
