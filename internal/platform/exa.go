package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ExaBaseURL is the provider's search endpoint.
const ExaBaseURL = "https://api.exa.ai"

// The ways a search fails. They are separate because the recruiter's next
// action differs for each: wait, retry, reconnect, or report a bug.
var (
	ErrSearchRateLimited  = errors.New("the search provider is rate limiting this key")
	ErrSearchTimeout      = errors.New("the search provider did not answer in time")
	ErrSearchOffline      = errors.New("the search provider could not be reached")
	ErrSearchUnauthorized = errors.New("the search provider rejected the key")
	ErrSearchMalformed    = errors.New("the search provider returned an unexpected shape")
	// ErrFetchNotAllowed is the deny-by-default refusal. It is not an error
	// condition — it is the policy working.
	ErrFetchNotAllowed = errors.New("direct fetching is not permitted for this source")
)

// SearchResult is one listing the provider returned.
type SearchResult struct {
	// SourceID is the provider's own identifier, when it gives one. It is the
	// first thing role identity resolves on.
	SourceID string
	// URL is the canonical address of the listing.
	URL   string
	Title string
	// Text is the listing content the provider supplied, which may be empty or
	// too thin to profile — in which case the recruiter pastes it.
	Text string
	// PublishedOn and ClosingOn are YYYY-MM-DD when stated, empty otherwise.
	PublishedOn string
	ClosingOn   string
}

// SearchResponse is one page of results and whether anything was dropped.
type SearchResponse struct {
	Results []SearchResult
	// NextCursor is empty when there are no more pages.
	NextCursor string
	// Skipped counts records that could not be read. A partial response is
	// reported as partial rather than presented as complete.
	Skipped int
}

// Exa is the role-listing search provider.
//
// It searches listings only: company, profile, and news searches are out of
// scope, and the client has no method for them.
type Exa struct {
	BaseURL string
	Client  *http.Client
	// Key is read at call time from wherever the caller keeps it — this client
	// never stores or logs it.
	Key string
}

// NewExa returns a client for the provider.
func NewExa(key string) *Exa {
	return &Exa{
		BaseURL: ExaBaseURL,
		Client:  &http.Client{Timeout: 30 * time.Second},
		Key:     key,
	}
}

// Search sends exactly the query it is given.
//
// Nothing is appended, templated, or rewritten here. The recruiter confirmed a
// specific string, and a client that decorated it would make that confirmation
// about something that was not sent.
func (e *Exa) Search(ctx context.Context, query string, limit int, cursor string) (*SearchResponse, error) {
	if strings.TrimSpace(e.Key) == "" {
		return nil, ErrSearchUnauthorized
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	body := map[string]any{
		"query":      query,
		"numResults": limit,
		"category":   "job posting",
		"contents":   map[string]any{"text": true},
	}
	if cursor != "" {
		body["cursor"] = cursor
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding the search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.BaseURL+"/search", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("building the search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.Key)

	resp, err := e.Client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, ErrSearchTimeout
		}
		return nil, ErrSearchOffline
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, ErrSearchOffline
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrSearchRateLimited
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrSearchUnauthorized
	case resp.StatusCode != http.StatusOK:
		// The provider's own words are not carried: they can quote the query,
		// and the query is the thing being kept out of logs.
		return nil, fmt.Errorf("the search provider returned %s: %w", resp.Status, ErrSearchMalformed)
	}

	var out struct {
		Results []struct {
			ID            string `json:"id"`
			URL           string `json:"url"`
			Title         string `json:"title"`
			Text          string `json:"text"`
			PublishedDate string `json:"publishedDate"`
			ClosingDate   string `json:"closingDate"`
		} `json:"results"`
		NextCursor string `json:"nextCursor"`
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, ErrSearchMalformed
	}

	got := &SearchResponse{NextCursor: out.NextCursor}
	for _, r := range out.Results {
		// A record with no way to identify it cannot become a role: it would
		// be a new role on every search, forever.
		if strings.TrimSpace(r.URL) == "" && strings.TrimSpace(r.ID) == "" {
			got.Skipped++
			continue
		}
		got.Results = append(got.Results, SearchResult{
			SourceID:    strings.TrimSpace(r.ID),
			URL:         strings.TrimSpace(r.URL),
			Title:       strings.TrimSpace(r.Title),
			Text:        r.Text,
			PublishedOn: dateOnly(r.PublishedDate),
			ClosingOn:   dateOnly(r.ClosingDate),
		})
	}
	return got, nil
}

// dateOnly keeps the YYYY-MM-DD prefix of whatever the provider sent, because a
// closing date is a day in the world rather than an instant.
func dateOnly(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10]
	}
	return ""
}

// fetchAllowlist is the developer-maintained list of sources whose access rules
// have been reviewed.
//
// It is empty, and that is the shipped state: the PRD requires a review before
// a source goes on it, and no review has happened. It exists rather than being
// absent so the rule is written down with a test beside it — an absent
// mechanism gets added later by someone who does not know the rule.
var fetchAllowlist = []string{}

// fetchDenylist is permanent. These are never fetched automatically, and adding
// them to the allowlist does not change that.
var fetchDenylist = []string{
	"seek.com.au", "seek.co.nz", "seek.com",
	"linkedin.com", "www.linkedin.com",
	"facebook.com", "instagram.com", "x.com", "twitter.com",
}

// FetchAllowed reports whether a URL may be fetched directly.
//
// There is no parameter that changes the answer. Deny-by-default is the policy,
// and a policy with an override is a default.
func FetchAllowed(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return fmt.Errorf("%w: %q is not a URL", ErrFetchNotAllowed, rawURL)
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: only http and https are considered", ErrFetchNotAllowed)
	}
	// Denied first, so allowlisting cannot enable one of these.
	for _, denied := range fetchDenylist {
		if host == denied || strings.HasSuffix(host, "."+denied) {
			return fmt.Errorf("%w: %s is never fetched automatically — open it yourself and paste the listing",
				ErrFetchNotAllowed, host)
		}
	}
	for _, allowed := range fetchAllowlist {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: %s has not completed an access review — paste the listing instead",
		ErrFetchNotAllowed, host)
}

// FetchAllowlist returns the allowlist, so a test can assert it is empty and a
// screen can say what is on it.
func FetchAllowlist() []string {
	return append([]string(nil), fetchAllowlist...)
}
