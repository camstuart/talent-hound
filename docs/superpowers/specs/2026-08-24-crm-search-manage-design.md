# CRM Search & Manage — Design

**Date:** 2026-08-24
**Status:** Approved in brainstorming; ready for implementation planning.

## Goal

Give the recruiter CRM-like capabilities over the people and organisations
Talent Hound already stores — candidates, companies, contacts, roles — by
adding: interaction history (calls, notes, placements, applications,
rejections), structured search/filtering, a semantic talent-pool search, and
a person-centric CRM tab. Interaction notes become first-class RAG evidence
so future searches and profiles can use them.

This is a single-user local app. "User types" are records the recruiter
manages, not login roles.

## What already exists (reused, not rebuilt)

- `RecordService`: Create/Update/Get/List for candidate, company, contact,
  role; `RecordForm`/`RecordsPanel` UI.
- Artifacts attach to any record type (`LinkTarget`: initiative, candidate,
  role, company, contact) and flow extract → chunk → embed; same-target
  duplicate bytes are refused.
- `SearchService`: FTS + vector retrieval over chunks, initiative-scoped,
  with `Cite` resolving a chunk back to its quote.
- `Profile`/`ProfileAspect`: versioned, evidence-cited derived facts per
  candidate or role, with source-hash staleness.
- `Match`: candidate-vs-role assessments (conclusions, not events).

The new subsystem is interactions/history plus people-centric search.

## Decisions (made during brainstorming)

1. **Interaction shape: typed note.** One `Interaction` table: a kind, a
   free-text note, a date, links to the records involved. Outcomes
   (placement, application, rejection) are kinds, not separate models.
2. **RAG tie-in: via artifacts.** Saving a note also ingests a companion
   artifact through the existing pipeline; notes become citable evidence.
3. **Search: two separate searches.** A structured filter UI over typed
   fields, and a separate semantic "talent search" box over evidence.
4. **UI: new top-level CRM tab, master-detail.**
5. **Edits: allowed, evidence replaced.** The recruiter's own note is
   editable/deletable; each edit replaces the companion artifact (and its
   chunks/embeddings). Profiles citing the old version go stale via the
   existing source-hash mechanism.
6. **Approach: reuse the chunk index, group by person** (Approach A). No
   per-person index or new embedding document; a dedicated person index is
   a possible later upgrade behind the same UI if ranking disappoints.

## Data model

New file `internal/models/interaction.go`, registered in `db.Open`'s
`AutoMigrate`:

```go
type Interaction struct {
    ID         uint       `gorm:"primarykey"`
    TargetType LinkTarget `gorm:"not null"` // candidate, contact, company, role
    TargetID   uint       `gorm:"not null"`
    Kind       string     `gorm:"not null"` // see kinds below
    Note       string     `gorm:"not null"` // recruiter's words, free text
    OccurredAt Date       // when it happened; defaults to today
    RoleID       *uint    // optional: the role involved
    InitiativeID *uint    // optional: the search it happened under
    ArtifactID   uint     `gorm:"not null"` // companion evidence artifact
    CreatedAt, UpdatedAt time.Time
}
```

- Kinds: `call`, `meeting`, `email`, `note`, `placement`, `application`,
  `rejection` — validated like other code sets in `values.go`.
- Outcome kinds (`placement`, `application`, `rejection`) require `RoleID`.
- `TargetType` reuses the artifact `LinkTarget` vocabulary (initiative is
  not a valid interaction target).
- Note text is a stranger-adjacent string by the codebase's rules:
  displayed, never rendered.

## Evidence flow

- **On create:** render a small Markdown document — header line
  (`Call with <name>, 2026-08-24, re: <role>`) + the note verbatim — and
  ingest it via `ArtifactService` with `source: "interaction"`, linked to
  the interaction's target. It rides the untouched extract → chunk → embed
  pipeline. The header makes each note's bytes naturally unique under the
  same-target dedup; a literally identical note logged twice the same day
  is refused, which is correct.
- **On edit:** delete the old artifact (with its chunks and embeddings,
  via the existing deletion machinery) and ingest a replacement, so search
  always reflects current wording.
- **On delete:** remove the interaction row and its artifact the same way.
- The companion artifact stays visible in the record's artifact list
  (honest evidence trail) but cannot be renamed or detached independently:
  the interaction owns it.

## Services

**`InteractionService`** (new `interactionservice.go`, registered in
`main.go`):

- `Log(input)` — validate, create row + companion artifact.
- `Update(input)` — edit note/kind/date/links; replaces the artifact.
- `Delete(id)` — removes row + artifact/chunks/embeddings.
- `Timeline(targetType, targetID)` — history newest-first by `OccurredAt`,
  each entry carrying kind, note, and role/initiative display names so the
  panel needs no second query.

**`RecordService` additions** — structured search, plain SQL:

- `SearchCandidates(filter)` with
  `{ text, workRights, employmentType, arrangement, availableBy }` —
  `text` LIKE-matches full name, preferred name, emails, location; the
  rest are exact/range WHERE clauses. Ordered by name.
- `SearchCompanies(text)` / `SearchContacts(text)` — name/email LIKE only.
- No vector math in this path; a filter behaves like a filter.

**`SearchService.People(query, limit)`** — semantic talent search:

- Same FTS + vector retrieval as the existing `search`, but scoped to
  chunks whose artifact links to a candidate (any initiative, or none)
  instead of to one initiative.
- Hits grouped by candidate; a candidate's score is their best chunk's
  score; returns `PersonHit { candidate, snippet, chunkID, artifactName }`
  ranked by score. `chunkID` keeps the existing `Cite` path working.
- No changes to the embedding pipeline, index, or `Cite`.

## UI

New top-level **CRM tab** (same mechanism as Settings/Help), `CrmPanel.tsx`,
master-detail:

**Left pane:**
- Record-type picker: Candidates / Companies / Contacts / Roles.
- **Filter** box: text + structured filters (filters only for Candidates).
- **Talent search** box (Candidates only): calls `SearchService.People`,
  lists ranked people with their "why" snippet.
- Results list; "New …" opens the existing `RecordForm` create form.

**Right pane** (selected record):
- **Details** — editable fields via `RecordForm`, saved through `Update*`.
- **Artifacts** — existing `ArtifactsPanel` with a new target prop
  (storage already supports non-initiative targets).
- **History** — interaction timeline from `Timeline()`, plus a
  "Log interaction" form (kind, note, date, optional role/initiative) and
  edit/delete per entry.
- **Profile** — for candidates, the existing `CandidateProfilePanel`
  embedded beside the raw fields.

`RecordsPanel` in the initiative workspace stays untouched; retiring it is
a separate later decision.

## Error handling

- Backend errors surface verbatim in a `role="alert"` per pane, matching
  every existing panel.
- Kind/target validation errors come from `Interaction.Validate`.
- Talent search with no embed model reports it in the existing model-state
  vocabulary (e.g. "Ollama is not running").

## Testing

- **Go:** `interactionservice_test.go` — log/edit/delete drives the
  artifact lifecycle; outcome kinds require a role; timeline ordering.
  `recordservice_test.go` — filter behaviour. `searchservice_test.go` —
  `People` groups by candidate, ignores initiative scope, excludes chunks
  whose artifact links to no candidate.
- **Vitest:** `CrmPanel.test.tsx` with mocked bindings — mode switching,
  filter vs talent search, timeline render, log/edit forms.
- **Playwright:** `e2e/crm.spec.ts` against the real backend — create
  candidate → log a note → find them via talent search → see the note in
  history. Per-run-unique fixtures, locators scoped to own rows.

## Out of scope

- Per-person embedding index (Approach B) — later upgrade if ranking
  disappoints.
- Relationship strength / interaction analytics for companies (P1).
- Retiring `RecordsPanel`.
- Deriving outcomes automatically from `Match` records — outcomes are
  logged by the recruiter.
