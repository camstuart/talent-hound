package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A schema-constrained call is extraction, not writing. Sampling makes the same
// sources produce different profiles — measured at 83% capture on one benchmark
// run and 64% on the next with no code between them — and the product's profile
// identity assumes a profile is a function of its sources.
func TestAConstrainedCallDecodesDeterministically(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "{}"}}},
		})
	}))
	defer server.Close()
	client := &Ollama{BaseURL: server.URL, Client: &http.Client{Timeout: 5 * time.Second}}

	schema := map[string]any{"type": "object"}
	if _, err := client.Chat(context.Background(), "m", "prompt", schema); err != nil {
		t.Fatalf("chatting: %v", err)
	}
	if got["temperature"] != float64(0) {
		t.Fatalf("temperature = %v, want 0", got["temperature"])
	}
	if got["top_p"] != float64(1) {
		t.Fatalf("top_p = %v, want 1", got["top_p"])
	}
	if got["seed"] == nil {
		t.Fatal("no seed was sent")
	}
}

// Free-form generation is left alone: a draft is writing, and this change is
// about the calls whose answer the sources already determine.
func TestAnUnconstrainedCallIsNotForcedDeterministic(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "hello"}}},
		})
	}))
	defer server.Close()
	client := &Ollama{BaseURL: server.URL, Client: &http.Client{Timeout: 5 * time.Second}}

	if _, err := client.Chat(context.Background(), "m", "prompt", nil); err != nil {
		t.Fatalf("chatting: %v", err)
	}
	if _, ok := got["temperature"]; ok {
		t.Fatalf("an unconstrained call carried a temperature: %v", got["temperature"])
	}
}

// A model digest identifies a model, and it has to mean the same thing on two
// machines or the record it goes into cannot compare them.
//
// The modelfile fallback used to keep the whole line it found "sha256" in,
// which is a FROM line naming a blob under somebody's home directory. Every
// benchmark record written before this stores one.
func TestTheModelDigestIsADigestAndNotAPathOnOneMachine(t *testing.T) {
	const hex = "2049f5674b1e92b4464e5729975c9689fcfbf0b0e4443ccf10b5339f370f9a54"

	t.Run("the native field is used as given", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"digest":"sha256:` + hex + `","modelfile":"FROM /home/someone/blobs/sha256-` + hex + `"}`))
		}))
		defer server.Close()
		got, err := (&Ollama{BaseURL: server.URL, Client: server.Client()}).
			Show(context.Background(), "qwen2.5:14b-instruct")
		if err != nil {
			t.Fatalf("showing: %v", err)
		}
		if got.Digest != "sha256:"+hex {
			t.Fatalf("digest = %q", got.Digest)
		}
	})

	t.Run("the modelfile fallback keeps the digest and not the path", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"digest":"","modelfile":"# generated\nFROM /Users/someone/.ollama/models/blobs/sha256-` + hex + `\nPARAMETER temperature 0"}`))
		}))
		defer server.Close()
		got, err := (&Ollama{BaseURL: server.URL, Client: server.Client()}).
			Show(context.Background(), "qwen2.5:14b-instruct")
		if err != nil {
			t.Fatalf("showing: %v", err)
		}
		if got.Digest != "sha256:"+hex {
			t.Fatalf("digest = %q, want the digest rather than the line holding it", got.Digest)
		}
		if strings.Contains(got.Digest, "/") || strings.Contains(got.Digest, "Users") {
			t.Fatalf("the digest carries a path from one machine: %q", got.Digest)
		}
		if got.Name != "qwen2.5:14b-instruct" {
			t.Fatalf("name = %q", got.Name)
		}
	})

	t.Run("nothing that is not a digest is reported as one", func(t *testing.T) {
		for _, modelfile := range []string{
			"FROM /blobs/sha256-tooshort",
			"# sha256 is mentioned here and nowhere useful",
			"FROM /blobs/sha256-" + hex + "extra",
			"",
		} {
			body, _ := json.Marshal(map[string]any{"digest": "", "modelfile": modelfile})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(body)
			}))
			got, err := (&Ollama{BaseURL: server.URL, Client: server.Client()}).
				Show(context.Background(), "m")
			server.Close()
			if err != nil {
				t.Fatalf("showing: %v", err)
			}
			if got.Digest != "" {
				t.Fatalf("modelfile %q produced digest %q", modelfile, got.Digest)
			}
		}
	})
}
