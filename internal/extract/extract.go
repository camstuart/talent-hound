// Package extract turns document bytes into Markdown: it verifies the bundled
// sidecar, gives it one anonymous temporary copy of one file, and makes sure
// that copy does not outlive the run.
//
// Nothing here stores anything. It answers with Markdown or with a reason code,
// and the reason codes are deliberately lossy — a document parser's errors
// quote the document, and a quoted document is candidate content.
package extract

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// PinnedSidecarVersion is the MarkItDown build this application ships with; see
// build/sidecar/PIN.md. A sidecar reporting anything else is not ours.
const PinnedSidecarVersion = "0.1.2"

// Extractor names recorded against an artifact as provenance.
const (
	SidecarExtractor = "markitdown-sidecar"
	NativeExtractor  = "native-text"
)

// stagingRoot is the single directory every temporary input lives under, so the
// startup sweep is one unambiguous listing rather than a pattern match over the
// recruiter's data folder.
const stagingRoot = "extract"

// SidecarPathEnv overrides where the sidecar is found. The Phase 1 gates
// already use it; a packaged install needs no override.
const SidecarPathEnv = "TH_SIDECAR_EXE"

// Sidecar is the verified extraction sidecar, or the reason there is not one.
type Sidecar struct {
	path    string
	version string
	// reason is a models reason code, empty when verification succeeded.
	reason string
	// detail explains a verification failure for a log line. It describes the
	// install, never a document, so it is safe to print.
	detail string
}

// DefaultSidecarPath is where a packaged install keeps the sidecar: beside the
// application binary, in its own one-dir folder.
func DefaultSidecarPath() string {
	if p := os.Getenv(SidecarPathEnv); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "markitdown-sidecar"
	if isWindowsExe(exe) {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(exe), "markitdown-sidecar", name)
}

func isWindowsExe(path string) bool { return strings.EqualFold(filepath.Ext(path), ".exe") }

// Verify checks the sidecar once: absolute path, present, a regular file, and
// reporting the pinned version. The result is cached for the process, because
// the answer cannot change without the install directory changing — but see
// Path, which re-checks presence before any document is written to disk.
func Verify(ctx context.Context, exe string) *Sidecar {
	switch {
	case exe == "":
		return &Sidecar{reason: models.ReasonSidecarMissing, detail: "no sidecar path configured"}
	case !filepath.IsAbs(exe):
		// A relative path is resolved against a working directory nobody
		// controls, which is a different binary depending on how we were
		// launched. That is exactly the substitution this check exists for.
		return &Sidecar{reason: models.ReasonSidecarMissing, detail: fmt.Sprintf("sidecar path %q is not absolute", exe)}
	}
	info, err := os.Stat(exe)
	switch {
	case err != nil:
		return &Sidecar{reason: models.ReasonSidecarMissing, detail: fmt.Sprintf("sidecar not found at %s", exe)}
	case info.IsDir():
		return &Sidecar{reason: models.ReasonSidecarMissing, detail: fmt.Sprintf("sidecar path %s is a directory", exe)}
	}

	out, err := platform.SidecarVersion(ctx, exe)
	if err != nil {
		return &Sidecar{reason: models.ReasonSidecarVersion, detail: fmt.Sprintf("sidecar at %s did not report a version", exe)}
	}
	got := out
	if fields := strings.Fields(out); len(fields) > 0 {
		got = fields[len(fields)-1]
	}
	if got != PinnedSidecarVersion {
		return &Sidecar{
			reason: models.ReasonSidecarVersion,
			detail: fmt.Sprintf("sidecar at %s reports %q, want %q", exe, got, PinnedSidecarVersion),
		}
	}
	return &Sidecar{path: exe, version: got}
}

// Available reports whether verification succeeded.
func (s *Sidecar) Available() bool { return s.reason == "" }

// Reason is the code an extraction needing the sidecar fails with.
func (s *Sidecar) Reason() string { return s.reason }

// Detail describes a verification failure. It names the install, not a
// document, so it is safe to log.
func (s *Sidecar) Detail() string { return s.detail }

// Version is the verified version, empty when verification failed.
func (s *Sidecar) Version() string { return s.version }

// path re-checks that the verified binary is still there. Called before a
// staging directory exists, so a sidecar swapped or removed after start-up
// fails the extraction rather than seeing the document.
func (s *Sidecar) resolve() (string, error) {
	if !s.Available() {
		return "", codedError{code: s.reason}
	}
	info, err := os.Stat(s.path)
	if err != nil || info.IsDir() {
		return "", codedError{code: models.ReasonSidecarMissing}
	}
	return s.path, nil
}

// codedError is a failure reduced to its reason code. It is the only kind of
// error this package returns for an extraction: everything richer would be a
// sentence about the document.
type codedError struct{ code string }

func (c codedError) Error() string { return "extraction failed: " + c.code }

// Code returns err's reason code, or the generic extraction failure.
func Code(err error) string {
	var c codedError
	if errors.As(err, &c) {
		return c.code
	}
	return models.ReasonExtractFailed
}

// baseType drops the parameters content sniffing attaches, so "text/plain;
// charset=utf-8" and "text/plain" are the one type they describe.
func baseType(mediaType string) string {
	return strings.TrimSpace(strings.Split(mediaType, ";")[0])
}

// Native reports whether a media type is read directly, without the sidecar.
func Native(mediaType string) bool {
	t := baseType(mediaType)
	return t == "text/plain" || t == "text/markdown"
}

// NeedsSidecar reports whether a media type is one the sidecar converts.
func NeedsSidecar(mediaType string) bool {
	switch baseType(mediaType) {
	case "application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return true
	}
	return false
}

// extensions the sidecar dispatches on. It is the only thing about the original
// file that reaches the staging path.
var extensions = map[string]string{
	"application/pdf": ".pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
}

// Result is one extraction's output and the provenance of who produced it.
type Result struct {
	Markdown  string
	Extractor string
	Version   string
}

// Run converts one artifact's bytes to Markdown. dataDir is the folder that
// holds the database — the encrypted one — because that is where a temporary
// plaintext copy is allowed to exist.
//
// Every failure is a reason code and nothing else. Read it with Code.
func (s *Sidecar) Run(ctx context.Context, dataDir, mediaType string, data []byte) (Result, error) {
	switch {
	case Native(mediaType):
		if len(data) > models.MaxMarkdownBytes {
			return Result{}, codedError{code: models.ReasonExtractOutput}
		}
		if len(data) == 0 {
			return Result{}, codedError{code: models.ReasonExtractEmpty}
		}
		return Result{Markdown: string(data), Extractor: NativeExtractor, Version: PinnedSidecarVersion}, nil
	case !NeedsSidecar(mediaType):
		return Result{}, codedError{code: models.ReasonUnsupported}
	}

	// Before anything is written: is the binary we verified still the binary?
	exe, err := s.resolve()
	if err != nil {
		return Result{}, err
	}

	dir, input, err := stage(dataDir, extensions[baseType(mediaType)], data)
	if err != nil {
		return Result{}, codedError{code: models.ReasonExtractFailed}
	}
	// However this ends — success, failure, timeout, panic — the plaintext goes.
	defer func() { _ = os.RemoveAll(dir) }()

	md, err := platform.ExtractMarkdown(ctx, exe, input, platform.DefaultLimits)
	if err != nil {
		return Result{}, codedError{code: reasonFor(err)}
	}
	if len(md) == 0 {
		return Result{}, codedError{code: models.ReasonExtractEmpty}
	}
	if len(md) > models.MaxMarkdownBytes {
		// A cap is a refusal, not a truncation: half a document is evidence of
		// nothing and nobody can tell which half is missing.
		return Result{}, codedError{code: models.ReasonExtractOutput}
	}
	return Result{Markdown: string(md), Extractor: SidecarExtractor, Version: s.version}, nil
}

// reasonFor reduces a platform failure to a code. The error's text is dropped
// on purpose: it is the sidecar quoting the document.
func reasonFor(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return models.ReasonExtractFailed
	case errors.Is(err, platform.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return models.ReasonExtractTimeout
	case errors.Is(err, platform.ErrMemoryLimit):
		return models.ReasonExtractMemory
	case errors.Is(err, platform.ErrOutputLimit):
		return models.ReasonExtractOutput
	case errors.Is(err, platform.ErrExtract):
		if strings.Contains(err.Error(), "empty output") {
			return models.ReasonExtractEmpty
		}
		return models.ReasonExtractFailed
	}
	return models.ReasonExtractFailed
}

// stage writes data to a randomly named, current-user-only directory under the
// data folder and returns the directory and the file inside it.
//
// The name carries no identity. Paths end up in error strings, crash dumps, and
// over the recruiter's shoulder, and "Priya Raman CV.pdf" in any of those is a
// leak that no amount of care elsewhere undoes. Only the extension survives,
// because the sidecar dispatches on it.
func stage(dataDir, ext string, data []byte) (dir, file string, err error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("naming staging directory: %w", err)
	}
	dir = filepath.Join(dataDir, stagingRoot, hex.EncodeToString(buf))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", fmt.Errorf("creating staging directory: %w", err)
	}
	file = filepath.Join(dir, "input"+ext)
	if err := os.WriteFile(file, data, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", fmt.Errorf("writing staging file: %w", err)
	}
	return dir, file, nil
}

// Sweep removes every staging directory under dataDir. A crash is the only way
// one survives an extraction, and this is how long it survives: until the next
// launch. Nothing outside the staging root is touched.
func Sweep(dataDir string) error {
	root := filepath.Join(dataDir, stagingRoot)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading staging root: %w", err)
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
			return fmt.Errorf("sweeping staging directory: %w", err)
		}
	}
	return nil
}
