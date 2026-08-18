package cloud

import (
	"strings"
	"testing"

	"camstuart/talent-hound/internal/scrub"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// The complete matrix, asserted by trying every task rather than by reading the
// list — so a task added later without a decision fails here.
func TestTheAllowDenyMatrixIsComplete(t *testing.T) {
	cases := map[Task]bool{
		RoleExtraction:      true,
		Assessment:          true,
		Drafting:            true,
		Chat:                true,
		CandidateExtraction: false,
		Embedding:           false,
		RawArtifact:         false,
	}
	for task, allowed := range cases {
		t.Run(string(task), func(t *testing.T) {
			err := Allowed(task)
			if allowed && err != nil {
				t.Fatalf("%s should be eligible: %v", task, err)
			}
			if !allowed && err == nil {
				t.Fatalf("%s was permitted", task)
			}
		})
	}
	if len(Eligible) != 4 {
		t.Fatalf("the eligible list has %d tasks, the PRD names four", len(Eligible))
	}
	if len(Denied()) != 3 {
		t.Fatalf("the denied list has %d tasks, the PRD names three", len(Denied()))
	}
}

// A boundary with a parameter is a default, and a default is a thing that gets
// changed.
func TestADeniedTaskIsRefusedForTheBoundaryNotForApproval(t *testing.T) {
	for _, task := range Denied() {
		err := Allowed(task)
		if err == nil {
			t.Fatalf("%s was permitted", task)
		}
		if !strings.Contains(err.Error(), "local-only") {
			t.Errorf("%s was refused for the wrong reason: %v", task, err)
		}
		if !strings.Contains(err.Error(), "any configuration") {
			t.Errorf("the refusal does not say it is unconditional: %v", err)
		}
	}
}

func TestAnUnknownTaskIsRefused(t *testing.T) {
	for _, task := range []Task{"", "anything", "ROLE_EXTRACTION", "summarisation"} {
		if err := Allowed(task); err == nil {
			t.Errorf("%q was permitted", task)
		}
	}
}

// Repeated because determinism is the property: no clock, no configuration, no
// state.
func TestTheBoundaryIsAFunctionOfItsArgument(t *testing.T) {
	for range 100 {
		if Allowed(Embedding) == nil {
			t.Fatal("embeddings became permitted")
		}
		if Allowed(Drafting) != nil {
			t.Fatal("drafting became refused")
		}
	}
}

var ids = scrub.Identifiers{
	Names:   []string{"Kalinda Reyes"},
	Emails:  []string{"kalinda.reyes@example.invalid"},
	Phones:  []string{"+61 400 123 456"},
	Address: "12 Wattle Street, Fitzroy",
}

func TestKnownIdentifiersBecomePlaceholders(t *testing.T) {
	payload := "Kalinda Reyes (kalinda.reyes@example.invalid, +61 400 123 456, " +
		"12 Wattle Street, Fitzroy) is a senior platform engineer with five years of Go."

	got := Redact(payload, ids)
	for _, identifier := range []string{
		"Kalinda", "Reyes", "kalinda.reyes@example.invalid", "400 123 456", "Wattle Street",
	} {
		if strings.Contains(got, identifier) {
			t.Errorf("%q survived redaction: %q", identifier, got)
		}
	}
	// The payload is still about someone, and still useful.
	for _, placeholder := range []string{NamePlaceholder, EmailPlaceholder} {
		if !strings.Contains(got, placeholder) {
			t.Errorf("%q is missing from the redacted payload: %q", placeholder, got)
		}
	}
	if !strings.Contains(got, "senior platform engineer with five years of Go") {
		t.Errorf("redaction removed the professional content: %q", got)
	}
}

func TestShapesAreRedactedEvenWhenTheRecordDoesNotKnowThem(t *testing.T) {
	payload := "Reach them at tobias.fenn@elsewhere.invalid or 03 9123 4567."
	got := Redact(payload, scrub.Identifiers{})
	for _, identifier := range []string{"tobias.fenn@elsewhere.invalid", "9123 4567"} {
		if strings.Contains(got, identifier) {
			t.Errorf("%q survived shape redaction: %q", identifier, got)
		}
	}
}

func TestRedactionIsIdempotent(t *testing.T) {
	once := Redact("Kalinda Reyes is available.", ids)
	twice := Redact(once, ids)
	if once != twice {
		t.Fatalf("redacting twice changed the result: %q then %q", once, twice)
	}
}

func TestRedactionLeavesOrdinaryTextAlone(t *testing.T) {
	payload := "five years of production Go, has led a platform team, open to hybrid work"
	if got := Redact(payload, ids); got != payload {
		t.Fatalf("redaction changed ordinary text: %q became %q", payload, got)
	}
}

func TestEmptyInputIsHandled(t *testing.T) {
	if got := Redact("", ids); got != "" {
		t.Errorf("redacting empty text gave %q", got)
	}
	if got := Redact("anything", scrub.Identifiers{}); got != "anything" {
		t.Errorf("redacting with no identifiers changed the text: %q", got)
	}
}
