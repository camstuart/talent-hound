package platform

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// ManagedOllamaBaseURL is where a copy this application launched listens. It is
// a different port from OllamaBaseURL so a recruiter's own Ollama and ours can
// never fight over one socket.
const ManagedOllamaBaseURL = "http://127.0.0.1:11435"

// OllamaPathEnv overrides where the bundled binary is looked for, the same way
// TALENT_HOUND_SIDECAR_PATH does for the extraction sidecar.
const OllamaPathEnv = "TALENT_HOUND_OLLAMA_PATH"

// NewOllamaAt returns a client for a specific endpoint.
func NewOllamaAt(baseURL string) *Ollama {
	return &Ollama{BaseURL: baseURL, Client: &http.Client{Timeout: 5 * time.Minute}}
}

// ollamaAnswers reports whether an Ollama answers at baseURL right now. Two
// seconds is the whole budget: a server that cannot say its version in two
// seconds is not one to route model calls to.
func ollamaAnswers(ctx context.Context, baseURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// BundledOllamaPath is where a packaged install keeps its own Ollama: beside
// the application binary, in its own folder — the extraction sidecar's pattern.
// Empty means nothing is bundled.
func BundledOllamaPath() string {
	if p := os.Getenv(OllamaPathEnv); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "ollama"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(exe), "ollama", name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// managedModelsDir is where the managed copy keeps model weights: the user
// cache, because weights are large, re-downloadable, and not candidate data —
// the recovery copy of the data folder must not carry gigabytes of them.
func managedModelsDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "talent-hound", "ollama-models")
}

// managedEnv is the spawned server's environment: the inherited one plus the
// pinned loopback host and the model store.
func managedEnv() []string {
	env := append(os.Environ(), "OLLAMA_HOST=127.0.0.1:11435")
	if dir := managedModelsDir(); dir != "" {
		env = append(env, "OLLAMA_MODELS="+dir)
	}
	return env
}

// startManaged launches exe as a serving Ollama and returns the function that
// stops it.
//
// ponytail: Kill on the one process we started. Ollama shuts its GPU runner
// children down itself on most exits; if orphaned runners turn out to happen in
// practice, the upgrade is a job object on Windows and a process group
// elsewhere, alongside jobobject_windows.go.
func startManaged(exe string) (func(), error) {
	cmd := exec.Command(exe, "serve") // #nosec G204 -- a fixed name beside our own binary, or an explicit operator override.
	cmd.Env = managedEnv()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting the bundled Ollama at %s: %w", exe, err)
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}, nil
}

// ResolveOllama returns the client to use for this run and the function to call
// when the application exits.
//
// Preference order: an Ollama already running at the standard endpoint (the
// recruiter's own install), then one at the managed endpoint (our own copy
// surviving a crash-restart), then spawning the bundled binary. Only a process
// this call started is ever stopped. With nothing running and nothing bundled
// it returns the standard client unspawned, and the first-run wizard's Ollama
// step says why nothing answers — exactly the behavior before bundling existed.
func ResolveOllama(ctx context.Context) (*Ollama, func()) {
	return resolveOllama(ctx, []string{OllamaBaseURL, ManagedOllamaBaseURL}, BundledOllamaPath(), startManaged)
}

func resolveOllama(ctx context.Context, candidates []string, bundled string, start func(string) (func(), error)) (*Ollama, func()) {
	noop := func() {}
	for _, url := range candidates {
		if ollamaAnswers(ctx, url) {
			return NewOllamaAt(url), noop
		}
	}
	if bundled == "" {
		return NewOllama(), noop
	}
	stop, err := start(bundled)
	if err != nil {
		// A bundled binary that will not start leaves the recruiter exactly
		// where a missing one would: the wizard explains the endpoint.
		return NewOllama(), noop
	}
	// No readiness wait: the client's existing ErrEndpointUnavailable states
	// already cover the warm-up seconds, and the wizard recomputes on demand.
	return NewOllamaAt(ManagedOllamaBaseURL), stop
}
