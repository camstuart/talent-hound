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

// The search client had no test. Its base URL is a field, so everything except
// the provider itself is reachable from here.

// The property the PRD's preview rests on: the recruiter confirms a specific
// string, and exactly that string is what leaves the machine.
//
// A client that appended a location, a synonym, or a site filter would make
// that confirmation about something else — and the recruiter would have
// approved a query nobody showed them.
func TestTheQueryIsSentExactlyAsConfirmed(t *testing.T) {
	const confirmed = `platform engineer "production Go" Melbourne -recruiter`
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		if got := r.Header.Get("x-api-key"); got != "not-a-real-key-EXA-4c19f7" {
			t.Errorf("the key header is %q", got)
		}
		_, _ = w.Write([]byte(`{"results":[],"nextCursor":""}`))
	}))
	defer server.Close()

	client := &Exa{BaseURL: server.URL, Client: server.Client(), Key: "not-a-real-key-EXA-4c19f7"}
	if _, err := client.Search(context.Background(), confirmed, 20, ""); err != nil {
		t.Fatalf("searching: %v", err)
	}
	if sent["query"] != confirmed {
		t.Fatalf("sent %q, the recruiter confirmed %q", sent["query"], confirmed)
	}
	// And listings only: the PRD puts company, profile and news searches out of
	// scope, and the category is how that is expressed on the wire.
	if sent["category"] != "job posting" {
		t.Fatalf("the search category is %q", sent["category"])
	}
}

// No key is not a request. A client that asked anyway would send the recruiter's
// query to a provider that will refuse it, which is a disclosure for nothing.
func TestNoKeyMeansNoRequestAtAll(t *testing.T) {
	asked := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		asked = true
	}))
	defer server.Close()

	client := &Exa{BaseURL: server.URL, Client: server.Client(), Key: "   "}
	_, err := client.Search(context.Background(), "anything", 20, "")
	if !errors.Is(err, ErrSearchUnauthorized) {
		t.Fatalf("err = %v, want unauthorized", err)
	}
	if asked {
		t.Fatal("the query was sent to a provider with no key to use it")
	}
}

// What the provider says back is never carried into an error, because it can
// quote the query — and the query is the thing being kept out of logs.
func TestTheProvidersOwnWordsNeverReachTheError(t *testing.T) {
	const leak = "invalid query: platform engineer Kalinda Reyes"
	for _, c := range []struct {
		status int
		want   error
	}{
		{http.StatusTooManyRequests, ErrSearchRateLimited},
		{http.StatusUnauthorized, ErrSearchUnauthorized},
		{http.StatusForbidden, ErrSearchUnauthorized},
		{http.StatusInternalServerError, ErrSearchMalformed},
		{http.StatusBadRequest, ErrSearchMalformed},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(leak))
		}))
		client := &Exa{BaseURL: server.URL, Client: server.Client(), Key: "k"}
		_, err := client.Search(context.Background(), "platform engineer", 20, "")
		if !errors.Is(err, c.want) {
			t.Errorf("status %d gave %v, want %v", c.status, err, c.want)
		}
		if err != nil && strings.Contains(err.Error(), "Kalinda") {
			t.Errorf("status %d quoted the provider back: %v", c.status, err)
		}
		server.Close()
	}
}

// A result with no identifier cannot become a role — it would be a new role on
// every search, forever — so it is counted rather than dropped silently.
func TestAResultWithNothingToIdentifyItIsCountedNotKept(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[
			{"id":"a","url":"https://example.invalid/1","title":" Platform engineer ","publishedDate":"2026-03-04T11:22:33Z"},
			{"id":"  ","url":"   ","title":"Nameless"},
			{"id":"","url":"https://example.invalid/2","title":"Backend","closingDate":"2026-04-01"}
		],"nextCursor":"next"}`))
	}))
	defer server.Close()

	client := &Exa{BaseURL: server.URL, Client: server.Client(), Key: "k"}
	got, err := client.Search(context.Background(), "q", 20, "")
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if got.Skipped != 1 {
		t.Fatalf("skipped %d, want the one with neither an id nor a url", got.Skipped)
	}
	if len(got.Results) != 2 {
		t.Fatalf("kept %d results, want 2", len(got.Results))
	}
	if got.Results[0].Title != "Platform engineer" {
		t.Fatalf("the title kept its whitespace: %q", got.Results[0].Title)
	}
	// A closing date is a day in the world, not an instant.
	if got.Results[0].PublishedOn != "2026-03-04" {
		t.Fatalf("published on %q", got.Results[0].PublishedOn)
	}
	if got.NextCursor != "next" {
		t.Fatalf("the cursor is %q", got.NextCursor)
	}
}

// Malformed JSON is malformed, not a panic and not an empty result set that
// reads as "the provider found nothing".
func TestMalformedJSONIsNotAnEmptyResultSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[`))
	}))
	defer server.Close()

	client := &Exa{BaseURL: server.URL, Client: server.Client(), Key: "k"}
	if _, err := client.Search(context.Background(), "q", 20, ""); !errors.Is(err, ErrSearchMalformed) {
		t.Fatalf("err = %v, want malformed", err)
	}
}

func TestADateIsKeptAsTheDayItNames(t *testing.T) {
	for in, want := range map[string]string{
		"2026-03-04T11:22:33Z": "2026-03-04",
		"2026-03-04":           "2026-03-04",
		"  2026-03-04  ":       "2026-03-04",
		"2026-03":              "",
		"":                     "",
		"soon":                 "",
	} {
		if got := dateOnly(in); got != want {
			t.Errorf("dateOnly(%q) = %q, want %q", in, got, want)
		}
	}
}

// A people search is the same request with one word changed. The query is
// still sent exactly as confirmed, and the category is the only thing that
// says what kind of page comes back.
func TestAPeopleSearchSendsTheQueryUnchangedWithThePeopleCategory(t *testing.T) {
	const confirmed = `platform engineer local-first desktop tools Melbourne`
	var sent map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &sent)
		_, _ = w.Write([]byte(`{"results":[{"id":"p1","url":"https://example.org/quokka","title":"Quokka","text":"Builds things."}],"nextCursor":""}`))
	}))
	defer server.Close()

	client := &Exa{BaseURL: server.URL, Client: server.Client(), Key: "not-a-real-key-EXA-4c19f7"}
	got, err := client.SearchPeople(context.Background(), confirmed, 20, "")
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if sent["query"] != confirmed {
		t.Fatalf("sent %q, the recruiter confirmed %q", sent["query"], confirmed)
	}
	if sent["category"] != "people" {
		t.Fatalf("the search category is %q", sent["category"])
	}
	if len(got.Results) != 1 || got.Results[0].URL != "https://example.org/quokka" {
		t.Fatalf("results = %+v", got.Results)
	}
}

// No key means no request for people either.
func TestNoKeyMeansNoPeopleRequestEither(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	client := &Exa{BaseURL: server.URL, Client: server.Client(), Key: " "}
	if _, err := client.SearchPeople(context.Background(), "anything", 20, ""); !errors.Is(err, ErrSearchUnauthorized) {
		t.Fatalf("err = %v", err)
	}
	if requests != 0 {
		t.Fatalf("%d requests were made", requests)
	}
}
