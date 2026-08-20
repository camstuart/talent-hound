//go:build livemodel

package main

import (
	"os"
	"testing"
	"time"

	"camstuart/talent-hound/internal/bench"
)

// The classifier's four conditions measured on the tuning corpus, which exists
// so that a rule is never chosen against the frozen set. A change that looks
// like an improvement on the corpus it was written for is a coincidence; this
// is where it is allowed to be judged.
//
//	go test -tags livemodel -run TestTuneClassifier -v -timeout 60m .
func TestTuneClassifier(t *testing.T) {
	model := os.Getenv("TH_CLASSIFY_MODEL")
	if model == "" {
		t.Skip("set TH_CLASSIFY_MODEL")
	}
	corpus, err := bench.LoadTuning()
	if err != nil {
		t.Fatalf("loading the tuning corpus: %v", err)
	}

	elapsed := []time.Duration{}
	scores := runClassifier(t, corpus, model, &elapsed)
	record := &bench.Record{Classifier: scores}
	totals := record.ClassifierTotals()
	t.Logf("tuning classifier: capture %.0f%% (%d/%d), uncited %d, unsupported %d, misreported %d",
		totals.CaptureRate*100, totals.Captured, totals.Material,
		totals.Uncited, totals.Unsupported, totals.Misreported)
	for _, s := range scores {
		for _, why := range append(append([]string{}, s.Unsupported...), s.Misreported...) {
			t.Logf("  %s: %s", s.Listing, why)
		}
	}
}
