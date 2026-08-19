//go:build livemodel

package main

import (
	"os"
	"testing"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
)

// What the model actually cited for the constraints that keep coming back
// wrong, rather than what I keep assuming it cited.
//
//	go test -tags livemodel -run TestDiagnoseConstraints -v .
func TestDiagnoseConstraints(t *testing.T) {
	model := os.Getenv("TH_CLASSIFY_MODEL")
	if model == "" {
		t.Skip("set TH_CLASSIFY_MODEL")
	}
	corpus, err := bench.Load()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	for _, want := range []string{"backend-contract-melbourne", "data-engineer-melbourne"} {
		for _, listing := range corpus.Listings {
			if listing.ID != want {
				continue
			}
			e := newClassifyEnv(t)
			e.classify = NewClassifyService(e.db, e.registry, platform.NewOllama())
			e.assignClassify(t, model)
			ids := e.withSource(t, listing.ID, listing.Markdown)
			for _, c := range e.chunks2 {
				t.Logf("%s: chunk %d = %q", listing.ID, c.ID, c.Text)
			}

			p, err := e.classify.Classify(ClassifyInput{
				SubjectKind: profile.SubjectRole, SubjectID: 1, ChunkIDs: ids})
			if err != nil {
				t.Logf("%s: refused: %v", listing.ID, err)
				continue
			}
			aspects, err := e.classify.Aspects(p.ID)
			if err != nil {
				t.Fatalf("reading aspects: %v", err)
			}
			for _, a := range aspects {
				if a.Type != profile.Location && a.Type != profile.WorkRights &&
					a.Type != profile.WorkArrangement {
					continue
				}
				t.Logf("%s: %s %+v wording=%q", listing.ID, a.Type, a.Structured, a.Wording)
				for _, c := range a.Citations {
					t.Logf("    cites chunk %d: %q", c.ChunkID, c.Quote)
				}
			}
		}
	}
}
