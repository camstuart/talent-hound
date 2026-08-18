package bench

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented, as is the frozen corpus itself. No real
// candidate information appears in this repository.

func TestTheCorpusIsCompleteAndSaysItIsInvented(t *testing.T) {
	corpus, err := Load()
	if err != nil {
		t.Fatalf("loading the corpus: %v", err)
	}
	if len(corpus.Scenarios) != 5 {
		t.Fatalf("%d scenarios, want 5", len(corpus.Scenarios))
	}
	if len(corpus.Listings) != 20 {
		t.Fatalf("%d listings, want 20", len(corpus.Listings))
	}
	if !corpus.Synthetic || !strings.Contains(corpus.Note, "invented") {
		t.Fatal("the corpus does not say its content is invented")
	}
	for _, listing := range corpus.Listings {
		if len(listing.Material) == 0 || len(listing.Structured) == 0 {
			t.Fatalf("listing %q is unlabelled", listing.ID)
		}
		if listing.Markdown == "" {
			t.Fatalf("listing %q has no text to classify", listing.ID)
		}
	}
	for _, scenario := range corpus.Scenarios {
		if scenario.Resume == "" || len(scenario.Ratings) == 0 {
			t.Fatalf("scenario %q is incomplete", scenario.ID)
		}
	}
}

// A hash that depends on the order files came back in is a hash that changes
// when nothing did — so it is checked across a separate process too.
func TestTheCorpusHashIsStableAcrossProcesses(t *testing.T) {
	first, err := Hash()
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	second, err := Hash()
	if err != nil {
		t.Fatalf("hashing again: %v", err)
	}
	if first != second {
		t.Fatalf("two hashes in one process differ:\n%s\n%s", first, second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "run", "./hashcheck").CombinedOutput() // #nosec G204
	if err != nil {
		t.Skipf("could not run the subprocess check: %v: %s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != first {
		t.Fatalf("a separate process hashed %q, want %q", got, first)
	}
}

func TestOneChangedByteChangesTheHash(t *testing.T) {
	// The embedded corpus cannot be changed at run time, so the rule is checked
	// against the same construction over an altered copy.
	before, err := Hash()
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	raw, err := corpusFS.ReadFile("testdata/corpus.json")
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	altered := append(append([]byte{}, raw...), ' ')
	if hashOf(map[string][]byte{"testdata/corpus.json": altered}) == before {
		t.Fatal("one changed byte did not change the hash")
	}
	if hashOf(map[string][]byte{"testdata/corpus.json": raw}) != before {
		t.Fatal("the same bytes did not reproduce the hash")
	}
}

// aspect is one labelled aspect, and quoting is the source it cites.
func aspect(kind profile.AspectType, wording string, structured map[string]any) profile.Aspect {
	return profile.Aspect{Type: kind, Wording: wording, Structured: structured}
}

func cite(a profile.Aspect, quote string) profile.Aspect {
	a.Citations = []profile.Citation{{ChunkID: 1, Quote: quote}}
	return a
}

func listingFixture() (Listing, map[uint]string) {
	sources := map[uint]string{
		1: "Must have Go. Melbourne, hybrid, permanent, AUD 180,000 base plus superannuation.",
	}
	material := []profile.Aspect{
		aspect(profile.Skill, "Go", nil),
		aspect(profile.Skill, "SQLite", nil),
		aspect(profile.Skill, "multi-region systems", nil),
		aspect(profile.Location, "Melbourne", map[string]any{"place": "Melbourne"}),
		aspect(profile.Compensation, "AUD 180,000", map[string]any{"currency": "AUD", "amount": 180000}),
	}
	structured := material[3:]
	return Listing{ID: "fixture", Material: material, Structured: structured}, sources
}

func TestTheClassifierPassesOnlyWhenEveryConditionHolds(t *testing.T) {
	listing, sources := listingFixture()
	all := []profile.Aspect{
		cite(aspect(profile.Skill, "Go", nil), "Must have Go"),
		cite(aspect(profile.Skill, "SQLite", nil), "Must have Go"),
		cite(aspect(profile.Skill, "multi-region systems", nil), "Must have Go"),
		cite(aspect(profile.Location, "Melbourne", map[string]any{"place": "Melbourne"}), "Melbourne"),
		cite(aspect(profile.Compensation, "AUD 180,000",
			map[string]any{"currency": "AUD", "amount": 180000}), "AUD 180,000"),
	}

	score := ScoreClassifier(listing, all, sources)
	if !score.Pass {
		t.Fatalf("a complete, cited, exact extraction failed: %+v", score)
	}
	if score.CaptureRate != 1 {
		t.Fatalf("capture = %.2f, want 1", score.CaptureRate)
	}
}

func TestAnUncitedAspectFailsHoweverHighTheCapture(t *testing.T) {
	listing, sources := listingFixture()
	extracted := []profile.Aspect{
		aspect(profile.Skill, "Go", nil), // no citation at all
		cite(aspect(profile.Skill, "SQLite", nil), "Must have Go"),
		cite(aspect(profile.Skill, "multi-region systems", nil), "Must have Go"),
		cite(aspect(profile.Location, "Melbourne", map[string]any{"place": "Melbourne"}), "Melbourne"),
		cite(aspect(profile.Compensation, "AUD 180,000",
			map[string]any{"currency": "AUD", "amount": 180000}), "AUD 180,000"),
	}
	score := ScoreClassifier(listing, extracted, sources)
	if score.CaptureRate != 1 {
		t.Fatalf("capture = %.2f, want 1", score.CaptureRate)
	}
	if score.Pass || score.AllCited {
		t.Fatalf("an uncited aspect passed: %+v", score)
	}
	if len(score.Uncited) != 1 {
		t.Fatalf("the uncited aspect was not named: %+v", score.Uncited)
	}
}

// A citation that quotes something the chunk does not say is not a citation.
func TestAQuoteThatIsNotInTheSourceIsNotACitation(t *testing.T) {
	listing, sources := listingFixture()
	extracted := []profile.Aspect{
		cite(aspect(profile.Skill, "Rust", nil), "Must have Rust"),
	}
	score := ScoreClassifier(listing, extracted, sources)
	if score.AllCited {
		t.Fatal("an invented quote counted as a citation")
	}
}

func TestAnUnsupportedCriticalConstraintIsNamedAndFails(t *testing.T) {
	listing, sources := listingFixture()
	extracted := []profile.Aspect{
		// Invented: nothing in the source says anything about work rights.
		aspect(profile.WorkRights, "citizenship required", map[string]any{"rights": "citizen"}),
	}
	score := ScoreClassifier(listing, extracted, sources)
	if score.NoUnsupported || score.Pass {
		t.Fatalf("an invented work-rights value passed: %+v", score)
	}
	if len(score.Unsupported) != 1 || !strings.Contains(score.Unsupported[0], "work_rights") {
		t.Fatalf("the unsupported constraint was not named: %+v", score.Unsupported)
	}
}

// 79% is a fail and 80% is a pass: the boundary is the PRD's, so it is tested
// at the boundary.
func TestCaptureIsMeasuredAtTheThreshold(t *testing.T) {
	material := make([]profile.Aspect, 100)
	for i := range material {
		material[i] = aspect(profile.Skill, skillName(i), nil)
	}
	listing := Listing{ID: "boundary", Material: material}
	sources := map[uint]string{1: strings.Join(allNames(100), " ")}

	for _, tc := range []struct {
		captured int
		want     bool
	}{{79, false}, {80, true}, {100, true}} {
		extracted := []profile.Aspect{}
		for i := 0; i < tc.captured; i++ {
			extracted = append(extracted, cite(aspect(profile.Skill, skillName(i), nil), skillName(i)))
		}
		score := ScoreClassifier(listing, extracted, sources)
		if score.CaptureMet != tc.want {
			t.Fatalf("%d of 100 captured: met = %v, want %v (rate %.2f)",
				tc.captured, score.CaptureMet, tc.want, score.CaptureRate)
		}
	}
}

func TestAMisreportedStructuredConstraintFailsAndIsNamed(t *testing.T) {
	listing, sources := listingFixture()
	extracted := []profile.Aspect{
		cite(aspect(profile.Location, "Melbourne", map[string]any{"place": "Sydney"}), "Melbourne"),
		cite(aspect(profile.Compensation, "AUD 180,000",
			map[string]any{"currency": "AUD", "amount": 180000}), "AUD 180,000"),
	}
	score := ScoreClassifier(listing, extracted, sources)
	if score.ConstraintsExact || score.Pass {
		t.Fatalf("a wrong location passed: %+v", score)
	}
	if len(score.Misreported) == 0 || !strings.Contains(score.Misreported[0], "location") {
		t.Fatalf("the constraint was not named: %+v", score.Misreported)
	}
}

// Field order and case are not differences; a different value is.
func TestStructuredComparisonIgnoresOrderAndCase(t *testing.T) {
	listing := Listing{
		ID:         "case",
		Material:   []profile.Aspect{aspect(profile.Location, "Melbourne", map[string]any{"place": "Melbourne"})},
		Structured: []profile.Aspect{aspect(profile.Location, "Melbourne", map[string]any{"place": "Melbourne"})},
	}
	sources := map[uint]string{1: "Melbourne"}
	extracted := []profile.Aspect{
		cite(aspect(profile.Location, "Melbourne", map[string]any{"place": "melbourne"}), "Melbourne"),
	}
	if score := ScoreClassifier(listing, extracted, sources); !score.ConstraintsExact {
		t.Fatalf("a case difference counted as a wrong value: %+v", score.Misreported)
	}
}

func TestMatchingPassesOnFourOfFiveScenarios(t *testing.T) {
	corpus := ratedCorpus()
	results := []TopFive{
		top("s1", "a", "b", "c", "x", "y"),
		top("s2", "a", "b", "c", "x", "y"),
		top("s3", "a", "b", "c", "x", "y"),
		top("s4", "a", "b", "c", "x", "y"),
		top("s5", "x", "y", "z", "x", "y"),
	}
	score := ScoreMatching(corpus, results)
	if score.MetCount != 4 || !score.Pass {
		t.Fatalf("four of five did not pass: %+v", score)
	}
}

func TestThreeOfFiveIsNotEnough(t *testing.T) {
	corpus := ratedCorpus()
	results := []TopFive{
		top("s1", "a", "b", "c", "x", "y"),
		top("s2", "a", "b", "c", "x", "y"),
		top("s3", "a", "b", "c", "x", "y"),
		top("s4", "a", "b", "x", "y", "z"),
		top("s5", "x", "y", "z", "x", "y"),
	}
	if score := ScoreMatching(corpus, results); score.Pass {
		t.Fatalf("three of five passed: %+v", score)
	}
}

// A duplicate does not fill a slot, and the freed slot is not a free plausible.
func TestDuplicatesCollapseBeforeCounting(t *testing.T) {
	corpus := ratedCorpus()
	score := ScoreMatching(corpus, []TopFive{top("s1", "a", "a", "a", "x", "y")})
	got := score.Scenarios[0]
	if len(got.Distinct) != 3 {
		t.Fatalf("distinct = %v, want three", got.Distinct)
	}
	if got.Plausible != 1 || got.Met {
		t.Fatalf("a repeated plausible role counted more than once: %+v", got)
	}
}

func TestAnAbsentSlotIsNotPlausible(t *testing.T) {
	corpus := ratedCorpus()
	score := ScoreMatching(corpus, []TopFive{top("s1", "a", "b")})
	if got := score.Scenarios[0]; got.Plausible != 2 || got.Met {
		t.Fatalf("a short list was padded: %+v", got)
	}
}

// A scenario that was not run did not pass.
func TestAMissingScenarioDoesNotPass(t *testing.T) {
	corpus := ratedCorpus()
	results := []TopFive{
		top("s1", "a", "b", "c"), top("s2", "a", "b", "c"),
		top("s3", "a", "b", "c"), top("s4", "a", "b", "c"),
	}
	if score := ScoreMatching(corpus, results); score.Pass {
		t.Fatalf("four scenarios out of five passed: %+v", score)
	}
}

func TestAThinLiveRunIsInconclusiveRatherThanAResult(t *testing.T) {
	if got := Conclude(9, false, true); got != OutcomeInconclusive {
		t.Fatalf("nine eligible roles concluded %q", got)
	}
	if got := Conclude(9, false, false); got != OutcomeInconclusive {
		t.Fatalf("nine eligible roles with a miss concluded %q", got)
	}
	if got := Conclude(10, false, true); got != OutcomePass {
		t.Fatalf("ten eligible roles concluded %q", got)
	}
	if got := Conclude(10, false, false); got != OutcomeFail {
		t.Fatalf("ten eligible roles with a miss concluded %q", got)
	}
}

func TestACloudAssistedRunCannotPass(t *testing.T) {
	for _, eligible := range []int{0, 10, 100} {
		if got := Conclude(eligible, true, true); got != OutcomeCloudAssisted {
			t.Fatalf("a cloud-assisted run concluded %q", got)
		}
	}
}

func TestARecordCarriesWhatItRanWith(t *testing.T) {
	corpus, err := Load()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	hash, err := Hash()
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	record := NewRecord("2026-08-18T00:00:00Z", "0.1.0-poc", "windows/amd64", corpus, hash,
		map[string]Assignment{
			"embed":    {Model: "nomic-embed-text", Digest: "sha256:abc"},
			"classify": {Model: "qwen2.5:7b-instruct", Digest: "sha256:def"},
			// Used, but never assigned: recorded, not omitted.
			"generate": {},
		})
	record.EligibleRoles = 12
	record.Conclude()

	if record.CorpusHash != hash || !strings.Contains(record.CorpusIs, "invented") {
		t.Fatalf("the record does not carry its corpus: %+v", record)
	}
	if record.SchemaVersion != profile.SchemaVersion || record.PromptVersion != profile.PromptVersion {
		t.Fatal("the record does not carry the prompt and schema versions")
	}
	if len(record.Assignments) != 3 {
		t.Fatalf("%d assignments recorded, want every role used", len(record.Assignments))
	}
	found := false
	for _, a := range record.Assignments {
		if a.Role == "generate" && a.Model == "unassigned" {
			found = true
		}
	}
	if !found {
		t.Fatal("an unassigned role was omitted rather than stated")
	}

	raw, err := record.JSON()
	if err != nil {
		t.Fatalf("writing: %v", err)
	}
	if !strings.Contains(string(raw), "corpusHash") {
		t.Fatal("the stored record has no corpus hash")
	}
	summary := record.Summary()
	for _, want := range []string{"nomic-embed-text", "sha256:abc", hash, "Outcome:"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("the summary omits %q:\n%s", want, summary)
		}
	}
}

// A failing run's value is knowing which of the four conditions failed.
func TestTheRecordSaysWhichConditionFailed(t *testing.T) {
	corpus, _ := Load()
	record := NewRecord("2026-08-18T00:00:00Z", "0.1.0-poc", "windows/amd64", corpus, "hash",
		map[string]Assignment{"classify": {Model: "m", Digest: "d"}})
	record.Classifier = []ClassifierScore{{
		Material: 10, Captured: 7, CaptureRate: 0.7,
		AllCited: true, NoUnsupported: true, CaptureMet: false, ConstraintsExact: true,
	}}
	record.EligibleRoles = 12
	record.Conclude()

	if record.Outcome != OutcomeFail {
		t.Fatalf("outcome = %q, want a failure", record.Outcome)
	}
	summary := record.Summary()
	if !strings.Contains(summary, "capture 70%") {
		t.Fatalf("the summary does not say what failed:\n%s", summary)
	}
	if !strings.Contains(summary, "cited true") || !strings.Contains(summary, "constraints exact true") {
		t.Fatalf("the summary does not report the conditions that passed:\n%s", summary)
	}
}

func TestAnInconclusiveRecordSaysWhy(t *testing.T) {
	corpus, _ := Load()
	record := NewRecord("2026-08-18T00:00:00Z", "0.1.0-poc", "windows/amd64", corpus, "hash", nil)
	record.EligibleRoles = 4
	record.Conclude()
	if record.Outcome != OutcomeInconclusive {
		t.Fatalf("outcome = %q", record.Outcome)
	}
	if !strings.Contains(record.Summary(), "source coverage") {
		t.Fatalf("an inconclusive record did not say why:\n%s", record.Summary())
	}
}

// A measurement below its provisional target is recorded as measured, not
// restated as a pass.
func TestAMissedMeasurementIsRecordedAsMeasured(t *testing.T) {
	corpus, _ := Load()
	record := NewRecord("2026-08-18T00:00:00Z", "0.1.0-poc", "windows/amd64", corpus, "hash", nil)
	record.Measurements = []Measurement{{
		Name: "retrieval p95", Value: 4.2, Unit: "s", Target: 2, Met: false,
		Conditions: "cold cache, 20 roles, laptop on battery",
	}}
	summary := record.Summary()
	if !strings.Contains(summary, "4.20 s") || !strings.Contains(summary, "met false") {
		t.Fatalf("the measurement was not recorded as measured:\n%s", summary)
	}
	if !strings.Contains(summary, "battery") {
		t.Fatalf("the measurement lost its conditions:\n%s", summary)
	}
}

func TestTheCorpusHasNoRealCandidateInformation(t *testing.T) {
	raw, err := os.ReadFile("testdata/corpus.json")
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	text := string(raw)
	// Invented content does not carry contact details.
	for _, shape := range []string{"@gmail", "@outlook", "@yahoo", "linkedin.com/in/"} {
		if strings.Contains(strings.ToLower(text), shape) {
			t.Fatalf("the corpus contains %q", shape)
		}
	}
}

func skillName(i int) string { return "skill-" + strconv.Itoa(i) }

func allNames(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = skillName(i)
	}
	return out
}

func top(scenario string, roles ...string) TopFive {
	return TopFive{ScenarioID: scenario, RoleIDs: roles}
}

// ratedCorpus is five scenarios where a, b, c are plausible and x, y, z are not.
func ratedCorpus() *Corpus {
	corpus := &Corpus{Synthetic: true}
	for _, id := range []string{"s1", "s2", "s3", "s4", "s5"} {
		scenario := Scenario{ID: id}
		for _, role := range []string{"a", "b", "c"} {
			scenario.Ratings = append(scenario.Ratings, Rating{RoleID: role, Plausible: true})
		}
		for _, role := range []string{"x", "y", "z"} {
			scenario.Ratings = append(scenario.Ratings, Rating{RoleID: role, Plausible: false})
		}
		corpus.Scenarios = append(corpus.Scenarios, scenario)
	}
	return corpus
}

// The first live run scored zero capture on listings the model had plainly
// decomposed, because a terse label never equals a fuller extracted wording.
// Capture asks whether the substance is there.
func TestALabelIsCapturedByAFullerWordingOfTheSameThing(t *testing.T) {
	listing := Listing{
		ID: "wording",
		Material: []profile.Aspect{
			aspect(profile.Skill, "Go", nil),
			aspect(profile.Skill, "multi-region systems", nil),
		},
	}
	sources := map[uint]string{1: "Must have strong Go and production SQLite experience. " +
		"Experience operating multi-region systems is essential."}
	extracted := []profile.Aspect{
		cite(aspect(profile.Skill, "Must have strong Go and production SQLite experience", nil),
			"Must have strong Go"),
		cite(aspect(profile.Skill, "Experience operating multi-region systems is essential", nil),
			"multi-region systems is essential"),
	}
	score := ScoreClassifier(listing, extracted, sources)
	if score.Captured != 2 || score.CaptureRate != 1 {
		t.Fatalf("capture = %d of %d (%.2f), want both", score.Captured, score.Material, score.CaptureRate)
	}
}

// Containment runs one way: the label is the terser statement, and a one-word
// extraction does not capture a detailed requirement.
func TestAShorterExtractionDoesNotCaptureADetailedLabel(t *testing.T) {
	listing := Listing{
		ID:       "reverse",
		Material: []profile.Aspect{aspect(profile.Skill, "eight years of production Go on multi-region systems", nil)},
	}
	sources := map[uint]string{1: "Go"}
	extracted := []profile.Aspect{cite(aspect(profile.Skill, "Go", nil), "Go")}
	if score := ScoreClassifier(listing, extracted, sources); score.Captured != 0 {
		t.Fatalf("a one-word extraction captured a detailed label: %+v", score)
	}
}

// The type still has to match: a location named in a skill is not the location
// constraint.
func TestCaptureStillRequiresTheSameType(t *testing.T) {
	listing := Listing{
		ID:       "types",
		Material: []profile.Aspect{aspect(profile.Location, "Melbourne", map[string]any{"place": "Melbourne"})},
	}
	sources := map[uint]string{1: "Melbourne"}
	extracted := []profile.Aspect{cite(aspect(profile.Skill, "Melbourne-based team", nil), "Melbourne")}
	if score := ScoreClassifier(listing, extracted, sources); score.Captured != 0 {
		t.Fatalf("a skill captured a location label: %+v", score)
	}
}

// The PRD states its classifier bars over the corpus, not per listing. A run
// where one listing is weak and the rest are strong passes if the corpus as a
// whole clears 80% — requiring every listing to clear it alone would be a
// harder test than the one the product was accepted against.
func TestTheClassifierBarIsMeasuredOverTheCorpus(t *testing.T) {
	corpus, _ := Load()
	record := NewRecord("2026-08-18T00:00:00Z", "0.1.0-poc", "darwin/arm64", corpus, "hash", nil)
	record.Classifier = []ClassifierScore{
		{Material: 10, Captured: 10, CaptureRate: 1},
		{Material: 10, Captured: 10, CaptureRate: 1},
		// Well under the bar by itself, and the corpus is still at 87%.
		{Material: 10, Captured: 6, CaptureRate: 0.6},
	}
	totals := record.ClassifierTotals()
	if totals.Captured != 26 || totals.Material != 30 {
		t.Fatalf("totals = %d/%d, want 26/30", totals.Captured, totals.Material)
	}
	if !totals.CaptureMet || !totals.Pass {
		t.Fatalf("a corpus at %.0f%% did not pass: %+v", totals.CaptureRate*100, totals)
	}
}

func TestOneUncitedAspectAnywhereFailsTheCorpus(t *testing.T) {
	corpus, _ := Load()
	record := NewRecord("2026-08-18T00:00:00Z", "0.1.0-poc", "darwin/arm64", corpus, "hash", nil)
	record.Classifier = []ClassifierScore{
		{Material: 10, Captured: 10, CaptureRate: 1},
		{Material: 10, Captured: 10, CaptureRate: 1, Uncited: []string{"invented"}},
	}
	totals := record.ClassifierTotals()
	if totals.AllCited || totals.Pass {
		t.Fatalf("one uncited aspect passed: %+v", totals)
	}
}

func TestACorpusJustUnderTheBarFails(t *testing.T) {
	corpus, _ := Load()
	record := NewRecord("2026-08-18T00:00:00Z", "0.1.0-poc", "darwin/arm64", corpus, "hash", nil)
	record.Classifier = []ClassifierScore{{Material: 100, Captured: 79, CaptureRate: 0.79}}
	if totals := record.ClassifierTotals(); totals.CaptureMet || totals.Pass {
		t.Fatalf("79%% over the corpus passed: %+v", totals)
	}
	record.Classifier = []ClassifierScore{{Material: 100, Captured: 80, CaptureRate: 0.80}}
	if totals := record.ClassifierTotals(); !totals.CaptureMet {
		t.Fatalf("80%% over the corpus failed: %+v", totals)
	}
}

// Reproduction is a subset check: the label says what the listing states, and
// an extraction carrying a field the listing is silent about has not
// misreported the constraint.
func TestAnExtraFieldIsNotAMisreportedConstraint(t *testing.T) {
	listing := Listing{
		ID:         "extra",
		Material:   []profile.Aspect{aspect(profile.Location, "Melbourne", map[string]any{"city": "Melbourne"})},
		Structured: []profile.Aspect{aspect(profile.Location, "Melbourne", map[string]any{"city": "Melbourne"})},
	}
	sources := map[uint]string{1: "Melbourne, Australia"}
	extracted := []profile.Aspect{
		cite(aspect(profile.Location, "Melbourne, Australia",
			map[string]any{"city": "Melbourne", "country": "Australia"}), "Melbourne, Australia"),
	}
	score := ScoreClassifier(listing, extracted, sources)
	if !score.ConstraintsExact {
		t.Fatalf("an extra field counted as misreported: %+v", score.Misreported)
	}
	// It is scored under its own name instead.
	if len(score.Unsupported) != 1 || !strings.Contains(score.Unsupported[0], "country") {
		t.Fatalf("the introduced value was not named: %+v", score.Unsupported)
	}
}

// A wrong value for a field the listing does state is still a misreported
// constraint.
func TestAWrongValueForAStatedFieldStillFails(t *testing.T) {
	listing := Listing{
		ID:         "wrong",
		Material:   []profile.Aspect{aspect(profile.Location, "Melbourne", map[string]any{"city": "Melbourne"})},
		Structured: []profile.Aspect{aspect(profile.Location, "Melbourne", map[string]any{"city": "Melbourne"})},
	}
	sources := map[uint]string{1: "Melbourne"}
	extracted := []profile.Aspect{
		cite(aspect(profile.Location, "Melbourne", map[string]any{"city": "Sydney"}), "Melbourne"),
	}
	if score := ScoreClassifier(listing, extracted, sources); score.ConstraintsExact {
		t.Fatalf("a wrong city passed: %+v", score)
	}
}

// A missing field the listing does state is a misreported constraint, not an
// absence to be forgiven.
func TestAMissingStatedFieldFails(t *testing.T) {
	listing := Listing{
		ID: "missing",
		Material: []profile.Aspect{aspect(profile.Compensation, "AUD 180,000 base",
			map[string]any{"currency": "AUD", "minimum": 180000, "basis": "base"})},
		Structured: []profile.Aspect{aspect(profile.Compensation, "AUD 180,000 base",
			map[string]any{"currency": "AUD", "minimum": 180000, "basis": "base"})},
	}
	sources := map[uint]string{1: "AUD 180,000 base"}
	extracted := []profile.Aspect{
		cite(aspect(profile.Compensation, "AUD 180,000 base",
			map[string]any{"currency": "AUD", "minimum": 180000}), "AUD 180,000 base"),
	}
	if score := ScoreClassifier(listing, extracted, sources); score.ConstraintsExact {
		t.Fatalf("a dropped basis passed: %+v", score)
	}
}
