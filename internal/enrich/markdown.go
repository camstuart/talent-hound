// Package enrich renders a provider's answer about a public handle as
// Markdown evidence: something the extraction pipeline can chunk, cite, and
// classify like any other document.
//
// Pure functions, so a test can pin exactly what is and is not written down.
package enrich

import (
	"fmt"
	"strings"

	"camstuart/talent-hound/internal/platform"
)

// Profile renders a public profile. There is no email line, and there never
// will be: the provider shows one when the person chose to, and this
// application does not collect it.
func Profile(p *platform.GitHubProfile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GitHub profile: %s\n\n", p.Login)
	line(&b, "Name", p.Name)
	line(&b, "Company", p.Company)
	line(&b, "Location", p.Location)
	line(&b, "Website", p.Blog)
	line(&b, "Bio", p.Bio)
	if p.Hireable {
		line(&b, "Hireable", "yes, by their own flag")
	}
	fmt.Fprintf(&b, "- Public repositories: %d\n- Followers: %d\n", p.PublicRepos, p.Followers)
	line(&b, "Account created", p.CreatedOn)
	line(&b, "Profile updated", p.UpdatedOn)
	return b.String()
}

// Repos renders the repositories a person owns, newest push first.
func Repos(login string, repos []platform.GitHubRepo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GitHub repositories: %s\n\n", login)
	if len(repos) == 0 {
		b.WriteString("No public repositories of their own.\n")
		return b.String()
	}
	languages := map[string]int{}
	for _, r := range repos {
		if r.Language != "" {
			languages[r.Language]++
		}
	}
	if len(languages) > 0 {
		b.WriteString("## Languages\n\n")
		for _, lang := range sortedKeys(languages) {
			fmt.Fprintf(&b, "- %s: %d repositories\n", lang, languages[lang])
		}
		b.WriteString("\n")
	}
	b.WriteString("## Repositories\n\n")
	for _, r := range repos {
		fmt.Fprintf(&b, "### %s\n\n", r.Name)
		line(&b, "Language", r.Language)
		fmt.Fprintf(&b, "- Stars: %d\n", r.Stars)
		line(&b, "Last pushed", r.PushedOn)
		line(&b, "URL", r.URL)
		if r.Description != "" {
			fmt.Fprintf(&b, "\n%s\n", r.Description)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Events renders recent public activity as a summary and a list.
func Events(login string, events []platform.GitHubEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# GitHub activity: %s\n\n", login)
	if len(events) == 0 {
		b.WriteString("No recent public activity.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d public events, most recent on %s, earliest on %s.\n\n",
		len(events), events[0].CreatedOn, events[len(events)-1].CreatedOn)
	kinds := map[string]int{}
	repos := map[string]int{}
	for _, e := range events {
		kinds[e.Type]++
		repos[e.Repo]++
	}
	b.WriteString("## By kind\n\n")
	for _, k := range sortedKeys(kinds) {
		fmt.Fprintf(&b, "- %s: %d\n", k, kinds[k])
	}
	b.WriteString("\n## By repository\n\n")
	for _, r := range sortedKeys(repos) {
		fmt.Fprintf(&b, "- %s: %d\n", r, repos[r])
	}
	return b.String()
}

func line(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(b, "- %s: %s\n", label, strings.TrimSpace(value))
}

// sortedKeys orders by count descending, then name, so the rendering is
// stable and the most telling line is first.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && (m[keys[j]] > m[keys[j-1]] || (m[keys[j]] == m[keys[j-1]] && keys[j] < keys[j-1])); j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
