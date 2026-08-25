package models

import (
	"strings"
	"testing"
)

func validLead() Lead {
	return Lead{
		SearchID:     1,
		InitiativeID: 1,
		Provider:     ProviderExa,
		URL:          "https://example.org/people/quokka",
		Title:        "Quokka — platform engineer",
		Snippet:      "Builds local-first desktop tools.",
		State:        LeadNew,
	}
}

func TestLeadValidation(t *testing.T) {
	l := validLead()
	if err := l.Validate(); err != nil {
		t.Fatalf("valid lead refused: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*Lead)
		want string
	}{
		{"missing url", func(l *Lead) { l.URL = " " }, "URL"},
		{"relative url", func(l *Lead) { l.URL = "people/quokka" }, "URL"},
		{"unknown state", func(l *Lead) { l.State = "maybe" }, "state"},
		{"unknown provider", func(l *Lead) { l.Provider = "carrier_pigeon" }, "provider"},
		{"missing search", func(l *Lead) { l.SearchID = 0 }, "search"},
		{"missing initiative", func(l *Lead) { l.InitiativeID = 0 }, "initiative"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := validLead()
			c.mut(&l)
			err := l.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error mentioning %q, got %v", c.want, err)
			}
		})
	}

	// An empty state defaults to new; the column default says the same.
	l = validLead()
	l.State = ""
	if err := l.Validate(); err != nil || l.State != LeadNew {
		t.Fatalf("empty state: err=%v state=%q", err, l.State)
	}
}
