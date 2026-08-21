//go:build livemodel

package main

import (
	"os"
	"testing"
	"time"

	"camstuart/talent-hound/internal/bench"
)

// The matching benchmark run against the tuning corpus, with the provenance of
// every ranked role.
//
// Measured properly — real decomposed role profiles rather than a hand-built
// aspect holding the title — the frozen corpus scores 3 of 5, under the PRD's
// bar of 4. Whatever is wrong with the ranking gets diagnosed here, on the
// corpus that exists for it, and the frozen set only ever scores the result.
//
//	go test -tags livemodel -run TestTuneMatching -v -timeout 120m .
func TestTuneMatching(t *testing.T) {
	classifyModel := os.Getenv("TH_CLASSIFY_MODEL")
	embedModel := os.Getenv("TH_EMBED_MODEL")
	if classifyModel == "" || embedModel == "" {
		t.Skip("set TH_CLASSIFY_MODEL and TH_EMBED_MODEL")
	}
	corpus, err := bench.LoadTuning()
	if err != nil {
		t.Fatalf("loading the tuning corpus: %v", err)
	}

	candidate, retrieval := []time.Duration{}, []time.Duration{}
	score, eligible := runMatching(t, corpus, classifyModel, embedModel, &candidate, &retrieval)
	t.Logf("tuning matching: %d of %d scenarios reached three plausible, %d eligible roles",
		score.MetCount, len(score.Scenarios), eligible)
	for _, s := range score.Scenarios {
		t.Logf("  %s: %d plausible of %d distinct %s", s.ScenarioID, s.Plausible, len(s.Distinct), s.Note)
	}
}
