## Why

The recruiter this product is for works alone, with no IT support, on one laptop. When something confuses them there is nobody to ask — so the help has to answer, and it has to answer when the rest of the application cannot.

That last point decides the design. Help is most needed exactly when things are broken: no model installed, no data folder chosen, the volume unencrypted, Ollama not running. A help system that needs the model to answer is a help system that goes quiet in the moment it exists for. So retrieval is deterministic and local, shipped in the binary, and works before first run completes. A model, when there is one, adds an answer on top of that — never underneath it.

The second decision follows from the product's own rules. This is an application whose whole argument is that it does not send candidate information anywhere. A help search box that phoned home for answers would contradict that in the one place a recruiter looks when they are already unsure whether to trust it.

## What Changes

- Add a help system: an index of topics, a searchable set of articles, and a tutorial that walks the flagship loop end to end.
- Search deterministically over the shipped articles, with no model, no network, and no database — it works on first launch and in aeroplane mode.
- Offer an answer built from the retrieved sections when a generate model is assigned, cited to the sections it used, refusing rather than inventing when they do not support the question.
- Show help without leaving the workspace, and make every topic reachable without searching.
- Ship no help content that quotes a recruiter's data, and send no query anywhere.

## Capabilities

### New Capabilities
- `help-search`: what a search returns, what it never needs, and what an answer may claim.
- `help-content`: the topics, the tutorial, and what the articles must cover.

## Impact

- New `internal/help/` — the article corpus, its index, and the ranking, all pure so relevance is a table test.
- New `internal/help/content/` — the articles, embedded in the binary.
- New `helpservice.go` — topics, article, search, and the optional cited answer.
- `frontend/src/components/HelpPanel.tsx` — the index, the search, the reader, and the tutorial.
- Go tests over ranking and refusal, Vitest over the panel, Playwright over searching and reading.
