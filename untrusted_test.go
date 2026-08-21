package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Untrusted text is displayed, never rendered.
//
// Seven components say so in a comment beside the element that shows a résumé,
// a role listing, a model's answer or a recruiter's own words. None of it was
// enforced: oxlint, run with its defaults, accepts a component that hands
// somebody's document to innerHTML without a word.
//
// Everything this application displays came from outside it. A résumé is a file
// a stranger wrote, a discovered listing is a page nobody here controls, and a
// model's answer is a sentence about both. There is no case where rendering one
// as markup is the right call, so this does not ask which case it was.
//
// untrusted-check-exempt: this file names the ways markup gets rendered
func TestNothingRendersUntrustedTextAsMarkup(t *testing.T) {
	// The ways a string becomes markup: Solid's prop, the DOM properties, the
	// document-level writes, and the two that turn a string into code.
	renderers := []string{
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write",
		"dangerouslySetInnerHTML", "eval(", "new Function(",
	}
	const marker = "untrusted-check-exempt: this file names the ways markup gets rendered"

	found := []string{}
	err := filepath.Walk(filepath.Join("frontend", "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".ts", ".tsx", ".js", ".jsx":
		default:
			return nil
		}
		body := string(mustRead(path))
		if strings.Contains(body, marker) {
			return nil
		}
		for _, renderer := range renderers {
			if strings.Contains(body, renderer) {
				found = append(found, path+" uses "+renderer)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the frontend: %v", err)
	}
	if len(found) > 0 {
		t.Fatalf("untrusted text can reach the DOM as markup:\n  %s",
			strings.Join(found, "\n  "))
	}
}
