//go:build livemodel

package platform_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"camstuart/talent-hound/internal/platform"
)

// Models under evaluation. Override per run:
//
//	TH_INSTRUCT_MODELS=qwen2.5:7b,llama3.1:8b TH_EMBED_MODELS=nomic-embed-text
func models(t *testing.T, env, fallback string) []string {
	t.Helper()
	v := os.Getenv(env)
	if v == "" {
		v = fallback
	}
	var out []string
	for _, m := range splitComma(v) {
		if m != "" {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s is empty", env)
	}
	return out
}

func splitComma(s string) []string {
	var out, cur = []string{}, ""
	for _, r := range s {
		if r == ',' {
			out, cur = append(out, cur), ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

func TestGateOllamaChat(t *testing.T) {
	o := platform.NewOllama()
	for _, m := range models(t, "TH_INSTRUCT_MODELS", "qwen2.5:7b-instruct") {
		t.Run(m, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			start := time.Now()
			got, err := o.Chat(ctx, m, "Reply with the single word: ready.", nil)
			if err != nil {
				t.Fatalf("chat: %v", err)
			}
			info, err := o.Show(ctx, m)
			if err != nil {
				t.Fatalf("show: %v", err)
			}
			size, vram, err := o.LoadedBytes(ctx, m)
			if err != nil {
				t.Fatalf("ps: %v", err)
			}
			t.Logf("EVIDENCE chat model=%s digest=%s wall=%s resident=%dB vram=%dB reply=%q",
				m, info.Digest, time.Since(start), size, vram, got)
		})
	}
}

func TestGateOllamaConstrainedJSON(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"years": map[string]any{"type": "integer"},
		},
		"required":             []string{"title", "years"},
		"additionalProperties": false,
	}
	o := platform.NewOllama()
	for _, m := range models(t, "TH_INSTRUCT_MODELS", "qwen2.5:7b-instruct") {
		t.Run(m, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			start := time.Now()
			raw, err := o.Chat(ctx,
				m,
				"Extract the job title and years of experience: "+
					"\"Senior Platform Engineer with 9 years of experience.\"",
				schema)
			if err != nil {
				t.Fatalf("constrained chat: %v", err)
			}
			var out struct {
				Title string `json:"title"`
				Years int    `json:"years"`
			}
			if err := json.Unmarshal([]byte(raw), &out); err != nil {
				t.Fatalf("response is not valid JSON (%v): %s", err, raw)
			}
			if out.Title == "" || out.Years == 0 {
				t.Fatalf("response missing required fields: %s", raw)
			}
			t.Logf("EVIDENCE constrained-json model=%s wall=%s out=%s", m, time.Since(start), raw)
		})
	}
}

func TestGateOllamaEmbeddings(t *testing.T) {
	o := platform.NewOllama()
	for _, m := range models(t, "TH_EMBED_MODELS", "nomic-embed-text") {
		t.Run(m, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			start := time.Now()
			vec, err := o.Embed(ctx, m, "Senior Platform Engineer, Go, SQLite, Windows desktop.")
			if err != nil {
				t.Fatalf("embed: %v", err)
			}
			info, err := o.Show(ctx, m)
			if err != nil {
				t.Fatalf("show: %v", err)
			}
			size, vram, err := o.LoadedBytes(ctx, m)
			if err != nil {
				t.Fatalf("ps: %v", err)
			}
			t.Logf("EVIDENCE embeddings model=%s digest=%s dims=%d wall=%s resident=%dB vram=%dB",
				m, info.Digest, len(vec), time.Since(start), size, vram)
		})
	}
}
