package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"camstuart/talent-hound/internal/help"
	"camstuart/talent-hound/internal/models"
)

// answerTimeout bounds the optional written answer. Short on purpose: help
// already has an answer to show, and a recruiter waiting on a manual is a
// recruiter who has stopped working.
const helpAnswerTimeout = 90 * time.Second

// HelpService serves the shipped documentation.
//
// Everything except the written answer works with no model, no data folder, no
// database, and no network — because help is read when those are the problem.
type HelpService struct {
	registry *ModelService
	model    Classifier
}

// NewHelpService returns a HelpService. The registry and model may be nil: help
// still answers, without the written part.
func NewHelpService(registry *ModelService, model Classifier) *HelpService {
	return &HelpService{registry: registry, model: model}
}

// Topic is one article as the index shows it.
type Topic struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Group   string `json:"group"`
	Summary string `json:"summary"`
}

// Index is every topic, grouped, so a recruiter who does not know the word for
// what they want can still find it.
type Index struct {
	Group  string  `json:"group"`
	Topics []Topic `json:"topics"`
}

// Topics returns the whole index in reading order.
func (s *HelpService) Topics() ([]Index, error) {
	articles, err := help.Articles()
	if err != nil {
		return nil, err
	}
	out := []Index{}
	at := map[string]int{}
	for _, a := range articles {
		topic := Topic{ID: a.ID, Title: a.Title, Group: a.Group, Summary: a.Summary}
		if i, ok := at[a.Group]; ok {
			out[i].Topics = append(out[i].Topics, topic)
			continue
		}
		at[a.Group] = len(out)
		out = append(out, Index{Group: a.Group, Topics: []Topic{topic}})
	}
	return out, nil
}

// Article returns one article with its sections.
func (s *HelpService) Article(id string) (*help.Article, error) { return help.Find(id) }

// Search returns the sections that answer, best first.
func (s *HelpService) Search(query string) ([]help.Hit, error) {
	return help.Search(query, 8)
}

// Answer is a written answer with the sections it was built from.
type Answer struct {
	// Text is empty when no answer could be given, which is a state rather than
	// a failure: the results are the answer then.
	Text string `json:"text"`
	// Why explains an empty answer in the recruiter's terms.
	Why      string     `json:"why"`
	Cited    []help.Hit `json:"cited"`
	Results  []help.Hit `json:"results"`
	Composed bool       `json:"composed"`
}

// helpAnswer is what the model returns.
type helpAnswer struct {
	Answered bool     `json:"answered"`
	Text     string   `json:"text"`
	Sections []string `json:"sections"`
}

// Ask searches, and composes an answer from the retrieved sections when a
// model is available.
//
// The results come first and are returned whatever happens to the answer. A
// written answer is a bonus on top of them, never a replacement: help that
// needs a model goes quiet exactly when it is needed most.
func (s *HelpService) Ask(question string) (*Answer, error) {
	results, err := help.Search(question, 6)
	if err != nil {
		return nil, err
	}
	out := &Answer{Cited: []help.Hit{}, Results: results}
	if len(results) == 0 {
		out.Why = "nothing in the manual matches that. Try the topic index — every page is listed there."
		return out, nil
	}
	if s.registry == nil || s.model == nil {
		out.Why = "no model is assigned, so there is no written answer. The sections below are the closest matches."
		return out, nil
	}
	res, err := s.registry.Resolve(models.RoleGenerate)
	if err != nil || res.Assignment == nil {
		out.Why = "no generate model is assigned, so there is no written answer. The sections below are the closest matches."
		return out, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), helpAnswerTimeout)
	defer cancel()
	raw, err := s.model.Chat(ctx, res.Assignment.Model, helpPrompt(question, results), helpSchema())
	if err != nil {
		out.Why = "the model did not answer, so the sections below are the closest matches."
		return out, nil
	}

	var answer helpAnswer
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &answer); err != nil {
		out.Why = "the model's answer could not be read, so the sections below are the closest matches."
		return out, nil
	}
	if !answer.Answered || strings.TrimSpace(answer.Text) == "" {
		out.Why = "the manual does not cover that. The sections below are the closest matches."
		return out, nil
	}

	// An answer citing no section is not shown. Help that invents an
	// instruction is worse than help that says "these are the closest I have".
	for _, anchor := range answer.Sections {
		for _, hit := range results {
			if hit.Section.Anchor == anchor && hit.Section.ArticleID != "" {
				out.Cited = append(out.Cited, hit)
			}
		}
	}
	if len(out.Cited) == 0 {
		out.Why = "the model's answer cited nothing in the manual, so it is not shown. The sections below are the closest matches."
		return out, nil
	}
	out.Text = strings.TrimSpace(answer.Text)
	out.Composed = true
	return out, nil
}

// helpPrompt asks for an answer from the given sections and nothing else.
func helpPrompt(question string, hits []help.Hit) string {
	var b strings.Builder
	b.WriteString("You answer a question about an application, using only the manual sections below.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Answer only from these sections. Never add anything you know from elsewhere.\n")
	b.WriteString("- Cite the anchor of every section you used.\n")
	b.WriteString("- If the sections do not answer the question, set answered to false and say nothing else.\n")
	b.WriteString("- Be brief: a few sentences, in the second person.\n\n")
	b.WriteString("Question:\n" + strings.TrimSpace(question) + "\n\nSections:\n")
	for _, h := range hits {
		fmt.Fprintf(&b, "\n[%s] %s — %s\n%s\n",
			h.Section.Anchor, h.Section.Article, h.Section.Heading, h.Section.Text)
	}
	return b.String()
}

func helpSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answered": map[string]any{"type": "boolean"},
			"text":     map[string]any{"type": "string"},
			"sections": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required":             []any{"answered", "text", "sections"},
		"additionalProperties": false,
	}
}
