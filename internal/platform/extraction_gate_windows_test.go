//go:build windowsgate

// Phase 6's Windows-only proofs. Everything that can be shown without a real
// Job Object and a real MarkItDown is a plain unit test elsewhere; what is left
// here needs the target laptop and the packaged sidecar.
package platform_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/extract"
	"camstuart/talent-hound/internal/models"
)

// The structure the golden DOCX fixture carries; see testdata/docs/generate.py.
const (
	goldenHeading = "Synthetic Experience"
	goldenBullet  = "Ran a synthetic pipeline"
	goldenUnicode = "Café Ǆ 東京 — naïve résumé"
	goldenCellA   = "Skill"
	goldenCellB   = "Years"
)

// verifiedSidecar verifies the packaged sidecar the way the application does.
func verifiedSidecar(t *testing.T) *extract.Sidecar {
	t.Helper()
	exe := sidecarExe(t)
	s := extract.Verify(context.Background(), exe)
	if !s.Available() {
		t.Fatalf("packaged sidecar failed verification: %s", s.Detail())
	}
	return s
}

func TestGateGoldenDOCXKeepsItsStructure(t *testing.T) {
	s := verifiedSidecar(t)
	data, err := os.ReadFile(filepath.Join("testdata", "docs", "fixture.docx"))
	if err != nil {
		t.Fatalf("reading the golden fixture: %v", err)
	}
	res, err := s.Run(context.Background(), t.TempDir(),
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
	if err != nil {
		t.Fatalf("extracting the golden DOCX: %v", err)
	}
	for _, want := range []string{fixtureMarker, goldenHeading, goldenBullet, goldenUnicode, goldenCellA, goldenCellB} {
		if !strings.Contains(res.Markdown, want) {
			t.Errorf("the extraction lost %q", want)
		}
	}
	// Heading, list, and table markup are what Markdown is for; losing them
	// costs the citations Phase 7 resolves back to a place in a document.
	if !strings.Contains(res.Markdown, "#") {
		t.Error("no heading markup survived")
	}
	if !strings.Contains(res.Markdown, "|") {
		t.Error("no table markup survived")
	}
	t.Logf("EVIDENCE golden-docx bytes=%d", len(res.Markdown))
}

func TestGateGoldenPDFExtracts(t *testing.T) {
	s := verifiedSidecar(t)
	data, err := os.ReadFile(filepath.Join("testdata", "docs", "fixture.pdf"))
	if err != nil {
		t.Fatalf("reading the golden fixture: %v", err)
	}
	res, err := s.Run(context.Background(), t.TempDir(), "application/pdf", data)
	if err != nil {
		t.Fatalf("extracting the golden PDF: %v", err)
	}
	if !strings.Contains(res.Markdown, fixtureMarker) {
		t.Errorf("the extraction lost the fixture marker: %q", res.Markdown)
	}
	t.Logf("EVIDENCE golden-pdf bytes=%d", len(res.Markdown))
}

func TestGateCorruptPDFIsACode(t *testing.T) {
	s := verifiedSidecar(t)
	data, err := os.ReadFile(filepath.Join("testdata", "docs", "corrupt.pdf"))
	if err != nil {
		t.Fatalf("reading the corrupt fixture: %v", err)
	}
	_, err = s.Run(context.Background(), t.TempDir(), "application/pdf", data)
	if err == nil {
		t.Fatal("a corrupt PDF extracted without error")
	}
	code := extract.Code(err)
	switch code {
	case models.ReasonExtractFailed, models.ReasonExtractEmpty:
	default:
		t.Errorf("corrupt input gave %q, want a retryable extraction code", code)
	}
	// Whatever MarkItDown said about the file, none of it is in the code.
	if strings.ContainsAny(code, " '\"") {
		t.Errorf("the reason %q is not a code", code)
	}
}

func TestGateStagingDirectoryDeniesOtherUsers(t *testing.T) {
	s := verifiedSidecar(t)
	dir := t.TempDir()
	// Something slow enough to inspect while it exists: the hanging fixture is
	// not packaged, so this checks the staging root the run leaves behind.
	data, err := os.ReadFile(filepath.Join("testdata", "docs", "fixture.pdf"))
	if err != nil {
		t.Fatalf("reading the golden fixture: %v", err)
	}
	if _, err := s.Run(context.Background(), dir, "application/pdf", data); err != nil {
		t.Fatalf("extracting: %v", err)
	}

	root := filepath.Join(dir, "extract")
	out, err := exec.Command("icacls", root).CombinedOutput()
	if err != nil {
		t.Fatalf("reading the staging root's ACL: %v\n%s", err, out)
	}
	acl := string(out)
	me := os.Getenv("USERNAME")
	if me == "" {
		t.Fatal("USERNAME is unset; cannot tell which user should have access")
	}
	// Everyone, Users, and Authenticated Users are the three that would make a
	// recruiter's plaintext readable by another account on the laptop.
	for _, forbidden := range []string{"Everyone", "BUILTIN\\Users", "Authenticated Users"} {
		if strings.Contains(acl, forbidden) {
			t.Errorf("the staging root grants %s:\n%s", forbidden, acl)
		}
	}
	if !strings.Contains(acl, me) {
		t.Errorf("the staging root does not grant the current user:\n%s", acl)
	}
	t.Logf("EVIDENCE staging-acl\n%s", acl)
}

func TestGatePluginsAndNetworkStayOff(t *testing.T) {
	exe := sidecarExe(t)
	// The invocation is the proof: MarkItDown enables plugins only when it is
	// asked to, so the argument list is where "off" is either true or not.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("running --help: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "--use-plugins") {
		t.Fatalf("this MarkItDown does not have the plugin flag this check assumes:\n%s", out)
	}
	t.Logf("EVIDENCE plugins-flag-present; the application never passes it")

	// And a document naming a remote resource must extract to the text of the
	// link, not to whatever is at the other end of it.
	s := verifiedSidecar(t)
	data, err := os.ReadFile(filepath.Join("testdata", "docs", "fixture.docx"))
	if err != nil {
		t.Fatalf("reading the golden fixture: %v", err)
	}
	res, err := s.Run(ctx, t.TempDir(),
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document", data)
	if err != nil {
		t.Fatalf("extracting: %v", err)
	}
	if strings.Contains(res.Markdown, "<html") {
		t.Errorf("the extraction contains fetched markup:\n%s", res.Markdown)
	}
}
