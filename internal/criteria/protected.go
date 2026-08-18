// Package criteria decides what a search criterion is allowed to say.
//
// It has no database, no service, and no model. That is the point: a criterion
// naming a protected attribute is refused by a fixed list and a deterministic
// match, so the refusal cannot depend on a model's availability, its mood, or
// its judgement. A model that is right 95% of the time permits an unlawful
// search one time in twenty, silently, with the application's apparent blessing.
package criteria

import (
	"fmt"
	"strings"
	"unicode"
)

// Category is one protected attribute the provisional list covers.
type Category string

// The provisional protected categories from the PRD. It requires specialist
// confirmation before public release, which is why the whole list lives in one
// slice: replacing it should be one edit.
const (
	Age               Category = "age"
	Sex               Category = "sex"
	GenderIdentity    Category = "gender identity"
	SexualOrientation Category = "sexual orientation"
	RaceOrOrigin      Category = "race or national origin"
	Religion          Category = "religion"
	Disability        Category = "disability"
	FamilyOrCarer     Category = "family or carer status"
	Pregnancy         Category = "pregnancy"
	MaritalStatus     Category = "marital status"
	PoliticalOpinion  Category = "political opinion"
	UnionMembership   Category = "union membership"
)

// rule is one category and the phrases that explicitly name it.
//
// Phrases are matched as whole words against a normalized criterion, so "age"
// does not fire on "agenda" and "sex" does not fire on "sexagenarian" — the
// near-misses have fixtures, because over-blocking is how a well-meaning list
// gets switched off.
type rule struct {
	category Category
	phrases  []string
}

// protected is the whole list. Multi-word phrases are matched as consecutive
// words, so "national origin" needs both.
var protected = []rule{
	{Age, []string{
		"age", "aged", "ages", "young", "younger", "youthful", "old", "older", "elderly",
		"under 30", "under 35", "under 40", "over 40", "over 50",
		"date of birth", "birth year", "born after", "born before",
	}},
	{Sex, []string{
		"male", "female", "man", "men", "woman", "women", "sex",
	}},
	{GenderIdentity, []string{
		"gender", "transgender", "trans", "cisgender", "nonbinary", "non binary",
	}},
	{SexualOrientation, []string{
		// No bare "straight": "straight forward" is ordinary prose, and
		// "heterosexual" covers the explicit case this list is for.
		"sexual orientation", "gay", "lesbian", "bisexual", "heterosexual", "homosexual", "queer",
	}},
	{RaceOrOrigin, []string{
		"race", "racial", "ethnicity", "ethnic", "nationality", "national origin",
		"citizen", "citizens", "citizenship", "born in", "native of",
		"white", "black", "asian", "european", "african", "caucasian", "indigenous",
	}},
	{Religion, []string{
		"religion", "religious", "christian", "muslim", "jewish", "hindu", "buddhist",
		"sikh", "catholic", "atheist", "faith", "church", "mosque", "synagogue",
	}},
	{Disability, []string{
		"disability", "disabled", "handicap", "handicapped", "impairment", "able bodied",
		"wheelchair", "medical condition", "mental health", "neurodivergent",
	}},
	{FamilyOrCarer, []string{
		"children", "childless", "kids", "childcare", "carer", "caregiver",
		"family status", "dependants", "dependents", "no dependants",
	}},
	{Pregnancy, []string{
		"pregnant", "pregnancy", "maternity", "expecting a child",
	}},
	{MaritalStatus, []string{
		// "single" only where it is plainly about a person: bare "single"
		// blocks "single sign-on", and a list that blocks that gets switched off.
		"married", "unmarried", "divorced", "widowed", "marital status", "spouse",
		"be single", "is single", "single applicant", "single candidate",
	}},
	{PoliticalOpinion, []string{
		// Qualified forms only: bare "political" blocks "political science
		// research platform", and bare "liberal" blocks "liberal use of caching".
		"politically", "political opinion", "political affiliation", "political view",
		"political party", "party member", "party membership",
		"liberal party", "labour party", "labor party", "conservative", "the greens",
	}},
	{UnionMembership, []string{
		// Not bare "union": TypeScript has union types, and databases have
		// unions, and neither is a protected attribute.
		"unionised", "unionized", "union member", "union membership",
		"trade union", "non union", "in a union", "join a union",
	}},
}

// permitted phrases are checked first and, when one is present, that region of
// wording is not available for a protected match.
//
// The whole reason this exists: "must have Australian work rights" is lawful and
// necessary, and "must be an Australian citizen" is not. The distinction is in
// the terms rather than in a clever rule, and it is precisely the boundary
// someone will break while tidying the list — so both sides have fixtures.
var permitted = []string{
	"work rights", "right to work", "working rights", "work authorisation",
	"work authorization", "visa", "work permit", "eligible to work",
	"legally entitled to work", "no sponsorship", "sponsorship",
}

// Finding is a refusal: the category matched and the phrase that matched it.
type Finding struct {
	Category Category
	Phrase   string
}

// Error renders a Finding as the refusal a recruiter reads.
func (f Finding) Error() string {
	return fmt.Sprintf("this criterion names %s (%q), which cannot be a search criterion", f.Category, f.Phrase)
}

// Check reports whether a criterion explicitly names a protected attribute.
//
// It returns the first match, which is enough: the criterion is refused either
// way, and naming one reason a person can act on beats listing several.
func Check(text string) *Finding {
	words := normalize(text)
	if len(words) == 0 {
		return nil
	}

	// Lawful phrasing masks the words it covers, so "work rights" does not
	// leave "rights" available and, more to the point, a criterion that is only
	// about the right to work cannot be caught by a nationality term appearing
	// inside it.
	masked := mask(words, permitted)

	for _, r := range protected {
		for _, phrase := range r.phrases {
			if containsWords(masked, strings.Fields(phrase)) {
				return &Finding{Category: r.category, Phrase: phrase}
			}
		}
	}
	return nil
}

// Categories returns the provisional list, for a screen that wants to say what
// is blocked rather than only that something is.
func Categories() []Category {
	out := make([]Category, 0, len(protected))
	for _, r := range protected {
		out = append(out, r.category)
	}
	return out
}

// normalize lowercases, replaces every non-alphanumeric rune with a space, and
// splits — so "Under-35s", "under 35", and "UNDER  35" all become the same
// words. Trailing plural "s" is trimmed so a list of singulars matches both.
func normalize(text string) []string {
	lowered := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, text)
	fields := strings.Fields(lowered)
	out := make([]string, 0, len(fields))
	for _, w := range fields {
		// "citizens" → "citizen", "ages" → "age", "35s" → "35". Applied to
		// both the input and the list, so trimming cannot make the two
		// disagree — which is why it can be this crude.
		if len(w) > 2 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
			w = strings.TrimSuffix(w, "s")
		}
		out = append(out, w)
	}
	return out
}

// mask blanks out the words covered by any permitted phrase, so lawful wording
// cannot be the thing that trips a protected term.
func mask(words []string, phrases []string) []string {
	out := append([]string(nil), words...)
	for _, phrase := range phrases {
		want := normalize(phrase)
		for i := 0; i+len(want) <= len(out); i++ {
			if matchAt(out, want, i) {
				for j := range want {
					out[i+j] = ""
				}
			}
		}
	}
	return out
}

// containsWords reports whether want appears as consecutive whole words.
func containsWords(words []string, phrase []string) bool {
	want := make([]string, 0, len(phrase))
	for _, w := range phrase {
		want = append(want, normalize(w)...)
	}
	if len(want) == 0 {
		return false
	}
	for i := 0; i+len(want) <= len(words); i++ {
		if matchAt(words, want, i) {
			return true
		}
	}
	return false
}

func matchAt(words, want []string, at int) bool {
	for j, w := range want {
		if words[at+j] != w {
			return false
		}
	}
	return true
}
