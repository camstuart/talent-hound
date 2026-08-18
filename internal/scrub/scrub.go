// Package scrub decides what may leave the machine in a search query.
//
// It has no network and no model. Everything here is a pure function over a
// string and a set of known identifiers, because the one thing that must not
// happen — a candidate's name reaching a third party's logs — must not depend
// on an endpoint being up.
package scrub

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Identifiers are the direct identifiers of one candidate: the values that
// name a person rather than describe their work.
type Identifiers struct {
	Names   []string
	Emails  []string
	Phones  []string
	Address string
}

// Kind says which sort of thing was found, so the caller can warn differently
// about an organization and about a person.
type Kind string

const (
	// KindIdentifier is a direct identifier — a name, an email, a phone, an
	// address. Finding one is the serious case.
	KindIdentifier Kind = "identifier"
	// KindOrganization is a named employer, client, project, or school.
	KindOrganization Kind = "organization"
)

// Found is one thing detected in a query.
type Found struct {
	Kind Kind
	Text string
}

// emailPattern and phonePattern catch the shapes rather than the values, so an
// identifier the record does not know about is still removed.
var (
	emailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	phonePattern = regexp.MustCompile(`\+?\d[\d\s().\-]{6,}\d`)
	// A street address: a number followed by words ending in a street type.
	addressPattern = regexp.MustCompile(`(?i)\b\d+[a-z]?[ ,]+[a-z' ]+\b(street|st|road|rd|avenue|ave|drive|dr|lane|ln|court|ct|place|pl|parade|pde|crescent|cres|highway|hwy|terrace|tce)\b\.?`)
)

// organizationSuffixes are the words that mark a token sequence as the name of
// an organization rather than a description of work.
var organizationSuffixes = []string{
	"pty", "ltd", "limited", "inc", "llc", "plc", "gmbh", "corp", "corporation",
	"company", "co", "group", "holdings", "partners", "consulting", "consultancy",
	"university", "college", "school", "institute", "academy", "polytechnic", "tafe",
}

// Text removes every direct identifier from text.
//
// Both the known values and the recognizable shapes are removed: the record is
// what the recruiter typed, and the shapes catch what a document said that the
// record does not know about.
func Text(text string, ids Identifiers) string {
	out := text
	for _, name := range ids.Names {
		out = removePhrase(out, name)
		// A full name is also removed by its parts, because "Kalinda" alone
		// identifies someone in a small market just as well.
		for _, part := range strings.Fields(name) {
			if len([]rune(part)) > 2 {
				out = removePhrase(out, part)
			}
		}
	}
	for _, email := range ids.Emails {
		out = removePhrase(out, email)
	}
	for _, phone := range ids.Phones {
		out = removePhrase(out, phone)
	}
	if strings.TrimSpace(ids.Address) != "" {
		out = removePhrase(out, ids.Address)
	}

	out = emailPattern.ReplaceAllString(out, " ")
	out = phonePattern.ReplaceAllString(out, " ")
	out = addressPattern.ReplaceAllString(out, " ")
	return collapse(out)
}

// Generalize removes named organizations, leaving the description of the work.
//
// "Senior platform engineer at Northwind Pty Ltd" becomes "Senior platform
// engineer" — a candidate's employer identifies them almost as well as their
// name in a small market, and a query is a disclosure.
//
// ponytail: organizational suffixes plus the capitalized-run heuristic after
// "at"/"for"/"with". It over-generalizes some ordinary nouns, which costs
// recall on a query the recruiter can edit — the visible failure direction.
func Generalize(text string) string {
	out := text
	// "… at Northwind Pty Ltd", "… for Harbourline", "… with Quokkabeam Group".
	out = trimAfterPreposition(out)
	// A remaining suffixed name anywhere: "Northwind Pty Ltd", "University of
	// Melbourne".
	out = removeSuffixedNames(out)
	return collapse(out)
}

// Detect reports the identifiers and organizations present in a query, so the
// editor can warn about what a person deliberately put back.
func Detect(query string, ids Identifiers) []Found {
	found := []Found{}
	seen := map[string]bool{}
	add := func(kind Kind, text string) {
		key := string(kind) + "|" + strings.ToLower(text)
		if text == "" || seen[key] {
			return
		}
		seen[key] = true
		found = append(found, Found{Kind: kind, Text: text})
	}

	lowered := strings.ToLower(query)
	for _, name := range ids.Names {
		if name != "" && strings.Contains(lowered, strings.ToLower(name)) {
			add(KindIdentifier, name)
		}
		for _, part := range strings.Fields(name) {
			if len([]rune(part)) > 2 && containsWord(lowered, strings.ToLower(part)) {
				add(KindIdentifier, part)
			}
		}
	}
	for _, email := range ids.Emails {
		if email != "" && strings.Contains(lowered, strings.ToLower(email)) {
			add(KindIdentifier, email)
		}
	}
	for _, phone := range ids.Phones {
		if phone != "" && strings.Contains(query, phone) {
			add(KindIdentifier, phone)
		}
	}
	if a := strings.TrimSpace(ids.Address); a != "" && strings.Contains(lowered, strings.ToLower(a)) {
		add(KindIdentifier, a)
	}
	for _, m := range emailPattern.FindAllString(query, -1) {
		add(KindIdentifier, m)
	}
	for _, m := range phonePattern.FindAllString(query, -1) {
		add(KindIdentifier, strings.TrimSpace(m))
	}
	for _, m := range addressPattern.FindAllString(query, -1) {
		add(KindIdentifier, strings.TrimSpace(m))
	}
	for _, org := range findOrganizations(query) {
		add(KindOrganization, org)
	}
	return found
}

// Warnings turns findings into the messages shown before sending.
//
// Two messages, deliberately distinct: an organization is a legitimate thing to
// search for, and a direct identifier is the thing this whole package exists to
// keep out of a third party's logs.
func Warnings(found []Found) (organization, identifier string) {
	orgs := []string{}
	idents := []string{}
	for _, f := range found {
		if f.Kind == KindOrganization {
			orgs = append(orgs, f.Text)
		} else {
			idents = append(idents, f.Text)
		}
	}
	if len(orgs) > 0 {
		organization = fmt.Sprintf(
			"this query names %s — searching for a specific organization is allowed, and it tells the provider who you are looking around",
			strings.Join(orgs, ", "))
	}
	if len(idents) > 0 {
		identifier = fmt.Sprintf(
			"this query contains what looks like a direct identifier (%s). Sending it discloses who the candidate is to the search provider",
			strings.Join(idents, ", "))
	}
	return organization, identifier
}

// findOrganizations returns the organization names present in text.
func findOrganizations(text string) []string {
	out := []string{}
	words := strings.Fields(text)
	for i, w := range words {
		if !isSuffix(w) {
			continue
		}
		// Walk back over the capitalized run this suffix belongs to.
		start := i
		for start > 0 && isNameToken(words[start-1]) {
			start--
		}
		if start == i {
			continue
		}
		end := i
		for end+1 < len(words) && isSuffix(words[end+1]) {
			end++
		}
		out = append(out, strings.Trim(strings.Join(words[start:end+1], " "), ".,;:"))
	}
	// "at Northwind", with no suffix at all.
	for i, w := range words {
		if !isPreposition(w) || i+1 >= len(words) {
			continue
		}
		end := i
		for end+1 < len(words) && isNameToken(words[end+1]) {
			end++
		}
		if end > i {
			out = append(out, strings.Trim(strings.Join(words[i+1:end+1], " "), ".,;:"))
		}
	}
	return out
}

// trimAfterPreposition drops "at <Name…>" and its kin.
func trimAfterPreposition(text string) string {
	words := strings.Fields(text)
	kept := make([]string, 0, len(words))
	for i := 0; i < len(words); i++ {
		if !isPreposition(words[i]) {
			kept = append(kept, words[i])
			continue
		}
		// Skip the preposition and the capitalized run after it.
		j := i + 1
		for j < len(words) && isNameToken(words[j]) {
			j++
		}
		if j == i+1 {
			// Nothing organizational followed; keep the preposition.
			kept = append(kept, words[i])
			continue
		}
		i = j - 1
	}
	return strings.Join(kept, " ")
}

// removeSuffixedNames drops runs that end in an organizational suffix.
func removeSuffixedNames(text string) string {
	words := strings.Fields(text)
	drop := make([]bool, len(words))
	for i, w := range words {
		if !isSuffix(w) {
			continue
		}
		start := i
		for start > 0 && isNameToken(words[start-1]) {
			start--
		}
		if start == i {
			continue
		}
		end := i
		for end+1 < len(words) && isSuffix(words[end+1]) {
			end++
		}
		for j := start; j <= end; j++ {
			drop[j] = true
		}
	}
	kept := make([]string, 0, len(words))
	for i, w := range words {
		if !drop[i] {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}

func isPreposition(w string) bool {
	switch strings.ToLower(strings.Trim(w, ".,;:")) {
	case "at", "for", "with":
		return true
	}
	return false
}

func isSuffix(w string) bool {
	bare := strings.ToLower(strings.Trim(w, ".,;:"))
	for _, s := range organizationSuffixes {
		if bare == s {
			return true
		}
	}
	return false
}

// isNameToken reports whether a word could be part of a proper name: it starts
// with an uppercase letter, or joins two that do.
func isNameToken(w string) bool {
	bare := strings.Trim(w, ".,;:")
	if bare == "" {
		return false
	}
	switch strings.ToLower(bare) {
	case "of", "and", "the":
		return true
	}
	return unicode.IsUpper([]rune(bare)[0])
}

// removePhrase deletes every case-insensitive occurrence of phrase.
func removePhrase(text, phrase string) string {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return text
	}
	var b strings.Builder
	lowered := strings.ToLower(text)
	target := strings.ToLower(phrase)
	for {
		at := strings.Index(lowered, target)
		if at < 0 {
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(text[:at])
		b.WriteString(" ")
		text = text[at+len(target):]
		lowered = lowered[at+len(target):]
	}
}

// containsWord reports whether needle appears in haystack as a whole word.
func containsWord(haystack, needle string) bool {
	for _, w := range strings.Fields(haystack) {
		if strings.Trim(w, ".,;:()\"'") == needle {
			return true
		}
	}
	return false
}

// collapse tidies the holes removal leaves behind.
func collapse(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	// Punctuation left stranded by a removal.
	text = strings.ReplaceAll(text, " ,", ",")
	text = strings.ReplaceAll(text, " .", ".")
	text = strings.TrimSpace(strings.Trim(text, ",.;: "))
	return text
}
