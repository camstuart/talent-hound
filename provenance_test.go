package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Content a model wrote is marked as such wherever it is shown.
//
// The stylesheet gives model output a dashed border and the label "Written by a
// model — check it", and an end-to-end test proves those three treatments
// differ from one another. Nothing proved every screen used them.
//
// It did not. A role's requirements are lifted out of a listing by the same
// classifier that produces a candidate's aspects, and the role panel branched
// on the aspect's origin to print the word "extracted" in small grey text while
// the candidate panel gave the identical content its marking. The same sentence
// was labelled "Written by a model — check it" on one screen and not the other.
//
// So: a component that knows an aspect's origin has to say so in the attribute
// the stylesheet reads, not only in prose.
func TestEveryScreenThatKnowsAnAspectsOriginMarksIt(t *testing.T) {
	const components = "frontend/src/components"
	entries, err := os.ReadDir(components)
	if err != nil {
		t.Fatalf("reading the components: %v", err)
	}

	knew := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".tsx") || strings.HasSuffix(name, ".test.tsx") {
			continue
		}
		body := string(mustRead(filepath.Join(components, name)))
		// recruiter_supplied is an aspect's origin specifically. A role record
		// also has an origin — recruiter_entered or discovered — which is how
		// the record was created rather than who wrote the words, and marking a
		// form field with it would label a recruiter's own role as AI output.
		if !strings.Contains(body, "recruiter_supplied") {
			continue
		}
		knew++
		if !strings.Contains(body, "data-provenance") {
			t.Errorf("%s branches on where an aspect came from and never marks it, "+
				"so the stylesheet cannot tell the reader", name)
		}
	}
	if knew == 0 {
		t.Fatal("no component appears to know an aspect's origin, which cannot be right")
	}
}

// And the three values the stylesheet understands are the only three used: a
// fourth is a silent no-op, styled like nothing and labelled as nothing.
func TestOnlyTheProvenanceValuesTheStylesheetKnowsAreUsed(t *testing.T) {
	styled := map[string]bool{}
	css := string(mustRead(filepath.Join("frontend", "public", "style.css")))
	for _, part := range strings.Split(css, `[data-provenance="`)[1:] {
		if name, _, ok := strings.Cut(part, `"`); ok {
			styled[name] = true
		}
	}
	if len(styled) == 0 {
		t.Fatal("the stylesheet gives provenance no treatment at all")
	}

	err := filepath.Walk("frontend/src", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".tsx") {
			return err
		}
		for _, part := range strings.Split(string(mustRead(path)), `data-provenance="`)[1:] {
			name, _, ok := strings.Cut(part, `"`)
			if ok && !styled[name] {
				t.Errorf("%s marks content %q, which the stylesheet does not style", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the components: %v", err)
	}
}
