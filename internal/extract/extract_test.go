package extract

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// All fixtures here are invented.

func TestReasonForKeepsNoErrorText(t *testing.T) {
	// The memory limit is only ever produced on Windows, so the mapping is
	// proven here and the enforcement is proven by the gate.
	cases := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("%w after 2m0s", platform.ErrTimeout), models.ReasonExtractTimeout},
		{fmt.Errorf("%w of 1073741824 bytes", platform.ErrMemoryLimit), models.ReasonExtractMemory},
		{fmt.Errorf("%w: over 10485760 bytes of stdout", platform.ErrOutputLimit), models.ReasonExtractOutput},
		{fmt.Errorf("%w: empty output for C:\\x\\input.pdf", platform.ErrExtract), models.ReasonExtractEmpty},
		{fmt.Errorf("%w: exit status 2", platform.ErrExtract), models.ReasonExtractFailed},
		{context.DeadlineExceeded, models.ReasonExtractTimeout},
		{errors.New("ParseError at offset 41 in 'Priya Raman'"), models.ReasonExtractFailed},
	}
	for _, c := range cases {
		got := reasonFor(c.err)
		if got != c.want {
			t.Errorf("reasonFor(%v) = %q, want %q", c.err, got, c.want)
		}
		if !models.ValidReason(got) {
			t.Errorf("%q is not a storable reason code", got)
		}
	}
}

func TestCodeFallsBackToTheGenericFailure(t *testing.T) {
	if got := Code(codedError{code: models.ReasonExtractTimeout}); got != models.ReasonExtractTimeout {
		t.Errorf("Code lost the code: %q", got)
	}
	if got := Code(errors.New("something with a name in it")); got != models.ReasonExtractFailed {
		t.Errorf("Code(%q) = %q, want the generic failure", "something…", got)
	}
}

func TestMediaTypeRoutingIgnoresParameters(t *testing.T) {
	if !Native("text/plain; charset=utf-8") {
		t.Error("a sniffed text type with a charset should be native")
	}
	if !NeedsSidecar("application/pdf") || Native("application/pdf") {
		t.Error("a PDF should route to the sidecar")
	}
	if NeedsSidecar("image/png") || Native("image/png") {
		t.Error("a PNG should route nowhere")
	}
}

func TestStageNamesCarryNothingButTheExtension(t *testing.T) {
	dir := t.TempDir()
	staged, file, err := stage(dir, ".pdf", []byte("%PDF-1.4\n"))
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	if base := filepath.Base(file); base != "input.pdf" {
		t.Errorf("staged file is %q, want input.pdf", base)
	}
	if got := filepath.Base(staged); len(got) != 24 {
		t.Errorf("staging directory %q is not a random name", got)
	}
	// A second run of the same input must not reuse the directory.
	other, _, err := stage(dir, ".pdf", []byte("%PDF-1.4\n"))
	if err != nil {
		t.Fatalf("staging again: %v", err)
	}
	if other == staged {
		t.Error("two runs shared a staging directory")
	}
}

func TestSweepRemovesOnlyStagingDirectories(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := stage(dir, ".pdf", []byte("%PDF-1.4\n")); err != nil {
		t.Fatalf("staging: %v", err)
	}
	keep := filepath.Join(dir, "talent-hound.db")
	if err := os.WriteFile(keep, []byte("not the sweep's business"), 0o600); err != nil {
		t.Fatalf("writing a bystander: %v", err)
	}

	if err := Sweep(dir); err != nil {
		t.Fatalf("sweeping: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, stagingRoot))
	if err != nil {
		t.Fatalf("reading the staging root: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("%d staging directories survived the sweep", len(entries))
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("the sweep touched something outside the staging root: %v", err)
	}

	// Sweeping a data folder that has never staged anything is not an error.
	if err := Sweep(t.TempDir()); err != nil {
		t.Errorf("sweeping a fresh data folder: %v", err)
	}
}

func TestVerifyRefusesWhatIsNotOurSidecar(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		exe  string
		want string
	}{
		{"empty", "", models.ReasonSidecarMissing},
		{"relative", filepath.Join("build", "sidecar", "markitdown"), models.ReasonSidecarMissing},
		{"missing", filepath.Join(t.TempDir(), "absent"), models.ReasonSidecarMissing},
		{"directory", t.TempDir(), models.ReasonSidecarMissing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Verify(ctx, c.exe)
			if s.Available() {
				t.Fatal("verification succeeded")
			}
			if s.Reason() != c.want {
				t.Errorf("reason is %q, want %q", s.Reason(), c.want)
			}
			if _, err := s.resolve(); Code(err) != c.want {
				t.Errorf("resolve gave %q, want %q", Code(err), c.want)
			}
		})
	}
}

// Where the sidecar is looked for, and what happens when what is there is not
// what was expected. These are the laptop's first four failure modes and none
// of them had a test.
func TestTheSidecarIsFoundBesideTheApplicationOrRefusedClearly(t *testing.T) {
	t.Run("the override wins, because that is what it is for", func(t *testing.T) {
		t.Setenv(SidecarPathEnv, filepath.Join("/opt", "somewhere", "markitdown-sidecar"))
		if got := DefaultSidecarPath(); got != filepath.Join("/opt", "somewhere", "markitdown-sidecar") {
			t.Fatalf("DefaultSidecarPath = %q", got)
		}
	})

	t.Run("otherwise it sits in its own folder beside the binary", func(t *testing.T) {
		t.Setenv(SidecarPathEnv, "")
		got := DefaultSidecarPath()
		if got == "" {
			t.Skip("this process has no resolvable executable")
		}
		if !filepath.IsAbs(got) {
			t.Fatalf("the default path is relative: %q", got)
		}
		if filepath.Base(filepath.Dir(got)) != "markitdown-sidecar" {
			t.Fatalf("the sidecar is not in its own one-dir folder: %q", got)
		}
		// The name carries .exe exactly when the application binary does.
		wantExe := strings.EqualFold(filepath.Ext(os.Args[0]), ".exe")
		if strings.EqualFold(filepath.Ext(got), ".exe") != wantExe {
			t.Fatalf("the sidecar name is %q for an application named %q",
				filepath.Base(got), filepath.Base(os.Args[0]))
		}
	})
}

// Verification refuses everything it cannot prove, with a code and a detail
// that names the install rather than a document.
func TestVerificationRefusesWhatItCannotProve(t *testing.T) {
	dir := t.TempDir()
	relative := "markitdown-sidecar"
	absent := filepath.Join(dir, "not-there")

	for _, c := range []struct {
		name, path, wantIn string
	}{
		{"nothing configured", "", "no sidecar path configured"},
		{"a relative path", relative, "is not absolute"},
		{"a path with nothing at it", absent, "not found"},
		{"a directory", dir, "is a directory"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := Verify(context.Background(), c.path)
			if s.Available() {
				t.Fatal("verification accepted it")
			}
			if s.Reason() != models.ReasonSidecarMissing {
				t.Fatalf("reason = %q", s.Reason())
			}
			if !strings.Contains(s.Detail(), c.wantIn) {
				t.Fatalf("detail = %q, want it to mention %q", s.Detail(), c.wantIn)
			}
			if s.Version() != "" {
				t.Fatalf("a refused sidecar reports version %q", s.Version())
			}
			// The detail is logged, so it must never carry a document.
			if strings.Contains(strings.ToLower(s.Detail()), "resume") {
				t.Fatalf("the detail mentions a document: %q", s.Detail())
			}
		})
	}
}
