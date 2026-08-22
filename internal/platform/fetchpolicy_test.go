package platform

import (
	"strings"
	"testing"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// A denied host stays denied after somebody allowlists it.
//
// "SEEK, LinkedIn, authenticated pages, robots-disallowed paths, and anti-bot
// challenges SHALL never be fetched automatically, and SHALL remain refused
// even if their hosts were added to the allowlist."
//
// The existing tests run against the shipped allowlist, which is empty, so a
// denied host is refused by the deny list and by the fallthrough alike — they
// pass whichever order the two lists are consulted in. The order is the
// requirement: it is what makes the deny list permanent rather than a default
// somebody can review their way out of.
func TestAllowlistingADeniedHostDoesNotAllowIt(t *testing.T) {
	restore := fetchAllowlist
	t.Cleanup(func() { fetchAllowlist = restore })

	// The reviewer who decides SEEK is fine after all.
	fetchAllowlist = []string{"seek.com.au", "linkedin.com", "example.invalid"}

	for _, denied := range []string{
		"https://seek.com.au/job/12345",
		"https://www.seek.com.au/job/12345",
		"https://linkedin.com/jobs/view/1",
		"https://au.linkedin.com/jobs/view/1",
	} {
		err := FetchAllowed(denied)
		if err == nil {
			t.Errorf("%s was fetched because somebody allowlisted it", denied)
			continue
		}
		if !strings.Contains(err.Error(), "never fetched automatically") {
			t.Errorf("%s was refused for the wrong reason: %v", denied, err)
		}
	}

	// And the allowlist still works for a host that is not denied, so this is
	// testing precedence rather than a broken allowlist.
	if err := FetchAllowed("https://example.invalid/jobs/1"); err != nil {
		t.Fatalf("an allowlisted host was refused: %v", err)
	}
}
