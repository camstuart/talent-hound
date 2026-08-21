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

// Job parameters name records; they do not carry what is in them.
//
// The Job type says it plainly: "It carries no content — Params names records
// by ID, and FailureReason is a code — so a job record can be shown, logged,
// and exported freely." Everything downstream trusts that. The reason is
// enforced, by CheckReason on the way into the database. The parameters are
// enforced by nobody.
//
// Six workers get it right today, all of them carrying identifiers, counts and
// settings. The seventh is the problem: a discovery worker holding the query, an
// extraction worker holding a filename, a draft worker holding the text — each
// one reasonable in isolation, each one putting a stranger's words somewhere the
// comment above promises is safe to export.
//
// So a string in a params struct has to be a setting, and the ones that are are
// named here. A new one fails until somebody decides which it is.
func TestJobParametersNameRecordsRatherThanCarryingThem(t *testing.T) {
	// Settings: which model, which role, which kind of owner. Not content.
	settings := map[string]bool{
		"Model": true, "Role": true, "Kind": true, "OwnerKind": true,
	}

	checked := 0
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && skipped[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		lines := strings.Split(string(mustRead(path)), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasSuffix(trimmed, "arams struct {") || !strings.HasPrefix(trimmed, "type ") {
				continue
			}
			checked++
			for _, field := range lines[i+1:] {
				field = strings.TrimSpace(field)
				if field == "}" {
					break
				}
				parts := strings.Fields(field)
				// An exported field: anything else is a comment, a tag, or
				// an embedded type.
				if len(parts) < 2 || parts[0][0] < 'A' || parts[0][0] > 'Z' {
					continue
				}
				name, typ := parts[0], parts[1]
				if typ != "string" && typ != "[]string" {
					continue
				}
				if settings[name] {
					continue
				}
				t.Errorf("%s: %s carries %s %s — job parameters are shown, logged and "+
					"exported as safe, so a string here has to be a setting rather than "+
					"something somebody wrote", path, trimmed, name, typ)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if checked == 0 {
		t.Fatal("no job parameter type was found, so this checked nothing")
	}
}
