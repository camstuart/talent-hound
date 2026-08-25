# Candidate Sourcing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Find people for a role through Exa as `Lead` records, promote a lead into a `Candidate` with a durable `Identity`, and enrich a candidate from GitHub into artifacts that flow through the existing evidence pipeline — with every outbound request previewed, scrubbed, and recorded as a "that, not what" disclosure.

**Architecture:** Two new models (`Lead`, `Identity`) and one migration. `SourcingService` mirrors `DiscoveryService` (preview → scrub → send) with task `candidate_sourcing`; the shared preview/disclosure code moves into an unexported helper both use. `EnrichService` owns a new `platform.GitHub` client and writes artifacts, never candidate fields. UI: a *Find people* section on the role page, an *Identities* + *Enrich* section on the CRM candidate detail, a GitHub token in Settings.

**Tech Stack:** Go + GORM + glebarez/sqlite (no CGO), hand-written SQL migrations, Wails v3 bindings, SolidJS + Vitest + Playwright, Bun only.

**Spec:** `docs/superpowers/specs/2026-08-26-candidate-sourcing-design.md`

## Global Constraints

- Bun is the only JS package manager/runtime. Never npm/yarn/pnpm.
- Never hand-edit `frontend/bindings/` — regenerate with `wails3 generate bindings -clean=true -ts -i`.
- Schema changes are hand-written SQL migrations in `internal/db/migrations.go` (next version: **18**). There is no AutoMigrate.
- Snippets, titles, and anything from a provider are displayed, never rendered.
- Backend errors surface verbatim in the UI inside `role="alert"` elements.
- `platform.FetchAllowed` is deny-by-default and its allowlist stays empty; provider clients (Exa, GitHub) have fixed endpoints and never go through it. LinkedIn stays on the denylist.
- A `DisclosureEvent` is written only after bytes actually leave the machine, and never holds a query, a handle, or a result.
- Provider keys are read at call time through `CredentialService.secret(provider)`, never at start-up (see `DiscoveryService.searcher` for why).
- Go run: `go test ./...`. Vitest: `cd frontend && bun run test:unit`. E2E: `cd frontend && bun run test:e2e`.
- Commit messages: sentence-case imperative, no conventional-commit prefixes, ending with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>` and `Claude-Session: https://claude.ai/code/session_01R7LDX287Ls1d8A4jhaZ5hL` trailers.

---

### Task 1: `Lead` and `Identity` models, constants, migration 18

**Files:**
- Create: `internal/models/lead.go`, `internal/models/identity.go`
- Modify: `internal/models/discovery.go` (constants), `internal/db/migrations.go`
- Test: `internal/models/lead_test.go`, `internal/models/identity_test.go`

**Interfaces:**
- `models.Lead` with states `LeadNew`, `LeadPromoted`, `LeadDismissed`; `(*Lead).Validate() error`.
- `models.Identity` with `IdentityProviders()` = `github, website, linkedin, hn`; `(*Identity).Validate() error` — normalises handle (trim, lowercase for github), requires an absolute URL via `requireAbsoluteURL`.
- `models.TaskCandidateSourcing`, `models.TaskCandidateEnrich`, `models.ProviderGitHub`.
- Tables `leads` (index on `search_id`, `initiative_id`, `url`; CHECK on state) and `identities` (UNIQUE `(provider, handle)`, FK to candidates, CHECK on provider).

- [ ] **Step 1: Write the failing tests** — table-driven like `interaction_test.go`: valid lead accepted; empty URL, relative URL, unknown state, unknown provider refused with the field named. Identity: `GitHub` handle lowercased and `@` stripped; `linkedin` handle kept as-is; empty handle refused; unknown provider refused.
- [ ] **Step 2: Verify RED** — `go test ./internal/models/` fails on undefined types.
- [ ] **Step 3: Write the models** — follow `candidate.go`'s comment style; the `Lead` doc comment states it is not a candidate and why.
- [ ] **Step 4: Verify GREEN.**
- [ ] **Step 5: Append migration Version 18** `"leads_and_identities"` with both tables and indexes, `ON DELETE` handled by `DeletionService` (not cascades — match how other tables do it).
- [ ] **Step 6: `go test ./...`** — the migration test that checks every model has a table will catch the rest.
- [ ] **Step 7: Commit** — "Add lead and identity records for candidate sourcing".

---

### Task 2: Share the preview/disclosure core with `DiscoveryService`

**Files:**
- Create: `outbound.go` (root package, unexported)
- Modify: `discoveryservice.go`
- Test: existing `discoveryservice_test.go` must stay green unchanged.

**Interfaces:**
- `type outbound struct { db *gorm.DB; credentials *CredentialService; now Clock }`
- `func (o *outbound) describe(query string, ids scrub.Identifiers) *QueryPreview`
- `func (o *outbound) record(sentAt time.Time, provider, task, categories string, refs disclosureRefs) error` — writes the `DisclosureEvent`.
- `func (o *outbound) exaClient(override Searcher) (Searcher, error)` — the body of today's `searcher()`.

- [ ] **Step 1: Extract** — move `describe`, `recordDisclosure`'s write, and `searcher` into `outbound`; `DiscoveryService` embeds `*outbound` and delegates. No behaviour change.
- [ ] **Step 2: `go test -run 'Discovery' ./`** then `go test ./...` — all green with no test edits. If a test needed editing, the extraction changed behaviour; stop and fix.
- [ ] **Step 3: Commit** — "Share the outbound preview and disclosure core between search services".

---

### Task 3: Exa people search

**Files:**
- Modify: `internal/platform/exa.go`
- Test: `internal/platform/exa_test.go`

**Interfaces:**
- `func (e *Exa) SearchPeople(ctx, query string, limit int, cursor string) (*SearchResponse, error)` — identical to `Search` but `"category": "people"`. Share the request/response code through an unexported `search(ctx, body)`; the doc comment on `Exa` ("searches listings only") is updated to name both.

- [ ] **Step 1: Failing test** — fake server asserts body `category == "people"` and that `query` is byte-for-byte what was passed; error mapping (401, 429, timeout) reuses the existing table-driven test by parameterising over both methods.
- [ ] **Step 2: RED**, **Step 3: implement**, **Step 4: GREEN**, `go test ./...`.
- [ ] **Step 5: Commit** — "Teach the Exa client to search for people".

---

### Task 4: `SourcingService` — Preview and Send

**Files:**
- Create: `sourcingservice.go`
- Modify: `main.go` (register)
- Test: `sourcingservice_test.go`

**Interfaces:**
```go
type PeopleSearcher interface {
    SearchPeople(ctx context.Context, query string, limit int, cursor string) (*platform.SearchResponse, error)
}
func NewSourcingService(db, exa PeopleSearcher, roleProfiles *RoleProfileService, records *RecordService, credentials *CredentialService) *SourcingService
func (s *SourcingService) Preview(initiativeID, roleID uint) (*QueryPreview, error)
func (s *SourcingService) Inspect(roleID uint, query string) (*QueryPreview, error)
func (s *SourcingService) Send(in SourcingSendInput) (*SourcingOutcome, error)   // {InitiativeID, RoleID, Query, Limit}
// SourcingOutcome {SearchID uint; LeadIDs []uint; Created, AlreadyInPool, Skipped int; Partial bool}
```

**Preview rules:** query = approved role profile aspects of `aspectTypesForQuery` joined with ", "; identifiers to scrub = the role's company name, every contact name/email/phone at that company, and the role title's employer suffix; then `scrub.Generalize`. Refuses when the role profile is not approved ("a query is built from an approved role profile").

**Send rules:** mirror `DiscoveryService.Send` exactly for the no-credential path (Search row with `ReasonUnauthorized`, no disclosure). On success: `Search` row (`Provider: exa`), one `DisclosureEvent` (`Task: candidate_sourcing`, `Categories: "professional requirements"` plus `"organization name"`/`"identifier"` per `Detect` warnings, `InitiativeID`, `RoleID`), then a `Lead` per result upserted on `(search_id, url)`. Dedup: a lead whose URL equals an `Identity.URL`, or whose hostname+path matches an identity's provider+handle (github.com/<login>), gets `CandidateID` set and is counted in `AlreadyInPool`.

- [ ] **Step 1: Failing tests** — (a) preview uses approved role aspects and scrubs the company and contact names; (b) preview refuses unapproved profile; (c) send with no key records a Search with `ReasonUnauthorized` and zero disclosure events; (d) send with fake searcher writes Search + one DisclosureEvent whose row has no query text anywhere (assert with `SELECT *` string scan, as `discoveryservice_test.go` does); (e) leads upsert idempotently on resend; (f) existing github identity marks the lead in-pool.
- [ ] **Step 2: RED**, **Step 3: implement**, **Step 4: GREEN**, `go test ./...`.
- [ ] **Step 5: Register** `NewSourcingService(gdb, nil, roleProfiles, records, credentials)` in `main.go`.
- [ ] **Step 6: Commit** — "Find people for a role through a previewed, scrubbed search".

---

### Task 5: `SourcingService` — Leads, Dismiss, Promote

**Files:** `sourcingservice.go`, `sourcingservice_test.go`, `deletionservice.go`, `deletionservice_test.go`

**Interfaces:**
```go
func (s *SourcingService) Leads(initiativeID uint, state string) ([]LeadView, error)  // LeadView = Lead + CandidateName + Host
func (s *SourcingService) Dismiss(leadID uint) error
func (s *SourcingService) Suggest(leadID uint) (*CandidateInput, error)   // name/location guess from title+snippet; never saved
func (s *SourcingService) Promote(leadID uint, in CandidateInput) (*models.Candidate, error)
```
Promote, in one transaction: `Candidate.Validate` → create → `Identity{Provider: providerForHost(url), Handle, URL}` → artifact via `artifacts.Create` with `Source: "exa:"+url`, `DisplayName: title`, Markdown body = title + URL + snippet, linked to the candidate → lead state `promoted`, `CandidateID` set. `SourceNote` = `"Sourced from exa on YYYY-MM-DD"`. Promoting a dismissed or already-promoted lead is refused.

`DeletionService`: `DeleteCandidate` removes its identities and nulls `leads.candidate_id`; `DeleteInitiative` removes its leads. Previews list the counts.

- [ ] **Step 1: Failing tests** — promote creates all four rows and refuses twice; `Suggest` never writes; `Dismiss` then `Leads(state="new")` excludes it; deletion cascades; delete-all leaves both tables empty (extend the existing delete-all test).
- [ ] **Step 2–4: RED → implement → GREEN**, `go test ./...`.
- [ ] **Step 5: Commit** — "Promote a lead into a candidate with its identity and evidence".

---

### Task 6: `platform.GitHub` client and fetch allowlist

**Files:**
- Create: `internal/platform/github.go`, `internal/platform/github_test.go`
- The page-fetch allowlist is NOT touched: it is pinned empty by test and by the PRD review rule. The GitHub client is a provider with a fixed endpoint, like the Exa client — it never goes through `FetchAllowed`. A test pins that `github.com` pages stay unfetchable.

**Interfaces:**
```go
const GitHubBaseURL = "https://api.github.com"
type GitHub struct { BaseURL string; Client *http.Client; Token string; Cache ETagCache }
type ETagCache interface { Get(url string) (etag string, body []byte, ok bool); Put(url, etag string, body []byte) error }
type GitHubProfile struct { Login, Name, Company, Location, Bio, Blog, Email string; Hireable bool; PublicRepos, Followers int; CreatedAt, UpdatedAt string }
type GitHubRepo struct { Name, Description, Language, URL string; Stars int; Fork bool; PushedAt string }
type GitHubEvent struct { Type, Repo, CreatedAt string }
func (g *GitHub) Profile(ctx, login) (*GitHubProfile, error)
func (g *GitHub) Repos(ctx, login) ([]GitHubRepo, error)      // own repos, forks dropped, sorted by PushedAt desc, capped 50
func (g *GitHub) Events(ctx, login) ([]GitHubEvent, error)   // public, last page only (≤100)
```
Errors reuse `ErrSearchUnauthorized/RateLimited/Timeout/Offline/Malformed`; rate-limited wraps the `X-RateLimit-Reset` time in the message. Empty token → `ErrSearchUnauthorized` before any request. Every request sends `If-None-Match` when the cache has an ETag; 304 returns the cached body. The client cannot fetch an arbitrary URL: only its three endpoints, by login.

- [ ] **Step 1: Failing tests** — fake server: auth header present; 304 path returns cached body and increments no request counter beyond the conditional request; forks filtered; 429/403-with-reset mapped; empty token refused with zero requests; a `BaseURL` on a non-allowlisted host is refused by policy.
- [ ] **Step 2–4: RED → implement → GREEN.**
- [ ] **Step 5: Commit** — "Add a GitHub client behind the fetch policy".

---

### Task 7: `EnrichService`

**Files:**
- Create: `enrichservice.go`, `enrichservice_test.go`, `internal/models/httpcache.go` (+ migration **19** `http_cache(url PK, etag, body, fetched_at)`)
- Modify: `main.go`, `credentialservice.go` (allow provider `"github"` wherever providers are enumerated), `credentialservice_test.go`

**Interfaces:**
```go
type Enricher interface { Profile(...); Repos(...); Events(...) }   // satisfied by *platform.GitHub
func NewEnrichService(db, gh Enricher, records *RecordService, artifacts *ArtifactService, credentials *CredentialService) *EnrichService
func (s *EnrichService) Identities(candidateID uint) ([]models.Identity, error)
func (s *EnrichService) AddIdentity(candidateID uint, provider, url string) (*models.Identity, error)  // handle parsed from URL
func (s *EnrichService) RemoveIdentity(id uint) error
func (s *EnrichService) Preview(candidateID uint) (*EnrichPreview, error)   // {Handles []string; Endpoints []string; TokenStored bool}
func (s *EnrichService) Run(candidateID uint) (*EnrichOutcome, error)      // {ArtifactIDs []uint; Unchanged int; Partial bool; FailureReason string}
```
`Run`: refuse before any request when no `github` identity or no token. Fetch profile → repos → events; render each to Markdown (`internal/enrich/markdown.go`, pure functions, table-tested) and `artifacts.Create` with `Source: "github:<login>/profile"` etc., `OriginalFilename` empty. A same-bytes refusal counts as `Unchanged`, not an error. One `DisclosureEvent` (`Provider: github`, `Task: candidate_enrich`, `Categories: "public handle"`, `CandidateID`) written after the first request succeeds; a run that fails on the first request writes none. A later failure marks `Partial` and keeps what landed. `Identity.VerifiedAt` set to today on a successful profile fetch.

- [ ] **Step 1: Failing tests** — refuses with no identity / no token and writes nothing; happy path yields three artifacts linked to the candidate with the right sources and one disclosure holding no login; rerun with identical fake responses yields `Unchanged == 3` and no new artifacts; rate limit on `Repos` gives one artifact, `Partial`, and one disclosure; Markdown renderer never includes email when `Email` is set (spec: not harvested); `AddIdentity` with `https://github.com/@Octocat/` stores handle `octocat`.
- [ ] **Step 2–4: RED → implement → GREEN**, `go test ./...`.
- [ ] **Step 5: Register** in `main.go`; credential UI provider list gains `github`.
- [ ] **Step 6: Commit** — "Enrich a candidate from GitHub into cited artifacts".

---

### Task 8: Bindings and the role page — Find people

**Files:**
- Regenerate `frontend/bindings/`
- Create: `frontend/src/components/PeopleSourcingPanel.tsx`, `.test.tsx`
- Modify: the role view inside the initiative workspace (where `RoleProfilePanel` is mounted) to host the new region `role="region" aria-label="Find people"`.

Behaviour mirrors `RoleDiscoveryPanel`: *Build a query* → editable `Query to send` textarea with the two warnings → *Send this search* / *Cancel this search* → results list `aria-label="Leads"` with title, host, snippet, *In pool* badge, and per-lead *Promote*, *Dismiss*, *Open* (`window.open` via the Wails browser-open runtime; never an iframe). *Promote* opens `RecordForm` prefilled from `Suggest`, saving through `Promote`. Past searches list as in discovery.

- [ ] **Step 1: `wails3 generate bindings -clean=true -ts -i`.**
- [ ] **Step 2: Failing Vitest** with mocked bindings: preview populates the box; cancel calls nothing; send renders leads and the in-pool badge; promote prefills and calls `Promote`; dismiss removes the row; backend error lands in `role="alert"`.
- [ ] **Step 3–4: RED → implement → GREEN**, `bun run test:unit`, `bunx tsc --noEmit`.
- [ ] **Step 5: Commit** — "Find people for a role from the role page".

---

### Task 9: CRM candidate detail — Identities and Enrich

**Files:**
- Create: `frontend/src/components/IdentitiesSection.tsx`, `.test.tsx`
- Modify: `CrmPanel.tsx` (candidate right pane gains the section between Details and Artifacts), Settings credentials panel (GitHub token row, same three states as Exa).

Section: list of identities (provider, handle, link), *Add identity* (provider select + URL), *Remove*, and *Enrich from GitHub* → preview modal listing handles and endpoints and whether a token is stored → *Run*. Outcome line: "3 artifacts added" / "nothing new" / partial with reason. Disabled with reason when no token or no GitHub identity.

- [ ] **Step 1: Failing Vitest** — list/add/remove; enrich disabled reasons; preview → run → outcome; settings shows GitHub row.
- [ ] **Step 2–3: RED → implement → GREEN**, `bun run test:unit`, tsc.
- [ ] **Step 4: Commit** — "Manage a candidate's identities and enrich from GitHub".

---

### Task 10: End-to-end proof

**Files:** `frontend/e2e/sourcing.spec.ts`

As `discovery.spec.ts`: create initiative → role with an approved profile (follow `role-profile.spec.ts` for how a profile gets approved in E2E) → *Find people* → query box contains the role's aspect tag and not the company name → edit → cancel records nothing → send is refused with the missing-credential message and *Past searches* shows the refused attempt. Then CRM → candidate → add a GitHub identity → *Enrich* is disabled with "no GitHub token is stored".

- [ ] **Step 1: Write the spec** with per-run base-36 tags, locators scoped to the new regions.
- [ ] **Step 2: `bunx playwright test e2e/sourcing.spec.ts`.**
- [ ] **Step 3: `just check`, commit** — "Prove candidate sourcing end to end".

---

## Self-review notes

- The disclosure row is asserted by scanning the raw row text for the query, the login, and the company name in Tasks 4 and 7 — the audit rule is a test, not a comment.
- The Exa `people` category and its result shape are assumed from the provider's public API; Task 3's fake-server test pins what *we* send, and the first live run should be checked against a real response before Task 8 is considered done. Record anything surprising in the spec.
- Commit-email harvesting is excluded by a test (Task 7), matching the spec's "not deferred, excluded".
- Migration numbers 18 and 19 assume no other branch lands a migration first; check `migrations.go` before Task 1 and Task 7.
