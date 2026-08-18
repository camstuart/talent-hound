//go:build livemodel

package main

import (
	"os"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
)

// Why citations fail, in the model's own output.
//
// The benchmark reports "quotes wording that does not appear in chunk N" and
// deliberately does not log the quote — a citation quotes the document, and the
// document is the thing logs must never carry. This runs against the synthetic
// corpus only, where the "document" is invented, so the quote can be shown.
//
//	go test -tags livemodel -run TestDiagnoseCitations -v .
func TestDiagnoseCitations(t *testing.T) {
	model := os.Getenv("TH_CLASSIFY_MODEL")
	if model == "" {
		t.Skip("set TH_CLASSIFY_MODEL")
	}
	corpus, err := bench.Load()
	if err != nil {
		t.Fatalf("loading the corpus: %v", err)
	}

	exact, caseOnly, spaceOnly, absent := 0, 0, 0, 0
	for _, listing := range corpus.Listings[:4] {
		e := newClassifyEnv(t)
		e.classify = NewClassifyService(e.db, e.registry, platform.NewOllama())
		e.assignClassify(t, model)
		ids := e.withSource(t, listing.ID, listing.Markdown)
		sources := map[uint]string{}
		for _, chunk := range e.chunks2 {
			sources[chunk.ID] = chunk.Text
		}

		p, err := e.classify.Classify(ClassifyInput{
			SubjectKind: profile.SubjectRole, SubjectID: 1, ChunkIDs: ids,
		})
		if err != nil {
			t.Logf("%s: refused (%v)", listing.ID, err)
			continue
		}
		aspects, err := e.classify.Aspects(p.ID)
		if err != nil {
			t.Fatalf("reading aspects: %v", err)
		}
		for _, a := range aspects {
			for _, c := range a.Citations {
				text := sources[c.ChunkID]
				switch {
				case strings.Contains(text, c.Quote):
					exact++
				case strings.Contains(squash(text), squash(c.Quote)):
					spaceOnly++
				case strings.Contains(strings.ToLower(squash(text)), strings.ToLower(squash(c.Quote))):
					caseOnly++
					t.Logf("case-only mismatch: %q", c.Quote)
				default:
					absent++
					t.Logf("absent: %q", c.Quote)
				}
			}
		}
	}
	t.Logf("EVIDENCE citations exact=%d whitespace-only=%d case-only=%d absent=%d",
		exact, spaceOnly, caseOnly, absent)
}

func squash(s string) string { return strings.Join(strings.Fields(s), " ") }
