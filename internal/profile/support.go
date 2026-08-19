package profile

import (
	"fmt"
	"strconv"
	"strings"
)

// A structured value has to be supported by the same evidence as the aspect
// carrying it.
//
// The contract has always required an aspect to cite wording that appears in a
// source. It never asked the same of the normalized value beside it, so a model
// could cite "we do not sponsor" and record status "citizen", or cite a salary
// quoted as base and record a period of a year. Measured against the frozen
// corpus, those two accounted for forty-two of fifty-eight introduced values —
// and the PRD's rule is explicit that no unsupported location, work-rights,
// employment-type, or compensation value may be introduced.
//
// Unsupported fields are dropped rather than made fatal, exactly as null and
// undefined fields are: the aspect's wording still carries what the source
// said, and refusing a whole profile over a field the model guessed at throws
// away the evidence it got right.

// evidenceFor maps an enumerated or boolean value to the words that would show
// a source stated it. A value whose evidence is absent from the citation is not
// something the source said.
var evidenceFor = map[string]map[string][]string{
	"arrangement": {
		"onsite":  {"onsite", "on site", "on-site", "in the office", "in office"},
		"hybrid":  {"hybrid"},
		"remote":  {"remote"},
		"unknown": {},
	},
	"employment_type": {
		"permanent":  {"permanent", "ongoing", "perm"},
		"contract":   {"contract", "fixed term", "fixed-term", "day rate"},
		"casual":     {"casual"},
		"internship": {"intern"},
		"unknown":    {},
	},
	"status": {
		"citizen":              {"citizen"},
		"permanent_resident":   {"permanent resident", "residency"},
		"visa_holder":          {"visa"},
		"requires_sponsorship": {"sponsor"},
		"unknown":              {},
	},
	"period": {
		"hour":    {"hour", "hourly", "/hr", "per hr"},
		"day":     {"day", "daily"},
		"week":    {"week", "weekly"},
		"month":   {"month", "monthly"},
		"year":    {"year", "yearly", "annual", "annum", "p.a", "pa"},
		"unknown": {},
	},
	"basis": {
		"base":          {"base"},
		"total_package": {"package", "total remuneration", "tec"},
		"rate":          {"rate", "per day", "per hour", "day rate"},
		"unknown":       {},
	},
}

// booleanEvidence maps a boolean field to the words that would show a source
// addressed it at all. A source silent on sponsorship states neither that it is
// required nor that it is not.
var booleanEvidence = map[string][]string{
	"remote_ok":            {"remote", "work from home", "wfh", "anywhere"},
	"sponsorship_required": {"sponsor", "visa", "work rights", "right to work"},
}

// DropUnsupportedStructured removes structured fields the aspect's own
// citations do not support, and reports what it removed.
func DropUnsupportedStructured(p *Proposal) []string {
	dropped := []string{}
	for i := range p.Aspects {
		aspect := &p.Aspects[i]
		if len(aspect.Structured) == 0 {
			continue
		}
		quoted := ""
		for _, c := range aspect.Citations {
			quoted += " " + strings.ToLower(normalizeSpace(c.Quote))
		}
		// The wording is the model's own restatement of the source, and it had
		// to be citable to get here, so it counts as evidence too.
		quoted += " " + strings.ToLower(normalizeSpace(aspect.Wording))

		for field, value := range aspect.Structured {
			if supportedBy(field, value, quoted) {
				continue
			}
			delete(aspect.Structured, field)
			dropped = append(dropped, fmt.Sprintf("%s.%s", aspect.Type, field))
		}
	}
	return dropped
}

// supportedBy reports whether the evidence shows the source stated this value.
func supportedBy(field string, value any, evidence string) bool {
	if values, ok := evidenceFor[field]; ok {
		text, isString := value.(string)
		if !isString {
			return false
		}
		words, known := values[strings.ToLower(text)]
		if !known {
			// Outside the enumeration: Validate refuses it by name, and this is
			// not the place to decide that.
			return true
		}
		return containsAny(evidence, words)
	}
	if words, ok := booleanEvidence[field]; ok {
		return containsAny(evidence, words)
	}
	switch typed := value.(type) {
	case string:
		// A place or a currency has to appear in the words that were cited.
		return strings.Contains(evidence, strings.ToLower(strings.TrimSpace(typed)))
	case float64:
		// Numbers are compared as whole numbers, not as digits inside other
		// numbers. Stripping the separators from the whole sentence and asking
		// whether it contained the digits let days_onsite of 0 pass on a
		// listing quoting AUD 180,000 — there is a zero in there, and it means
		// nothing.
		for _, token := range numbersIn(evidence) {
			if token == int64(typed) {
				return true
			}
		}
		if word, ok := spelled[int64(typed)]; ok {
			return strings.Contains(evidence, word)
		}
		return false
	case int:
		return supportedBy(field, float64(typed), evidence)
	case bool:
		return true
	}
	return true
}

// spelled covers the small numbers a listing writes in words. Beyond a working
// week nobody spells them.
var spelled = map[int64]string{
	0: "zero", 1: "one", 2: "two", 3: "three", 4: "four", 5: "five",
	6: "six", 7: "seven",
}

// numbersIn reads every number in a piece of text, with the separators a
// listing writes them with removed: "AUD 180,000 base" holds one number.
func numbersIn(text string) []int64 {
	out := []int64{}
	current := strings.Builder{}
	flush := func() {
		if current.Len() == 0 {
			return
		}
		if n, err := strconv.ParseInt(current.String(), 10, 64); err == nil {
			out = append(out, n)
		}
		current.Reset()
	}
	for i, r := range text {
		switch {
		case r >= '0' && r <= '9':
			current.WriteRune(r)
		case r == ',' && current.Len() > 0 && i+1 < len(text) &&
			text[i+1] >= '0' && text[i+1] <= '9':
			// A thousands separator, not the end of the number. A decimal point
			// is the end of it: the whole number is what a salary or a count of
			// days is written as.
		default:
			flush()
		}
	}
	flush()
	return out
}

func containsAny(haystack string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

// derivable maps a field to the phrasings that state each of its values
// outright. Only phrasings with one reading are here: "base salary" states a
// base, "day rate" states a rate, and anything needing a judgement call is
// absent on purpose.
var derivable = []struct {
	types []AspectType
	field string
	value any
	words []string
	// blockedBy suppresses the derivation when the evidence also says this,
	// which is how a negation is handled rather than guessed at.
	blockedBy []string
}{
	{[]AspectType{WorkRights}, "sponsorship_required", false,
		[]string{"do not sponsor", "don't sponsor", "cannot sponsor", "no sponsorship",
			"without sponsorship", "not offer sponsorship", "unable to sponsor"}, nil},
	{[]AspectType{WorkRights}, "sponsorship_required", true,
		[]string{"sponsorship available", "will sponsor", "can sponsor", "offer sponsorship",
			"sponsorship provided"}, []string{"not", "no ", "cannot", "don't"}},
	// A country stated by name or by demonym. Deliberately short: this is the
	// PoC's market and the two countries its listings name, not a gazetteer.
	// "Australian work rights" states Australia, and the model records the
	// sponsorship and drops the country on six of twenty listings.
	{[]AspectType{WorkRights, Location}, "country", "Australia",
		[]string{"australia", "australian"}, nil},
	{[]AspectType{WorkRights, Location}, "country", "New Zealand",
		[]string{"new zealand", "nz "}, nil},
	{[]AspectType{Compensation}, "basis", "base",
		[]string{"base salary", "base plus", "base package", " base "}, nil},
	{[]AspectType{Compensation}, "basis", "rate",
		[]string{"day rate", "hourly rate", "per day", "per hour"}, nil},
	{[]AspectType{Compensation}, "basis", "total_package",
		[]string{"total package", "package of", "total remuneration"}, nil},
	{[]AspectType{Compensation}, "period", "day", []string{"per day", "day rate", "daily"}, nil},
	{[]AspectType{Compensation}, "period", "hour", []string{"per hour", "hourly", "per hr"}, nil},
	{[]AspectType{Compensation}, "period", "year",
		[]string{"per year", "per annum", "annually", "a year"}, nil},
	{[]AspectType{WorkArrangement}, "arrangement", "hybrid", []string{"hybrid"}, nil},
	{[]AspectType{WorkArrangement}, "arrangement", "remote", []string{"fully remote", "remote role"}, nil},
	{[]AspectType{WorkArrangement}, "arrangement", "onsite",
		[]string{"onsite", "on site", "on-site"}, []string{"hybrid"}},
	{[]AspectType{EmploymentType}, "employment_type", "permanent",
		[]string{"permanent", "ongoing"}, nil},
	{[]AspectType{EmploymentType}, "employment_type", "contract",
		[]string{"contract", "fixed term", "fixed-term"}, nil},
}

// DeriveStructured fills a structured field the evidence states outright and
// the model left out, and reports what it filled.
//
// Normalizing "AUD 180,000 base plus superannuation" into a basis of base is
// not a judgement about the role; it is reading a word that is there. Measured
// against the frozen corpus, a capable model states these in its wording and
// then omits them from the value beside it, on forty of a hundred constraints.
// Doing it in code is both more reliable and auditable — and it fills only what
// is absent, so a value the model did state is never overwritten.
func DeriveStructured(p *Proposal) []string {
	filled := []string{}
	for i := range p.Aspects {
		aspect := &p.Aspects[i]
		if _, carries := StructuredFields(aspect.Type); !carries {
			continue
		}
		evidence := strings.ToLower(normalizeSpace(aspect.Wording))
		for _, c := range aspect.Citations {
			evidence += " " + strings.ToLower(normalizeSpace(c.Quote))
		}

		for _, rule := range derivable {
			if !appliesTo(rule.types, aspect.Type) {
				continue
			}
			if aspect.Structured != nil {
				if _, already := aspect.Structured[rule.field]; already {
					continue
				}
			}
			if !containsAny(evidence, rule.words) || containsAny(evidence, rule.blockedBy) {
				continue
			}
			if aspect.Structured == nil {
				aspect.Structured = map[string]any{}
			}
			aspect.Structured[rule.field] = rule.value
			filled = append(filled, fmt.Sprintf("%s.%s", aspect.Type, rule.field))
		}
	}
	return filled
}

func appliesTo(types []AspectType, t AspectType) bool {
	for _, candidate := range types {
		if candidate == t {
			return true
		}
	}
	return false
}
