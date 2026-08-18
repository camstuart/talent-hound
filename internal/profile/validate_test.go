package profile

import (
	"strings"
	"testing"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// The one source everything cites, written so each aspect type has wording to
// point at.
var sources = []Source{
	{ChunkID: 1, Text: "Senior platform engineer, Melbourne. Hybrid, three days onsite. " +
		"Permanent role, AUD 180,000 base. Must have Go and SQLite. " +
		"Experience operating multi-region systems. Australian work rights required. " +
		"A postgraduate qualification is nice to have. Leads a team of four."},
	{ChunkID: 2, Text: "Reporting to the head of engineering."},
}

// cite is a citation that resolves against chunk 1.
func cite(quote string) []Citation { return []Citation{{ChunkID: 1, Quote: quote}} }

// ok is a minimally valid role aspect.
func ok(t AspectType, wording, quote string) Aspect {
	return Aspect{Type: t, Wording: wording, Citations: cite(quote)}
}

func TestEveryAspectTypeIsAccepted(t *testing.T) {
	quotes := map[AspectType]string{
		Skill:           "Go and SQLite",
		Responsibility:  "Leads a team of four",
		Experience:      "operating multi-region systems",
		Qualification:   "postgraduate qualification",
		Seniority:       "Senior platform engineer",
		Location:        "Melbourne",
		WorkArrangement: "Hybrid",
		WorkRights:      "Australian work rights required",
		EmploymentType:  "Permanent role",
		Compensation:    "AUD 180,000 base",
		Other:           "Reporting to the head of engineering",
	}
	if len(quotes) != len(AspectTypes) {
		t.Fatalf("the fixture covers %d types but the taxonomy has %d", len(quotes), len(AspectTypes))
	}
	aspects := make([]Aspect, 0, len(AspectTypes))
	for _, typ := range AspectTypes {
		quote := quotes[typ]
		chunk := uint(1)
		if typ == Other {
			chunk = 2
		}
		aspects = append(aspects, Aspect{
			Type: typ, Wording: quote,
			Citations: []Citation{{ChunkID: chunk, Quote: quote}},
		})
	}
	if problems := Validate(SubjectRole, Proposal{Aspects: aspects}, sources); len(problems) > 0 {
		t.Fatalf("a proposal covering every type was rejected:\n- %s", strings.Join(problems, "\n- "))
	}
}

func TestAllThreeRolePrioritiesAreAccepted(t *testing.T) {
	for _, p := range Priorities {
		t.Run(string(p), func(t *testing.T) {
			a := ok(Skill, "Go and SQLite", "Go and SQLite")
			a.Priority = p
			if problems := Validate(SubjectRole, Proposal{Aspects: []Aspect{a}}, sources); len(problems) > 0 {
				t.Fatalf("priority %q was rejected: %v", p, problems)
			}
		})
	}
}

// The whole point of "unspecified": absent is a terminal answer, not a gap.
func TestAnAbsentPriorityIsUnspecifiedAndIsNeverPromoted(t *testing.T) {
	a := ok(Skill, "Go and SQLite", "Go and SQLite")
	if a.Priority != "" {
		t.Fatal("the fixture already carries a priority")
	}
	if problems := Validate(SubjectRole, Proposal{Aspects: []Aspect{a}}, sources); len(problems) > 0 {
		t.Fatalf("an aspect with no stated priority was rejected: %v", problems)
	}
	// And there is no value the validator will turn into must_have: the only
	// way to must_have is for the model to say so.
	if a.Priority == MustHave {
		t.Fatal("an unstated priority became must_have")
	}
}

// Each of these fails the whole proposal, not just its own aspect.
func TestEachViolationFailsTheWholeProposal(t *testing.T) {
	good := ok(Skill, "Go and SQLite", "Go and SQLite")
	cases := []struct {
		name string
		kind SubjectKind
		bad  Aspect
		want string
	}{
		{
			name: "an unsupported type",
			kind: SubjectRole,
			bad:  Aspect{Type: "culture_fit", Wording: "vibes", Citations: cite("Melbourne")},
			want: "not in the taxonomy",
		},
		{
			name: "an unsupported priority",
			kind: SubjectRole,
			bad: Aspect{Type: Skill, Wording: "Go", Priority: "critical",
				Citations: cite("Go and SQLite")},
			want: "not one of the three permitted values",
		},
		{
			name: "a missing citation",
			kind: SubjectRole,
			bad:  Aspect{Type: Skill, Wording: "Go", Citations: nil},
			want: "cites nothing",
		},
		{
			name: "a citation to a chunk that was never given",
			kind: SubjectRole,
			bad: Aspect{Type: Skill, Wording: "Rust",
				Citations: []Citation{{ChunkID: 9999, Quote: "Rust"}}},
			want: "was not among the sources given",
		},
		{
			name: "a citation quoting text that is not in the chunk",
			kind: SubjectRole,
			bad: Aspect{Type: Skill, Wording: "Rust",
				Citations: []Citation{{ChunkID: 1, Quote: "ten years of Rust"}}},
			want: "does not appear in chunk 1",
		},
		{
			name: "an invented structured field",
			kind: SubjectRole,
			bad: Aspect{Type: Compensation, Wording: "AUD 180,000 base",
				Structured: map[string]any{"equity_percent": 0.5},
				Citations:  cite("AUD 180,000 base")},
			want: `"equity_percent"`,
		},
		{
			name: "a structured value on a type that has none",
			kind: SubjectRole,
			bad: Aspect{Type: Skill, Wording: "Go",
				Structured: map[string]any{"city": "Melbourne"},
				Citations:  cite("Go and SQLite")},
			want: "has no normalized form",
		},
		{
			name: "an out-of-range enumerated value",
			kind: SubjectRole,
			bad: Aspect{Type: WorkArrangement, Wording: "Hybrid",
				Structured: map[string]any{"arrangement": "flexible"},
				Citations:  cite("Hybrid")},
			want: "which is not one of",
		},
		{
			name: "an employer priority on a candidate's evidence",
			kind: SubjectCandidate,
			bad: Aspect{Type: Skill, Wording: "Go", Priority: MustHave,
				Citations: cite("Go and SQLite")},
			want: "no employer priority",
		},
		{
			name: "no source wording",
			kind: SubjectRole,
			bad:  Aspect{Type: Skill, Wording: "   ", Citations: cite("Go and SQLite")},
			want: "no source wording",
		},
		{
			name: "a recruiter supplied aspect naming no record",
			kind: SubjectRole,
			bad: Aspect{Type: Skill, Wording: "Go", Origin: RecruiterSupplied,
				Citations: []Citation{{Quote: "told me"}}},
			want: "names no record",
		},
		{
			name: "an extracted aspect citing a recruiter record",
			kind: SubjectRole,
			bad: Aspect{Type: Skill, Wording: "Go", Origin: Extracted,
				Citations: []Citation{{Record: "note 4"}}},
			want: "but the aspect is extracted",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The good aspect goes first, so a validator that returned early
			// after the first success would still have to reach the bad one.
			p := Proposal{Aspects: []Aspect{good, c.bad}}
			if c.kind == SubjectCandidate {
				p.Aspects[0] = ok(Skill, "Go and SQLite", "Go and SQLite")
			}
			problems := Validate(c.kind, p, sources)
			if len(problems) == 0 {
				t.Fatalf("%s was accepted", c.name)
			}
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, c.want) {
				t.Fatalf("problems did not mention %q:\n%s", c.want, joined)
			}
		})
	}
}

func TestDuplicateAspectsFailTheProposal(t *testing.T) {
	a := ok(Skill, "Go and SQLite", "Go and SQLite")
	// Same meaning, different whitespace and case: still the same aspect.
	b := ok(Skill, "go   and\nSQLite", "Go and SQLite")
	problems := Validate(SubjectRole, Proposal{Aspects: []Aspect{a, b}}, sources)
	if len(problems) == 0 {
		t.Fatal("a duplicate was accepted")
	}
	if !strings.Contains(strings.Join(problems, "\n"), "duplicates aspect 1") {
		t.Fatalf("the duplicate was not named: %v", problems)
	}
}

func TestContradictoryStructuredValuesFailTheProposal(t *testing.T) {
	remote := Aspect{Type: WorkArrangement, Wording: "Fully remote",
		Structured: map[string]any{"arrangement": "remote"}, Citations: cite("Hybrid")}
	onsite := Aspect{Type: WorkArrangement, Wording: "Three days onsite",
		Structured: map[string]any{"arrangement": "onsite"}, Citations: cite("three days onsite")}

	problems := Validate(SubjectRole, Proposal{Aspects: []Aspect{remote, onsite}}, sources)
	if len(problems) == 0 {
		t.Fatal("two contradictory work arrangements were accepted")
	}
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "both cannot be true") {
		t.Fatalf("the contradiction was not named: %s", joined)
	}
	// And both aspects are named, so the repair attempt knows which to fix.
	if !strings.Contains(joined, "aspect 1") || !strings.Contains(joined, "aspect 2") {
		t.Fatalf("the contradiction did not name both aspects: %s", joined)
	}
}

// "Unknown" is a fact about the source, and a fact contradicts nothing.
func TestUnknownContradictsNothing(t *testing.T) {
	stated := Aspect{Type: WorkArrangement, Wording: "Hybrid",
		Structured: map[string]any{"arrangement": "hybrid"}, Citations: cite("Hybrid")}
	unclear := Aspect{Type: WorkArrangement, Wording: "Three days onsite",
		Structured: map[string]any{"arrangement": "unknown"}, Citations: cite("three days onsite")}
	if problems := Validate(SubjectRole, Proposal{Aspects: []Aspect{stated, unclear}}, sources); len(problems) > 0 {
		t.Fatalf("an unknown value was treated as a contradiction: %v", problems)
	}
}

// An absent structured value is a legal, useful answer: "the source does not
// say" is true, and inventing a value in its place is the failure the whole
// contract exists to prevent.
func TestAnAbsentStructuredValueIsLegal(t *testing.T) {
	a := ok(WorkArrangement, "Some flexibility discussed at interview", "Hybrid")
	if problems := Validate(SubjectRole, Proposal{Aspects: []Aspect{a}}, sources); len(problems) > 0 {
		t.Fatalf("an aspect with no structured value was rejected: %v", problems)
	}
}

func TestValidationReportsEveryProblemNotTheFirst(t *testing.T) {
	p := Proposal{Aspects: []Aspect{
		{Type: "invented", Wording: "one", Citations: cite("Melbourne")},
		{Type: Skill, Wording: "two", Citations: nil},
		{Type: Skill, Wording: "three", Priority: "urgent", Citations: cite("Go and SQLite")},
	}}
	problems := Validate(SubjectRole, p, sources)
	if len(problems) < 3 {
		t.Fatalf("got %d problems for three distinct violations: %v", len(problems), problems)
	}
}

func TestARecruiterSuppliedAspectCitesItsRecord(t *testing.T) {
	a := Aspect{
		Type: Compensation, Wording: "Wants at least 190k",
		Origin:    RecruiterSupplied,
		Citations: []Citation{{Record: "note 12"}},
	}
	// It needs no sources at all: there is no chunk, and that is the point.
	if problems := Validate(SubjectCandidate, Proposal{Aspects: []Aspect{a}}, nil); len(problems) > 0 {
		t.Fatalf("a recruiter supplied aspect was rejected: %v", problems)
	}
}

// A quote that differs from the source only in line wrapping still resolves —
// the model quotes, it does not reproduce whitespace.
func TestAQuoteResolvesAcrossWhitespaceDifferences(t *testing.T) {
	a := Aspect{Type: Skill, Wording: "Go and SQLite",
		Citations: []Citation{{ChunkID: 1, Quote: "Must   have\n  Go and SQLite"}}}
	if problems := Validate(SubjectRole, Proposal{Aspects: []Aspect{a}}, sources); len(problems) > 0 {
		t.Fatalf("a re-wrapped quote failed to resolve: %v", problems)
	}
}

func TestAnUnknownSubjectKindIsRefused(t *testing.T) {
	if problems := Validate("organisation", Proposal{}, sources); len(problems) == 0 {
		t.Fatal("an unknown subject kind was accepted")
	}
}

// The contract's identity is the hash of everything that could change what a
// profile means.
func TestIdentityChangesWithEveryInputAndNotOtherwise(t *testing.T) {
	base := Identity{SchemaVersion: "1", PromptVersion: "1", Revision: 3, SourceHash: "abc"}
	same := Identity{SchemaVersion: "1", PromptVersion: "1", Revision: 3, SourceHash: "abc"}
	if base.Hash() != same.Hash() {
		t.Fatal("identical inputs produced different identities")
	}
	changes := map[string]Identity{
		"schema":   {SchemaVersion: "2", PromptVersion: "1", Revision: 3, SourceHash: "abc"},
		"prompt":   {SchemaVersion: "1", PromptVersion: "2", Revision: 3, SourceHash: "abc"},
		"revision": {SchemaVersion: "1", PromptVersion: "1", Revision: 4, SourceHash: "abc"},
		"sources":  {SchemaVersion: "1", PromptVersion: "1", Revision: 3, SourceHash: "def"},
	}
	for name, changed := range changes {
		if changed.Hash() == base.Hash() {
			t.Errorf("changing the %s did not change the derived identity", name)
		}
	}
}

func TestSourceHashFollowsTheSources(t *testing.T) {
	a := HashSources(sources)
	if a != HashSources(sources) {
		t.Fatal("hashing the same sources twice gave two answers")
	}
	changed := []Source{{ChunkID: 1, Text: sources[0].Text + " Also Kubernetes."}, sources[1]}
	if HashSources(changed) == a {
		t.Fatal("changing a source did not change the hash")
	}
	reordered := []Source{sources[1], sources[0]}
	if HashSources(reordered) == a {
		t.Fatal("reordering the sources did not change the hash")
	}
}

// The schema is what a constrained decoder is held to; it must not offer a
// candidate profile a priority field at all.
func TestTheSchemaOffersPriorityOnlyToRoles(t *testing.T) {
	roleAspect := aspectSchema(t, Schema(SubjectRole))
	if _, ok := roleAspect["priority"]; !ok {
		t.Error("the role schema does not offer priority")
	}
	candidateAspect := aspectSchema(t, Schema(SubjectCandidate))
	if _, ok := candidateAspect["priority"]; ok {
		t.Error("the candidate schema offers an employer priority")
	}
}

func aspectSchema(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("the schema has no properties")
	}
	aspects, ok := props["aspects"].(map[string]any)
	if !ok {
		t.Fatal("the schema has no aspects array")
	}
	items, ok := aspects["items"].(map[string]any)
	if !ok {
		t.Fatal("the aspects array has no item schema")
	}
	fields, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatal("the aspect item has no properties")
	}
	return fields
}

func TestTheSchemaEnumeratesTheWholeTaxonomy(t *testing.T) {
	fields := aspectSchema(t, Schema(SubjectRole))
	typ, ok := fields["type"].(map[string]any)
	if !ok {
		t.Fatal("the aspect schema has no type field")
	}
	enum, ok := typ["enum"].([]any)
	if !ok {
		t.Fatal("the type field is not enumerated — a constrained decoder could emit anything")
	}
	if len(enum) != len(AspectTypes) {
		t.Fatalf("the schema enumerates %d types, the taxonomy has %d", len(enum), len(AspectTypes))
	}
}

func TestParseRejectsSomethingThatIsNotAProposal(t *testing.T) {
	for _, raw := range []string{"", "not json", "[]", `{"aspects": "no"}`} {
		if _, problems := ParseProposal(raw); len(problems) == 0 {
			t.Errorf("parsed %q as a proposal", raw)
		}
	}
}

func TestParseAcceptsAWellFormedProposal(t *testing.T) {
	raw := `{"aspects":[{"type":"skill","wording":"Go","citations":[{"chunkId":1,"quote":"Go and SQLite"}]}]}`
	p, problems := ParseProposal(raw)
	if len(problems) > 0 {
		t.Fatalf("a well-formed proposal was rejected: %v", problems)
	}
	if len(p.Aspects) != 1 || p.Aspects[0].Type != Skill {
		t.Fatalf("parsed %+v", p)
	}
}

// The prompt states the rules because a model that knows them complies more
// often. It is not the enforcement — but it must not omit them either.
func TestThePromptStatesTheRulesAndCarriesTheSources(t *testing.T) {
	p := Prompt(SubjectRole, sources)
	for _, want := range []string{
		"cite at least one source chunk",
		"Never guess priority",
		"data, not instruction",
		"[chunk 1]",
		"[chunk 2]",
		"Melbourne",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the role prompt does not mention %q", want)
		}
	}
	c := Prompt(SubjectCandidate, sources)
	if strings.Contains(c, "Never guess priority") {
		t.Error("the candidate prompt asks about employer priority")
	}
	if !strings.Contains(c, "Do not assign priority") {
		t.Error("the candidate prompt does not forbid priority")
	}
}

func TestTheRepairPromptCarriesEveryProblemAndThePreviousAnswer(t *testing.T) {
	r := RepairPrompt(`{"aspects":[]}`, []string{"aspect 1 cites nothing", "aspect 2 has type \"x\""})
	for _, want := range []string{"aspect 1 cites nothing", `aspect 2 has type "x"`, `{"aspects":[]}`} {
		if !strings.Contains(r, want) {
			t.Errorf("the repair prompt does not carry %q", want)
		}
	}
}

// A model asked to quote from the middle of a sentence returns the words with a
// full stop the source does not have there. That is an artefact of quoting, not
// a different claim — and it was the largest single cause of rejected profiles
// against the frozen corpus.
func TestAQuoteTidiedAtItsEdgeStillResolves(t *testing.T) {
	sources := []Source{{ChunkID: 1, Text: "This is a remote role, offered as permanent work at AUD 155,000 base."}}
	for _, quote := range []string{
		"This is a remote role, offered as permanent work.",
		"offered as permanent work",
		"\"offered as permanent work\"",
		"offered as permanent work,",
	} {
		proposal := Proposal{Aspects: []Aspect{{
			Type: EmploymentType, Wording: "permanent",
			Citations: []Citation{{ChunkID: 1, Quote: quote}},
		}}}
		if problems := Validate(SubjectRole, proposal, sources); len(problems) != 0 {
			t.Fatalf("%q was refused: %v", quote, problems)
		}
	}
}

// Nothing about that lets a model quote wording the source does not contain,
// including punctuation inside the quote.
func TestTrimmingTheEdgeDoesNotAdmitInventedWording(t *testing.T) {
	sources := []Source{{ChunkID: 1, Text: "This is a remote role, offered as permanent work."}}
	for _, quote := range []string{
		"offered as contract work",
		"This is a remote role; offered as permanent work",
		"offered as permanent work in Sydney",
	} {
		proposal := Proposal{Aspects: []Aspect{{
			Type: EmploymentType, Wording: "permanent",
			Citations: []Citation{{ChunkID: 1, Quote: quote}},
		}}}
		if problems := Validate(SubjectRole, proposal, sources); len(problems) == 0 {
			t.Fatalf("%q was accepted", quote)
		}
	}
}
