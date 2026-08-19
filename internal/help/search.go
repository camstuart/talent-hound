package help

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Ranking is BM25 over a term index built once, in memory.
//
// The same family as the FTS5 ranking the product uses for evidence, so a
// section that mentions a term twice does not beat one that is about it. A few
// dozen sections is small enough that the index costs a millisecond and needs
// no database — which is the point, because help has to answer before a
// database exists.
const (
	bm25K1 = 1.2
	bm25B  = 0.75
	// prefixMin is how many leading letters two words must share to count as
	// the same word. Five: "delete" and "deleting" share five, "compare" and
	// "compensation" share four.
	prefixMin = 5
)

type termIndex struct {
	// postings maps a term to the sections holding it and how often.
	postings map[string]map[int]int
	// terms holds each section's terms in order, for snippets.
	lengths   []int
	avgLength float64
	total     int
}

func newTermIndex(sections []Section) *termIndex {
	idx := &termIndex{postings: map[string]map[int]int{}, total: len(sections)}
	for i, s := range sections {
		terms := tokenize(s.Heading + " " + s.Article + " " + s.Text)
		// The heading and the article title count twice: a section titled
		// "Deleting things" is more about deletion than one that mentions it.
		terms = append(terms, tokenize(s.Heading+" "+s.Article)...)
		idx.lengths = append(idx.lengths, len(terms))
		for _, t := range terms {
			if idx.postings[t] == nil {
				idx.postings[t] = map[int]int{}
			}
			idx.postings[t][i]++
		}
	}
	sum := 0
	for _, l := range idx.lengths {
		sum += l
	}
	if len(idx.lengths) > 0 {
		idx.avgLength = float64(sum) / float64(len(idx.lengths))
	}
	return idx
}

// Search returns the sections that answer, best first.
//
// It needs no model, no network, and no database: this is the search that has
// to work when nothing else does.
func Search(query string, limit int) ([]Hit, error) {
	load()
	if loadErr != nil {
		return nil, loadErr
	}
	if limit <= 0 || limit > 50 {
		limit = 8
	}
	terms := tokenize(query)
	if len(terms) == 0 {
		return []Hit{}, nil
	}

	scores := map[int]float64{}
	for _, term := range terms {
		for section, weight := range matches(term) {
			frequency := float64(weight)
			documents := float64(len(matchingSections(term)))
			if documents == 0 {
				continue
			}
			idf := math.Log(1 + (float64(index.total)-documents+0.5)/(documents+0.5))
			length := float64(index.lengths[section])
			norm := frequency * (bm25K1 + 1) /
				(frequency + bm25K1*(1-bm25B+bm25B*length/atLeast(index.avgLength, 1)))
			scores[section] += idf * norm
		}
	}

	hits := make([]Hit, 0, len(scores))
	for section, score := range scores {
		if score <= 0 {
			continue
		}
		hits = append(hits, Hit{
			Section: sections[section], Score: score,
			Snippet: snippet(sections[section].Text, terms),
		})
	}
	// Highest first; on a tie the earlier section, so repeated searches return
	// the same order.
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].Score != hits[b].Score {
			return hits[a].Score > hits[b].Score
		}
		return hits[a].Section.Anchor < hits[b].Section.Anchor
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// matches returns the sections holding a term, exactly or by prefix.
func matches(term string) map[int]int {
	out := map[int]int{}
	for indexed, postings := range index.postings {
		if !related(term, indexed) {
			continue
		}
		for section, count := range postings {
			out[section] += count
		}
	}
	return out
}

func matchingSections(term string) map[int]int { return matches(term) }

// related reports whether two words are close enough to be the same word.
//
// Equal, or sharing a prefix long enough to mean it. "delete" and "deleting"
// share five letters and are the same word; "compare" and "compensation" share
// four and are not, which is why the threshold is five.
func related(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) < prefixMin || len(b) < prefixMin {
		return false
	}
	shared := 0
	for shared < len(a) && shared < len(b) && a[shared] == b[shared] {
		shared++
	}
	return shared >= prefixMin
}

// snippet returns the sentence that best shows why a section matched.
func snippet(text string, terms []string) string {
	sentences := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' })
	best, bestScore := "", -1
	for _, s := range sentences {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		score := 0
		lowered := tokenize(trimmed)
		for _, t := range terms {
			for _, w := range lowered {
				if related(t, w) {
					score++
					break
				}
			}
		}
		if score > bestScore {
			best, bestScore = trimmed, score
		}
	}
	if len(best) > 280 {
		best = strings.TrimSpace(best[:280]) + "…"
	}
	return best
}

// tokenize splits text into lowercase words, dropping the words that carry no
// signal in a manual.
func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 2 || stopWords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// stopWords are the words every article contains.
var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true,
	"by": true, "can": true, "do": true, "does": true, "for": true, "from": true,
	"has": true, "have": true, "how": true, "in": true, "is": true, "it": true,
	"its": true, "of": true, "on": true, "or": true, "that": true, "the": true,
	"their": true, "them": true, "then": true, "there": true, "they": true,
	"this": true, "to": true, "was": true, "what": true, "when": true, "which": true,
	"why": true, "will": true, "with": true, "you": true, "your": true,
}

// atLeast keeps a divisor off zero on an empty corpus.
func atLeast(value, floor float64) float64 {
	if value > floor {
		return value
	}
	return floor
}
