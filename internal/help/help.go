// Package help holds the shipped documentation, its index, and the search over
// it.
//
// Help is used when the application is not working, so it assumes nothing: no
// model, no data folder, no database, no network. The articles are embedded in
// the binary and the index is built in memory at startup.
package help

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

//go:embed content
var contentFS embed.FS

// Section is one heading and the text under it: the unit a search returns.
//
// A section rather than an article, for the reason Phase 21 arrived at for
// aspects — a whole article matches everything, and a section answers one
// question.
type Section struct {
	ArticleID string `json:"articleId"`
	Article   string `json:"article"`
	Group     string `json:"group"`
	Heading   string `json:"heading"`
	Anchor    string `json:"anchor"`
	Text      string `json:"text"`
}

// Article is one shipped document.
type Article struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Group    string    `json:"group"`
	Summary  string    `json:"summary"`
	Markdown string    `json:"markdown"`
	Sections []Section `json:"sections"`
}

// Hit is one section a search matched, with why.
type Hit struct {
	Section Section `json:"section"`
	Score   float64 `json:"score"`
	// Snippet is the part of the section worth reading first.
	Snippet string `json:"snippet"`
}

var (
	once     sync.Once
	articles []Article
	sections []Section
	index    *termIndex
	loadErr  error
)

// Articles returns every shipped article, in reading order.
func Articles() ([]Article, error) {
	load()
	return articles, loadErr
}

// Find returns one article by its identifier.
func Find(id string) (*Article, error) {
	load()
	if loadErr != nil {
		return nil, loadErr
	}
	for i := range articles {
		if articles[i].ID == id {
			return &articles[i], nil
		}
	}
	return nil, fmt.Errorf("no help article called %q", id)
}

func load() {
	once.Do(func() {
		entries, err := fs.ReadDir(contentFS, "content")
		if err != nil {
			loadErr = fmt.Errorf("reading help content: %w", err)
			return
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				names = append(names, e.Name())
			}
		}
		// Sorted by file name, which is why they are numbered: reading order is
		// a decision, not whatever the filesystem returns.
		sort.Strings(names)

		for _, name := range names {
			raw, err := contentFS.ReadFile("content/" + name)
			if err != nil {
				loadErr = fmt.Errorf("reading %s: %w", name, err)
				return
			}
			article, err := parse(string(raw))
			if err != nil {
				loadErr = fmt.Errorf("reading %s: %w", name, err)
				return
			}
			articles = append(articles, article)
			sections = append(sections, article.Sections...)
		}
		index = newTermIndex(sections)
	})
}

// parse reads the front matter and splits the body at its headings.
func parse(raw string) (Article, error) {
	body := raw
	article := Article{}
	if strings.HasPrefix(raw, "---\n") {
		end := strings.Index(raw[4:], "\n---\n")
		if end < 0 {
			return Article{}, fmt.Errorf("front matter is not closed")
		}
		front := raw[4 : 4+end]
		body = raw[4+end+len("\n---\n"):]
		for _, line := range strings.Split(front, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			switch strings.TrimSpace(key) {
			case "id":
				article.ID = value
			case "title":
				article.Title = value
			case "group":
				article.Group = value
			case "summary":
				article.Summary = value
			}
		}
	}
	if article.ID == "" || article.Title == "" {
		return Article{}, fmt.Errorf("an article needs an id and a title")
	}
	article.Markdown = strings.TrimSpace(body)

	heading := ""
	current := strings.Builder{}
	flush := func() {
		text := strings.TrimSpace(current.String())
		current.Reset()
		if heading == "" || text == "" {
			return
		}
		article.Sections = append(article.Sections, Section{
			ArticleID: article.ID, Article: article.Title, Group: article.Group,
			Heading: heading, Anchor: anchor(heading), Text: text,
		})
	}
	for _, line := range strings.Split(article.Markdown, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	flush()
	if len(article.Sections) == 0 {
		return Article{}, fmt.Errorf("article %q has no sections", article.ID)
	}
	return article, nil
}

// anchor turns a heading into a stable identifier.
func anchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
