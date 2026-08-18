//go:build windowsgate

package platform_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"camstuart/talent-hound/internal/platform"
)

// fixtureMarker is the known text embedded in the synthetic fixtures.
const fixtureMarker = "TALENTHOUND-FIXTURE-MARKER"

// sidecarExe resolves the packaged MarkItDown sidecar. TH_SIDECAR_EXE must
// point at it; a missing sidecar fails the gate rather than skipping it.
func sidecarExe(t *testing.T) string {
	t.Helper()
	exe := os.Getenv("TH_SIDECAR_EXE")
	if exe == "" {
		t.Fatal("TH_SIDECAR_EXE is unset: build the sidecar (just sidecar) before running the gate")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("sidecar not found at %s: %v", exe, err)
	}
	return exe
}

func TestGateSidecarExtractsPDFAndDOCX(t *testing.T) {
	exe := sidecarExe(t)
	for _, name := range []string{"fixture.pdf", "fixture.docx"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			start := time.Now()
			md, err := platform.ExtractMarkdown(ctx,
				exe, filepath.Join("testdata", "docs", name), platform.DefaultLimits)
			if err != nil {
				t.Fatalf("extracting %s: %v", name, err)
			}
			if !strings.Contains(string(md), fixtureMarker) {
				t.Fatalf("markdown for %s lacks the fixture marker:\n%s", name, truncate(md))
			}
			t.Logf("EVIDENCE sidecar file=%s bytes=%d wall=%s", name, len(md), time.Since(start))
		})
	}
}

func TestGateSidecarCorruptInput(t *testing.T) {
	exe := sidecarExe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := platform.ExtractMarkdown(ctx,
		exe, filepath.Join("testdata", "docs", "corrupt.pdf"), platform.DefaultLimits); err == nil {
		t.Fatal("corrupt input extracted without error")
	} else {
		t.Logf("EVIDENCE sidecar corrupt-input error=%v", err)
	}

	// The parent must still be able to run a successful extraction afterwards.
	md, err := platform.ExtractMarkdown(ctx,
		exe, filepath.Join("testdata", "docs", "fixture.docx"), platform.DefaultLimits)
	if err != nil {
		t.Fatalf("parent unhealthy after corrupt input: %v", err)
	}
	if !strings.Contains(string(md), fixtureMarker) {
		t.Fatal("recovery extraction lacks the fixture marker")
	}
}

func truncate(b []byte) string {
	if len(b) > 400 {
		return string(b[:400]) + "…"
	}
	return string(b)
}
