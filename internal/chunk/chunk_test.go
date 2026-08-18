package chunk

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// docs are the golden inputs. Every test that asserts a general property runs
// over all of them, so a change to the chunker that breaks the offset contract
// on tables is caught by the table fixture rather than by luck.
var docs = map[string]string{
	"empty":     "",
	"blank":     "\n\n   \n\t\n\n",
	"headings":  headingDoc,
	"lists":     listDoc,
	"table":     tableDoc,
	"long":      longDoc,
	"abbrev":    abbrevDoc,
	"unicode":   unicodeDoc,
	"code":      codeDoc,
	"noheading": "A single paragraph with no heading above it at all.\n",
}

const headingDoc = `# Overview

A short introduction to the document.

## Experience

Worked on a payments platform for three years.

### Highlights

Reduced latency and wrote the runbook.

## Education

Studied computer science.
`

const listDoc = `## Skills

- Go, including the standard library
  and its concurrency primitives
- TypeScript
  - SolidJS
  - Vitest
1. First numbered item
2. Second numbered item

A paragraph after the list.
`

const tableDoc = `## Roles

| Role | Years | Location |
| --- | --- | --- |
| Engineer | 4 | Melbourne |
| Lead | 2 | Remote |

After the table.
`

// Abbreviations, initials, and decimals mid-sentence; two real full stops.
//
// "Ltd." at the end of a sentence is deliberately not in here: a fixed
// abbreviation list cannot tell that period from the one inside "Pty. Ltd", and
// the design says so rather than pretending otherwise.
const abbrevDoc = `Dr. Amara Okonkwo reported to Ms. J. Lin at Northwind Pty. Ltd in Sydney. ` +
	`The team shipped approx. 3.5 releases per quarter, i.e. roughly monthly. ` +
	`She left in 2021.
`

const unicodeDoc = `# Résumé — 東京

Café Ǆ naïve résumé. Работал в Москве. 🚀 Shipped it.

## 技能

Go、TypeScript、SQLite。
`

const codeDoc = "## Sample\n\n```go\nfunc main() {\n\n\t// a blank line inside a fence\n}\n```\n\nAfter the fence.\n"

// longDoc is one paragraph well over the maximum, so it must be segmented.
var longDoc = "## Summary\n\n" + strings.Repeat(
	"She led the migration of a billing service to a new datastore. "+
		"The work took two quarters and involved four engineers. "+
		"Throughput doubled and the on-call load fell. ", 30) + "\n"

func TestOffsetsSelectTheStoredText(t *testing.T) {
	for name, md := range docs {
		for _, c := range Split(md) {
			if err := Verify(md, c); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
}

func TestChunkingIsDeterministic(t *testing.T) {
	for name, md := range docs {
		first, second := Split(md), Split(md)
		if len(first) != len(second) {
			t.Fatalf("%s: %d chunks then %d", name, len(first), len(second))
		}
		for i := range first {
			a, b := first[i], second[i]
			if a.Text != b.Text || a.Start != b.Start || a.End != b.End ||
				a.Hash != b.Hash || a.TokenCount != b.TokenCount ||
				!slices.Equal(a.HeadingPath, b.HeadingPath) {
				t.Errorf("%s: chunk %d differs between runs", name, i)
			}
		}
	}
}

func TestOrdinalsAreContiguousAndNoChunkIsEmpty(t *testing.T) {
	for name, md := range docs {
		for i, c := range Split(md) {
			if c.Ordinal != i {
				t.Errorf("%s: chunk at index %d has ordinal %d", name, i, c.Ordinal)
			}
			if strings.TrimSpace(c.Text) == "" {
				t.Errorf("%s: chunk %d is blank", name, i)
			}
		}
	}
}

func TestBlankInputProducesNoChunks(t *testing.T) {
	for _, name := range []string{"empty", "blank"} {
		if got := Split(docs[name]); len(got) != 0 {
			t.Errorf("%s produced %d chunks, want none", name, len(got))
		}
	}
}

func TestHeadingStaysWithTheTextItIntroduces(t *testing.T) {
	chunks := Split(headingDoc)
	first := chunks[0].Text
	if !strings.Contains(first, "# Overview") || !strings.Contains(first, "short introduction") {
		t.Fatalf("heading and its paragraph were separated: %q", first)
	}
}

func TestHeadingChangeEndsAChunk(t *testing.T) {
	// Every section here is tiny; without the heading-path rule they would all
	// pack into one chunk.
	chunks := Split(headingDoc)
	if len(chunks) != 4 {
		t.Fatalf("got %d chunks, want one per section: %v", len(chunks), texts(chunks))
	}
	want := [][]string{
		{"Overview"},
		{"Overview", "Experience"},
		{"Overview", "Experience", "Highlights"},
		{"Overview", "Education"},
	}
	for i, w := range want {
		if !slices.Equal(chunks[i].HeadingPath, w) {
			t.Errorf("chunk %d heading path %v, want %v", i, chunks[i].HeadingPath, w)
		}
	}
}

func TestHeadingPathNamesTheSection(t *testing.T) {
	for _, c := range Split(headingDoc) {
		if strings.Contains(c.Text, "Reduced latency") {
			want := []string{"Overview", "Experience", "Highlights"}
			if !slices.Equal(c.HeadingPath, want) {
				t.Fatalf("heading path %v, want %v", c.HeadingPath, want)
			}
			return
		}
	}
	t.Fatal("the highlights section was not chunked")
}

func TestDocumentWithNoHeadingsHasAnEmptyPath(t *testing.T) {
	chunks := Split(docs["noheading"])
	if len(chunks) != 1 || len(chunks[0].HeadingPath) != 0 {
		t.Fatalf("got %d chunks with path %v", len(chunks), chunks[0].HeadingPath)
	}
}

func TestListItemsAreNeverDivided(t *testing.T) {
	// Each item, including its continuation lines, must lie wholly inside one
	// chunk. Splitting "Go, including the standard library" from "and its
	// concurrency primitives" would cite half a skill.
	items := []string{
		"- Go, including the standard library\n  and its concurrency primitives",
		"  - SolidJS",
		"1. First numbered item",
	}
	chunks := Split(listDoc)
	for _, item := range items {
		if !slices.ContainsFunc(chunks, func(c Chunk) bool { return strings.Contains(c.Text, item) }) {
			t.Errorf("no chunk contains the whole item %q", item)
		}
	}
}

func TestTableRowsStayTogether(t *testing.T) {
	chunks := Split(tableDoc)
	for _, c := range chunks {
		if strings.Contains(c.Text, "| Engineer |") {
			for _, row := range []string{"| Role | Years |", "| --- |", "| Lead |"} {
				if !strings.Contains(c.Text, row) {
					t.Errorf("the table was divided: %q missing from %q", row, c.Text)
				}
			}
			return
		}
	}
	t.Fatal("the table was not chunked")
}

func TestCodeFenceIsOneBlock(t *testing.T) {
	for _, c := range Split(codeDoc) {
		if strings.Contains(c.Text, "func main") {
			if !strings.Contains(c.Text, "```go") || !strings.Contains(c.Text, "a blank line inside a fence") {
				t.Fatalf("the fence was divided at its blank line: %q", c.Text)
			}
			return
		}
	}
	t.Fatal("the code block was not chunked")
}

func TestLongParagraphIsSegmentedAtSentenceBoundaries(t *testing.T) {
	chunks := Split(longDoc)
	if len(chunks) < 3 {
		t.Fatalf("a paragraph of %d words produced %d chunks", countTokens(longDoc), len(chunks))
	}
	for i, c := range chunks {
		if c.TokenCount > MaxTokens {
			t.Errorf("chunk %d has %d tokens, over the maximum %d", i, c.TokenCount, MaxTokens)
		}
		text := strings.TrimSpace(c.Text)
		// A chunk that starts mid-sentence starts lower case; one that ends
		// mid-sentence does not end in a terminator.
		body := strings.TrimPrefix(text, "## Summary")
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		if !strings.HasSuffix(body, ".") {
			t.Errorf("chunk %d ends inside a sentence: %q", i, tail(body))
		}
		if !strings.HasPrefix(body, "She") && !strings.HasPrefix(body, "The") && !strings.HasPrefix(body, "Throughput") {
			t.Errorf("chunk %d begins inside a sentence: %q", i, head(body))
		}
	}
}

func TestAbbreviationsAreNotSentenceBoundaries(t *testing.T) {
	starts := sentenceStarts(abbrevDoc, 0, len(abbrevDoc))
	// "Dr.", "Ms.", "J.", "Pty.", "approx.", "3.5", "i.e." are not boundaries;
	// the two real full stops are.
	if len(starts) != 3 {
		var got []string
		for _, s := range starts {
			got = append(got, head(abbrevDoc[s:]))
		}
		t.Fatalf("got %d sentences, want 3: %q", len(starts), got)
	}
	for _, want := range []string{"The team shipped", "She left in 2021"} {
		if !slices.ContainsFunc(starts, func(s int) bool { return strings.HasPrefix(abbrevDoc[s:], want) }) {
			t.Errorf("no sentence starts at %q", want)
		}
	}
}

func TestUnicodeSurvivesChunking(t *testing.T) {
	chunks := Split(unicodeDoc)
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString(c.Text)
	}
	joined := b.String()
	for _, s := range []string{"Résumé", "東京", "Café Ǆ naïve résumé", "Работал в Москве", "🚀", "技能"} {
		if !strings.Contains(joined, s) {
			t.Errorf("%q did not survive chunking", s)
		}
	}
	for i, c := range chunks {
		if !utf8.ValidString(c.Text) {
			t.Errorf("chunk %d is not valid UTF-8", i)
		}
	}
}

func TestContentExactlyAtTheTargetIsOneChunk(t *testing.T) {
	// A single paragraph of exactly TargetTokens words: the boundary is not a
	// split, because "at the target" is still within it.
	body := strings.TrimSpace(strings.Repeat("word ", TargetTokens))
	chunks := Split(body + "\n")
	if len(chunks) != 1 {
		t.Fatalf("%d tokens produced %d chunks, want 1", TargetTokens, len(chunks))
	}
	if chunks[0].TokenCount != TargetTokens {
		t.Errorf("token count %d, want %d", chunks[0].TokenCount, TargetTokens)
	}
}

func TestOneTokenOverTheTargetStartsANewChunk(t *testing.T) {
	// Two paragraphs that together are one token over the target: the second
	// must not join the first.
	half := strings.TrimSpace(strings.Repeat("word ", TargetTokens))
	md := half + "\n\nextra\n"
	if got := len(Split(md)); got != 2 {
		t.Fatalf("got %d chunks, want 2", got)
	}
}

func TestVerifyRejectsAStaleChunk(t *testing.T) {
	md := "The first sentence. The second sentence.\n"
	c := Split(md)[0]
	if err := Verify(md, c); err != nil {
		t.Fatalf("a fresh chunk failed verification: %v", err)
	}
	if err := Verify("Something else entirely, and longer than before.\n", c); err == nil {
		t.Fatal("a chunk verified against the wrong markdown")
	}
	c.Hash = "0000"
	if err := Verify(md, c); err == nil {
		t.Fatal("a chunk with the wrong hash verified")
	}
	c = Split(md)[0]
	c.End = len(md) + 100
	if err := Verify(md, c); err == nil {
		t.Fatal("a chunk reaching past the markdown verified")
	}
}

func texts(chunks []Chunk) []string {
	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, head(c.Text))
	}
	return out
}

func head(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

func tail(s string) string {
	if len(s) > 40 {
		return "…" + s[len(s)-40:]
	}
	return s
}
