## 1. Corpus

- [x] 1.1 `internal/help/`: articles embedded in the binary, split into sections at their headings
- [x] 1.2 Fourteen articles covering first steps, working, rules, and setup
- [x] 1.3 A tutorial walking the flagship loop in order, saying what the recruiter decides and what the application does
- [x] 1.4 Front matter giving every article an id, title, group, and summary

## 2. Search

- [x] 2.1 BM25 over a term index built in memory at startup
- [x] 2.2 Word forms matched by shared prefix, so "deleting" finds "delete"
- [x] 2.3 Headings and titles weighted, so a section about a subject beats one mentioning it
- [x] 2.4 Snippets showing why a section matched
- [x] 2.5 Nothing matching returns nothing, rather than unrelated sections

## 3. Answers

- [x] 3.1 `helpservice.go`: topics, article, search, and the optional answer
- [x] 3.2 An answer composed only from retrieved sections, citing them
- [x] 3.3 An uncited answer withheld, with the results shown instead
- [x] 3.4 A missing model, a model failure, and an unanswerable question each explained
- [x] 3.5 No endpoint, no telemetry, no feedback control

## 4. Frontend

- [x] 4.1 `HelpPanel.tsx`: index, search, reader, and the answer with its sections
- [x] 4.2 Reachable from the sidebar, ahead of the first-run gate
- [x] 4.3 A written answer marked as model-written, like every other generated text

## 5. Tests

- [x] 5.1 Go tests over loading, ranking, word forms, ordering, and the surprising rules
- [x] 5.2 Go tests proving help answers with no model, no database, and no network
- [x] 5.3 Go tests over citation, withholding, and refusal
- [x] 5.4 Vitest over the index, the reader, and both answer states
- [x] 5.5 Playwright over reaching help with no initiative, the tutorial, searching, and reading
- [x] 5.6 Articles carry no candidate information

## 6. Exit gate

- [x] 6.1 Help answers on a fresh install, offline, with nothing configured
- [x] 6.2 `just check` passes
