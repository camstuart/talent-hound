## 1. Schema

- [x] 1.1 Migration 15: `drafts` with its two states and its evidence map, and `answers`
- [x] 1.2 The CopiedOut task on the existing audit table, with no content column

## 2. Question answering

- [x] 2.1 `qaservice.go`: retrieval scoped by initiative and by approval, in the query
- [x] 2.2 The answer contract: prose, citations, and a supported flag
- [x] 2.3 Citations validated for resolvability; a supported answer with none is refused
- [x] 2.4 An unsupported question returns an explicit unknown with no invented prose
- [x] 2.5 Proposals returned to the screen and never written

## 3. Drafts

- [x] 3.1 `draftservice.go`: generation with a claim-to-evidence map
- [x] 3.2 Two states, editing, and repeated copying
- [x] 3.3 One metadata-only CopiedOut event per copy, carrying no draft text
- [x] 3.4 Discarding writes no copy event and keeps the history

## 4. No transport

- [x] 4.1 A repository scan for senders and their protocols
- [x] 4.2 A runtime assertion that a full cycle opens no mail or messaging connection

## 5. Frontend

- [x] 5.1 An ask panel with citation navigation and the explicit unknown
- [x] 5.2 Proposals listed with an explicit apply
- [x] 5.3 A drafts panel: generate, edit, copy, discard, and the claim map
- [x] 5.4 Keyboard operable; backend messages surfaced verbatim

## 6. Tests

- [x] 6.1 Scope excludes other initiatives and unapproved evidence
- [x] 6.2 Factual answers cite resolvable evidence; unsupported returns unknown
- [x] 6.3 Injection cannot change scope, apply changes, reach a provider, delete, or copy
- [x] 6.4 Every factual draft claim maps to evidence; invalid output is refused
- [x] 6.5 Editing and repeated copying preserve the draft and create separate events
- [x] 6.6 Copy events contain no draft text, payload, query, or document content
- [x] 6.7 Discarding records no copy and no send
- [x] 6.8 The repository and runtime transport checks both pass and would fail on a sender
- [x] 6.9 Vitest: citation navigation, apply confirmation, draft editing, copy feedback, discard, keyboard
- [x] 6.10 Playwright: ask, draft, edit, copy twice, discard through the real backend
- [x] 6.11 Fixtures are synthetic only — no real candidate information anywhere

## 7. Exit gate

- [x] 7.1 The recruiter can ask, draft, edit, and copy locally; the application cannot send outreach
- [x] 7.2 `just check` passes
