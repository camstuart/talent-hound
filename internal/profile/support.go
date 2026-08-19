package profile

import (
	"fmt"
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
		// Numbers are written with separators the value does not carry, and
		// small ones are often spelled: "three days onsite".
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, evidence)
		if strings.Contains(digits, fmt.Sprintf("%d", int64(typed))) {
			return true
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

func containsAny(haystack string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}
