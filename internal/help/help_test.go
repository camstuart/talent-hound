package help

import (
	"strings"
	"testing"
)

// Help is used when the application is not working, so these tests assume
// nothing either: no model, no database, no network.

func TestEveryArticleLoadsWithItsSections(t *testing.T) {
	articles, err := Articles()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	if len(articles) < 10 {
		t.Fatalf("%d articles shipped, want the full manual", len(articles))
	}
	groups := map[string]bool{}
	for _, a := range articles {
		if a.ID == "" || a.Title == "" || a.Group == "" || a.Summary == "" {
			t.Fatalf("article %q is missing front matter: %+v", a.ID, a)
		}
		if len(a.Sections) == 0 {
			t.Fatalf("article %q has no sections", a.ID)
		}
		groups[a.Group] = true
		for _, s := range a.Sections {
			if s.Heading == "" || s.Anchor == "" || strings.TrimSpace(s.Text) == "" {
				t.Fatalf("article %q has an empty section: %+v", a.ID, s)
			}
		}
	}
	if len(groups) < 2 {
		t.Fatal("every article is in one group, so the index cannot be browsed")
	}
}

func TestAnArticleIsFoundByIdentifier(t *testing.T) {
	a, err := Find("tutorial")
	if err != nil {
		t.Fatalf("finding the tutorial: %v", err)
	}
	if a.Title == "" || len(a.Sections) < 10 {
		t.Fatalf("the tutorial has %d sections", len(a.Sections))
	}
	if _, err := Find("no-such-article"); err == nil {
		t.Fatal("an unknown article was found")
	}
}

// The tutorial walks the flagship loop in the order a recruiter does it.
func TestTheTutorialCoversTheLoopInOrder(t *testing.T) {
	a, err := Find("tutorial")
	if err != nil {
		t.Fatalf("finding: %v", err)
	}
	steps := []string{
		"initiative", "candidate", "document", "extract", "profile",
		"criteria", "roles", "shortlist", "assessment", "draft",
	}
	at := 0
	for _, s := range a.Sections {
		heading := strings.ToLower(s.Heading)
		if at < len(steps) && strings.Contains(heading, steps[at]) {
			at++
		}
	}
	if at != len(steps) {
		t.Fatalf("the tutorial reached step %d of %d in order", at, len(steps))
	}
}

func TestASearchFindsTheSectionThatAnswers(t *testing.T) {
	hits, err := Search("delete a candidate", 5)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no result for a question the manual answers")
	}
	if hits[0].Section.ArticleID != "deleting-things" {
		t.Fatalf("the first result is %q/%q", hits[0].Section.ArticleID, hits[0].Section.Heading)
	}
	if hits[0].Snippet == "" {
		t.Fatal("the result carries no snippet")
	}
}

// "deleting" has to find "delete".
func TestAWordFormIsMatched(t *testing.T) {
	hits, err := Search("deleting candidates", 5)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	found := false
	for _, h := range hits {
		if h.Section.ArticleID == "deleting-things" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no deletion section for a query saying deleting: %+v", headings(hits))
	}
}

// A section about a subject beats one that mentions it.
func TestASectionAboutASubjectRanksAboveOneThatMentionsIt(t *testing.T) {
	hits, err := Search("encrypted volume", 5)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no result")
	}
	top := hits[0].Section
	if !strings.Contains(strings.ToLower(top.Article+" "+top.Heading+" "+top.Text), "encrypt") {
		t.Fatalf("the top result is not about encryption: %q/%q", top.Article, top.Heading)
	}
}

func TestNothingMatchingIsSaidPlainly(t *testing.T) {
	hits, err := Search("zzzqqxx", 5)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("unrelated sections were returned as answers: %+v", headings(hits))
	}
}

func TestAnEmptyQueryReturnsNothingRatherThanEverything(t *testing.T) {
	hits, err := Search("   ", 5)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("an empty query returned %d results", len(hits))
	}
}

func TestRepeatedSearchesReturnTheSameOrder(t *testing.T) {
	first, err := Search("what does the application send", 8)
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	for i := 0; i < 10; i++ {
		again, err := Search("what does the application send", 8)
		if err != nil {
			t.Fatalf("searching: %v", err)
		}
		if len(again) != len(first) {
			t.Fatalf("run %d returned %d results, first returned %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Section.Anchor != first[j].Section.Anchor {
				t.Fatalf("run %d differs at %d: %q then %q",
					i, j, first[j].Section.Anchor, again[j].Section.Anchor)
			}
		}
	}
}

// The rules a recruiter would otherwise discover by being refused.
func TestTheSurprisingRulesAreDocumented(t *testing.T) {
	for _, tc := range []struct{ query, want string }{
		{"send an email to a candidate", "cannot send"},
		{"why can't I delete this candidate", "archived"},
		{"real data unencrypted volume", "encrypt"},
		{"stale profile", "stale"},
		{"protected criterion refused", "protected"},
	} {
		hits, err := Search(tc.query, 5)
		if err != nil {
			t.Fatalf("searching %q: %v", tc.query, err)
		}
		joined := ""
		for _, h := range hits {
			joined += " " + strings.ToLower(h.Section.Text)
		}
		if !strings.Contains(joined, tc.want) {
			t.Fatalf("%q found nothing saying %q: %+v", tc.query, tc.want, headings(hits))
		}
	}
}

// Help ships no candidate information, and no example drawn from anyone's
// records.
func TestArticlesCarryNoContactDetails(t *testing.T) {
	articles, err := Articles()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	for _, a := range articles {
		lowered := strings.ToLower(a.Markdown)
		for _, shape := range []string{"@gmail", "@outlook", "@example.com", "linkedin.com/in/", "+61 4"} {
			if strings.Contains(lowered, shape) {
				t.Fatalf("article %q contains %q", a.ID, shape)
			}
		}
	}
}

func headings(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Section.Article+" / "+h.Section.Heading)
	}
	return out
}

func TestEachWayOfWorkingHasItsOwnTutorial(t *testing.T) {
	// One tutorial per initiative type, plus one for the CRM: a recruiter's
	// first question is "how do I do my kind of work here".
	for _, id := range []string{
		"tutorial",
		"tutorial-talent-search",
		"tutorial-business-development",
		"tutorial-crm",
	} {
		a, err := Find(id)
		if err != nil {
			t.Fatalf("finding %q: %v", id, err)
		}
		if a.Group != "First steps" {
			t.Fatalf("%q is in group %q, want it beside the other tutorials", id, a.Group)
		}
		if len(a.Sections) < 3 {
			t.Fatalf("%q has %d sections, too thin to demonstrate anything", id, len(a.Sections))
		}
	}

	// The tutorials sit together, in reading order, right after the flagship one.
	articles, err := Articles()
	if err != nil {
		t.Fatalf("loading: %v", err)
	}
	order := map[string]int{}
	for i, a := range articles {
		order[a.ID] = i
	}
	if !(order["tutorial"] < order["tutorial-talent-search"] &&
		order["tutorial-talent-search"] < order["tutorial-business-development"] &&
		order["tutorial-business-development"] < order["tutorial-crm"] &&
		order["tutorial-crm"] < order["initiatives-and-records"]) {
		t.Fatalf("tutorials are out of reading order: %v", order)
	}
}
