//go:build livemodel

package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
)

// Why the contract rejects a profile, in the model's own output.
//
// This calls the model directly with the product's prompt and schema and runs
// the product's validator over the answer, so a rejected profile can be
// inspected — which the service deliberately does not allow, because its errors
// must not quote the document. The corpus here is the synthetic one, where the
// "document" is invented.
//
//	go test -tags livemodel -run TestDiagnoseRejections -v .
func TestDiagnoseRejections(t *testing.T) {
	model := os.Getenv("TH_CLASSIFY_MODEL")
	if model == "" {
		t.Skip("set TH_CLASSIFY_MODEL")
	}
	corpus, err := bench.Load()
	if err != nil {
		t.Fatalf("loading the corpus: %v", err)
	}
	ollama := platform.NewOllama()

	kinds := map[string]int{}
	// The resumes are chunked in the real flow, so a citation can name the
	// wrong chunk while quoting wording that is plainly in another one.
	for _, scenario := range corpus.Scenarios {
		sources := []profile.Source{}
		for i, part := range strings.Split(scenario.Resume, "\n## ") {
			if i > 0 {
				part = "## " + part
			}
			sources = append(sources, profile.Source{ChunkID: uint(i + 1), Text: part})
		}
		listing := bench.Listing{ID: scenario.ID, Markdown: scenario.Resume}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		raw, err := ollama.Chat(ctx, model, profile.Prompt(profile.SubjectRole, sources),
			profile.Schema(profile.SubjectRole))
		cancel()
		if err != nil {
			kinds["endpoint: "+err.Error()]++
			continue
		}
		proposal, problems := profile.ParseProposal(raw)
		if len(problems) > 0 {
			kinds["unparsable"]++
			continue
		}
		problems = profile.Validate(profile.SubjectRole, proposal, sources)
		if len(problems) == 0 {
			kinds["accepted"]++
			continue
		}
		for _, p := range problems {
			kinds[classify(p)]++
			t.Logf("%s: %s", listing.ID, p)
		}
		// And the quotes themselves, so a mismatch can be read rather than
		// guessed at.
		for _, a := range proposal.Aspects {
			for _, c := range a.Citations {
				cited, elsewhere := "", ""
				for _, src := range sources {
					if !strings.Contains(squash(src.Text), squash(c.Quote)) {
						continue
					}
					if src.ChunkID == c.ChunkID {
						cited = "here"
					} else {
						elsewhere = "elsewhere"
					}
				}
				switch {
				case cited != "":
				case elsewhere != "":
					kinds["quote is in another supplied chunk"]++
					t.Logf("  wrong chunk: cited %d for %q", c.ChunkID, c.Quote)
				default:
					kinds["quote is in no supplied chunk"]++
					t.Logf("  absent everywhere: %q", c.Quote)
				}
			}
		}
	}
	for kind, n := range kinds {
		t.Logf("EVIDENCE rejection kind=%q count=%d", kind, n)
	}
}

// classify buckets a validator problem by its kind, without its detail.
func classify(problem string) string {
	switch {
	case strings.Contains(problem, "quotes wording"):
		return "quote does not resolve"
	case strings.Contains(problem, "duplicates aspect"):
		return "duplicate aspect"
	case strings.Contains(problem, "is not one of"):
		return "value outside the enumeration"
	case strings.Contains(problem, "not a permitted"):
		return "field outside the type"
	}
	return "other: " + problem
}

func squash(s string) string { return strings.Join(strings.Fields(s), " ") }
