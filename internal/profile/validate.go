package profile

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Validate checks a whole proposal and returns everything wrong with it.
//
// The caller treats a non-empty result as a rejection of the entire proposal.
// It never returns "these seven aspects are fine", because the aspects most
// likely to fail are the ones about the least clearly written parts of the
// source — which are exactly the requirements a recruiter needs to see. A
// profile that silently drops them reads as "the role does not require that".
//
// Every problem is collected rather than returning at the first, because the
// list is what the single repair attempt gets told.
func Validate(kind SubjectKind, p Proposal, sources []Source) []string {
	problems := []string{}
	if !kind.Valid() {
		return []string{fmt.Sprintf("unknown subject kind %q", kind)}
	}

	known := make(map[uint]string, len(sources))
	for _, s := range sources {
		known[s.ChunkID] = s.Text
	}

	// seen keys a normalized meaning to the first aspect index that claimed it,
	// so a duplicate can name both.
	seen := map[string]int{}
	// structured keys type and field to the value asserted, so a contradiction
	// can name both.
	asserted := map[string]assertedValue{}

	for i, a := range p.Aspects {
		where := fmt.Sprintf("aspect %d", i+1)
		problems = append(problems, validateType(where, a)...)
		problems = append(problems, validatePriority(where, kind, a)...)
		problems = append(problems, validateOrigin(where, a)...)
		problems = append(problems, validateCitations(where, a, known)...)
		problems = append(problems, validateStructured(where, a)...)

		key := meaningKey(a)
		if first, ok := seen[key]; ok {
			problems = append(problems, fmt.Sprintf(
				"%s duplicates aspect %d: the same %s with the same meaning", where, first+1, a.Type))
		} else {
			seen[key] = i
		}
		problems = append(problems, collectContradictions(where, i, a, asserted)...)
	}
	return problems
}

func validateType(where string, a Aspect) []string {
	if !a.Type.Valid() {
		return []string{fmt.Sprintf("%s has type %q, which is not in the taxonomy", where, a.Type)}
	}
	if strings.TrimSpace(a.Wording) == "" {
		return []string{fmt.Sprintf("%s has no source wording", where)}
	}
	return nil
}

// validatePriority enforces the rule that gives "unspecified" its meaning: it
// is a terminal value, and a candidate's evidence has no employer priority at
// all.
func validatePriority(where string, kind SubjectKind, a Aspect) []string {
	if kind == SubjectCandidate {
		if a.Priority != "" && a.Priority != Unspecified {
			return []string{fmt.Sprintf(
				"%s carries priority %q, but a candidate's evidence has no employer priority",
				where, a.Priority)}
		}
		return nil
	}
	// Absent is unspecified, which is the whole of "never invent priority": the
	// default is the honest answer rather than a gap something later fills.
	if a.Priority == "" {
		return nil
	}
	if !a.Priority.Valid() {
		return []string{fmt.Sprintf("%s has priority %q, which is not one of the three permitted values",
			where, a.Priority)}
	}
	return nil
}

func validateOrigin(where string, a Aspect) []string {
	if a.Origin == "" || a.Origin == Extracted {
		return nil
	}
	if !a.Origin.Valid() {
		return []string{fmt.Sprintf("%s has origin %q, which is not extracted or recruiter supplied",
			where, a.Origin)}
	}
	return nil
}

// validateCitations is the check fluency cannot satisfy. "Has a citation field"
// lets a model comply by inventing a plausible chunk identifier, which it will,
// because the prompt asked for one.
func validateCitations(where string, a Aspect, known map[uint]string) []string {
	problems := []string{}
	if len(a.Citations) == 0 {
		return []string{fmt.Sprintf("%s cites nothing", where)}
	}
	recruiter := a.Origin == RecruiterSupplied
	for j, c := range a.Citations {
		at := fmt.Sprintf("%s citation %d", where, j+1)
		if recruiter {
			// No chunk to resolve against; the evidence is that a person
			// asserted it, in a record that exists.
			if strings.TrimSpace(c.Record) == "" {
				problems = append(problems, at+" is recruiter supplied but names no record")
			}
			continue
		}
		if c.Record != "" {
			problems = append(problems, at+" names a recruiter record but the aspect is extracted")
			continue
		}
		text, ok := known[c.ChunkID]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"%s names chunk %d, which was not among the sources given", at, c.ChunkID))
			continue
		}
		quote := strings.TrimSpace(c.Quote)
		if quote == "" {
			problems = append(problems, at+" quotes nothing")
			continue
		}
		// Substring containment, not offsets: the model quotes, it does not
		// count bytes, and asking it to would trade a check that works for one
		// that fails on whitespace.
		if !strings.Contains(normalizeSpace(text), trimBoundary(normalizeSpace(quote))) {
			problems = append(problems, fmt.Sprintf(
				"%s quotes wording that does not appear in chunk %d", at, c.ChunkID))
		}
	}
	return problems
}

func validateStructured(where string, a Aspect) []string {
	if len(a.Structured) == 0 {
		return nil
	}
	allowed, ok := StructuredFields(a.Type)
	if !ok {
		return []string{fmt.Sprintf("%s is a %s aspect, which has no normalized form", where, a.Type)}
	}
	problems := []string{}
	// Sorted so the message is the same on every run, which matters because it
	// goes into a repair prompt.
	fields := make([]string, 0, len(a.Structured))
	for name := range a.Structured {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	for _, name := range fields {
		if !slices.Contains(allowed, name) {
			problems = append(problems, fmt.Sprintf(
				"%s carries the field %q, which is not defined for %s", where, name, a.Type))
			continue
		}
		values, enumerated := StructuredEnum(name)
		if !enumerated {
			continue
		}
		got, isString := a.Structured[name].(string)
		if !isString || !slices.Contains(values, got) {
			problems = append(problems, fmt.Sprintf(
				"%s sets %s to %v, which is not one of: %s",
				where, name, a.Structured[name], strings.Join(values, ", ")))
		}
	}
	return problems
}

type assertedValue struct {
	value string
	index int
}

// collectContradictions catches two aspects of one type whose structured values
// cannot both be true. Only the enumerated fields are compared: two different
// city strings are two facts, but two different work arrangements are one fact
// asserted twice, differently.
func collectContradictions(where string, i int, a Aspect, asserted map[string]assertedValue) []string {
	problems := []string{}
	names := make([]string, 0, len(a.Structured))
	for name := range a.Structured {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, enumerated := StructuredEnum(name); !enumerated {
			continue
		}
		got, ok := a.Structured[name].(string)
		if !ok || got == "unknown" {
			// "The source does not say" contradicts nothing.
			continue
		}
		key := string(a.Type) + "/" + name
		prev, exists := asserted[key]
		if exists && prev.value != got {
			problems = append(problems, fmt.Sprintf(
				"%s sets %s to %q, but aspect %d already set it to %q — both cannot be true",
				where, name, got, prev.index+1, prev.value))
			continue
		}
		if !exists {
			asserted[key] = assertedValue{value: got, index: i}
		}
	}
	return problems
}

// MeaningKey is what makes two aspects the same aspect, for callers that need
// to check a new aspect against already-stored ones without re-validating
// those. Re-validating stored aspects is not available in general: their
// citations resolve against the sources of the classification that produced
// them, which a later caller does not have.
func MeaningKey(a Aspect) string { return meaningKey(a) }

// meaningKey is what makes two aspects the same aspect: type, normalized
// wording, and normalized structured value.
func meaningKey(a Aspect) string {
	var b strings.Builder
	b.WriteString(string(a.Type))
	b.WriteString("|")
	b.WriteString(strings.ToLower(normalizeSpace(a.Wording)))
	names := make([]string, 0, len(a.Structured))
	for name := range a.Structured {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "|%s=%v", name, a.Structured[name])
	}
	return b.String()
}

// RepairCitations points a citation at the chunk that actually contains its
// quote, when it named a different one.
//
// A model reading four chunks and quoting the third while writing "chunkId: 2"
// has not invented evidence — the wording is there, in a source it was given,
// and the pointer is off by one. Refusing the whole profile for it discards
// every other aspect over a bookkeeping slip, and storing the corrected
// pointer is strictly better data than storing nothing.
//
// It repairs nothing else. A quote that appears in no supplied chunk — the
// model welding a heading to a body, or paraphrasing — is left exactly as it
// was, for Validate to refuse.
func RepairCitations(p *Proposal, sources []Source) {
	byID := make(map[uint]string, len(sources))
	for _, s := range sources {
		byID[s.ChunkID] = normalizeSpace(s.Text)
	}
	for i := range p.Aspects {
		for j, c := range p.Aspects[i].Citations {
			quote := trimBoundary(normalizeSpace(c.Quote))
			if quote == "" {
				continue
			}
			if text, ok := byID[c.ChunkID]; ok && strings.Contains(text, quote) {
				continue
			}
			// Sorted, so a quote appearing in two chunks resolves the same way
			// on every run.
			ids := make([]uint, 0, len(byID))
			for id := range byID {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
			for _, id := range ids {
				if strings.Contains(byID[id], quote) {
					p.Aspects[i].Citations[j].ChunkID = id
					break
				}
			}
		}
	}
}

// normalizeSpace collapses runs of whitespace, so a quote that differs from its
// source only in line wrapping still resolves.
func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// trimBoundary drops punctuation and quotation marks at the ends of a quote.
//
// Models cut mid-sentence and tidy the edge: asked to quote from "...offered as
// permanent work at AUD 155,000...", they return "This is a remote role,
// offered as permanent work." — the same words, with a full stop the source
// does not have there. That is a boundary artefact of quoting, in the same
// class as the line wrapping already tolerated above, and it is the single
// largest cause of rejected profiles measured against the frozen corpus.
//
// Only the ends are trimmed. Punctuation inside the quote still has to match,
// so nothing here lets a model quote wording the source does not contain.
func trimBoundary(quote string) string {
	return strings.Trim(quote, " \t.,;:!?\"'“”‘’()[]")
}
