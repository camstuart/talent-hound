# Candidate Sourcing from Web Data — Design

**Date:** 2026-08-26
**Status:** Brainstormed; awaiting approval before implementation planning.

## Goal

Let the recruiter grow the CRM's talent pool from public web data: find
people who look like a fit for a role, and enrich people already in the pool
with their public professional footprint. Exa (already integrated) does the
finding; GitHub does the enriching. Nothing becomes a `Candidate` without the
recruiter promoting it, and every fact that arrives from the web keeps its
source.

The existing discovery flow runs the other way — candidate → roles. This is
role → people, and the privacy shape differs: the disclosure is the *role's*
requirements at search time, and a *person's handle* at enrichment time.

## What already exists (reused, not rebuilt)

- `DiscoveryService`: preview → scrub → confirm → `Send`, writing a
  `Search` row and a "that, not what" `DisclosureEvent`. The same
  four-step shape is used here with a new task.
- `platform.Exa` client with typed failure modes and a deny-by-default
  fetch policy (`ErrFetchNotAllowed`). Sourcing never fetches a page the
  policy does not allow; Exa's `contents` is the only page text used.
- `CredentialService` (with the Windows credential gate) for API keys.
- `Artifact` → extract → chunk → embed → classify → versioned `Profile`
  with `ProfileAspect` evidence. Enrichment feeds this pipeline; it does
  not write skills or employment onto `Candidate` directly (the model's own
  comment says those are evidence-backed aspects, not fields).
- `RecordService`, `CrmPanel` master-detail, `DeletionService`.

## Decisions (made during brainstorming)

1. **Two providers, two jobs.** Exa discovers people (category `people`,
   profile pages, personal sites, speaker pages, HN "who wants to be hired"
   posts). GitHub enriches a known handle. GitHub user search is not used
   for discovery in this phase: its qualifiers are too coarse to be worth a
   disclosure.
2. **Leads, not candidates.** A search result is a `Lead`. Candidates are
   shared, owned-by-nobody records; auto-creating them from web hits would
   pollute the pool and create personal-data records nobody asked for.
   Promotion is an explicit recruiter act.
3. **Enrichment is opt-in per candidate**, happens on promote or on demand,
   and lands as artifacts. Commit-author email harvesting is off by default
   and labelled when on.
4. **Identity is a first-class link.** A small `Identity` table (provider,
   handle, URL) on a candidate is how dedup and re-enrichment find the
   same person again; no free-text handles in `SourceNote`.
5. **LinkedIn is surfaced, never fetched.** Exa may return a LinkedIn URL
   as a lead; the app never requests linkedin.com itself. The fetch policy
   already refuses it.
6. **Disclosure model extended, not bypassed.** Two new tasks; the audit
   row still records *that*, never *what*.

## Data model

New `internal/models/lead.go`, with a hand-written migration in
`internal/db/migrations.go` (Version 18; there is no AutoMigrate):

```go
type Lead struct {
    ID           uint   `gorm:"primarykey"`
    SearchID     uint   `gorm:"not null;index"`     // the Search that produced it
    InitiativeID uint   `gorm:"not null;index"`
    RoleID       *uint
    Provider     string `gorm:"not null"`           // exa
    SourceID     string                             // provider's own id, if any
    URL          string `gorm:"not null;index"`
    Title        string
    Snippet      string                             // provider text; displayed, never rendered
    State        string `gorm:"not null;default:'new'"` // new, promoted, dismissed
    CandidateID  *uint                              // set on promote; also set when dedup finds one
    CreatedAt, UpdatedAt time.Time
}
```

New `internal/models/identity.go`:

```go
type Identity struct {
    ID          uint   `gorm:"primarykey"`
    CandidateID uint   `gorm:"not null;index"`
    Provider    string `gorm:"not null"`   // github, website, linkedin, hn
    Handle      string `gorm:"not null"`   // login, domain, or profile slug
    URL         string `gorm:"not null"`
    VerifiedAt  Date                       // last time the provider confirmed it exists
    CreatedAt, UpdatedAt time.Time
}
// unique (provider, handle)
```

`discovery.go` additions:

```go
TaskCandidateSourcing = "candidate_sourcing"   // role requirements → Exa
TaskCandidateEnrich   = "candidate_enrich"     // one handle → GitHub
ProviderGitHub        = "github"
```

`DisclosureEvent.Categories` for enrichment is `"public handle"`; for
sourcing it is `"professional requirements"`, as today.

Enrichment output is not a new model. Each GitHub fetch becomes an
`Artifact` on the candidate (`Source: "github:<login>/profile"`,
`"github:<login>/repos"`, `"github:<login>/activity"`) with `CapturedAt`
set, so it flows through extract → chunk → embed and is citable. Re-running
enrichment with unchanged bytes is refused as a repeat upload by the
existing same-target rule, which is the desired behaviour.

## Flows

**Sourcing (role → leads)**

1. Recruiter opens a role inside an initiative and chooses *Find people*.
2. `SourcingService.Preview(initiativeID, roleID)` builds the query from
   the approved role profile, scrubbing the company name, contact names,
   and any client-identifying text via the existing `scrub` package; shows
   exactly what will be sent.
3. `Send` posts to Exa (`category: people`, `numResults` capped, optional
   `includeDomains`), writes `Search` + `DisclosureEvent`, upserts `Lead`
   rows keyed by `(SearchID, URL)`.
4. Dedup pass: a lead whose URL or extracted handle matches an `Identity`,
   or whose title matches a candidate's name and location, gets
   `CandidateID` set and is shown as *already in pool*.

**Promote (lead → candidate)**

1. Recruiter picks a lead; `Promote(leadID, CandidateInput)` opens the
   existing `RecordForm` pre-filled with name/location parsed from the
   snippet, which the recruiter corrects before saving.
2. Creates the `Candidate`, an `Identity` for the lead URL, an `Artifact`
   holding the Exa snippet (`Source: "exa:<url>"`), and marks the lead
   `promoted`. `SourceNote` is set to the provider and date.
3. If a GitHub identity was detected, offers enrichment; does not run it
   unasked.

**Enrich (candidate → artifacts)**

1. `EnrichService.Preview(candidateID)` lists the identities that will be
   disclosed (just the handle) and the endpoints that will be called.
2. `Run` calls `GET /users/{login}`, `/users/{login}/repos` (own, not
   forks, sorted by push), and `/users/{login}/events/public`; GraphQL
   `contributionsCollection` is a later addition. ETags cached in a small
   `HTTPCache` table so repeats cost no quota. Writes one
   `DisclosureEvent` for the run.
3. Each response is rendered to Markdown by the app (not stored as raw
   JSON) and ingested as an artifact. The classify step then proposes
   `ProfileAspect`s — languages, activity recency, notable repos — as a
   new profile version the recruiter approves like any other.

## Services

**`SourcingService`** (`sourcingservice.go`): `Preview`, `Send`, `Leads(
initiativeID, state)`, `Dismiss(leadID)`, `Promote(leadID, input)`. Mirrors
`DiscoveryService` closely enough that the scrub/preview/disclosure code is
shared, not copied; extract the common part into an unexported helper if
the second copy would otherwise appear.

**`EnrichService`** (`enrichservice.go`): `Preview(candidateID)`,
`Run(candidateID)`, `Identities(candidateID)`, `AddIdentity`,
`RemoveIdentity`. Owns the `platform.GitHub` client.

**`platform.GitHub`**: token from `CredentialService` (unauthenticated is
60 req/hr and refused up front, not discovered mid-run); typed errors
mirroring the Exa set; rate-limit headers surfaced as `ErrSearchRateLimited`
with the reset time. The fetch allow-list gains `api.github.com` only.

**`DeletionService`**: deleting a candidate deletes its identities and
leads pointing at it; deleting an initiative deletes its leads. Delete-all
must learn the two new tables — a test pins that a delete-all leaves
them empty.

## UI

- **Role page** (initiative workspace): *Find people* button → preview
  modal (same component as role discovery) → results list of leads with
  title, domain, snippet, and *In pool* badge. Per lead: *Promote*,
  *Dismiss*, *Open* (external browser; the app never renders the page).
- **CRM tab, candidate detail**: new **Identities** section (list, add,
  remove) and an *Enrich from GitHub* action with its own preview. Enriched
  artifacts appear in the existing Artifacts and Profile sections with
  their `Source` visible.
- **Settings → Credentials**: GitHub token beside the Exa key, with the
  same stored/absent/invalid states.

## Privacy and policy

- Nothing is stored about a person the recruiter has not promoted except
  the lead row (URL, title, snippet). Dismissed leads are purged after 30
  days by the existing housekeeping job; a test pins the window.
- Every web-derived fact is an artifact with a source and capture time, so
  a subject-access or deletion request can be answered from the record.
- The audit trail holds provider, task, category, and record ids — not the
  query, not the handle, not the results.
- Direct fetching stays deny-by-default; the only additions are Exa
  `contents` (already allowed) and `api.github.com`.

## Error handling

- Provider failures reuse the Exa error vocabulary; a rate-limited GitHub
  run reports the reset time and keeps whatever artifacts already landed —
  a partial run is shown as partial.
- Missing token: the *Enrich* action is disabled with the reason, and
  `Run` refuses before any disclosure is recorded.
- Promote with a name the recruiter blanked out fails on `Candidate.
  Validate` like any create.

## Testing

- **Go:** `sourcingservice_test.go` — preview scrubs client identifiers;
  `Send` writes Search/Disclosure/Leads; dedup links to an existing
  identity; promote creates candidate + identity + artifact; failure before
  send records no disclosure. `enrichservice_test.go` with a fake GitHub
  transport — artifacts per endpoint, ETag 304 costs no artifact, missing
  token refuses early, rate limit yields partial. `deletionservice_test.go`
  — cascade rules. Model tests for `Lead.Validate`/`Identity.Validate`.
- **Vitest:** lead list states, promote form prefill, identity section,
  enrich preview.
- **Playwright:** as `discovery.spec.ts` does, with no provider key stored:
  preview builds and scrubs the query, cancel records nothing, send is
  refused with the missing-credential message and no disclosure. Promote needs a
  lead, and a lead needs a provider, so promote and identity management are
  proved in Go and Vitest rather than end to end.

## Out of scope

- GitHub-native discovery (contributors of a repo, stargazers, code
  search) — a natural second step once leads exist.
- Commit-email harvesting — deliberately excluded rather than deferred;
  revisit only with an explicit product decision.
- Any LinkedIn fetching or scraping.
- Automatic re-enrichment on a schedule.
- Ranking leads against the role profile via `Match` — use the existing
  assessment after promotion instead.
