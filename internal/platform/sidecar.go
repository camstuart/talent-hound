package platform

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Limits bound one sidecar invocation. Zero values mean "no limit".
type Limits struct {
	Timeout   time.Duration
	MemoryMax uint64 // bytes, enforced on the whole process tree
	OutputMax int64  // bytes of stdout read before the tree is killed
}

// DefaultLimits follow the PRD envelope: 25 MB input, 10 MB extracted Markdown.
var DefaultLimits = Limits{
	Timeout:   2 * time.Minute,
	MemoryMax: 1 << 30, // 1 GiB
	OutputMax: 10 << 20,
}

// Failure kinds. All are retryable except FailureExtract, which is terminal for
// the given input.
var (
	ErrTimeout     = errors.New("sidecar timed out")
	ErrMemoryLimit = errors.New("sidecar exceeded memory limit")
	ErrOutputLimit = errors.New("sidecar exceeded output limit")
	ErrExtract     = errors.New("sidecar failed to extract")
)

// versionProbeLimits bound the startup version probe: it prints one line, so
// anything slow or chatty is already a broken install.
var versionProbeLimits = Limits{Timeout: 10 * time.Second, MemoryMax: 1 << 28, OutputMax: 4096}

// SidecarVersion runs exe's --version probe and returns its trimmed output. It
// is the same containment as an extraction, in miniature: a sidecar that hangs
// on --version must not hang start-up.
func SidecarVersion(ctx context.Context, exe string) (string, error) {
	out, err := runContained(ctx, exe, []string{"--version"}, nil, versionProbeLimits)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ExtractMarkdown converts one file to Markdown with the packaged MarkItDown
// sidecar: one file per process, invoked by verified absolute path, plugins and
// network features disabled, contained by the platform's process limits.
func ExtractMarkdown(ctx context.Context, sidecarExe, inputPath string, lim Limits) ([]byte, error) {
	exe, err := filepath.Abs(sidecarExe)
	if err != nil {
		return nil, fmt.Errorf("resolving sidecar path: %w", err)
	}
	info, err := os.Stat(exe)
	if err != nil {
		return nil, fmt.Errorf("sidecar not found at %s: %w", exe, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("sidecar path %s is a directory", exe)
	}
	in, err := filepath.Abs(inputPath)
	if err != nil {
		return nil, fmt.Errorf("resolving input path: %w", err)
	}

	// MarkItDown writes Markdown for one input file to stdout; plugins are off
	// unless --use-plugins is passed, and no network converters are enabled.
	// ponytail: env is cleared rather than filtered — the sidecar is
	// self-contained (PyInstaller one-dir) and needs nothing from ours.
	out, err := runContained(ctx, exe, []string{in}, nil, lim)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty output for %s", ErrExtract, in)
	}
	return out, nil
}
