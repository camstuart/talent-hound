package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OllamaBaseURL is the required local endpoint from the PRD.
const OllamaBaseURL = "http://localhost:11434"

// The ways a model endpoint fails. They are separate errors rather than one
// because the caller turns them into separate states, and the recruiter's next
// action differs for each: start Ollama, wait, pull the model, close something,
// report a bug.
var (
	ErrEndpointUnavailable = errors.New("the local model endpoint is not reachable")
	ErrEndpointTimeout     = errors.New("the local model endpoint did not answer in time")
	ErrMalformedResponse   = errors.New("the local model endpoint returned an unexpected shape")
	ErrModelMemory         = errors.New("the model could not be loaded in the memory available")
	ErrModelNotFound       = errors.New("the model is not installed at the local endpoint")
)

// Ollama talks the OpenAI-compatible surface at /v1 plus Ollama's own /api/show
// for model identity. ponytail: net/http + encoding/json, no SDK — three calls.
type Ollama struct {
	BaseURL string
	Client  *http.Client
}

// NewOllama returns a client for the required local endpoint.
func NewOllama() *Ollama {
	return &Ollama{BaseURL: OllamaBaseURL, Client: &http.Client{Timeout: 5 * time.Minute}}
}

// Chat returns the assistant message for a single user prompt. When schema is
// non-nil the response is constrained to it and returned as raw JSON.
func (o *Ollama) Chat(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	body := map[string]any{
		"model":    model,
		"stream":   false,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	if schema != nil {
		body["response_format"] = map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "gate",
				"strict": true,
				"schema": schema,
			},
		}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := o.post(ctx, "/v1/chat/completions", body, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("model %s returned no content", model)
	}
	return out.Choices[0].Message.Content, nil
}

// Embed returns the embedding vector for text.
func (o *Ollama) Embed(ctx context.Context, model, text string) ([]float32, error) {
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	body := map[string]any{"model": model, "input": text}
	if err := o.post(ctx, "/v1/embeddings", body, &out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("model %s returned no embedding", model)
	}
	return out.Data[0].Embedding, nil
}

// ModelInfo is the identity recorded alongside every embedding space.
type ModelInfo struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// Show reports the model's immutable digest via Ollama's native API.
func (o *Ollama) Show(ctx context.Context, model string) (ModelInfo, error) {
	var out struct {
		Details struct {
			ParentModel string `json:"parent_model"`
		} `json:"details"`
		Digest    string `json:"digest"`
		Modelfile string `json:"modelfile"`
	}
	if err := o.post(ctx, "/api/show", map[string]any{"model": model}, &out); err != nil {
		return ModelInfo{}, err
	}
	digest := out.Digest
	if digest == "" {
		// Older builds report the digest only inside the modelfile header.
		for _, line := range strings.Split(out.Modelfile, "\n") {
			if strings.Contains(line, "sha256") {
				digest = strings.TrimSpace(line)
				break
			}
		}
	}
	return ModelInfo{Name: model, Digest: digest}, nil
}

// LoadedBytes reports how much memory Ollama currently holds for model (total
// and VRAM), per /api/ps. Zero means the model is not resident.
func (o *Ollama) LoadedBytes(ctx context.Context, model string) (total, vram int64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"/api/ps", nil)
	if err != nil {
		return 0, 0, fmt.Errorf("building /api/ps request: %w", err)
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("calling /api/ps: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Models []struct {
			Name     string `json:"name"`
			Model    string `json:"model"`
			Size     int64  `json:"size"`
			SizeVRAM int64  `json:"size_vram"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return 0, 0, fmt.Errorf("decoding /api/ps: %w", err)
	}
	for _, m := range out.Models {
		if m.Name == model || m.Model == model {
			return m.Size, m.SizeVRAM, nil
		}
	}
	return 0, 0, nil
}

func (o *Ollama) post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", path, transportError(ctx, err))
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("reading %s response: %w", path, transportError(ctx, err))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %s: %w", path, resp.Status, statusError(payload))
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decoding %s response: %w", path, ErrMalformedResponse)
	}
	return nil
}

// transportError names what went wrong at the connection, so the caller does
// not have to read Go's error strings to tell "not running" from "not
// answering".
func transportError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) ||
		os.IsTimeout(err) {
		return ErrEndpointTimeout
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	return ErrEndpointUnavailable
}

// statusError classifies a non-200 answer. The body is read for the two things
// the endpoint says that the caller must act on differently, and is otherwise
// discarded: a model endpoint's error text can quote the prompt, and the prompt
// is the one thing that must not end up in a stored state or a log.
func statusError(payload []byte) error {
	body := strings.ToLower(string(payload))
	switch {
	case strings.Contains(body, "memory") || strings.Contains(body, "out of vram"):
		return ErrModelMemory
	case strings.Contains(body, "not found") || strings.Contains(body, "no such model") ||
		strings.Contains(body, "try pulling"):
		return ErrModelNotFound
	}
	return ErrMalformedResponse
}

// Tags lists the models installed at the endpoint.
func (o *Ollama) Tags(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("building /api/tags request: %w", err)
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling /api/tags: %w", transportError(ctx, err))
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("reading /api/tags: %w", transportError(ctx, err))
	}
	if resp.StatusCode != http.StatusOK {
		// Classified the same way every other answer is: an endpoint that is up
		// but out of memory must not read as an endpoint that is talking
		// nonsense, because the recruiter does something different about it.
		return nil, fmt.Errorf("/api/tags returned %s: %w", resp.Status, statusError(payload))
	}
	var out struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("decoding /api/tags: %w", ErrMalformedResponse)
	}
	names := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		if m.Name != "" {
			names = append(names, m.Name)
		}
		if m.Model != "" && m.Model != m.Name {
			names = append(names, m.Model)
		}
	}
	return names, nil
}

// Pull downloads a model. It is minutes of network transfer, so the caller runs
// it as a background job rather than inline.
func (o *Ollama) Pull(ctx context.Context, model string) error {
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := o.post(ctx, "/api/pull", map[string]any{"model": model, "stream": false}, &out); err != nil {
		return err
	}
	if out.Error != "" {
		// The endpoint's own words are not carried: they are the endpoint's,
		// and the caller stores a code.
		return fmt.Errorf("pulling %s: %w", model, ErrModelNotFound)
	}
	return nil
}

// HasModel reports whether the endpoint has model installed. Ollama's tags
// carry the ":latest" suffix that a bare model name omits.
func HasModel(installed []string, model string) bool {
	for _, name := range installed {
		if name == model || strings.TrimSuffix(name, ":latest") == strings.TrimSuffix(model, ":latest") {
			return true
		}
	}
	return false
}
