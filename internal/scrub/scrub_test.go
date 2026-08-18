package scrub

import (
	"strings"
	"testing"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

var ids = Identifiers{
	Names:   []string{"Kalinda Reyes"},
	Emails:  []string{"kalinda.reyes@example.invalid"},
	Phones:  []string{"+61 400 123 456"},
	Address: "12 Wattle Street, Fitzroy",
}

func TestEveryDirectIdentifierIsRemoved(t *testing.T) {
	source := "Kalinda Reyes, kalinda.reyes@example.invalid, +61 400 123 456, " +
		"12 Wattle Street, Fitzroy — senior platform engineer with Go and SQLite"

	got := Text(source, ids)
	for _, secret := range []string{
		"Kalinda", "Reyes", "kalinda.reyes@example.invalid", "400 123 456", "Wattle Street",
	} {
		if strings.Contains(got, secret) {
			t.Errorf("%q survived scrubbing: %q", secret, got)
		}
	}
	// The professional description is what is left, which is the point.
	if !strings.Contains(got, "senior platform engineer") {
		t.Errorf("scrubbing removed the professional description too: %q", got)
	}
}

// The record is what the recruiter typed; the shapes catch what a document said
// that the record does not know about.
func TestUnknownIdentifiersAreRemovedByShape(t *testing.T) {
	source := "contact tobias.fenn@elsewhere.invalid or 03 9123 4567 about 8 Grattan Road, Carlton"
	got := Text(source, Identifiers{})
	for _, secret := range []string{"tobias.fenn@elsewhere.invalid", "9123 4567", "Grattan Road"} {
		if strings.Contains(got, secret) {
			t.Errorf("%q survived scrubbing by shape: %q", secret, got)
		}
	}
}

func TestOrganizationsAreGeneralizedByDefault(t *testing.T) {
	cases := []struct {
		name   string
		source string
		gone   []string
		kept   string
	}{
		{
			name:   "an employer after 'at'",
			source: "Senior platform engineer at Northwind Pty Ltd",
			gone:   []string{"Northwind"},
			kept:   "Senior platform engineer",
		},
		{
			name:   "a client after 'for'",
			source: "Built billing systems for Harbourline Group",
			gone:   []string{"Harbourline"},
			kept:   "Built billing systems",
		},
		{
			name:   "a university",
			source: "BSc from the University of Melbourne",
			gone:   []string{"University of Melbourne"},
			kept:   "BSc",
		},
		{
			name:   "a suffixed name mid-sentence",
			source: "Quokkabeam Holdings migrated to Go and SQLite",
			gone:   []string{"Quokkabeam"},
			kept:   "migrated to Go and SQLite",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Generalize(c.source)
			for _, name := range c.gone {
				if strings.Contains(got, name) {
					t.Errorf("%q survived generalization: %q", name, got)
				}
			}
			if !strings.Contains(got, c.kept) {
				t.Errorf("generalization lost the work description %q: %q", c.kept, got)
			}
		})
	}
}

// Over-generalizing costs recall on a query the recruiter can edit; losing the
// whole description costs the search.
func TestGeneralizationKeepsOrdinaryProfessionalWording(t *testing.T) {
	kept := []string{
		"five years of production Go",
		"experience operating multi-region systems",
		"comfortable with on-call rotation",
		"has led a platform team",
	}
	for _, text := range kept {
		t.Run(text, func(t *testing.T) {
			if got := Generalize(text); got != text {
				t.Errorf("generalization changed ordinary wording: %q became %q", text, got)
			}
		})
	}
}

func TestDetectDistinguishesOrganizationsFromIdentifiers(t *testing.T) {
	found := Detect("platform engineer at Northwind Pty Ltd for Kalinda Reyes", ids)

	var orgs, idents int
	for _, f := range found {
		switch f.Kind {
		case KindOrganization:
			orgs++
		case KindIdentifier:
			idents++
		}
	}
	if orgs == 0 {
		t.Errorf("no organization detected in %+v", found)
	}
	if idents == 0 {
		t.Errorf("no direct identifier detected in %+v", found)
	}
}

func TestAGeneratedQueryTriggersNeitherWarning(t *testing.T) {
	// What the builder produces after scrubbing and generalizing.
	query := Generalize(Text("Senior platform engineer at Northwind Pty Ltd, Kalinda Reyes, Go and SQLite", ids))
	found := Detect(query, ids)
	if len(found) != 0 {
		t.Fatalf("a scrubbed, generalized query still detects %+v (query: %q)", found, query)
	}
	org, ident := Warnings(found)
	if org != "" || ident != "" {
		t.Fatalf("a clean query warned: %q / %q", org, ident)
	}
}

func TestWarningsAreDistinctAndSayWhy(t *testing.T) {
	org, ident := Warnings([]Found{
		{Kind: KindOrganization, Text: "Northwind Pty Ltd"},
		{Kind: KindIdentifier, Text: "Kalinda Reyes"},
	})
	if org == "" || ident == "" {
		t.Fatalf("one of the warnings is empty: %q / %q", org, ident)
	}
	if org == ident {
		t.Fatal("the two warnings are the same message")
	}
	if !strings.Contains(org, "Northwind") {
		t.Errorf("the organization warning does not name it: %q", org)
	}
	if !strings.Contains(ident, "Kalinda") {
		t.Errorf("the identifier warning does not name it: %q", ident)
	}
	// The identifier warning has to say what is at stake.
	if !strings.Contains(ident, "disclose") {
		t.Errorf("the identifier warning does not say what sending it does: %q", ident)
	}
}

func TestOnlyOrganizationDetectedWhenOnlyAnOrganizationIsAdded(t *testing.T) {
	found := Detect("senior platform engineer at Northwind Pty Ltd", ids)
	for _, f := range found {
		if f.Kind == KindIdentifier {
			t.Errorf("an organization-only query reported a direct identifier: %+v", f)
		}
	}
	org, ident := Warnings(found)
	if org == "" {
		t.Error("adding an organization produced no warning")
	}
	if ident != "" {
		t.Errorf("adding an organization produced an identifier warning: %q", ident)
	}
}

func TestScrubbingIsIdempotent(t *testing.T) {
	once := Generalize(Text("Kalinda Reyes, engineer at Northwind Pty Ltd", ids))
	twice := Generalize(Text(once, ids))
	if once != twice {
		t.Fatalf("scrubbing twice changed the result: %q then %q", once, twice)
	}
}

func TestEmptyInputIsHandled(t *testing.T) {
	if got := Text("", ids); got != "" {
		t.Errorf("scrubbing empty text gave %q", got)
	}
	if got := Generalize(""); got != "" {
		t.Errorf("generalizing empty text gave %q", got)
	}
	if found := Detect("", ids); len(found) != 0 {
		t.Errorf("detecting in empty text found %+v", found)
	}
}
