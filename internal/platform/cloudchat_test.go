package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

const inventedCloudKey = "not-a-real-key-CLOUD-7f31a2"

func TestTheCloudRequestCarriesTheCredentialAndThePromptAndNothingElse(t *testing.T) {
	var (
		path  string
		auth  string
		sent  map[string]any
		gotCT string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth, gotCT = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"a pitch"}}]}`))
	}))
	defer server.Close()

	client := NewCloud(server.URL+"/", inventedCloudKey)
	client.Client = server.Client()
	answer, err := client.Chat(context.Background(), "cloud-model", "Write a pitch.", nil)
	if err != nil {
		t.Fatalf("sending: %v", err)
	}
	if answer != "a pitch" {
		t.Fatalf("answer = %q", answer)
	}
	if path != "/v1/chat/completions" {
		t.Fatalf("posted to %q", path)
	}
	if auth != "Bearer "+inventedCloudKey {
		t.Fatalf("authorization = %q", auth)
	}
	if gotCT != "application/json" {
		t.Fatalf("content type = %q", gotCT)
	}
	if sent["model"] != "cloud-model" {
		t.Fatalf("model = %v", sent["model"])
	}
	// The prompt is sent as given: the recruiter approved a preview of it.
	messages, _ := sent["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("sent %d messages", len(messages))
	}
	first, _ := messages[0].(map[string]any)
	if first["content"] != "Write a pitch." {
		t.Fatalf("content = %v", first["content"])
	}
	// A trailing slash on the configured endpoint does not become a double one.
	if strings.Contains(client.Endpoint(), "//v1") {
		t.Fatalf("endpoint = %q", client.Endpoint())
	}
}

// A schema is refused by the task boundary long before this, and honouring one
// here would make this look like a place structured extraction may go.
func TestASchemaIsIgnoredRatherThanHonoured(t *testing.T) {
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	client := NewCloud(server.URL, inventedCloudKey)
	client.Client = server.Client()
	if _, err := client.Chat(context.Background(), "m", "p",
		map[string]any{"type": "object"}); err != nil {
		t.Fatalf("sending: %v", err)
	}
	if _, present := sent["response_format"]; present {
		t.Fatal("a schema reached the provider")
	}
}

// No credential is no request: sending the prompt to a provider that will
// refuse it is a disclosure for nothing.
func TestNoCloudCredentialMeansNoRequest(t *testing.T) {
	asked := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		asked = true
	}))
	defer server.Close()

	client := NewCloud(server.URL, "   ")
	client.Client = server.Client()
	if _, err := client.Chat(context.Background(), "m", "p", nil); !errors.Is(err, ErrCloudUnauthorized) {
		t.Fatalf("err = %v, want unauthorized", err)
	}
	if asked {
		t.Fatal("the prompt was sent to a provider with no credential to use it")
	}
}

// The provider's own words never reach an error: they can quote the prompt.
func TestTheCloudProvidersWordsNeverReachTheError(t *testing.T) {
	const leak = "invalid request: Write a pitch about Kalinda Reyes"
	for _, c := range []struct {
		status int
		want   error
	}{
		{http.StatusTooManyRequests, ErrCloudRateLimited},
		{http.StatusUnauthorized, ErrCloudUnauthorized},
		{http.StatusForbidden, ErrCloudUnauthorized},
		{http.StatusBadRequest, ErrCloudMalformed},
		{http.StatusInternalServerError, ErrCloudMalformed},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(leak))
		}))
		client := NewCloud(server.URL, inventedCloudKey)
		client.Client = server.Client()
		_, err := client.Chat(context.Background(), "m", "Write a pitch.", nil)
		server.Close()
		if err == nil {
			t.Errorf("status %d was accepted", c.status)
			continue
		}
		if !strings.Contains(err.Error(), c.want.Error()) {
			t.Errorf("status %d gave %v, want %v", c.status, err, c.want)
		}
		if strings.Contains(err.Error(), "Kalinda") {
			t.Errorf("status %d quoted the provider: %v", c.status, err)
		}
		// And never the credential.
		if strings.Contains(err.Error(), inventedCloudKey) {
			t.Errorf("status %d carried the credential: %v", c.status, err)
		}
	}
}

// An answer with no choices is malformed, not an empty draft the recruiter
// might mistake for the model having nothing to say.
func TestAnAnswerWithNoChoicesIsMalformed(t *testing.T) {
	for _, body := range []string{`{"choices":[]}`, `{}`, `{"choices":`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		client := NewCloud(server.URL, inventedCloudKey)
		client.Client = server.Client()
		_, err := client.Chat(context.Background(), "m", "p", nil)
		server.Close()
		if !errors.Is(err, ErrCloudMalformed) {
			t.Errorf("body %q gave %v, want malformed", body, err)
		}
	}
}
