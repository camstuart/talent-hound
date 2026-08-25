package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// GitHubBaseURL is the provider's API endpoint.
const GitHubBaseURL = "https://api.github.com"

// ETagCache remembers a provider's answer so a repeat costs no quota: GitHub
// answers a matching If-None-Match with 304 and does not count it.
type ETagCache interface {
	Get(url string) (etag string, body []byte, ok bool)
	Put(url, etag string, body []byte) error
}

// GitHubProfile is what a public profile says about a person. Email is
// deliberately absent: the profile endpoint returns one when the user chose to
// show it, and this application does not collect it.
type GitHubProfile struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	Bio         string `json:"bio"`
	Blog        string `json:"blog"`
	Hireable    bool   `json:"hireable"`
	PublicRepos int    `json:"publicRepos"`
	Followers   int    `json:"followers"`
	CreatedOn   string `json:"createdOn"`
	UpdatedOn   string `json:"updatedOn"`
}

// GitHubRepo is one repository a person owns.
type GitHubRepo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	URL         string `json:"url"`
	Stars       int    `json:"stars"`
	PushedOn    string `json:"pushedOn"`
}

// GitHubEvent is one public action: a push, a pull request, a review.
type GitHubEvent struct {
	Type      string `json:"type"`
	Repo      string `json:"repo"`
	CreatedOn string `json:"createdOn"`
}

// GitHub reads a person's public footprint.
//
// Like Exa it is a provider with a fixed endpoint, not a page fetcher: the
// deny-by-default policy on fetching pages is untouched, because this client
// can only ask the API about a login it was given.
type GitHub struct {
	BaseURL string
	Client  *http.Client
	// Token is read at call time from wherever the caller keeps it — this
	// client never stores or logs it.
	Token string
	Cache ETagCache
}

// NewGitHub returns a client for the provider.
func NewGitHub(token string, cache ETagCache) *GitHub {
	return &GitHub{
		BaseURL: GitHubBaseURL,
		Client:  &http.Client{Timeout: 30 * time.Second},
		Token:   token,
		Cache:   cache,
	}
}

// maxRepos bounds what one enrichment keeps: the most recently pushed.
const maxRepos = 50

// Profile reads the public profile of one login.
func (g *GitHub) Profile(ctx context.Context, login string) (*GitHubProfile, error) {
	var raw struct {
		Login       string `json:"login"`
		Name        string `json:"name"`
		Company     string `json:"company"`
		Location    string `json:"location"`
		Bio         string `json:"bio"`
		Blog        string `json:"blog"`
		Hireable    bool   `json:"hireable"`
		PublicRepos int    `json:"public_repos"`
		Followers   int    `json:"followers"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	if err := g.get(ctx, "/users/"+login, &raw); err != nil {
		return nil, err
	}
	return &GitHubProfile{
		Login: raw.Login, Name: raw.Name, Company: raw.Company, Location: raw.Location,
		Bio: raw.Bio, Blog: raw.Blog, Hireable: raw.Hireable,
		PublicRepos: raw.PublicRepos, Followers: raw.Followers,
		CreatedOn: dateOnly(raw.CreatedAt), UpdatedOn: dateOnly(raw.UpdatedAt),
	}, nil
}

// Repos lists the repositories a login owns, most recently pushed first.
// Forks are dropped: they say what someone looked at, not what they built.
func (g *GitHub) Repos(ctx context.Context, login string) ([]GitHubRepo, error) {
	var raw []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Language    string `json:"language"`
		HTMLURL     string `json:"html_url"`
		Stars       int    `json:"stargazers_count"`
		Fork        bool   `json:"fork"`
		PushedAt    string `json:"pushed_at"`
	}
	if err := g.get(ctx, "/users/"+login+"/repos?type=owner&sort=pushed&per_page=100", &raw); err != nil {
		return nil, err
	}
	out := []GitHubRepo{}
	for _, r := range raw {
		if r.Fork {
			continue
		}
		out = append(out, GitHubRepo{
			Name: r.Name, Description: r.Description, Language: r.Language,
			URL: r.HTMLURL, Stars: r.Stars, PushedOn: dateOnly(r.PushedAt),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PushedOn > out[j].PushedOn })
	if len(out) > maxRepos {
		out = out[:maxRepos]
	}
	return out, nil
}

// Events lists a login's recent public activity.
func (g *GitHub) Events(ctx context.Context, login string) ([]GitHubEvent, error) {
	var raw []struct {
		Type string `json:"type"`
		Repo struct {
			Name string `json:"name"`
		} `json:"repo"`
		CreatedAt string `json:"created_at"`
	}
	if err := g.get(ctx, "/users/"+login+"/events/public?per_page=100", &raw); err != nil {
		return nil, err
	}
	out := []GitHubEvent{}
	for _, e := range raw {
		out = append(out, GitHubEvent{Type: e.Type, Repo: e.Repo.Name, CreatedOn: dateOnly(e.CreatedAt)})
	}
	return out, nil
}

// get performs one conditional request and decodes the answer into out.
func (g *GitHub) get(ctx context.Context, path string, out any) error {
	if strings.TrimSpace(g.Token) == "" {
		return ErrSearchUnauthorized
	}
	url := g.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+g.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	var cachedETag string
	var cachedBody []byte
	if g.Cache != nil {
		if etag, body, ok := g.Cache.Get(url); ok {
			cachedETag, cachedBody = etag, body
			req.Header.Set("If-None-Match", etag)
		}
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrSearchTimeout
		}
		return ErrSearchOffline
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return ErrSearchOffline
	}
	switch {
	case resp.StatusCode == http.StatusNotModified && cachedETag != "":
		payload = cachedBody
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return fmt.Errorf("%w until %s", ErrSearchRateLimited, resetTime(resp.Header.Get("X-RateLimit-Reset")))
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ErrSearchUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: no such login", ErrSearchMalformed)
	case resp.StatusCode != http.StatusOK:
		// The provider's own words are not carried: they can quote the login.
		return fmt.Errorf("the provider returned %s: %w", resp.Status, ErrSearchMalformed)
	default:
		if g.Cache != nil {
			if etag := resp.Header.Get("ETag"); etag != "" {
				_ = g.Cache.Put(url, etag, payload)
			}
		}
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return ErrSearchMalformed
	}
	return nil
}

// resetTime renders a rate-limit reset epoch as a clock time.
func resetTime(epoch string) string {
	n, err := strconv.ParseInt(strings.TrimSpace(epoch), 10, 64)
	if err != nil || n == 0 {
		return "later"
	}
	return time.Unix(n, 0).UTC().Format("15:04 UTC")
}
