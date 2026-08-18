//go:build windowsgate

package platform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// buildFakeSidecar compiles testdata/fakesidecar and returns its path.
func buildFakeSidecar(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "fakesidecar.exe")
	cmd := exec.Command("go", "build", "-o", exe, "./testdata/fakesidecar")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building fake sidecar: %v\n%s", err, out)
	}
	return exe
}

func TestGateContainmentTimeout(t *testing.T) {
	exe := buildFakeSidecar(t)
	start := time.Now()
	_, err := runContained(context.Background(), exe, []string{"hang"}, nil,
		Limits{Timeout: 2 * time.Second, OutputMax: 1 << 20})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("timeout took %s, want prompt termination", elapsed)
	}
	assertParentHealthy(t, exe)
}

func TestGateContainmentKillsGrandchild(t *testing.T) {
	exe := buildFakeSidecar(t)
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	_, err := runContained(context.Background(), exe, []string{"spawn", pidFile}, nil,
		Limits{Timeout: 3 * time.Second, OutputMax: 1 << 20})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
	raw, readErr := os.ReadFile(pidFile) // #nosec G304 -- test-owned temp path
	if readErr != nil {
		t.Fatalf("fake sidecar never recorded a grandchild pid: %v", readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil {
		t.Fatalf("bad grandchild pid %q: %v", raw, convErr)
	}
	if !waitForExit(pid, 10*time.Second) {
		t.Fatalf("grandchild %d survived job termination", pid)
	}
	assertParentHealthy(t, exe)
}

func TestGateContainmentMemoryLimit(t *testing.T) {
	exe := buildFakeSidecar(t)
	_, err := runContained(context.Background(), exe, []string{"alloc"}, nil,
		Limits{Timeout: 60 * time.Second, MemoryMax: 128 << 20, OutputMax: 1 << 20})
	if !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("got %v, want ErrMemoryLimit", err)
	}
	assertParentHealthy(t, exe)
}

func TestGateContainmentOutputLimit(t *testing.T) {
	exe := buildFakeSidecar(t)
	_, err := runContained(context.Background(), exe, []string{"flood"}, nil,
		Limits{Timeout: 60 * time.Second, OutputMax: 1 << 20})
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("got %v, want ErrOutputLimit", err)
	}
	assertParentHealthy(t, exe)
}

// assertParentHealthy proves this process survived the containment failure and
// can still run the sidecar successfully.
func assertParentHealthy(t *testing.T, exe string) {
	t.Helper()
	out, err := runContained(context.Background(), exe, []string{"ok"}, nil,
		Limits{Timeout: 10 * time.Second, MemoryMax: 256 << 20, OutputMax: 1 << 20})
	if err != nil {
		t.Fatalf("parent unhealthy after containment failure: %v", err)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("unexpected output after recovery: %q", out)
	}
}
