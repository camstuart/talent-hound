package models

import (
	"strings"
	"testing"
)

func validIdentity() Identity {
	return Identity{
		CandidateID: 1,
		Provider:    IdentityGitHub,
		Handle:      "Quokka",
		URL:         "https://github.com/Quokka",
	}
}

func TestIdentityValidation(t *testing.T) {
	i := validIdentity()
	if err := i.Validate(); err != nil {
		t.Fatalf("valid identity refused: %v", err)
	}
	// GitHub logins are case-insensitive, so one spelling is stored.
	if i.Handle != "quokka" {
		t.Fatalf("github handle not lowercased: %q", i.Handle)
	}

	i = validIdentity()
	i.Handle = "@Quokka"
	if err := i.Validate(); err != nil || i.Handle != "quokka" {
		t.Fatalf("@ prefix: err=%v handle=%q", err, i.Handle)
	}

	// Other providers keep the handle as given, trimmed.
	i = validIdentity()
	i.Provider = IdentityLinkedIn
	i.Handle = " Quokka-Stack "
	i.URL = "https://www.linkedin.com/in/Quokka-Stack"
	if err := i.Validate(); err != nil || i.Handle != "Quokka-Stack" {
		t.Fatalf("linkedin handle: err=%v handle=%q", err, i.Handle)
	}

	cases := []struct {
		name string
		mut  func(*Identity)
		want string
	}{
		{"missing handle", func(i *Identity) { i.Handle = " @ " }, "handle"},
		{"unknown provider", func(i *Identity) { i.Provider = "myspace" }, "provider"},
		{"missing url", func(i *Identity) { i.URL = "" }, "URL"},
		{"bare domain", func(i *Identity) { i.URL = "github.com/quokka" }, "URL"},
		{"missing candidate", func(i *Identity) { i.CandidateID = 0 }, "candidate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			i := validIdentity()
			c.mut(&i)
			err := i.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error mentioning %q, got %v", c.want, err)
			}
		})
	}
}

func TestIdentityProvidersAreStable(t *testing.T) {
	want := []string{"github", "website", "linkedin", "hn"}
	got := IdentityProviders()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("providers = %v, want %v", got, want)
	}
}
