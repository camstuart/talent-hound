package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every fixture here is invented. No real person appears in these tests.

type memoryETags struct {
	entries  map[string][2]string
	puts     int
	putCalls []string
}

func newMemoryETags() *memoryETags { return &memoryETags{entries: map[string][2]string{}} }

func (m *memoryETags) Get(url string) (string, []byte, bool) {
	e, ok := m.entries[url]
	if !ok {
		return "", nil, false
	}
	return e[0], []byte(e[1]), true
}

func (m *memoryETags) Put(url, etag string, body []byte) error {
	m.puts++
	m.putCalls = append(m.putCalls, url)
	m.entries[url] = [2]string{etag, string(body)}
	return nil
}

const profileJSON = `{"login":"wombatdev","name":"Wombat Developer","company":"@quokkastack","location":"Melbourne",` +
	`"bio":"Local-first tools.","blog":"https://wombat.example.invalid","hireable":true,"public_repos":4,` +
	`"followers":12,"created_at":"2016-03-04T05:06:07Z","updated_at":"2026-08-01T00:00:00Z","email":"wombat@example.invalid"}`

func gitHubServer(t *testing.T, handler http.HandlerFunc) (*GitHub, *memoryETags, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	cache := newMemoryETags()
	return &GitHub{BaseURL: server.URL, Client: server.Client(), Token: "not-a-real-token-GH-7f2a", Cache: cache}, cache, server
}

func TestAProfileIsReadWithTheTokenAndWithoutTheEmail(t *testing.T) {
	var auth string
	client, _, _ := gitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/users/wombatdev" {
			t.Errorf("asked for %s", r.URL.Path)
		}
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(profileJSON))
	})
	p, err := client.Profile(context.Background(), "wombatdev")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if auth != "Bearer not-a-real-token-GH-7f2a" {
		t.Fatalf("authorization = %q", auth)
	}
	if p.Name != "Wombat Developer" || p.Location != "Melbourne" || p.CreatedOn != "2016-03-04" || !p.Hireable {
		t.Fatalf("profile = %+v", p)
	}
}

func TestNoTokenMeansNoRequestAtAll(t *testing.T) {
	requests := 0
	client, _, _ := gitHubServer(t, func(http.ResponseWriter, *http.Request) { requests++ })
	client.Token = " "
	if _, err := client.Profile(context.Background(), "wombatdev"); !errors.Is(err, ErrSearchUnauthorized) {
		t.Fatalf("err = %v", err)
	}
	if requests != 0 {
		t.Fatalf("%d requests were made without a token", requests)
	}
}

func TestARepeatIsConditionalAndAnswersFromTheCache(t *testing.T) {
	served := 0
	client, cache, _ := gitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		served++
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(profileJSON))
	})
	first, err := client.Profile(context.Background(), "wombatdev")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := client.Profile(context.Background(), "wombatdev")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.Name != second.Name || second.Name == "" {
		t.Fatalf("the cached answer differs: %+v vs %+v", first, second)
	}
	if served != 2 || cache.puts != 1 {
		t.Fatalf("served %d, cached %d", served, cache.puts)
	}
}

func TestForksAreDroppedAndReposAreNewestFirst(t *testing.T) {
	client, _, _ := gitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/users/wombatdev/repos") {
			t.Errorf("asked for %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"name":"old","language":"Go","html_url":"https://github.com/wombatdev/old","stargazers_count":3,"fork":false,"pushed_at":"2024-01-01T00:00:00Z"},
			{"name":"copied","language":"Rust","html_url":"https://github.com/wombatdev/copied","stargazers_count":900,"fork":true,"pushed_at":"2026-01-01T00:00:00Z"},
			{"name":"new","language":"Go","html_url":"https://github.com/wombatdev/new","stargazers_count":10,"fork":false,"pushed_at":"2026-08-01T00:00:00Z"}
		]`))
	})
	repos, err := client.Repos(context.Background(), "wombatdev")
	if err != nil {
		t.Fatalf("repos: %v", err)
	}
	if len(repos) != 2 || repos[0].Name != "new" || repos[1].Name != "old" {
		t.Fatalf("repos = %+v", repos)
	}
}

func TestProviderFailuresAreTypedAndNeverQuoted(t *testing.T) {
	const leak = `{"message":"Not Found for wombatdev"}`
	for _, c := range []struct {
		status    int
		remaining string
		want      error
	}{
		{http.StatusTooManyRequests, "", ErrSearchRateLimited},
		{http.StatusForbidden, "0", ErrSearchRateLimited},
		{http.StatusForbidden, "", ErrSearchUnauthorized},
		{http.StatusUnauthorized, "", ErrSearchUnauthorized},
		{http.StatusNotFound, "", ErrSearchMalformed},
		{http.StatusInternalServerError, "", ErrSearchMalformed},
	} {
		client, _, _ := gitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			if c.remaining != "" {
				w.Header().Set("X-RateLimit-Remaining", c.remaining)
				w.Header().Set("X-RateLimit-Reset", "1767225600")
			}
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(leak))
		})
		_, err := client.Events(context.Background(), "wombatdev")
		if !errors.Is(err, c.want) {
			t.Errorf("status %d/%q gave %v, want %v", c.status, c.remaining, err, c.want)
		}
		if err != nil && strings.Contains(err.Error(), "wombatdev") {
			t.Errorf("status %d quoted the provider back: %v", c.status, err)
		}
		if errors.Is(err, ErrSearchRateLimited) && c.remaining != "" && !strings.Contains(err.Error(), "UTC") {
			t.Errorf("a rate limit does not say when it resets: %v", err)
		}
	}
}

func TestTheGitHubClientIsNotAPageFetcher(t *testing.T) {
	// The page-fetch policy is unchanged by this client existing.
	if allow := FetchAllowlist(); len(allow) != 0 {
		t.Fatalf("the shipped allowlist is not empty: %v", allow)
	}
	if err := FetchAllowed("https://github.com/wombatdev"); err == nil {
		t.Fatal("a profile page was fetchable")
	}
}
