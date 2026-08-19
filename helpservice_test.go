package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/help"
	"camstuart/talent-hound/internal/models"
)

// Help is read when the application is not working, so the tests that matter
// most are the ones with nothing configured.

// scriptedChat returns whatever it was given, and records the prompt.
type scriptedChat struct {
	reply  string
	err    error
	prompt string
}

func (s *scriptedChat) Chat(_ context.Context, _, prompt string, _ map[string]any) (string, error) {
	s.prompt = prompt
	return s.reply, s.err
}

func TestHelpAnswersWithNoModelAndNoDatabase(t *testing.T) {
	// No registry, no model, no database, no data folder: the state a recruiter
	// is in when they need the manual most.
	svc := NewHelpService(nil, nil)

	topics, err := svc.Topics()
	if err != nil {
		t.Fatalf("topics: %v", err)
	}
	if len(topics) < 2 {
		t.Fatalf("%d groups in the index", len(topics))
	}

	hits, err := svc.Search("encrypted volume")
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no results with nothing configured")
	}

	answer, err := svc.Ask("do I need an encrypted disk")
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if len(answer.Results) == 0 {
		t.Fatal("asking returned no sections")
	}
	if answer.Composed || answer.Text != "" {
		t.Fatal("an answer was composed with no model")
	}
	if !strings.Contains(answer.Why, "no model") {
		t.Fatalf("the absent answer was not explained: %q", answer.Why)
	}
}

func TestTheIndexGroupsEveryTopic(t *testing.T) {
	svc := NewHelpService(nil, nil)
	topics, err := svc.Topics()
	if err != nil {
		t.Fatalf("topics: %v", err)
	}
	seen := 0
	for _, g := range topics {
		if g.Group == "" {
			t.Fatal("a group has no name")
		}
		for _, topic := range g.Topics {
			if topic.ID == "" || topic.Title == "" || topic.Summary == "" {
				t.Fatalf("topic %+v is incomplete", topic)
			}
			if _, err := svc.Article(topic.ID); err != nil {
				t.Fatalf("indexed topic %q cannot be opened: %v", topic.ID, err)
			}
			seen++
		}
	}
	if seen < 10 {
		t.Fatalf("the index lists %d topics", seen)
	}
}

func TestAComposedAnswerCitesTheSectionsItUsed(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "synthetic-generate"})

	hits, err := helpSearchFor(t, "delete a candidate")
	if err != nil {
		t.Fatal(err)
	}
	chat := &scriptedChat{reply: mustJSON(t, helpAnswer{
		Answered: true,
		Text:     "Delete every initiative that references them first, including archived ones.",
		Sections: []string{hits[0].Section.Anchor},
	})}
	svc := NewHelpService(e.models, chat)

	answer, err := svc.Ask("why can't I delete a candidate")
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if !answer.Composed || answer.Text == "" {
		t.Fatalf("no answer was composed: %+v", answer)
	}
	if len(answer.Cited) == 0 {
		t.Fatal("the answer cites nothing")
	}
	// The sections it was given are in the prompt, and nothing else is.
	if !strings.Contains(chat.prompt, "using only the manual sections below") {
		t.Fatal("the prompt does not confine the model to the sections")
	}
}

// An answer citing nothing is not shown: help that invents an instruction is
// worse than help that offers the closest sections.
func TestAnUncitedAnswerIsWithheld(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "synthetic-generate"})
	chat := &scriptedChat{reply: mustJSON(t, helpAnswer{
		Answered: true, Text: "Just press the send button.", Sections: []string{},
	})}
	svc := NewHelpService(e.models, chat)

	answer, err := svc.Ask("how do I send an email")
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if answer.Composed || answer.Text != "" {
		t.Fatalf("an uncited answer was shown: %+v", answer)
	}
	if len(answer.Results) == 0 {
		t.Fatal("the results were withheld along with the answer")
	}
	if !strings.Contains(answer.Why, "cited nothing") {
		t.Fatalf("the withholding was not explained: %q", answer.Why)
	}
}

// A citation to a section that was never retrieved is not a citation.
func TestACitationToASectionNeverRetrievedIsIgnored(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "synthetic-generate"})
	chat := &scriptedChat{reply: mustJSON(t, helpAnswer{
		Answered: true, Text: "Invented.", Sections: []string{"a-section-that-does-not-exist"},
	})}
	svc := NewHelpService(e.models, chat)

	answer, err := svc.Ask("why can't I delete a candidate")
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if answer.Composed {
		t.Fatalf("an answer citing an unretrieved section was shown: %+v", answer)
	}
}

// The model saying the manual does not cover it is an answer, not a failure.
func TestAnUnansweredQuestionSaysSo(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "synthetic-generate"})
	chat := &scriptedChat{reply: mustJSON(t, helpAnswer{Answered: false})}
	svc := NewHelpService(e.models, chat)

	// A question the manual has sections near but does not answer.
	answer, err := svc.Ask("how do I delete a candidate from an export")
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if answer.Composed {
		t.Fatal("an answer was composed for a question the manual does not cover")
	}
	if !strings.Contains(answer.Why, "does not cover") {
		t.Fatalf("the refusal was not explained: %q", answer.Why)
	}
}

// A model failure loses the written answer and nothing else.
func TestAModelFailureStillReturnsTheSections(t *testing.T) {
	e := newModelEnv(t)
	e.assign(t, AssignInput{Role: models.RoleGenerate, Model: "synthetic-generate"})
	chat := &scriptedChat{err: context.DeadlineExceeded}
	svc := NewHelpService(e.models, chat)

	answer, err := svc.Ask("how do I delete a candidate")
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if answer.Composed || len(answer.Results) == 0 {
		t.Fatalf("a model failure took the results with it: %+v", answer)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	return string(raw)
}

// helpSearchFor is the search a test needs before it can script a citation.
func helpSearchFor(t *testing.T, query string) ([]help.Hit, error) {
	t.Helper()
	hits, err := NewHelpService(nil, nil).Search(query)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		t.Fatalf("no help results for %q", query)
	}
	return hits, nil
}
