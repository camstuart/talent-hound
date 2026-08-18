// Package cloud decides what may leave the machine for a cloud model, and what
// never may.
//
// An optional cloud endpoint is the kind of feature that starts as an escape
// hatch and becomes the default runtime, one convenience at a time. The
// interesting work is not making it function — it is making that drift
// impossible, which is why the deny list is a function with no parameter rather
// than a setting with a default.
package cloud

import (
	"fmt"
	"strings"

	"camstuart/talent-hound/internal/scrub"
)

// Task is a kind of work a cloud override might cover.
type Task string

// The tasks a cloud override may cover, and the ones it never may.
const (
	// Eligible: public role listings and the reasoning over approved evidence.
	RoleExtraction Task = "role_extraction"
	Assessment     Task = "assessment"
	Drafting       Task = "drafting"
	Chat           Task = "chat"

	// Never: raw candidate material and the vectors derived from it.
	CandidateExtraction Task = "candidate_extraction"
	Embedding           Task = "embedding"
	RawArtifact         Task = "raw_artifact"
)

// Eligible is exactly the four tasks a cloud override may cover.
var Eligible = []Task{RoleExtraction, Assessment, Drafting, Chat}

// denied is the permanent boundary. It is a list rather than a flag because a
// flag is something someone sets: raw candidate artifacts, Candidate Profile
// extraction, and embeddings are local-only in the PRD, and this is where that
// stops being a sentence and becomes a refusal.
var denied = map[Task]string{
	CandidateExtraction: "Candidate Profile extraction is local-only",
	Embedding:           "embeddings are local-only",
	RawArtifact:         "raw candidate artifacts are local-only",
}

// Allowed reports whether a task may ever use a cloud endpoint.
//
// It takes no options, and that is the design: a boundary with a parameter is a
// default, and a default is a thing that gets changed.
func Allowed(task Task) error {
	if reason, refused := denied[task]; refused {
		return fmt.Errorf("%s and cannot use a cloud endpoint under any configuration", reason)
	}
	for _, e := range Eligible {
		if e == task {
			return nil
		}
	}
	return fmt.Errorf("%q is not a task a cloud endpoint may be used for", task)
}

// Denied returns the permanently refused tasks, so a screen can say what can
// never be enabled rather than only what is not enabled yet.
func Denied() []Task {
	out := make([]Task, 0, len(denied))
	// Listed in a fixed order so the screen does not reshuffle.
	for _, t := range []Task{CandidateExtraction, Embedding, RawArtifact} {
		if _, ok := denied[t]; ok {
			out = append(out, t)
		}
	}
	return out
}

// The placeholders a payload carries in place of identifiers.
const (
	NamePlaceholder    = "[candidate name]"
	EmailPlaceholder   = "[email]"
	PhonePlaceholder   = "[phone]"
	AddressPlaceholder = "[address]"
)

// Redact replaces the candidate's known structured identifiers with
// placeholders.
//
// It reuses the search scrubber's knowledge of what identifies a person, and it
// substitutes rather than deletes so the payload still reads as being about
// someone. What it cannot do is find an identifier the record does not know,
// spelled in a way the shapes miss — which is why the preview, not this
// function, is the recruiter's actual control over what leaves.
func Redact(text string, ids scrub.Identifiers) string {
	out := text
	// Most specific first. A name part replaced early turns
	// "kalinda.reyes@example.invalid" into something the email pattern no
	// longer matches, and the address out of the payload entirely — so the
	// compound identifiers go before the fragments they contain.
	for _, email := range ids.Emails {
		out = replaceAll(out, email, EmailPlaceholder)
	}
	for _, phone := range ids.Phones {
		out = replaceAll(out, phone, PhonePlaceholder)
	}
	if address := strings.TrimSpace(ids.Address); address != "" {
		out = replaceAll(out, address, AddressPlaceholder)
	}
	// Shapes next, for what the record does not know it has — still before the
	// name fragments, for the same reason.
	for _, found := range scrub.Detect(out, scrub.Identifiers{}) {
		if found.Kind != scrub.KindIdentifier {
			continue
		}
		out = replaceAll(out, found.Text, placeholderFor(found.Text))
	}
	for _, name := range ids.Names {
		out = replaceAll(out, name, NamePlaceholder)
		for _, part := range strings.Fields(name) {
			if len([]rune(part)) > 2 {
				out = replaceAll(out, part, NamePlaceholder)
			}
		}
	}
	return out
}

// placeholderFor guesses which placeholder a shape-detected identifier wants.
func placeholderFor(text string) string {
	switch {
	case strings.Contains(text, "@"):
		return EmailPlaceholder
	case strings.ContainsAny(text, "0123456789") && len(strings.Fields(text)) <= 4:
		return PhonePlaceholder
	}
	return AddressPlaceholder
}

// replaceAll substitutes case-insensitively, so "Kalinda" and "kalinda" both go.
func replaceAll(text, needle, placeholder string) string {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return text
	}
	var b strings.Builder
	lowered := strings.ToLower(text)
	target := strings.ToLower(needle)
	for {
		at := strings.Index(lowered, target)
		if at < 0 {
			b.WriteString(text)
			return b.String()
		}
		b.WriteString(text[:at])
		b.WriteString(placeholder)
		text = text[at+len(target):]
		lowered = lowered[at+len(target):]
	}
}
