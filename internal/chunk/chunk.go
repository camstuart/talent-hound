// Package chunk cuts extracted Markdown into the evidence units everything
// downstream cites: retrieval units big enough to carry meaning, and pointers
// precise enough that a person can find the same words in the original.
//
// The algorithm is fixed and deterministic on purpose. Same Markdown, same
// parameters, same chunks — byte for byte, hash for hash. Boundaries that move
// between runs silently invalidate every hash, citation, and cached embedding
// that referenced them, and nothing about that failure is loud.
package chunk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// The chunker's identity and its parameters. Version changes whenever a
// boundary could move — including a change to the token-counting method, which
// is why the method is named in Params rather than left to be inferred.
const (
	Name    = "structural"
	Version = "1"

	// TargetTokens is how large a chunk is packed to before the next block
	// starts a new one. MaxTokens is the point at which a single block is
	// segmented into sentences instead of being emitted whole.
	TargetTokens = 200
	MaxTokens    = 320
)

// Params is the parameter record stored with every chunk. "tokens" says what a
// token count in this version actually counts: whitespace-separated words, not
// a model's tokens, because the model has not been chosen yet.
const Params = `{"targetTokens":200,"maxTokens":320,"tokens":"whitespace-words"}`

// Chunk is one evidence unit. Start and End are byte offsets into the Markdown
// it came from, and the contract that makes a citation resolvable is
// markdown[Start:End] == Text — which is why nothing here is trimmed,
// normalized, or rewritten on its way in.
type Chunk struct {
	Ordinal     int
	Text        string
	Start       int
	End         int
	HeadingPath []string
	TokenCount  int
	Hash        string
}

// Split cuts md into chunks. It returns no empty chunks: blank lines, trailing
// whitespace, and a heading with no body under it are structure, not content.
func Split(md string) []Chunk {
	blocks := scan(md)
	var out []Chunk
	emit := func(start, end int, path []string) {
		text := md[start:end]
		if strings.TrimSpace(text) == "" {
			return
		}
		sum := sha256.Sum256([]byte(text))
		out = append(out, Chunk{
			Ordinal:     len(out),
			Text:        text,
			Start:       start,
			End:         end,
			HeadingPath: slices.Clone(path),
			TokenCount:  countTokens(text),
			Hash:        hex.EncodeToString(sum[:]),
		})
	}

	// Greedy packing, left to right: consecutive blocks join while they stay
	// under the target and stay inside one heading path. Without this a heading
	// is a chunk of three tokens that matches nothing, and the paragraph beneath
	// it loses the one line saying what it is about.
	open := false
	var openStart, openEnd int
	var openPath []string
	openTokens := 0
	flush := func() {
		if open {
			emit(openStart, openEnd, openPath)
			open = false
			openTokens = 0
		}
	}

	for _, b := range blocks {
		tokens := countTokens(md[b.start:b.end])
		if open && (!slices.Equal(openPath, b.path) || openTokens+tokens > TargetTokens) {
			flush()
		}
		if tokens > MaxTokens {
			flush()
			for _, span := range sentencePack(md, b.start, b.end) {
				emit(span[0], span[1], b.path)
			}
			continue
		}
		if !open {
			open, openStart, openPath, openTokens = true, b.start, b.path, 0
		}
		openEnd = b.end
		openTokens += tokens
	}
	flush()
	return out
}

// Verify re-checks the contract on a set of chunks against their source. It is
// used at citation time rather than only in tests: an offset that is trusted
// instead of checked is a citation that points confidently at the wrong words.
func Verify(md string, c Chunk) error {
	if c.Start < 0 || c.End > len(md) || c.Start > c.End {
		return fmt.Errorf("chunk %d offsets %d:%d fall outside %d bytes of markdown", c.Ordinal, c.Start, c.End, len(md))
	}
	if md[c.Start:c.End] != c.Text {
		return fmt.Errorf("chunk %d no longer selects its own text at %d:%d", c.Ordinal, c.Start, c.End)
	}
	if Hash(c.Text) != c.Hash {
		return fmt.Errorf("chunk %d text does not match its recorded hash", c.Ordinal)
	}
	return nil
}

// Hash is the content hash recorded with a chunk.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// countTokens is the token count this version records: whitespace-separated
// words.
//
// ponytail: a real tokenizer belongs to the embedding model, which arrives in
// Phase 9. When one exists, count with it and bump Version — every chunk
// records which method produced its count, so the stale ones are findable.
func countTokens(s string) int { return len(strings.Fields(s)) }

// block is one structural unit of the Markdown, with the heading path in force
// where it begins.
type block struct {
	start, end int
	path       []string
}

// scan walks md once and produces its blocks in order. Blank lines separate
// blocks and belong to none.
func scan(md string) []block {
	lines := splitLines(md)
	var blocks []block
	var path []string

	for i := 0; i < len(lines); i++ {
		l := lines[i]
		text := md[l.start:l.end]
		if strings.TrimSpace(text) == "" {
			continue
		}

		switch {
		case headingLevel(text) > 0:
			// A heading takes the path that includes itself, so it groups
			// forward with the section it introduces rather than backwards with
			// the one above.
			level := headingLevel(text)
			if level-1 < len(path) {
				path = path[:level-1]
			}
			for len(path) < level-1 {
				path = append(path, "")
			}
			path = append(path, strings.TrimSpace(strings.TrimLeft(text, "# ")))
			blocks = append(blocks, block{l.start, l.end, slices.Clone(path)})

		case fenceOf(text) != "":
			// A code block is opaque: its blank lines and hashes are content.
			fence := fenceOf(text)
			end := l.end
			for i++; i < len(lines); i++ {
				end = lines[i].end
				if strings.HasPrefix(strings.TrimSpace(md[lines[i].start:lines[i].end]), fence) {
					break
				}
			}
			blocks = append(blocks, block{l.start, end, slices.Clone(path)})

		case isTableLine(text):
			end := l.end
			for i+1 < len(lines) && isTableLine(md[lines[i+1].start:lines[i+1].end]) {
				i++
				end = lines[i].end
			}
			blocks = append(blocks, block{l.start, end, slices.Clone(path)})

		case isListItem(text):
			// One item is one block, continuation lines included. A nested item
			// starts its own, so no chunk boundary can ever fall inside an item.
			end := l.end
			for i+1 < len(lines) && isContinuation(md, lines[i+1]) {
				i++
				end = lines[i].end
			}
			blocks = append(blocks, block{l.start, end, slices.Clone(path)})

		default:
			end := l.end
			for i+1 < len(lines) && isContinuation(md, lines[i+1]) {
				i++
				end = lines[i].end
			}
			blocks = append(blocks, block{l.start, end, slices.Clone(path)})
		}
	}
	return blocks
}

// lineSpan is one line's byte range, excluding its newline.
type lineSpan struct{ start, end int }

func splitLines(md string) []lineSpan {
	var out []lineSpan
	start := 0
	for i := 0; i < len(md); i++ {
		if md[i] == '\n' {
			end := i
			if end > start && md[end-1] == '\r' {
				end--
			}
			out = append(out, lineSpan{start, end})
			start = i + 1
		}
	}
	if start <= len(md) {
		out = append(out, lineSpan{start, len(md)})
	}
	return out
}

// headingLevel returns the ATX heading level of a line, or zero.
func headingLevel(line string) int {
	t := strings.TrimLeft(line, " ")
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n == len(t) {
		return 0
	}
	if t[n] != ' ' && t[n] != '\t' {
		return 0
	}
	return n
}

// fenceOf returns the fence marker a line opens a code block with, or "".
func fenceOf(line string) string {
	t := strings.TrimLeft(line, " ")
	for _, f := range []string{"```", "~~~"} {
		if strings.HasPrefix(t, f) {
			return f
		}
	}
	return ""
}

func isTableLine(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " "), "|")
}

// isListItem reports whether a line starts a bullet or numbered item.
func isListItem(line string) bool {
	t := strings.TrimLeft(line, " \t")
	if t == "" {
		return false
	}
	if (t[0] == '-' || t[0] == '*' || t[0] == '+') && len(t) > 1 && (t[1] == ' ' || t[1] == '\t') {
		return true
	}
	n := 0
	for n < len(t) && t[n] >= '0' && t[n] <= '9' {
		n++
	}
	return n > 0 && n+1 < len(t) && (t[n] == '.' || t[n] == ')') && (t[n+1] == ' ' || t[n+1] == '\t')
}

// isContinuation reports whether a line carries on the block above rather than
// starting one of its own.
func isContinuation(md string, l lineSpan) bool {
	text := md[l.start:l.end]
	if strings.TrimSpace(text) == "" {
		return false
	}
	return headingLevel(text) == 0 && fenceOf(text) == "" && !isTableLine(text) && !isListItem(text)
}

// sentencePack segments a block that is over the maximum and packs the
// sentences back up to the target. A sentence is never split: half a sentence
// is a citation nobody can read, so one oversized chunk from an unpunctuated
// block is the better failure.
func sentencePack(md string, start, end int) [][2]int {
	starts := sentenceStarts(md, start, end)
	var out [][2]int
	from := 0
	tokens := 0
	boundary := func(i int) int {
		if i < len(starts) {
			return starts[i]
		}
		return end
	}
	for i := range starts {
		n := countTokens(md[starts[i]:boundary(i+1)])
		if i > from && tokens+n > TargetTokens {
			out = append(out, [2]int{starts[from], starts[i]})
			from, tokens = i, 0
		}
		tokens += n
	}
	out = append(out, [2]int{starts[from], end})
	return out
}

// abbreviations are the words whose trailing period is not a sentence end.
// Deliberately short and fixed: this scanner is not correct in general, it is
// correct on CVs and job ads, and it is the same wrong in every run.
var abbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true,
	"sr": true, "jr": true, "st": true, "vs": true, "etc": true,
	"eg": true, "ie": true, "al": true, "inc": true, "ltd": true,
	"pty": true, "co": true, "corp": true, "no": true, "fig": true,
	"approx": true, "dept": true, "univ": true, "est": true, "min": true,
	"max": true, "cf": true, "pp": true, "vol": true,
}

// closers may sit between a terminator and the whitespace that follows it.
const closers = `"'”’)]}»`

// sentenceStarts returns the offset of each sentence within [start,end). The
// first is always start, so the spans tile the block with no gaps.
func sentenceStarts(md string, start, end int) []int {
	starts := []int{start}
	for i := start; i < end; i++ {
		c := md[i]
		if c != '.' && c != '!' && c != '?' {
			continue
		}
		if c == '.' && !endsSentence(md, start, i) {
			continue
		}
		j := i + 1
		for j < end && strings.IndexByte(closers, md[j]) >= 0 {
			j++
		}
		if j < end && !isSpace(md[j]) {
			continue
		}
		for j < end && isSpace(md[j]) {
			j++
		}
		if j < end {
			starts = append(starts, j)
		}
		i = j - 1
	}
	return starts
}

// endsSentence reports whether the period at i is a full stop rather than part
// of an abbreviation, an initial, or a decimal number.
func endsSentence(md string, start, i int) bool {
	j := i
	for j > start && (unicode.IsLetter(rune(md[j-1])) || unicode.IsDigit(rune(md[j-1]))) {
		j--
	}
	word := strings.ToLower(md[j:i])
	if word == "" {
		return false
	}
	// A single letter is an initial ("J. Smith"), and the final period of
	// "e.g." is preceded by one too.
	if len([]rune(word)) == 1 {
		return false
	}
	if isAllDigits(word) {
		return false
	}
	return !abbreviations[word]
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }
