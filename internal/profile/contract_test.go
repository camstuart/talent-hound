package profile

import (
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
