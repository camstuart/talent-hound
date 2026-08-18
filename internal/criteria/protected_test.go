package criteria

import (
	"strings"
	"testing"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// The whole provisional list, one plainly-worded criterion per category. If a
// category is added to the list and not here, the count assertion fails.
var blockedByCategory = map[Category]string{
	Age:               "must be under 35",
	Sex:               "looking for a male engineer",
	GenderIdentity:    "prefer a nonbinary candidate",
	SexualOrientation: "must be heterosexual",
	RaceOrOrigin:      "must be an Australian citizen",
	Religion:          "prefer a Christian candidate",
	Disability:        "must be able bodied",
	FamilyOrCarer:     "no children please",
	Pregnancy:         "must not be pregnant",
	MaritalStatus:     "prefer someone married",
	PoliticalOpinion:  "must be politically conservative",
	UnionMembership:   "must not be a union member",
}

func TestEveryProtectedCategoryIsBlocked(t *testing.T) {
	if len(blockedByCategory) != len(Categories()) {
		t.Fatalf("the fixture covers %d categories, the list has %d",
			len(blockedByCategory), len(Categories()))
	}
	for want, text := range blockedByCategory {
		t.Run(string(want), func(t *testing.T) {
			got := Check(text)
			if got == nil {
				t.Fatalf("%q was permitted", text)
			}
			if got.Category != want {
				t.Errorf("%q was blocked as %q, want %q", text, got.Category, want)
			}
			if !strings.Contains(got.Error(), string(want)) {
				t.Errorf("the refusal does not name the category: %q", got.Error())
			}
		})
	}
}

// Case, punctuation, hyphenation, and spacing are the variants people actually
// type, and a list that only catches the canonical form catches nothing.
func TestWordingVariantsDoNotEvadeTheBlock(t *testing.T) {
	variants := []string{
		"must be under 35",
		"Must be Under 35",
		"MUST BE UNDER 35",
		"must be under-35",
		"must be under  35",
		"must be under 35!",
		"Must be under 35s",
		"...must be UNDER-35...",
	}
	for _, text := range variants {
		t.Run(text, func(t *testing.T) {
			if Check(text) == nil {
				t.Fatalf("%q evaded the block", text)
			}
		})
	}
}

// Over-blocking is how a well-meaning list gets switched off, so the near
// misses are fixtures too.
func TestNearMissWordsAreNotBlocked(t *testing.T) {
	lawful := []string{
		"must be able to set the agenda for a design review",
		"experience with package management",
		"comfortable with ambiguity",
		"has run a mentoring programme",
		"strong background in single sign-on",
		"experience with union types in TypeScript",
		"has worked on a political science research platform",
	}
	for _, text := range lawful {
		t.Run(text, func(t *testing.T) {
			if got := Check(text); got != nil {
				t.Fatalf("%q was blocked as %s (%q)", text, got.Category, got.Phrase)
			}
		})
	}
}

// Two of these look alike and are not: this is precisely the boundary someone
// will break while tidying the list.
func TestWorkRightsArePermittedAndNationalityIsNot(t *testing.T) {
	permittedCriteria := []string{
		"must have Australian work rights",
		"must have the right to work in Australia",
		"must hold a valid work visa",
		"eligible to work in New Zealand without sponsorship",
		"we do not offer sponsorship",
		"has full working rights",
	}
	for _, text := range permittedCriteria {
		t.Run("permitted/"+text, func(t *testing.T) {
			if got := Check(text); got != nil {
				t.Fatalf("a lawful work-rights criterion was blocked as %s (%q): %q",
					got.Category, got.Phrase, text)
			}
		})
	}

	blockedCriteria := []string{
		"must be an Australian citizen",
		"Australian citizenship required",
		"must be a citizen",
		"prefer candidates born in Australia",
		"must be of European nationality",
	}
	for _, text := range blockedCriteria {
		t.Run("blocked/"+text, func(t *testing.T) {
			got := Check(text)
			if got == nil {
				t.Fatalf("a nationality criterion was permitted: %q", text)
			}
			if got.Category != RaceOrOrigin {
				t.Errorf("%q was blocked as %q, want race or national origin", text, got.Category)
			}
		})
	}
}

// The block is a pure function of its input and the list — it cannot be
// affected by anything the caller has or has not configured.
func TestTheBlockIsDeterministic(t *testing.T) {
	const text = "must be under 35"
	first := Check(text)
	for range 100 {
		got := Check(text)
		if got == nil || got.Category != first.Category || got.Phrase != first.Phrase {
			t.Fatalf("the same criterion produced two answers: %+v then %+v", first, got)
		}
	}
}

func TestEmptyAndWhitespaceCriteriaAreNotBlocked(t *testing.T) {
	for _, text := range []string{"", "   ", "\n\t"} {
		if got := Check(text); got != nil {
			t.Errorf("empty input was blocked as %s", got.Category)
		}
	}
}

// Clearly lawful professional criteria — the ones the product exists to run —
// must pass untouched.
func TestOrdinaryProfessionalCriteriaPass(t *testing.T) {
	lawful := []string{
		"five years of production Go",
		"has led a platform team",
		"experience operating multi-region systems",
		"postgraduate qualification in computer science",
		"available within four weeks",
		"open to hybrid work in Melbourne",
		"permanent employment only",
		"comfortable with on-call rotation",
	}
	for _, text := range lawful {
		t.Run(text, func(t *testing.T) {
			if got := Check(text); got != nil {
				t.Fatalf("%q was blocked as %s (%q)", text, got.Category, got.Phrase)
			}
		})
	}
}
