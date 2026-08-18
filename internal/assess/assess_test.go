package assess

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

func baseInputs() Inputs {
	return Inputs{
		CandidateProfileVersion: 3,
		CandidateProfileState:   "approved",
		RoleProfileVersion:      2,
		RoleProfileState:        "ready",
		CriteriaVersion:         7,
		EvidenceHashes:          []string{"aaa", "bbb", "ccc"},
		ComparisonVersion:       ComparisonVersion,
		RankingVersion:          RankingVersion,
		EndpointRevision:        4,
		ModelDigest:             "sha256:abc",
		ModelName:               "synthetic-generate",
		PromptVersion:           PromptVersion,
		SchemaVersion:           SchemaVersion,
		GenerationParams:        `{"temperature":0}`,
		RoleStale:               false,
	}
}

// A missing field is a result that gets reused when it should not have been,
// which is the failure this whole mechanism exists to prevent. So every listed
// input is changed alone and must move the hash.
func TestEveryListedInputChangesTheHash(t *testing.T) {
	base := baseInputs().Hash()

	changes := map[string]func(*Inputs){
		"the candidate profile version": func(in *Inputs) { in.CandidateProfileVersion = 4 },
		"the candidate profile state":   func(in *Inputs) { in.CandidateProfileState = "proposed" },
		"the role profile version":      func(in *Inputs) { in.RoleProfileVersion = 3 },
		"the role profile state":        func(in *Inputs) { in.RoleProfileState = "stale" },
		"the criteria version":          func(in *Inputs) { in.CriteriaVersion = 8 },
		"an evidence hash":              func(in *Inputs) { in.EvidenceHashes = []string{"aaa", "bbb", "ddd"} },
		"the amount of evidence":        func(in *Inputs) { in.EvidenceHashes = []string{"aaa", "bbb"} },
		"the comparison rule version":   func(in *Inputs) { in.ComparisonVersion = "2" },
		"the ranking rule version":      func(in *Inputs) { in.RankingVersion = "2" },
		"the endpoint revision":         func(in *Inputs) { in.EndpointRevision = 5 },
		"the model digest":              func(in *Inputs) { in.ModelDigest = "sha256:def" },
		"the model name":                func(in *Inputs) { in.ModelName = "another-model" },
		"the prompt version":            func(in *Inputs) { in.PromptVersion = "2" },
		"the output schema version":     func(in *Inputs) { in.SchemaVersion = "2" },
		"the generation parameters":     func(in *Inputs) { in.GenerationParams = `{"temperature":0.7}` },
		"the role's staleness":          func(in *Inputs) { in.RoleStale = true },
	}
	for what, change := range changes {
		t.Run(what, func(t *testing.T) {
			in := baseInputs()
			change(&in)
			if in.Hash() == base {
				t.Fatalf("changing %s did not change the assessment hash", what)
			}
		})
	}
}

func TestUnchangedInputsHashIdentically(t *testing.T) {
	first := baseInputs().Hash()
	for range 100 {
		if again := baseInputs().Hash(); again != first {
			t.Fatalf("the same inputs hashed to %q then %q", first, again)
		}
	}
}

// The model sees the same text either way, so it is the same assessment.
func TestEvidenceOrderDoesNotChangeTheHash(t *testing.T) {
	a := baseInputs()
	a.EvidenceHashes = []string{"aaa", "bbb", "ccc"}
	b := baseInputs()
	b.EvidenceHashes = []string{"ccc", "aaa", "bbb"}
	if a.Hash() != b.Hash() {
		t.Fatal("reordering the same evidence changed the hash")
	}
}

// Length-prefixed, so two different splits of the same characters cannot
// collide.
func TestAdjacentFieldsCannotCollide(t *testing.T) {
	a := baseInputs()
	a.ModelDigest, a.ModelName = "ab", "c"
	b := baseInputs()
	b.ModelDigest, b.ModelName = "a", "bc"
	if a.Hash() == b.Hash() {
		t.Fatal("two different field splits produced the same hash")
	}
}

// Go randomizes map iteration, so a hash that varied with it would change
// between runs of the same binary. The subprocess is the real check.
func TestTheHashAgreesAcrossProcesses(t *testing.T) {
	want := baseInputs().Hash()
	if os.Getenv("TH_ASSESS_HASH_CHILD") == "1" {
		// Printed by the child and compared by the parent.
		_, _ = os.Stdout.WriteString(want)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// The command is this test binary, re-run with a fixed argument; there is
	// no external input in it.
	cmd := exec.CommandContext(ctx, os.Args[0], //nolint:gosec // re-runs this test binary
		"-test.run", "TestTheHashAgreesAcrossProcesses")
	cmd.Env = append(os.Environ(), "TH_ASSESS_HASH_CHILD=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the child process: %v", err)
	}
	got := string(out)
	// The child prints the hash before the test framework's own output.
	if len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("a separate process hashed to %q, this one to %q", got, want)
	}
}

func TestStructuredComparisonIsDeterministic(t *testing.T) {
	cases := []struct {
		name          string
		wanted, found Value
		want          Result
	}{
		{"identical values", Value{Text: "remote"}, Value{Text: "remote"}, Met},
		{"different case", Value{Text: "Remote"}, Value{Text: "remote"}, Met},
		{"surrounding space", Value{Text: " remote "}, Value{Text: "remote"}, Met},
		{"different values", Value{Text: "remote"}, Value{Text: "onsite"}, NotMet},
		{"an unstated want", Value{}, Value{Text: "onsite"}, Unknown},
		{"an unstated find", Value{Text: "remote"}, Value{}, Unknown},
		{"an explicit unknown want", Value{Text: "unknown"}, Value{Text: "onsite"}, Unknown},
		{"an explicit unknown find", Value{Text: "remote"}, Value{Text: "unknown"}, Unknown},
		{"both unstated", Value{}, Value{}, Unknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Compare(c.wanted, c.found)
			if got != c.want {
				t.Fatalf("comparing %+v with %+v gave %q, want %q", c.wanted, c.found, got, c.want)
			}
			// The same comparison always gives the same answer.
			for range 20 {
				if again := Compare(c.wanted, c.found); again != got {
					t.Fatalf("the same comparison gave %q then %q", got, again)
				}
			}
		})
	}
}

func TestCompensationComparesRanges(t *testing.T) {
	cases := []struct {
		name          string
		wanted, found Value
		want          Result
	}{
		{
			name:   "an open-ended want inside the offer",
			wanted: Value{Min: 180000, Currency: "AUD"},
			found:  Value{Min: 170000, Max: 190000, Currency: "AUD"},
			want:   Met,
		},
		{
			name:   "an offer below the want",
			wanted: Value{Min: 180000, Currency: "AUD"},
			found:  Value{Min: 120000, Max: 150000, Currency: "AUD"},
			want:   NotMet,
		},
		{
			name:   "ranges that just touch",
			wanted: Value{Min: 150000, Max: 170000, Currency: "AUD"},
			found:  Value{Min: 170000, Max: 200000, Currency: "AUD"},
			want:   Met,
		},
		{
			name:   "no stated want",
			wanted: Value{Currency: "AUD"},
			found:  Value{Min: 170000, Currency: "AUD"},
			want:   Unknown,
		},
		{
			name:   "no stated offer",
			wanted: Value{Min: 180000, Currency: "AUD"},
			found:  Value{Currency: "AUD"},
			want:   Unknown,
		},
		{
			name:   "different currencies are not converted",
			wanted: Value{Min: 180000, Currency: "AUD"},
			found:  Value{Min: 170000, Max: 200000, Currency: "USD"},
			want:   Unknown,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CompareCompensation(c.wanted, c.found); got != c.want {
				t.Fatalf("comparing %+v with %+v gave %q, want %q", c.wanted, c.found, got, c.want)
			}
		})
	}
}

// Each tie-break alone, so a broken step cannot hide behind a working one.
func TestEachRankingStepInIsolation(t *testing.T) {
	cases := []struct {
		name string
		a, b Tally
	}{
		{
			name: "no unmet must-haves beats some",
			a:    Tally{RoleID: 2, UnmetMustHaves: 0},
			b:    Tally{RoleID: 1, UnmetMustHaves: 1},
		},
		{
			name: "fewer unmet must-haves wins",
			a:    Tally{RoleID: 2, UnmetMustHaves: 1},
			b:    Tally{RoleID: 1, UnmetMustHaves: 3},
		},
		{
			name: "fewer unknown must-haves wins",
			a:    Tally{RoleID: 2, UnknownMustHaves: 0},
			b:    Tally{RoleID: 1, UnknownMustHaves: 2},
		},
		{
			name: "more met nice-to-haves wins",
			a:    Tally{RoleID: 2, MetNiceToHaves: 5},
			b:    Tally{RoleID: 1, MetNiceToHaves: 1},
		},
		{
			name: "a higher retrieval position wins",
			a:    Tally{RoleID: 2, RetrievalPosition: 1},
			b:    Tally{RoleID: 1, RetrievalPosition: 9},
		},
		{
			name: "not shortlisted sorts last",
			a:    Tally{RoleID: 2, RetrievalPosition: 20},
			b:    Tally{RoleID: 1, RetrievalPosition: 0},
		},
		{
			name: "the role identifier makes it total",
			a:    Tally{RoleID: 1},
			b:    Tally{RoleID: 2},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !Less(c.a, c.b) {
				t.Fatalf("%+v did not sort before %+v", c.a, c.b)
			}
			if Less(c.b, c.a) {
				t.Fatalf("%+v also sorted before %+v — the order is not antisymmetric", c.b, c.a)
			}
		})
	}
}

// And combined, because the steps are lexicographic: an earlier step must
// override every later one.
func TestAnEarlierRankingStepOverridesEveryLaterOne(t *testing.T) {
	// b is better on every later step and worse on the first.
	clean := Tally{RoleID: 9, UnmetMustHaves: 0, UnknownMustHaves: 9, MetNiceToHaves: 0, RetrievalPosition: 20}
	rich := Tally{RoleID: 1, UnmetMustHaves: 1, UnknownMustHaves: 0, MetNiceToHaves: 99, RetrievalPosition: 1}
	if !Less(clean, rich) {
		t.Fatal("a match with unmet must-haves outranked one without, on later steps")
	}

	// Equal on the first two, differing on the third.
	a := Tally{RoleID: 9, UnmetMustHaves: 1, UnknownMustHaves: 0, MetNiceToHaves: 0, RetrievalPosition: 20}
	b := Tally{RoleID: 1, UnmetMustHaves: 1, UnknownMustHaves: 3, MetNiceToHaves: 99, RetrievalPosition: 1}
	if !Less(a, b) {
		t.Fatal("fewer unknown must-haves did not override more met nice-to-haves")
	}
}

func TestRankingIsTotalAndRepeatable(t *testing.T) {
	tallies := []Tally{
		{RoleID: 5, UnmetMustHaves: 1, MetNiceToHaves: 2, RetrievalPosition: 3},
		{RoleID: 2, UnmetMustHaves: 0, MetNiceToHaves: 1, RetrievalPosition: 7},
		{RoleID: 9, UnmetMustHaves: 0, MetNiceToHaves: 1, RetrievalPosition: 7},
		{RoleID: 1, UnmetMustHaves: 2, MetNiceToHaves: 9, RetrievalPosition: 1},
		{RoleID: 7, UnmetMustHaves: 0, MetNiceToHaves: 4, RetrievalPosition: 2},
	}
	first := Rank(tallies)
	for run := range 20 {
		again := Rank(tallies)
		for i := range again {
			if again[i].RoleID != first[i].RoleID {
				t.Fatalf("run %d ordered %d at position %d, the first run had %d",
					run, again[i].RoleID, i, first[i].RoleID)
			}
		}
	}
	// The clean matches lead, and 2 before 9 because everything else ties.
	if first[0].RoleID != 7 {
		t.Errorf("the cleanest match is %d, want 7: %+v", first[0].RoleID, first)
	}
	if first[1].RoleID != 2 || first[2].RoleID != 9 {
		t.Errorf("tied clean matches are out of identifier order: %+v", first)
	}
	// And the ones with unmet must-haves are still present, at the bottom.
	if len(first) != len(tallies) {
		t.Fatalf("ranking dropped matches: %d of %d", len(first), len(tallies))
	}
	if first[len(first)-1].RoleID != 1 {
		t.Errorf("the worst match is %d, want the one with two unmet must-haves", first[len(first)-1].RoleID)
	}
}

func TestUnmetMustHavesSortDownButNeverHide(t *testing.T) {
	all := []Tally{
		{RoleID: 1, UnmetMustHaves: 3},
		{RoleID: 2, UnmetMustHaves: 1},
		{RoleID: 3, UnmetMustHaves: 2},
	}
	ranked := Rank(all)
	if len(ranked) != 3 {
		t.Fatalf("every match failing left %d in the list", len(ranked))
	}
	if ranked[0].RoleID != 2 || ranked[2].RoleID != 1 {
		t.Fatalf("failing matches are out of order: %+v", ranked)
	}
}

func TestResultAndDirectionEnumerationsAreClosed(t *testing.T) {
	for _, r := range []Result{Met, NotMet, Unknown} {
		if !r.Valid() {
			t.Errorf("%q is not accepted as a result", r)
		}
	}
	for _, r := range []Result{"", "maybe", "MET", "partially"} {
		if r.Valid() {
			t.Errorf("%q was accepted as a result", r)
		}
	}
	for _, d := range []Direction{RoleFitsCandidate, CandidateFitsRole} {
		if !d.Valid() {
			t.Errorf("%q is not accepted as a direction", d)
		}
	}
	for _, d := range []Direction{"", "both", "either"} {
		if d.Valid() {
			t.Errorf("%q was accepted as a direction", d)
		}
	}
}
