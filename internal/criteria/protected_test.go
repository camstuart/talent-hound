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

// Every category the screen is told to list has to be one the check actually
// refuses, and every category the check can return has to be listed.
//
// A screen that names a protected ground the service does not enforce makes a
// promise nothing keeps; a ground enforced but not listed is a refusal the
// recruiter cannot anticipate. The two lists come from the same table today,
// and this is what keeps them from drifting apart when someone adds a rule in
// one place.
func TestEveryListedCategoryIsRefusedAndEveryRefusalIsListed(t *testing.T) {
	listed := Categories()
	if len(listed) == 0 {
		t.Fatal("no protected category is listed at all")
	}

	seen := map[Category]bool{}
	for _, c := range listed {
		if strings.TrimSpace(string(c)) == "" {
			t.Fatal("a listed category is empty")
		}
		if seen[c] {
			t.Fatalf("%q is listed twice", c)
		}
		seen[c] = true
	}

	// Every rule's every phrase is caught, and caught as its own category.
	for _, r := range protected {
		if !seen[r.category] {
			t.Errorf("%q is refused but never listed", r.category)
		}
		for _, phrase := range r.phrases {
			found := Check("must be " + phrase)
			if found == nil {
				t.Errorf("%q is a %s phrase and is not refused", phrase, r.category)
				continue
			}
			if found.Category != r.category {
				t.Errorf("%q is refused as %s, want %s", phrase, found.Category, r.category)
			}
		}
	}

	// And nothing lawful is swept up by them.
	for _, lawful := range []string{
		"five years of production Go", "must have work rights in Australia",
		"senior platform engineer", "willing to work onsite in Melbourne",
		"holds a current security clearance", "available at short notice",
	} {
		if found := Check(lawful); found != nil {
			t.Errorf("%q was refused as %s (%q)", lawful, found.Category, found.Phrase)
		}
	}
}

// The twelve grounds the spec names are all there, by name.
//
// The other test asserts the reported list matches the rules, which is the list
// agreeing with itself: delete a ground from both and it still passes. This
// asserts the list against the requirement, which names them — age, sex, gender
// identity, sexual orientation, race or national origin, religion, disability,
// family or carer status, pregnancy, marital status, political opinion, and
// union membership.
//
// A missing ground is not a missing feature. It is a criterion on that ground
// being accepted, in a product whose refusal is deterministic precisely so that
// nobody has to trust a model with the question.
func TestEveryGroundTheSpecNamesIsRefused(t *testing.T) {
	required := []Category{
		Age, Sex, GenderIdentity, SexualOrientation, RaceOrOrigin, Religion,
		Disability, FamilyOrCarer, Pregnancy, MaritalStatus, PoliticalOpinion,
		UnionMembership,
	}
	listed := map[Category]bool{}
	for _, c := range Categories() {
		listed[c] = true
	}
	for _, ground := range required {
		if !listed[ground] {
			t.Errorf("%q is named in the requirement and is not refused", ground)
		}
	}
	if len(Categories()) != len(required) {
		t.Errorf("%d grounds are refused and the requirement names %d — a ground was "+
			"added without being written down, or the reverse",
			len(Categories()), len(required))
	}

	// And each one actually catches something, so a ground cannot be present as
	// a name with no phrases behind it.
	for _, ground := range required {
		matched := false
		for _, r := range protected {
			if r.category == ground && len(r.phrases) > 0 {
				matched = true
			}
		}
		if !matched {
			t.Errorf("%q is listed with nothing that would match it", ground)
		}
	}
}
