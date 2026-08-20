//go:build livemodel

package main

import (
	"context"
	"os"
	"testing"

	"camstuart/talent-hound/internal/bench"
	"camstuart/talent-hound/internal/platform"
	"camstuart/talent-hound/internal/profile"
)

// Why an employment type the listing states outright reaches no aspect at all.
//
//	go test -tags livemodel -run TestDiagnoseEmploymentType -v -timeout 60m .
func TestDiagnoseEmploymentType(t *testing.T) {
	model := os.Getenv("TH_CLASSIFY_MODEL")
	if model == "" {
		t.Skip("set TH_CLASSIFY_MODEL")
	}
	frozen, err := bench.Load()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	tuning, err := bench.LoadTuning()
	if err != nil {
		t.Fatalf("loading the tuning corpus: %v", err)
	}
	want := map[string]bool{
		"platform-engineer-melbourne":    true,
		"clinical-data-manager-hamilton": true,
		"marine-telemetry-nelson":        true,
	}

	for _, listing := range append(append([]bench.Listing{}, frozen.Listings...), tuning.Listings...) {
		if !want[listing.ID] {
			continue
		}
		e := newClassifyEnv(t)
		e.classify = NewClassifyService(e.db, e.registry, platform.NewOllama())
		e.assignClassify(t, model)
		ids := e.withSource(t, listing.ID, listing.Markdown)

		var sources []profile.Source
		for _, chunk := range e.chunks2 {
			sources = append(sources, profile.Source{ChunkID: chunk.ID, Text: chunk.Text})
		}

		// The constraints pass on its own: it is discarded whole if any one of
		// its aspects fails the contract, so a single bad aspect takes the
		// employment type down with it.
		raw, problems, _, err := e.classify.attempt(context.Background(), model,
			profile.ConstraintPrompt(profile.SubjectRole, sources),
			profile.ConstraintSchema(profile.SubjectRole), profile.SubjectRole, sources)
		if err != nil {
			t.Logf("%s: the constraints pass did not answer: %v", listing.ID, err)
		}
		for _, a := range raw.Aspects {
			t.Logf("%s: constraints pass produced %s %+v wording=%q", listing.ID, a.Type, a.Structured, a.Wording)
		}
		for _, p := range problems {
			t.Logf("%s: constraints pass REFUSED: %s", listing.ID, p)
		}

		p, err := e.classify.Classify(ClassifyInput{
			SubjectKind: profile.SubjectRole, SubjectID: 1, ChunkIDs: ids})
		if err != nil {
			t.Logf("%s: classify refused: %v", listing.ID, err)
			continue
		}
		aspects, err := e.classify.Aspects(p.ID)
		if err != nil {
			t.Fatalf("reading aspects: %v", err)
		}
		types := []string{}
		for _, a := range aspects {
			types = append(types, string(a.Type))
			if a.Type == profile.EmploymentType {
				t.Logf("%s: FINAL employment_type %+v wording=%q", listing.ID, a.Structured, a.Wording)
			}
		}
		t.Logf("%s: final aspect types %v", listing.ID, types)
	}
}
