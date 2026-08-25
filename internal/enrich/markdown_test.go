package enrich

import (
	"strings"
	"testing"

	"camstuart/talent-hound/internal/platform"
)

// Every fixture here is invented.

func TestAProfileRendersWhatIsThereAndNeverAnEmail(t *testing.T) {
	md := Profile(&platform.GitHubProfile{
		Login: "wombatdev", Name: "Wombat Developer", Location: "Melbourne",
		Blog: "https://wombat.example.invalid", Hireable: true, PublicRepos: 4, Followers: 12,
		CreatedOn: "2016-03-04",
	})
	for _, want := range []string{"# GitHub profile: wombatdev", "- Name: Wombat Developer", "- Location: Melbourne",
		"- Hireable: yes", "- Public repositories: 4", "- Account created: 2016-03-04"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "Company") || strings.Contains(md, "Bio") {
		t.Errorf("empty fields were rendered:\n%s", md)
	}
	if strings.Contains(strings.ToLower(md), "email") || strings.Contains(md, "@") {
		t.Errorf("an email line exists:\n%s", md)
	}
}

func TestReposSummariseLanguagesMostUsedFirst(t *testing.T) {
	md := Repos("wombatdev", []platform.GitHubRepo{
		{Name: "a", Language: "Go", PushedOn: "2026-08-01", URL: "https://github.com/wombatdev/a"},
		{Name: "b", Language: "TypeScript", PushedOn: "2026-07-01", URL: "https://github.com/wombatdev/b"},
		{Name: "c", Language: "Go", PushedOn: "2026-06-01", URL: "https://github.com/wombatdev/c", Description: "A burrow."},
	})
	goAt, tsAt := strings.Index(md, "- Go: 2 repositories"), strings.Index(md, "- TypeScript: 1 repositories")
	if goAt < 0 || tsAt < 0 || goAt > tsAt {
		t.Errorf("languages are not most-used first:\n%s", md)
	}
	if !strings.Contains(md, "### c\n") || !strings.Contains(md, "A burrow.") {
		t.Errorf("a repository is missing:\n%s", md)
	}
	if !strings.Contains(Repos("x", nil), "No public repositories") {
		t.Error("an empty list has no sentence")
	}
}

func TestEventsSummariseKindsAndRepositories(t *testing.T) {
	md := Events("wombatdev", []platform.GitHubEvent{
		{Type: "PushEvent", Repo: "wombatdev/a", CreatedOn: "2026-08-20"},
		{Type: "PushEvent", Repo: "wombatdev/a", CreatedOn: "2026-08-19"},
		{Type: "PullRequestReviewEvent", Repo: "quokka/b", CreatedOn: "2026-08-01"},
	})
	for _, want := range []string{"3 public events, most recent on 2026-08-20, earliest on 2026-08-01",
		"- PushEvent: 2", "- PullRequestReviewEvent: 1", "- wombatdev/a: 2", "- quokka/b: 1"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	if !strings.Contains(Events("x", nil), "No recent public activity") {
		t.Error("an empty list has no sentence")
	}
}
