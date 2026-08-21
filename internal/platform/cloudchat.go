package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// The failures a cloud send is reduced to. The provider's own words are never
// carried: they can quote the prompt, and the prompt is the thing being kept
// out of logs and records.
var (
	ErrCloudUnauthorized = errors.New("the cloud provider refused the credential")
	ErrCloudRateLimited  = errors.New("the cloud provider is rate limiting")
	ErrCloudTimeout      = errors.New("the cloud provider did not answer in time")
	ErrCloudOffline      = errors.New("the cloud provider could not be reached")
	ErrCloudMalformed    = errors.New("the cloud provider's answer could not be read")
)

// Cloud is the optional OpenAI-compatible endpoint.
//
// A separate type from Ollama rather than the same one with a different address.
// They speak the same protocol and mean opposite things: one is a local process
// and the other is somebody else's computer, and a single type pointed at either
// is one assignment away from sending a résumé to a company by accident. The
// distinction is in the type so that it survives refactoring.
type Cloud struct {
	BaseURL string
	Client  *http.Client
	// Key is supplied per request by the caller that read it from the operating
	// system's credential store. This type never stores or logs it.
	Key string
}

// NewCloud returns a client for one endpoint, holding the credential only for
// as long as the caller holds this value.
func NewCloud(baseURL, key string) *Cloud {
	return &Cloud{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Client:  &http.Client{Timeout: 2 * time.Minute},
		Key:     key,
	}
}

// Endpoint is where this client sends, so a caller that must reach a particular
// one can check rather than assume.
func (c *Cloud) Endpoint() string { return c.BaseURL }

// Chat sends one prompt and returns the answer.
//
// The signature matches the local runtime's so the two are interchangeable
// where a caller legitimately holds either — and only there. The schema
// argument is accepted and ignored: a structured extraction is never a cloud
// task, the task boundary refuses those before this is reached, and quietly
// honouring it here would make this look like somewhere they could go.
func (c *Cloud) Chat(ctx context.Context, model, prompt string, _ map[string]any) (string, error) {
	if strings.TrimSpace(c.Key) == "" {
		return "", ErrCloudUnauthorized
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return "", ErrCloudOffline
	}
	raw, err := json.Marshal(map[string]any{
		"model":    model,
		"stream":   false,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	})
	if err != nil {
		return "", fmt.Errorf("encoding the request: %w", ErrCloudMalformed)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", ErrCloudOffline
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)

	resp, err := c.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", ErrCloudTimeout
		}
		return "", ErrCloudOffline
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", ErrCloudOffline
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return "", ErrCloudRateLimited
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", ErrCloudUnauthorized
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("the cloud provider returned %s: %w", resp.Status, ErrCloudMalformed)
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return "", ErrCloudMalformed
	}
	if len(out.Choices) == 0 {
		return "", ErrCloudMalformed
	}
	return out.Choices[0].Message.Content, nil
}
