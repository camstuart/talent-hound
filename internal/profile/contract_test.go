package profile

import (
	"os"
	"strings"
	"testing"
)

// The schema must let a structured value carry fields. Declaring the object
// with no properties permitted nothing at all under strict decoding, and every
// model obliged by returning an empty object — which made one of the PoC's
// acceptance conditions unreachable.
func TestTheSchemaPermitsStructuredFields(t *testing.T) {
	for _, kind := range []SubjectKind{SubjectRole, SubjectCandidate} {
		schema := Schema(kind)
		props, _ := schema["properties"].(map[string]any)
		aspects, _ := props["aspects"].(map[string]any)
		items, _ := aspects["items"].(map[string]any)
		itemProps, _ := items["properties"].(map[string]any)
		structured, ok := itemProps["structured"].(map[string]any)
		if !ok {
			t.Fatalf("%s: the schema has no structured property", kind)
		}
		if structured["additionalProperties"] != true {
			t.Fatalf("%s: structured admits no fields: %+v", kind, structured)
		}
	}
}

// The prompt has to ask for them too, or a model that follows instructions
// leaves them out.
func TestThePromptRequiresStructuredValuesWhereTheSourceIsExplicit(t *testing.T) {
	prompt := Prompt(SubjectRole, []Source{{ChunkID: 1, Text: "Melbourne, hybrid, permanent."}})
	for _, want := range []string{"include the structured value whenever the source states one",
		"location", "work_rights", "employment_type", "compensation"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("the prompt does not mention %q", want)
		}
	}
	// And still forbids inventing one.
	if !strings.Contains(prompt, "Never guess a value") {
		t.Fatal("the prompt does not forbid guessing a structured value")
	}
}

// A null structured field means the source did not say, which is how this
// contract already represents absence. Treating it as a value made a whole
// profile invalid over a field the model was right to leave empty.
func TestANullStructuredFieldIsAbsenceNotAnInvalidValue(t *testing.T) {
	raw := `{"aspects":[{"type":"compensation","wording":"AUD 180,000",
		"structured":{"currency":"AUD","minimum":180000,"basis":null,"period":null},
		"citations":[{"chunkId":1,"quote":"AUD 180,000"}]}]}`
	proposal, problems := ParseProposal(raw)
	if len(problems) != 0 {
		t.Fatalf("parsing: %v", problems)
	}
	got := proposal.Aspects[0].Structured
	if _, ok := got["basis"]; ok {
		t.Fatalf("a null field survived: %+v", got)
	}
	if got["currency"] != "AUD" {
		t.Fatalf("a stated field was dropped: %+v", got)
	}

	sources := []Source{{ChunkID: 1, Text: "AUD 180,000"}}
	if problems := Validate(SubjectRole, proposal, sources); len(problems) != 0 {
		t.Fatalf("a proposal with null fields was rejected: %v", problems)
	}
}

// A field name beside a rule saying never guess a value asks a careful model to
// omit the field. The validator refuses anything outside these enumerations, so
// the prompt has to say what they are.
func TestThePromptNamesThePermittedValues(t *testing.T) {
	prompt := Prompt(SubjectRole, []Source{{ChunkID: 1, Text: "Melbourne, hybrid."}})
	for field, values := range structuredEnums {
		for _, value := range values {
			if !strings.Contains(prompt, value) {
				t.Fatalf("the prompt never mentions %q, a permitted value for %q", value, field)
			}
		}
	}
	if !strings.Contains(prompt, "arrangement (one of: onsite, hybrid, remote, unknown)") {
		t.Fatalf("the values are not listed beside their field:\n%s", prompt)
	}
}

// The worked examples teach the vocabulary, not the answers. Every value in
// them must be absent from the benchmark corpus, or the prompt is tuning
// against the held-out set — which is the one thing freezing it is meant to
// prevent.
func TestTheWorkedExamplesShareNothingWithTheBenchmarkCorpus(t *testing.T) {
	corpus, err := os.ReadFile("../bench/testdata/corpus.json")
	if err != nil {
		t.Skipf("no corpus to compare against: %v", err)
	}
	haystack := strings.ToLower(string(corpus))
	prompt := Prompt(SubjectRole, []Source{{ChunkID: 1, Text: "..."}})
	if !strings.Contains(prompt, "Worked examples") {
		t.Fatal("the prompt shows no worked examples")
	}
	for _, value := range []string{"Wellington", "New Zealand", "NZD", "87,400", "87400"} {
		if !strings.Contains(prompt, value) {
			t.Fatalf("the prompt lost the example value %q", value)
		}
		if strings.Contains(haystack, strings.ToLower(value)) {
			t.Fatalf("the example value %q appears in the frozen corpus", value)
		}
	}
}
