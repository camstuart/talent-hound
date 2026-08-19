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

// evidenceFrom is the text a structured value may be read from: the sentence
// each citation quotes, taken from the source it cites.
//
// The quote alone is too narrow — a model citing "Remote" for a listing reading
// "in Remote (Australia)" has quoted the place and dropped the country, and a
// model citing "Australian work rights" from a sentence that continues "; we do
// not sponsor" has quoted half of what it read. The whole chunk is too wide: a
// chunk holds the entire listing, which is how a location once took its country
// from a sentence about work rights. A sentence is what the citation is part of.
func evidenceFrom(a Aspect, sources []Source) string {
	byID := make(map[uint]string, len(sources))
	for _, s := range sources {
		byID[s.ChunkID] = s.Text
	}
	var b strings.Builder
	for _, c := range a.Citations {
		quote := trimBoundary(normalizeSpace(c.Quote))
		b.WriteString(" " + strings.ToLower(quote))
		text, ok := byID[c.ChunkID]
		if !ok || quote == "" {
			continue
		}
		for _, sentence := range sentencesIn(text) {
			if strings.Contains(normalizeSpace(sentence), quote) {
				b.WriteString(" " + strings.ToLower(normalizeSpace(sentence)))
			}
		}
	}
	return b.String()
}

// sentencesIn splits source text into sentences, which is where a citation
// lives. A heading is its own sentence, so a quote from one cannot borrow the
// paragraph beneath it.
func sentencesIn(text string) []string {
	out := []string{}
	current := strings.Builder{}
	flush := func() {
		if s := strings.TrimSpace(current.String()); s != "" {
			out = append(out, s)
		}
		current.Reset()
	}
	for _, r := range text {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '\n' {
			flush()
		}
	}
	flush()
	return out
}

// DropUnsupportedStructured removes structured fields the aspect's own
// citations do not support, and reports what it removed.
func DropUnsupportedStructured(p *Proposal, sources []Source) []string {
	dropped := []string{}
	for i := range p.Aspects {
		aspect := &p.Aspects[i]
		if len(aspect.Structured) == 0 {
			continue
		}
		// The cited sentences, never the model's own wording: nothing checks
		// that the wording appears in any source, so counting it let a model
		// write "Annual base salary of AUD 180,000" and support a period of a
		// year with its own sentence.
		quoted := evidenceFrom(*aspect, sources)

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
	// A country stated as a country. The adjective belongs to work rights —
	// "Australian work rights" — and reading a location's country off it was
	// eleven of twenty-three introduced values. A location says it in place
	// phrasing instead: "Remote (Australia)", "based in Australia".
	{[]AspectType{WorkRights}, "country", "Australia",
		[]string{"australia", "australian"}, nil},
	{[]AspectType{WorkRights}, "country", "New Zealand",
		[]string{"new zealand", "nz "}, nil},
	// Written without the closing bracket: a citation's trailing punctuation is
	// trimmed before it is read, so "(Australia)" arrives as "(australia".
	{[]AspectType{Location}, "country", "Australia",
		[]string{"(australia", "in australia", ", australia"}, nil},
	{[]AspectType{Location}, "country", "New Zealand",
		[]string{"(new zealand", "in new zealand", ", new zealand"}, nil},
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
	// A listing that says it is a remote role states that of its location too:
	// the taxonomy keeps remote_ok there, and a source saying "this is a remote
	// role" has said it.
	{[]AspectType{Location}, "remote_ok", true,
		[]string{"remote role", "fully remote", "remote (", "work from anywhere"}, nil},
	{[]AspectType{WorkArrangement}, "arrangement", "hybrid", []string{"hybrid"}, nil},
	{[]AspectType{WorkArrangement}, "arrangement", "remote", []string{"fully remote", "remote role"}, nil},
	{[]AspectType{WorkArrangement}, "arrangement", "onsite",
		[]string{"onsite", "on site", "on-site"}, []string{"hybrid"}},
	{[]AspectType{EmploymentType}, "employment_type", "permanent",
		[]string{"permanent", "ongoing"}, nil},
	{[]AspectType{EmploymentType}, "employment_type", "contract",
		[]string{"contract", "fixed term", "fixed-term"}, nil},
}

// AlignAcrossAspects fills a value one aspect states and another needs.
//
// The taxonomy keeps remote_ok on a location and the arrangement on a work
// arrangement, so a listing whose arrangement is remote has said its location
// is remote-friendly — in the same profile, from the same document, already
// evidenced. That is a restatement, not an inference about the world.
//
// It runs only in that direction. A location saying remote_ok does not set an
// arrangement, because a role can allow remote work without being a remote
// role.
func AlignAcrossAspects(p *Proposal) []string {
	remote := false
	for _, a := range p.Aspects {
		if a.Type == WorkArrangement && a.Structured["arrangement"] == "remote" {
			remote = true
		}
	}
	if !remote {
		return nil
	}
	filled := []string{}
	for i := range p.Aspects {
		if p.Aspects[i].Type != Location {
			continue
		}
		if p.Aspects[i].Structured == nil {
			p.Aspects[i].Structured = map[string]any{}
		}
		if _, already := p.Aspects[i].Structured["remote_ok"]; already {
			continue
		}
		p.Aspects[i].Structured["remote_ok"] = true
		filled = append(filled, "location.remote_ok")
	}
	return filled
}

// NormalizeStructured corrects a value the model put in the wrong field, where
// the correction is arithmetic rather than judgement.
//
// A rate quoted once — "AUD 900 per day" — states a floor, not a ceiling, and a
// model recording it as a maximum has misfiled the only number the source
// gives. Moving it is not inference: there is one figure, and the source says
// what it is.
func NormalizeStructured(p *Proposal) []string {
	moved := []string{}
	for i := range p.Aspects {
		aspect := &p.Aspects[i]
		if aspect.Type != Compensation || len(aspect.Structured) == 0 {
			continue
		}
		maximum, hasMax := aspect.Structured["maximum"]
		if _, hasMin := aspect.Structured["minimum"]; hasMin || !hasMax {
			continue
		}
		evidence := evidenceFrom(*aspect, nil)
		// Only when the source quotes exactly one figure. Two figures are a
		// range, and which is which is the model's to say.
		if len(numbersIn(evidence)) != 1 {
			continue
		}
		aspect.Structured["minimum"] = maximum
		delete(aspect.Structured, "maximum")
		moved = append(moved, string(aspect.Type)+".maximum->minimum")
	}
	return moved
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
func DeriveStructured(p *Proposal, sources []Source) []string {
	filled := []string{}
	for i := range p.Aspects {
		aspect := &p.Aspects[i]
		if _, carries := StructuredFields(aspect.Type); !carries {
			continue
		}
		// From the cited sentences, for the same reason: the wording is not
		// checked against any source, so deriving from it would be deriving
		// from the model.
		evidence := evidenceFrom(*aspect, sources)

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
