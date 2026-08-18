package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/platform"
)

// Every fixture here is invented. No real candidate information and no real
// provider key appears in this repository's tests, fixtures, or output.

// fakeOllama is an OpenAI-compatible endpoint that answers however a test tells
// it to, and records every request it received so the payload shape can be
// asserted rather than assumed.
type fakeOllama struct {
	server *httptest.Server

	mu        sync.Mutex
	installed []string
	requests  []recorded
	// behaviour overrides, keyed by path prefix.
	status  int
	body    string
	delay   time.Duration
	pullErr string
}

type recorded struct {
	Path string
	Body map[string]any
	Raw  string
}

func newFakeOllama(t *testing.T, installed ...string) *fakeOllama {
	t.Helper()
	f := &fakeOllama{installed: installed, status: http.StatusOK}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeOllama) handle(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	body := map[string]any{}
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.requests = append(f.requests, recorded{Path: r.URL.Path, Body: body, Raw: string(raw)})
	delay, status, custom, pullErr := f.delay, f.status, f.body, f.pullErr
	installed := append([]string(nil), f.installed...)
	f.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(custom))
		return
	}
	if custom != "" {
		_, _ = w.Write([]byte(custom))
		return
	}
	switch r.URL.Path {
	case "/api/tags":
		listed := make([]map[string]string, 0, len(installed))
		for _, name := range installed {
			listed = append(listed, map[string]string{"name": name, "model": name})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"models": listed})
	case "/api/pull":
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "error": pullErr})
	case "/v1/embeddings":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1, 0.2}}},
		})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "ok"}}},
		})
	}
}

func (f *fakeOllama) set(fn func(*fakeOllama)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeOllama) seen(path string) []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recorded
	for _, r := range f.requests {
		if r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

// modelEnv is a registry wired to a real database and a fake endpoint.
type modelEnv struct {
	db     *gorm.DB
	jobs   *JobService
	models *ModelService
	fake   *fakeOllama
}

func newModelEnv(t *testing.T, installed ...string) *modelEnv {
	t.Helper()
	gdb, err := db.Open(filepath.Join(t.TempDir(), "models.db"))
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	fake := newFakeOllama(t, installed...)
	jobs := NewJobService(gdb)
	ollama := &platform.Ollama{BaseURL: fake.server.URL, Client: &http.Client{Timeout: 2 * time.Second}}
	return &modelEnv{db: gdb, jobs: jobs, models: NewModelService(gdb, jobs, ollama), fake: fake}
}

func (e *modelEnv) assign(t *testing.T, in AssignInput) *models.ModelAssignment {
	t.Helper()
	got, err := e.models.Assign(in)
	if err != nil {
		t.Fatalf("assigning %s: %v", in.Role, err)
	}
	return got
}

func TestAssignmentRecordsTheModelIdentity(t *testing.T) {
	e := newModelEnv(t)
	got := e.assign(t, AssignInput{
		Role: models.RoleGenerate, Model: "qwen3:8b", Digest: "sha256:abc",
		Params: `{"temperature":0.2}`,
	})
	if got.Endpoint != platform.OllamaBaseURL {
		t.Errorf("endpoint %q, want the local one", got.Endpoint)
	}
	if got.Model != "qwen3:8b" || got.Digest != "sha256:abc" {
		t.Errorf("model %q digest %q", got.Model, got.Digest)
	}
	if got.Validation != models.Unvalidated {
		t.Errorf("a new assignment is %q, want unvalidated", got.Validation)
	}
	if got.Revision != 1 {
		t.Errorf("first revision is %d", got.Revision)
	}
}

func TestRegistryRefusesWhatItMustRefuse(t *testing.T) {
	e := newModelEnv(t)
	cases := []struct {
		name string
		in   AssignInput
	}{
		{"unknown role", AssignInput{Role: "summarise", Model: "qwen3:8b"}},
		{"no model", AssignInput{Role: models.RoleEmbed, Model: "   "}},
		{"cloud endpoint", AssignInput{Role: models.RoleGenerate, Endpoint: "https://api.example.com", Model: "gpt"}},
		{"bare host", AssignInput{Role: models.RoleGenerate, Endpoint: "localhost:11434", Model: "qwen3:8b"}},
		{"not a URL", AssignInput{Role: models.RoleGenerate, Endpoint: "not a url", Model: "qwen3:8b"}},
		{"params not JSON", AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b", Params: "temperature=1"}},
		{"unsupported param", AssignInput{Role: models.RoleEmbed, Model: "nomic", Params: `{"temperature":0.2}`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e.models.Assign(tc.in); err == nil {
				t.Fatalf("assignment was accepted: %+v", tc.in)
			}
		})
	}
	// And nothing was written by any of them.
	var n int64
	if err := e.db.Model(&models.ModelAssignment{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d assignments were written by refused requests", n)
	}
}

func TestARefusedAssignmentLeavesThePreviousOne(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b"})
	if _, err := e.models.Assign(AssignInput{
		Role: models.RoleGenerate, Endpoint: "https://api.example.com", Model: "gpt",
	}); err == nil {
		t.Fatal("a cloud endpoint was accepted for a required role")
	}
	res, err := e.models.Resolve(models.RoleGenerate)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assignment.Model != "qwen3:8b" || res.Assignment.Revision != 1 {
		t.Fatalf("the previous assignment changed: %+v", res.Assignment)
	}
}

func TestEveryConfigurationChangeIsARevision(t *testing.T) {
	e := newModelEnv(t)
	base := AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b", Digest: "sha256:a", Params: `{"temperature":0.2}`}
	e.assign(t, base)

	// Each step changes exactly one thing from the step before it, so the
	// revision it produces names the field that caused it.
	type step struct {
		name string
		in   AssignInput
		want int
	}
	changed := base
	steps := []step{
		{"identical is not a change", changed, 1},
		{"reordered parameters are not a change", withParams(changed, `{"temperature":0.2}`), 1},
	}
	add := func(name string, mutate func(*AssignInput), want int) {
		mutate(&changed)
		steps = append(steps, step{name, changed, want})
	}
	add("model", func(in *AssignInput) { in.Model = "qwen3:14b" }, 2)
	add("digest", func(in *AssignInput) { in.Digest = "sha256:b" }, 3)
	add("parameters", func(in *AssignInput) { in.Params = `{"temperature":0.7}` }, 4)
	add("endpoint", func(in *AssignInput) { in.Endpoint = "http://127.0.0.1:11435" }, 5)

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			got := e.assign(t, step.in)
			if got.Revision != step.want {
				t.Fatalf("revision %d, want %d", got.Revision, step.want)
			}
		})
	}

	// Every earlier revision is still there, unchanged.
	history, err := e.models.History(models.RoleGenerate)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 5 {
		t.Fatalf("%d revisions recorded, want 5", len(history))
	}
	if history[0].Model != "qwen3:8b" || history[0].Digest != "sha256:a" ||
		history[0].Params != `{"temperature":0.2}` {
		t.Fatalf("revision 1 was edited: %+v", history[0])
	}
}

func withParams(in AssignInput, params string) AssignInput { in.Params = params; return in }

func TestClassifyFollowsGenerateUntilItIsAssigned(t *testing.T) {
	e := newModelEnv(t)

	// Before anything is assigned, classify has nothing to inherit.
	res, err := e.models.Resolve(models.RoleClassify)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assignment != nil {
		t.Fatalf("classify resolved to %+v with no generate assignment", res.Assignment)
	}

	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b"})
	res, _ = e.models.Resolve(models.RoleClassify)
	if res.Assignment == nil || res.Assignment.Model != "qwen3:8b" || !res.Inherited {
		t.Fatalf("classify did not inherit generate: %+v", res)
	}

	// It keeps following.
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:14b"})
	res, _ = e.models.Resolve(models.RoleClassify)
	if res.Assignment.Model != "qwen3:14b" || !res.Inherited {
		t.Fatalf("classify stopped following generate: %+v", res)
	}

	// Until it is given a model of its own — and there is no second copy of the
	// generate configuration in the table.
	e.assign(t, AssignInput{Role: models.RoleClassify, Model: "qwen3:1.7b"})
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:32b"})
	res, _ = e.models.Resolve(models.RoleClassify)
	if res.Assignment.Model != "qwen3:1.7b" || res.Inherited {
		t.Fatalf("classify kept following after being assigned: %+v", res)
	}
	history, _ := e.models.History(models.RoleClassify)
	if len(history) != 1 {
		t.Fatalf("classify has %d revisions, want the one it was given", len(history))
	}
}

func TestValidationNeedsABenchmark(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b"})

	if err := e.models.MarkValidated(models.RoleGenerate, "  "); err == nil {
		t.Fatal("a model was validated with no benchmark reference")
	}
	res, _ := e.models.Resolve(models.RoleGenerate)
	if res.Assignment.Validation != models.Unvalidated {
		t.Fatalf("status is %q after a refused validation", res.Assignment.Validation)
	}

	// The database refuses it too, not only the service.
	err := e.db.Exec("UPDATE `model_assignments` SET `validation` = 'validated' WHERE 1").Error
	if err == nil {
		t.Fatal("the database recorded validated with no benchmark reference")
	}

	if err := e.models.MarkValidated(models.RoleGenerate, "benchmark-2026-08-17"); err != nil {
		t.Fatal(err)
	}
	res, _ = e.models.Resolve(models.RoleGenerate)
	if res.Assignment.Validation != models.Validated {
		t.Fatalf("status is %q after validation", res.Assignment.Validation)
	}

	// A new revision starts unvalidated again: it is a different configuration,
	// and nothing has benchmarked it.
	next := e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:14b"})
	if next.Validation != models.Unvalidated {
		t.Fatalf("revision %d is %q, want unvalidated", next.Revision, next.Validation)
	}
}

func TestAvailabilityStatesAreDistinct(t *testing.T) {
	e := newModelEnv(t, "qwen3:8b")
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b"})
	e.assign(t, AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"})

	byRole := func(t *testing.T) map[models.ModelRole]string {
		t.Helper()
		statuses, err := e.models.Check()
		if err != nil {
			t.Fatal(err)
		}
		out := map[models.ModelRole]string{}
		for _, s := range statuses {
			out[s.Role] = s.State
		}
		return out
	}

	got := byRole(t)
	if got[models.RoleGenerate] != models.ModelReady {
		t.Errorf("generate is %q, want ready", got[models.RoleGenerate])
	}
	// Assigned but not installed, which is not the same as not assigned.
	if got[models.RoleEmbed] != models.ModelMissing {
		t.Errorf("embed is %q, want model_missing", got[models.RoleEmbed])
	}
	// Classify inherits generate, so it is ready and says it inherited.
	if got[models.RoleClassify] != models.ModelReady {
		t.Errorf("classify is %q, want ready", got[models.RoleClassify])
	}

	// Declining is its own state, and only for the role that declined.
	if err := e.models.Decline(models.RoleEmbed); err != nil {
		t.Fatal(err)
	}
	if got := byRole(t); got[models.RoleEmbed] != models.ModelPullDeclined {
		t.Errorf("embed is %q after declining", got[models.RoleEmbed])
	}

	e.fake.set(func(f *fakeOllama) { f.delay = 3 * time.Second })
	if got := byRole(t); got[models.RoleGenerate] != models.ModelTimeout {
		t.Errorf("a silent endpoint is %q, want timeout", got[models.RoleGenerate])
	}
	e.fake.set(func(f *fakeOllama) { f.delay = 0; f.body = "{ this is not json" })
	if got := byRole(t); got[models.RoleGenerate] != models.ModelMalformed {
		t.Errorf("a malformed answer is %q", got[models.RoleGenerate])
	}
	e.fake.set(func(f *fakeOllama) {
		f.body = `{"error":"model requires more system memory than is available"}`
		f.status = http.StatusInsufficientStorage
	})
	if got := byRole(t); got[models.RoleGenerate] != models.ModelOutOfMemory {
		t.Errorf("a memory failure is %q", got[models.RoleGenerate])
	}

	// And with nothing listening at all.
	e.fake.server.Close()
	if got := byRole(t); got[models.RoleGenerate] != models.ModelEndpointDown {
		t.Errorf("a closed endpoint is %q, want endpoint_unavailable", got[models.RoleGenerate])
	}
}

func TestAnUnassignedRoleIsNotAMissingModel(t *testing.T) {
	e := newModelEnv(t)
	statuses, err := e.models.Check()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		if s.State != models.ModelUnassigned {
			t.Errorf("%s is %q with nothing assigned, want unassigned", s.Role, s.State)
		}
	}
}

func TestAnAvailabilityCheckCarriesNoContent(t *testing.T) {
	e := newModelEnv(t, "qwen3:8b")
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b"})
	if _, err := e.models.Check(); err != nil {
		t.Fatal(err)
	}
	seen := e.fake.seen("/api/tags")
	if len(seen) == 0 {
		t.Fatal("no listing request was made")
	}
	for _, r := range e.fake.requests {
		if strings.Contains(r.Raw, "messages") || strings.Contains(r.Raw, "input") {
			t.Fatalf("a check sent a payload with content: %q", r.Raw)
		}
	}
}

func TestEachRoleSendsItsOwnPayloadShape(t *testing.T) {
	e := newModelEnv(t, "qwen3:8b", "nomic-embed-text")
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "qwen3:8b"})
	e.assign(t, AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"})

	ollama := &platform.Ollama{BaseURL: e.fake.server.URL, Client: &http.Client{Timeout: 2 * time.Second}}
	ctx := t.Context()

	if _, err := ollama.Embed(ctx, "nomic-embed-text", "a sentence"); err != nil {
		t.Fatal(err)
	}
	embed := e.fake.seen("/v1/embeddings")
	if len(embed) != 1 || embed[0].Body["model"] != "nomic-embed-text" {
		t.Fatalf("embed payload: %+v", embed)
	}

	if _, err := ollama.Chat(ctx, "qwen3:8b", "hello", nil); err != nil {
		t.Fatal(err)
	}
	chat := e.fake.seen("/v1/chat/completions")
	if len(chat) != 1 || chat[0].Body["model"] != "qwen3:8b" || chat[0].Body["stream"] != false {
		t.Fatalf("chat payload: %+v", chat)
	}

	schema := map[string]any{"type": "object"}
	if _, err := ollama.Chat(ctx, "qwen3:8b", "classify this", schema); err != nil {
		t.Fatal(err)
	}
	chat = e.fake.seen("/v1/chat/completions")
	format, ok := chat[1].Body["response_format"].(map[string]any)
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("classify payload did not constrain the output: %+v", chat[1].Body)
	}
}

func TestAFailedPullIsDistinctFromAMissingModel(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"})

	e.fake.set(func(f *fakeOllama) { f.pullErr = "pull model manifest: file does not exist" })
	job, err := e.models.Pull(models.RoleEmbed)
	if err != nil {
		t.Fatal(err)
	}
	done := waitForJob(t, e.jobs, job.ID)
	if done.State != models.JobFailed || done.FailureReason != models.ModelPullFailed {
		t.Fatalf("pull job is %s (%q)", done.State, done.FailureReason)
	}
	statuses, _ := e.models.Check()
	for _, s := range statuses {
		if s.Role == models.RoleEmbed && s.State != models.ModelPullFailed {
			t.Fatalf("embed is %q after a failed pull, want pull_failed", s.State)
		}
	}
}

func TestADeclinedPullCanStillBeRun(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleEmbed, Model: "nomic-embed-text"})
	if err := e.models.Decline(models.RoleEmbed); err != nil {
		t.Fatal(err)
	}
	job, err := e.models.Pull(models.RoleEmbed)
	if err != nil {
		t.Fatalf("a declined role refused to pull: %v", err)
	}
	if done := waitForJob(t, e.jobs, job.ID); done.State != models.JobCompleted {
		t.Fatalf("pull job is %s (%q)", done.State, done.FailureReason)
	}
	if len(e.fake.seen("/api/pull")) != 1 {
		t.Fatal("no pull was attempted")
	}
}

func TestPullNeedsAnAssignedModel(t *testing.T) {
	e := newModelEnv(t)
	if _, err := e.models.Pull(models.RoleEmbed); err == nil {
		t.Fatal("a role with no assignment was pulled")
	}
}
