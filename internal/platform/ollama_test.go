package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
