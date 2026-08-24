package platform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOllamaAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if !ollamaAnswers(context.Background(), srv.URL) {
		t.Fatal("a live server should answer")
	}
	srv.Close()
	if ollamaAnswers(context.Background(), srv.URL) {
		t.Fatal("a closed server should not answer")
	}
}

func TestBundledOllamaPath(t *testing.T) {
	// The env override wins and is returned even without existing: it is an
	// explicit instruction, and a wrong one should fail loudly downstream.
	t.Setenv(OllamaPathEnv, "/explicit/ollama")
	if got := BundledOllamaPath(); got != "/explicit/ollama" {
		t.Fatalf("env override ignored: %q", got)
	}
	// Without the override and without a binary beside the executable, there
	// is nothing bundled.
	t.Setenv(OllamaPathEnv, "")
	if got := BundledOllamaPath(); got != "" {
		t.Fatalf("no bundled binary, but got %q", got)
	}
}

func TestResolveOllamaPrefersRunningEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	spawned := false
	o, stop := resolveOllama(context.Background(),
		[]string{srv.URL, "http://127.0.0.1:1"}, // first answers
		"/some/bundled/ollama",
		func(string) (func(), error) { spawned = true; return func() {}, nil })
	defer stop()
	if o.Endpoint() != srv.URL {
		t.Fatalf("should use the running endpoint, got %s", o.Endpoint())
	}
	if spawned {
		t.Fatal("must not spawn when an endpoint already answers")
	}
}

func TestResolveOllamaSpawnsWhenNothingAnswers(t *testing.T) {
	stopped := false
	o, stop := resolveOllama(context.Background(),
		[]string{"http://127.0.0.1:1", "http://127.0.0.1:1"},
		"/some/bundled/ollama",
		func(exe string) (func(), error) {
			if exe != "/some/bundled/ollama" {
				t.Fatalf("wrong exe: %s", exe)
			}
			return func() { stopped = true }, nil
		})
	if o.Endpoint() != ManagedOllamaBaseURL {
		t.Fatalf("spawned copy should be at the managed endpoint, got %s", o.Endpoint())
	}
	stop()
	if !stopped {
		t.Fatal("stop must stop what was spawned")
	}
}

func TestResolveOllamaFallsBackWithoutBundledBinary(t *testing.T) {
	o, stop := resolveOllama(context.Background(),
		[]string{"http://127.0.0.1:1", "http://127.0.0.1:1"},
		"", // nothing bundled
		func(string) (func(), error) { t.Fatal("nothing to spawn"); return nil, nil })
	defer stop()
	if o.Endpoint() != OllamaBaseURL {
		t.Fatalf("fallback must be today's endpoint, got %s", o.Endpoint())
	}
}

func TestStartManagedFailsOnMissingBinary(t *testing.T) {
	if _, err := startManaged(filepath.Join(t.TempDir(), "no-such-ollama")); err == nil {
		t.Fatal("starting a missing binary must fail")
	}
}

func TestManagedEnvPinsHostAndModels(t *testing.T) {
	env := managedEnv()
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "OLLAMA_HOST=127.0.0.1:11435") {
		t.Fatal("OLLAMA_HOST must pin the managed loopback port")
	}
	if !strings.Contains(joined, "OLLAMA_MODELS=") {
		t.Fatal("OLLAMA_MODELS must point at the managed model store")
	}
}

func TestManagedModelsDirIsUnderUserCache(t *testing.T) {
	base, err := os.UserCacheDir()
	if err != nil {
		t.Skip("no user cache dir on this machine")
	}
	want := filepath.Join(base, "talent-hound", "ollama-models")
	if got := managedModelsDir(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
