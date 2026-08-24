# CRM Search & Manage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Interaction history (calls, notes, placements, rejections) on CRM records, structured search over candidates/companies/contacts, a talent-pool search over evidence, and a CRM tab presenting it all — with interaction notes flowing into the existing RAG pipeline as artifacts.

**Architecture:** One new `Interaction` model whose rows each own a companion artifact carrying the note as Markdown evidence (chunked and FTS-indexed via the existing pipeline). New `InteractionService`; search methods added to `RecordService` (SQL filters) and `SearchService` (`People`, FTS grouped by candidate). New `CrmPanel` behind a new top-level `"crm"` utility tab.

**Tech Stack:** Go + GORM + glebarez/sqlite (no CGO), hand-written SQL migrations, Wails v3 bindings, SolidJS + Vitest + Playwright, Bun only.

**Spec:** `docs/superpowers/specs/2026-08-24-crm-search-manage-design.md`

## Global Constraints

- Bun is the only JS package manager/runtime. Never npm/yarn/pnpm.
- Never hand-edit `frontend/bindings/` — regenerate with `wails3 generate bindings -clean=true -ts -i`.
- Schema changes are hand-written SQL migrations in `internal/db/migrations.go` (next version: **17**). There is no AutoMigrate.
- Note text and anything derived from documents is displayed, never rendered or executed.
- Backend errors surface verbatim in the UI inside `role="alert"` elements.
- Go run: `go test ./...` from repo root. Vitest: `cd frontend && bun run test:unit`. E2E: `cd frontend && bun run test:e2e`.
- Commit messages: sentence-case imperative, no conventional-commit prefixes (match `git log`), ending with the session's `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>` and `Claude-Session: https://claude.ai/code/session_011MPS3vUP25CBrTnEKez7yS` trailers.
- Interaction targets are `candidate`, `contact`, `company`, `role` — never `initiative` (an initiative takes context via `InitiativeID`, not as a subject).

---

### Task 1: Interaction model and migration

**Files:**
- Create: `internal/models/interaction.go`
- Modify: `internal/db/migrations.go` (append migration Version 17)
- Test: `internal/models/interaction_test.go`

**Interfaces:**
- Produces: `models.Interaction` struct, `models.InteractionKinds()` (valid kinds), `(*Interaction).Validate() error`. Table `interactions`.

- [ ] **Step 1: Write the failing test**

```go
// internal/models/interaction_test.go
package models

import (
	"strings"
	"testing"
)

func validInteraction() Interaction {
	return Interaction{
		TargetType: LinkCandidate,
		TargetID:   1,
		Kind:       "call",
		Note:       "Spoke about availability.",
		OccurredAt: "2026-08-24",
	}
}

func TestInteractionValidation(t *testing.T) {
	if err := (func() error { i := validInteraction(); return i.Validate() })(); err != nil {
		t.Fatalf("valid interaction refused: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*Interaction)
		want string
	}{
		{"missing note", func(i *Interaction) { i.Note = "  " }, "note"},
		{"unknown kind", func(i *Interaction) { i.Kind = "carrier_pigeon" }, "kind"},
		{"initiative target", func(i *Interaction) { i.TargetType = LinkInitiative }, "target"},
		{"unknown target", func(i *Interaction) { i.TargetType = "spaceship" }, "target"},
		{"missing date", func(i *Interaction) { i.OccurredAt = "" }, "date"},
		{"bad date", func(i *Interaction) { i.OccurredAt = "yesterday" }, "date"},
		{"placement without role", func(i *Interaction) { i.Kind = "placement" }, "role"},
		{"rejection without role", func(i *Interaction) { i.Kind = "rejection" }, "role"},
		{"application without role", func(i *Interaction) { i.Kind = "application" }, "role"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			i := validInteraction()
			c.mut(&i)
			err := i.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error mentioning %q, got %v", c.want, err)
			}
		})
	}

	// An outcome with its role is fine.
	i := validInteraction()
	i.Kind = "placement"
	role := uint(3)
	i.RoleID = &role
	if err := i.Validate(); err != nil {
		t.Fatalf("placement with role refused: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestInteractionValidation ./internal/models/`
Expected: FAIL — `undefined: Interaction`

- [ ] **Step 3: Write the model**

```go
// internal/models/interaction.go
package models

import (
	"fmt"
	"time"
)

// Interaction is one thing that happened with a record: a call, a note, a
// placement. The recruiter's own words, so — unlike an artifact — it is
// editable; the companion artifact is replaced on every edit so search always
// reflects the current wording.
type Interaction struct {
	ID uint `gorm:"primarykey" json:"id"`
	// The record this happened with. Initiative is not a valid subject: an
	// interaction happens with a person or organisation, and the initiative it
	// happened under is context, carried by InitiativeID.
	TargetType LinkTarget `gorm:"not null" json:"targetType"`
	TargetID   uint       `gorm:"not null" json:"targetId"`
	Kind       string     `gorm:"not null" json:"kind"`
	// The recruiter's words. Free text: displayed, never rendered.
	Note string `gorm:"not null" json:"note"`
	// When it happened — distinct from CreatedAt, which is when it was logged.
	OccurredAt Date  `gorm:"not null" json:"occurredAt"`
	RoleID     *uint `json:"roleId"`
	InitiativeID *uint `json:"initiativeId"`
	// The evidence copy of the note. Owned by this row: replaced on edit,
	// deleted with it, never detached or renamed on its own.
	ArtifactID uint      `gorm:"not null" json:"artifactId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

var interactionKinds = []string{"call", "meeting", "email", "note", "placement", "application", "rejection"}

// outcomeKinds are the kinds that assert something about a role, so they
// require one.
var outcomeKinds = map[string]bool{"placement": true, "application": true, "rejection": true}

// InteractionKinds returns the valid kinds, in display order.
func InteractionKinds() []string { return interactionKinds }

// Validate normalises i in place and reports the first problem found.
func (i *Interaction) Validate() error {
	if !i.TargetType.Valid() || i.TargetType == LinkInitiative {
		return fmt.Errorf("interaction target must be a candidate, contact, company, or role")
	}
	if i.TargetID == 0 {
		return fmt.Errorf("interaction target record is required")
	}
	known := false
	for _, k := range interactionKinds {
		if i.Kind == k {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("unknown interaction kind %q", i.Kind)
	}
	var err error
	if i.Note, err = requireText("interaction note", i.Note); err != nil {
		return err
	}
	if i.OccurredAt == "" {
		return fmt.Errorf("interaction date is required")
	}
	if err := i.OccurredAt.Validate("interaction date"); err != nil {
		return err
	}
	if outcomeKinds[i.Kind] && (i.RoleID == nil || *i.RoleID == 0) {
		return fmt.Errorf("a %s needs the role it is about", i.Kind)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestInteractionValidation ./internal/models/`
Expected: PASS. (If `requireText`'s message for note doesn't contain "note", check `values.go:165` — it formats the field name in, so it will.)

- [ ] **Step 5: Append migration Version 17**

In `internal/db/migrations.go`, after the Version 16 entry (before the closing `}` of the migrations slice):

```go
	{
		Version: 17,
		Name:    "interactions",
		SQL: []string{
			"CREATE TABLE `interactions` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`target_type` text NOT NULL CHECK (`target_type` IN " +
				"('candidate','contact','company','role'))," +
				"`target_id` integer NOT NULL," +
				"`kind` text NOT NULL CHECK (`kind` IN " +
				"('call','meeting','email','note','placement','application','rejection'))," +
				"`note` text NOT NULL," +
				"`occurred_at` text NOT NULL," +
				"`role_id` integer REFERENCES `roles`(`id`)," +
				"`initiative_id` integer REFERENCES `initiatives`(`id`)," +
				"`artifact_id` integer NOT NULL REFERENCES `artifacts`(`id`)," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX `idx_interactions_target` ON " +
				"`interactions`(`target_type`,`target_id`,`occurred_at`)",
		},
	},
```

- [ ] **Step 6: Run the full Go suite**

Run: `go test ./...`
Expected: PASS (migration tests open the schema; existing suites unaffected).

- [ ] **Step 7: Commit**

`git add internal/models/interaction.go internal/models/interaction_test.go internal/db/migrations.go`
Commit: `Add the interaction record and its schema`

---

### Task 2: InteractionService — Log and Timeline

**Files:**
- Create: `interactionservice.go`
- Modify: `main.go` (register the service)
- Test: `interactionservice_test.go`

**Interfaces:**
- Consumes: `models.Interaction` (Task 1), `ChunkService.Chunk(artifactID, initiativeID uint)`, `linkWithin(tx, artifactID, targetType, targetID)` (artifactservice.go), `guardAllows`, `detectMediaType`.
- Produces: `InteractionService` with `Log(InteractionInput) (*models.Interaction, error)` and `Timeline(targetType models.LinkTarget, targetID uint) ([]TimelineEntry, error)`; `InteractionInput{ID, TargetType, TargetID, Kind, Note, OccurredAt, RoleID, InitiativeID}`; `TimelineEntry{models.Interaction; RoleTitle, InitiativeName string}`. Tasks 3, 6, 7 rely on these exact names.

- [ ] **Step 1: Write the failing test**

```go
// interactionservice_test.go
package main

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

type crmEnv struct {
	*indexEnv
	interactions *InteractionService
	candidate    uint
}

func newCrmEnv(t *testing.T) *crmEnv {
	t.Helper()
	e := newIndexEnv(t)
	cand := models.Candidate{FullName: "Quinn Sample"}
	if err := e.db.Create(&cand).Error; err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	return &crmEnv{
		indexEnv:     e,
		interactions: NewInteractionService(e.db, e.chunks),
		candidate:    cand.ID,
	}
}

func TestLoggingAnInteractionCreatesSearchableEvidence(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate,
		TargetID:   e.candidate,
		Kind:       "call",
		Note:       "Wants to move into fintech. Available from October.",
		OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if made.ArtifactID == 0 {
		t.Fatalf("interaction has no companion artifact")
	}

	// The companion artifact is linked to the candidate and already extracted:
	// the note is Markdown this application wrote, no sidecar involved.
	var artifact models.Artifact
	if err := e.db.First(&artifact, made.ArtifactID).Error; err != nil {
		t.Fatalf("loading artifact: %v", err)
	}
	if artifact.Source != "interaction" || artifact.ExtractionState != models.ExtractionExtracted {
		t.Fatalf("artifact source=%q state=%q", artifact.Source, artifact.ExtractionState)
	}
	if !strings.Contains(artifact.Markdown, "fintech") || !strings.Contains(artifact.Markdown, "Quinn Sample") {
		t.Fatalf("markdown lacks note or subject name: %q", artifact.Markdown)
	}

	// The chunk job was enqueued; once it runs, the note is findable by FTS.
	waitForLatestJob(t, e.jobs)
	hits := []models.Chunk{}
	if err := e.db.Where("artifact_id = ?", made.ArtifactID).Find(&hits).Error; err != nil || len(hits) == 0 {
		t.Fatalf("no chunks for the note (err %v)", err)
	}
}

// waitForLatestJob waits for the most recently enqueued job to finish.
func waitForLatestJob(t *testing.T, jobs *JobService) {
	t.Helper()
	all, err := jobs.List()
	if err != nil || len(all) == 0 {
		t.Fatalf("listing jobs: %v (%d jobs)", err, len(all))
	}
	newest := all[0]
	for _, j := range all {
		if j.ID > newest.ID {
			newest = j
		}
	}
	if done := waitForJob(t, jobs, newest.ID); done.State != models.JobCompleted {
		t.Fatalf("job %d finished %s: %s", newest.ID, done.State, done.FailureReason)
	}
}

func TestAnOutcomeNeedsItsRoleAndTheTimelineNamesIt(t *testing.T) {
	e := newCrmEnv(t)
	if _, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "placement", Note: "Placed.", OccurredAt: "2026-08-01",
	}); err == nil || !strings.Contains(err.Error(), "role") {
		t.Fatalf("placement without role accepted: %v", err)
	}

	role := models.Role{Title: "Staff Engineer", SourceURL: "https://example.test/r1"}
	if err := e.db.Create(&role).Error; err != nil {
		t.Fatalf("creating role: %v", err)
	}
	if _, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "placement", Note: "Placed after two rounds.", OccurredAt: "2026-08-01",
		RoleID: role.ID,
	}); err != nil {
		t.Fatalf("logging placement: %v", err)
	}
	if _, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "call", Note: "Follow-up call.", OccurredAt: "2026-08-10",
	}); err != nil {
		t.Fatalf("logging call: %v", err)
	}

	timeline, err := e.interactions.Timeline(models.LinkCandidate, e.candidate)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	if len(timeline) != 2 {
		t.Fatalf("want 2 entries, got %d", len(timeline))
	}
	// Newest first by occurred date.
	if timeline[0].Kind != "call" || timeline[1].Kind != "placement" {
		t.Fatalf("order wrong: %s then %s", timeline[0].Kind, timeline[1].Kind)
	}
	if timeline[1].RoleTitle != "Staff Engineer" {
		t.Fatalf("placement entry lacks role title: %q", timeline[1].RoleTitle)
	}
}

func TestAnInteractionDefaultsItsDateToToday(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "note", Note: "No date supplied.",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	if made.OccurredAt == "" {
		t.Fatalf("occurred date not defaulted")
	}
}

func TestAnInteractionOnAMissingTargetIsRefused(t *testing.T) {
	e := newCrmEnv(t)
	_, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: 999999,
		Kind: "note", Note: "Ghost.", OccurredAt: "2026-08-24",
	})
	if err == nil {
		t.Fatalf("interaction on missing candidate accepted")
	}
	// Nothing half-created: no interaction row, no orphaned artifact.
	var n int64
	if e.db.Model(&models.Interaction{}).Count(&n); n != 0 {
		t.Fatalf("%d interaction rows left behind", n)
	}
	var a int64
	if e.db.Model(&models.Artifact{}).Where("source = ?", "interaction").Count(&a); a != 0 {
		t.Fatalf("%d interaction artifacts left behind", a)
	}
}

var _ = gorm.ErrRecordNotFound // keep the import if unused after edits
```

Note: check `models.Role`'s required fields in `internal/models/role.go` before running — if `Validate` is not called by a direct `db.Create`, the raw insert above is fine; if the migration requires more NOT NULL columns (e.g. `source_url`), set them as shown or extend as the schema demands.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestLoggingAnInteraction|TestAnOutcomeNeeds|TestAnInteractionDefaults|TestAnInteractionOnAMissing" ./`
Expected: FAIL — `undefined: NewInteractionService`

- [ ] **Step 3: Implement the service**

```go
// interactionservice.go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/models"
)

// InteractionService records what happened with a CRM record — calls, notes,
// placements — and keeps each note's evidence copy flowing through the same
// pipeline as any other artifact. The note is the recruiter's own words, so it
// is editable; the artifact is replaced wholesale on every edit.
type InteractionService struct {
	db     *gorm.DB
	chunks *ChunkService
	Guard  DataGuard
}

// NewInteractionService returns an InteractionService backed by db.
func NewInteractionService(db *gorm.DB, chunks *ChunkService) *InteractionService {
	return &InteractionService{db: db, chunks: chunks}
}

// InteractionInput is one logged or edited interaction. Zero RoleID and
// InitiativeID mean none; an empty OccurredAt means today.
type InteractionInput struct {
	ID           uint              `json:"id"`
	TargetType   models.LinkTarget `json:"targetType"`
	TargetID     uint              `json:"targetId"`
	Kind         string            `json:"kind"`
	Note         string            `json:"note"`
	OccurredAt   models.Date       `json:"occurredAt"`
	RoleID       uint              `json:"roleId"`
	InitiativeID uint              `json:"initiativeId"`
}

// row builds the model from the input, defaulting the date to today.
func (in InteractionInput) row() models.Interaction {
	i := models.Interaction{
		ID:         in.ID,
		TargetType: in.TargetType,
		TargetID:   in.TargetID,
		Kind:       in.Kind,
		Note:       in.Note,
		OccurredAt: in.OccurredAt,
	}
	if i.OccurredAt == "" {
		i.OccurredAt = models.Date(time.Now().UTC().Format("2006-01-02"))
	}
	if in.RoleID != 0 {
		id := in.RoleID
		i.RoleID = &id
	}
	if in.InitiativeID != 0 {
		id := in.InitiativeID
		i.InitiativeID = &id
	}
	return i
}

// Log records one interaction and its evidence artifact, atomically.
func (s *InteractionService) Log(in InteractionInput) (*models.Interaction, error) {
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
	interaction := in.row()
	if err := interaction.Validate(); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		artifact, err := s.evidenceWithin(tx, &interaction)
		if err != nil {
			return err
		}
		interaction.ArtifactID = artifact.ID
		if err := tx.Create(&interaction).Error; err != nil {
			return fmt.Errorf("creating interaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Enqueued after commit: a job must never race a transaction it depends on.
	if _, err := s.chunks.Chunk(interaction.ArtifactID, in.InitiativeID); err != nil {
		return nil, fmt.Errorf("queueing the note for indexing: %w", err)
	}
	return &interaction, nil
}

// evidenceWithin creates the note's artifact inside tx and links it to the
// interaction's target. The Markdown is set here — this application wrote the
// note, so there is nothing to extract.
func (s *InteractionService) evidenceWithin(tx *gorm.DB, i *models.Interaction) (*models.Artifact, error) {
	subject, err := targetName(tx, i.TargetType, i.TargetID)
	if err != nil {
		return nil, err
	}
	header := fmt.Sprintf("# %s with %s, %s", strings.Title(i.Kind), subject, i.OccurredAt)
	if i.RoleID != nil {
		var role models.Role
		if err := tx.Select("title").First(&role, *i.RoleID).Error; err != nil {
			return nil, fmt.Errorf("loading role %d: %w", *i.RoleID, err)
		}
		header += fmt.Sprintf(" — re: %s", role.Title)
	}
	markdown := header + "\n\n" + i.Note + "\n"
	data := []byte(markdown)
	sum := sha256.Sum256(data)
	artifact := &models.Artifact{
		DisplayName:     fmt.Sprintf("%s — %s", strings.Title(i.Kind), i.OccurredAt),
		MediaType:       "text/markdown",
		ByteLength:      int64(len(data)),
		SHA256:          hex.EncodeToString(sum[:]),
		Source:          "interaction",
		CapturedAt:      time.Now().UTC(),
		Bytes:           data,
		Markdown:        markdown,
		ExtractionState: models.ExtractionExtracted,
		Extractor:       "interaction",
		ExtractorVersion: "1",
	}
	if err := tx.Create(artifact).Error; err != nil {
		return nil, fmt.Errorf("storing the note: %w", err)
	}
	if err := linkWithin(tx, artifact.ID, i.TargetType, i.TargetID); err != nil {
		return nil, err
	}
	return artifact, nil
}

// targetName loads the display name of the record an interaction is about, and
// in doing so proves the record exists.
func targetName(tx *gorm.DB, targetType models.LinkTarget, id uint) (string, error) {
	table, column := "", ""
	switch targetType {
	case models.LinkCandidate:
		table, column = "candidates", "full_name"
	case models.LinkContact:
		table, column = "contacts", "full_name"
	case models.LinkCompany:
		table, column = "companies", "name"
	case models.LinkRole:
		table, column = "roles", "title"
	default:
		return "", fmt.Errorf("interaction target must be a candidate, contact, company, or role")
	}
	var name string
	err := tx.Raw("SELECT "+column+" FROM "+table+" WHERE id = ?", id).Scan(&name).Error
	if err != nil {
		return "", fmt.Errorf("loading %s %d: %w", targetType, id, err)
	}
	if name == "" {
		return "", fmt.Errorf("%s %d does not exist", targetType, id)
	}
	return name, nil
}

// TimelineEntry is one interaction with the display names its links resolve to,
// so the panel needs no second query.
type TimelineEntry struct {
	models.Interaction
	RoleTitle      string `json:"roleTitle"`
	InitiativeName string `json:"initiativeName"`
}

// Timeline returns a record's history, newest first.
func (s *InteractionService) Timeline(targetType models.LinkTarget, targetID uint) ([]TimelineEntry, error) {
	if !targetType.Valid() || targetType == models.LinkInitiative {
		return nil, fmt.Errorf("interaction target must be a candidate, contact, company, or role")
	}
	entries := []TimelineEntry{}
	err := s.db.Raw(`
		SELECT i.*, COALESCE(r.title, '') AS role_title,
		       COALESCE(n.name, '') AS initiative_name
		FROM interactions i
		LEFT JOIN roles r ON r.id = i.role_id
		LEFT JOIN initiatives n ON n.id = i.initiative_id
		WHERE i.target_type = ? AND i.target_id = ?
		ORDER BY i.occurred_at DESC, i.id DESC`,
		targetType, targetID).Scan(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("loading the timeline: %w", err)
	}
	return entries, nil
}
```

Note: `strings.Title` is deprecated; if `golangci-lint` complains, replace with a tiny local `titleCase(kind string)` that uppercases the first byte — kinds are ASCII codes.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestLoggingAnInteraction|TestAnOutcomeNeeds|TestAnInteractionDefaults|TestAnInteractionOnAMissing" ./`
Expected: PASS.

- [ ] **Step 5: Register the service in main.go**

In `main.go`, capture the chunk service (line ~134 currently constructs it inline) and register:

```go
	chunks := NewChunkService(gdb, jobs)
	interactions := NewInteractionService(gdb, chunks)
	interactions.Guard = setupSv   // same guard wiring as records/artifacts — check how records.Guard is set and mirror it
```

and in `Services:` replace `application.NewService(NewChunkService(gdb, jobs))` with `application.NewService(chunks)` and add `application.NewService(interactions),`. Verify against how `records := NewRecordService(gdb)` and its `Guard` are wired (main.go:96 and nearby) and mirror exactly.

- [ ] **Step 6: Full suite + commit**

Run: `go test ./...` — PASS.
`git add interactionservice.go interactionservice_test.go main.go`
Commit: `Log interactions and surface a record's timeline`

---

### Task 3: InteractionService — Update and Delete

**Files:**
- Modify: `interactionservice.go`
- Test: `interactionservice_test.go`

**Interfaces:**
- Consumes: `deleteArtifactsWithin(tx *gorm.DB, ids []uint) error` (deletionservice.go:391) — deletes embeddings, chunks, links, artifacts.
- Produces: `Update(in InteractionInput) (*models.Interaction, error)` (in.ID required), `Delete(id uint) error`.

- [ ] **Step 1: Write the failing tests**

Append to `interactionservice_test.go`:

```go
func TestEditingAnInteractionReplacesItsEvidence(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "call", Note: "Interested in quokkastack roles.", OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	waitForLatestJob(t, e.jobs)
	oldArtifact := made.ArtifactID

	edited, err := e.interactions.Update(InteractionInput{
		ID: made.ID, TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "call", Note: "Interested in wombatscale roles.", OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("updating: %v", err)
	}
	waitForLatestJob(t, e.jobs)

	if edited.ArtifactID == oldArtifact {
		t.Fatalf("edit kept the old artifact")
	}
	var gone int64
	e.db.Model(&models.Artifact{}).Where("id = ?", oldArtifact).Count(&gone)
	if gone != 0 {
		t.Fatalf("old artifact survived the edit")
	}
	var stale int64
	e.db.Model(&models.Chunk{}).Where("artifact_id = ?", oldArtifact).Count(&stale)
	if stale != 0 {
		t.Fatalf("old chunks survived the edit")
	}
	// The new wording is what search finds.
	var fresh []models.Chunk
	e.db.Where("artifact_id = ?", edited.ArtifactID).Find(&fresh)
	if len(fresh) == 0 || !strings.Contains(fresh[0].Text, "wombatscale") {
		t.Fatalf("new chunks missing or stale: %+v", fresh)
	}
}

func TestDeletingAnInteractionRemovesItsEvidence(t *testing.T) {
	e := newCrmEnv(t)
	made, err := e.interactions.Log(InteractionInput{
		TargetType: models.LinkCandidate, TargetID: e.candidate,
		Kind: "note", Note: "Short-lived note.", OccurredAt: "2026-08-24",
	})
	if err != nil {
		t.Fatalf("logging: %v", err)
	}
	waitForLatestJob(t, e.jobs)

	if err := e.interactions.Delete(made.ID); err != nil {
		t.Fatalf("deleting: %v", err)
	}
	var rows, artifacts, chunks int64
	e.db.Model(&models.Interaction{}).Count(&rows)
	e.db.Model(&models.Artifact{}).Where("id = ?", made.ArtifactID).Count(&artifacts)
	e.db.Model(&models.Chunk{}).Where("artifact_id = ?", made.ArtifactID).Count(&chunks)
	if rows != 0 || artifacts != 0 || chunks != 0 {
		t.Fatalf("leftovers: %d rows, %d artifacts, %d chunks", rows, artifacts, chunks)
	}
}
```

- [ ] **Step 2: Verify RED** — `go test -run "TestEditingAnInteraction|TestDeletingAnInteraction" ./` fails with `undefined` methods.

- [ ] **Step 3: Implement**

Append to `interactionservice.go`:

```go
// Update edits an interaction and replaces its evidence artifact, so search
// always reflects the current wording. The target cannot change: a note about
// someone else is a new interaction.
func (s *InteractionService) Update(in InteractionInput) (*models.Interaction, error) {
	if err := guardAllows(s.Guard); err != nil {
		return nil, err
	}
	var existing models.Interaction
	if err := s.db.First(&existing, in.ID).Error; err != nil {
		return nil, fmt.Errorf("loading interaction %d: %w", in.ID, err)
	}
	interaction := in.row()
	interaction.TargetType, interaction.TargetID = existing.TargetType, existing.TargetID
	interaction.CreatedAt = existing.CreatedAt
	if err := interaction.Validate(); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteArtifactsWithin(tx, []uint{existing.ArtifactID}); err != nil {
			return err
		}
		artifact, err := s.evidenceWithin(tx, &interaction)
		if err != nil {
			return err
		}
		interaction.ArtifactID = artifact.ID
		if err := tx.Save(&interaction).Error; err != nil {
			return fmt.Errorf("updating interaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.chunks.Chunk(interaction.ArtifactID, in.InitiativeID); err != nil {
		return nil, fmt.Errorf("queueing the note for indexing: %w", err)
	}
	return &interaction, nil
}

// Delete removes an interaction and everything derived from its note.
func (s *InteractionService) Delete(id uint) error {
	var existing models.Interaction
	if err := s.db.First(&existing, id).Error; err != nil {
		return fmt.Errorf("loading interaction %d: %w", id, err)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteArtifactsWithin(tx, []uint{existing.ArtifactID}); err != nil {
			return err
		}
		if err := tx.Delete(&models.Interaction{}, id).Error; err != nil {
			return fmt.Errorf("deleting interaction: %w", err)
		}
		return nil
	})
}
```

- [ ] **Step 4: Verify GREEN** — targeted run passes, then `go test ./...` passes.

- [ ] **Step 5: Commit**

`git add interactionservice.go interactionservice_test.go`
Commit: `Edit and delete interactions, replacing their evidence`

---

### Task 4: RecordService structured search

**Files:**
- Modify: `recordservice.go`
- Test: `recordservice_test.go`

**Interfaces:**
- Produces: `CandidateFilter{Text, WorkRights, EmploymentType, Arrangement, AvailableBy}` (all string; AvailableBy is `models.Date`), `SearchCandidates(CandidateFilter) ([]models.Candidate, error)`, `SearchCompanies(text string) ([]models.Company, error)`, `SearchContacts(text string) ([]models.Contact, error)`. Task 6 calls these from the UI.

- [ ] **Step 1: Write the failing test**

Append to `recordservice_test.go` (mirror its existing setup helper for the db — read the file's first test for the constructor it uses):

```go
func TestSearchCandidatesFiltersByTextAndFields(t *testing.T) {
	gdb := newTestDB(t)
	s := NewRecordService(gdb)
	mk := func(name, location, rights, employment string, available models.Date) {
		t.Helper()
		_, err := s.CreateCandidate(models.Candidate{
			FullName: name, Location: location, WorkRights: rights,
			DesiredEmploymentType: employment, Availability: available,
		})
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
	}
	mk("Alice Amber", "Sydney", "citizen", "permanent", "2026-09-01")
	mk("Bob Blue", "Melbourne", "visa", "contract", "2026-12-01")
	mk("Cara Crimson", "Sydney", "citizen", "contract", "")

	got, err := s.SearchCandidates(CandidateFilter{Text: "sydney"})
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(got) != 2 || got[0].FullName != "Alice Amber" || got[1].FullName != "Cara Crimson" {
		t.Fatalf("text filter wrong: %+v", names(got))
	}

	got, _ = s.SearchCandidates(CandidateFilter{WorkRights: "citizen", EmploymentType: "contract"})
	if len(got) != 1 || got[0].FullName != "Cara Crimson" {
		t.Fatalf("field filters wrong: %+v", names(got))
	}

	// AvailableBy keeps people available on or before the date; an empty
	// availability means unknown and is kept.
	got, _ = s.SearchCandidates(CandidateFilter{AvailableBy: "2026-10-01"})
	if len(got) != 2 {
		t.Fatalf("availability filter wrong: %+v", names(got))
	}

	// No filters returns everyone, by name.
	got, _ = s.SearchCandidates(CandidateFilter{})
	if len(got) != 3 {
		t.Fatalf("unfiltered wrong: %+v", names(got))
	}
}

func names(cs []models.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.FullName
	}
	return out
}

func TestSearchCompaniesAndContactsMatchByName(t *testing.T) {
	gdb := newTestDB(t)
	s := NewRecordService(gdb)
	co, err := s.CreateCompany(models.Company{Name: "Northwind Analytics"})
	if err != nil {
		t.Fatalf("company: %v", err)
	}
	if _, err := s.CreateCompany(models.Company{Name: "Contoso"}); err != nil {
		t.Fatalf("company: %v", err)
	}
	if _, err := s.CreateContact(models.Contact{CompanyID: co.ID, FullName: "Dana Doe", Email: "dana@northwind.test"}); err != nil {
		t.Fatalf("contact: %v", err)
	}

	cos, err := s.SearchCompanies("north")
	if err != nil || len(cos) != 1 || cos[0].Name != "Northwind Analytics" {
		t.Fatalf("company search wrong: %v %+v", err, cos)
	}
	people, err := s.SearchContacts("northwind.test")
	if err != nil || len(people) != 1 || people[0].FullName != "Dana Doe" {
		t.Fatalf("contact search wrong: %v %+v", err, people)
	}
}
```

(Adjust `models.Company`/`models.Contact` required fields to what their `Validate` demands — read `internal/models/company.go` and `contact.go` first.)

- [ ] **Step 2: Verify RED** — `undefined: CandidateFilter`.

- [ ] **Step 3: Implement**

Append to `recordservice.go`:

```go
// CandidateFilter is the structured search over candidates. Text matches
// names, emails, and location; the rest are exact or range filters. This is a
// filter and behaves like one — the semantic search over evidence is
// SearchService.People, deliberately a different box in the UI.
type CandidateFilter struct {
	Text           string      `json:"text"`
	WorkRights     string      `json:"workRights"`
	EmploymentType string      `json:"employmentType"`
	Arrangement    string      `json:"arrangement"`
	AvailableBy    models.Date `json:"availableBy"`
}

// SearchCandidates returns the candidates matching every given filter, by name.
func (s *RecordService) SearchCandidates(f CandidateFilter) ([]models.Candidate, error) {
	q := s.db.Order("full_name asc")
	if t := strings.TrimSpace(f.Text); t != "" {
		like := "%" + t + "%"
		q = q.Where(
			"full_name LIKE ? COLLATE NOCASE OR preferred_name LIKE ? COLLATE NOCASE"+
				" OR emails LIKE ? COLLATE NOCASE OR location LIKE ? COLLATE NOCASE",
			like, like, like, like)
	}
	if f.WorkRights != "" {
		q = q.Where("work_rights = ?", f.WorkRights)
	}
	if f.EmploymentType != "" {
		q = q.Where("desired_employment_type = ?", f.EmploymentType)
	}
	if f.Arrangement != "" {
		q = q.Where("desired_work_arrangement = ?", f.Arrangement)
	}
	if f.AvailableBy != "" {
		if err := f.AvailableBy.Validate("available-by date"); err != nil {
			return nil, err
		}
		// Unknown availability is kept: a filter must not hide people the
		// recruiter simply has not dated yet.
		q = q.Where("availability = '' OR availability <= ?", f.AvailableBy)
	}
	out := []models.Candidate{}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("searching candidates: %w", err)
	}
	return out, nil
}

// SearchCompanies returns companies whose name matches text, by name.
func (s *RecordService) SearchCompanies(text string) ([]models.Company, error) {
	out := []models.Company{}
	q := s.db.Order("name asc")
	if t := strings.TrimSpace(text); t != "" {
		q = q.Where("name LIKE ? COLLATE NOCASE", "%"+t+"%")
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("searching companies: %w", err)
	}
	return out, nil
}

// SearchContacts returns contacts whose name or email matches text, by name.
func (s *RecordService) SearchContacts(text string) ([]models.Contact, error) {
	out := []models.Contact{}
	q := s.db.Order("full_name asc")
	if t := strings.TrimSpace(text); t != "" {
		like := "%" + t + "%"
		q = q.Where("full_name LIKE ? COLLATE NOCASE OR email LIKE ? COLLATE NOCASE", like, like)
	}
	if err := q.Find(&out).Error; err != nil {
		return nil, fmt.Errorf("searching contacts: %w", err)
	}
	return out, nil
}
```

Add `"strings"` to recordservice.go imports.

- [ ] **Step 4: Verify GREEN**, then `go test ./...`.

- [ ] **Step 5: Commit**

`git add recordservice.go recordservice_test.go`
Commit: `Filter candidates, companies, and contacts by field`

---

### Task 5: SearchService.People — talent-pool search

**Files:**
- Modify: `searchservice.go`
- Test: `searchservice_test.go`

**Interfaces:**
- Consumes: `ftsAnyQuery` (existing), chunks_fts, `artifact_links`.
- Produces: `PersonHit{Candidate models.Candidate; ChunkID uint; ArtifactName, Snippet string}` and `People(query string, limit int) ([]PersonHit, error)`. Task 6's talent search box calls it; `ChunkID` feeds the existing `Cite`.

- [ ] **Step 1: Write the failing test**

Append to `searchservice_test.go` (it uses `indexEnv` from chunkservice_test.go — same package):

```go
func TestPeopleSearchGroupsHitsByCandidate(t *testing.T) {
	e := newIndexEnv(t)
	mkCandidate := func(name string) uint {
		t.Helper()
		c := models.Candidate{FullName: name}
		if err := e.db.Create(&c).Error; err != nil {
			t.Fatalf("candidate: %v", err)
		}
		return c.ID
	}
	attach := func(candidate uint, name, markdown string) {
		t.Helper()
		a := e.extracted(t, name, markdown)
		if err := e.db.Create(&models.ArtifactLink{
			ArtifactID: a.ID, TargetType: models.LinkCandidate, TargetID: candidate,
		}).Error; err != nil {
			t.Fatalf("linking: %v", err)
		}
		e.chunkAndWait(t, a.ID)
	}

	alice := mkCandidate("Alice Amber")
	bob := mkCandidate("Bob Blue")
	attach(alice, "alice-resume", "# Resume\n\nDeep quokkastack experience across two startups.\n\nAlso quokkastack platform work.")
	attach(bob, "bob-note", "# Note\n\nSome quokkastack exposure, mostly wombatscale.")
	// An initiative-only artifact must not appear: it belongs to no candidate.
	e.chunkAndWait(t, e.extracted(t, "brief", "# Brief\n\nquokkastack quokkastack quokkastack").ID)

	hits, err := e.search.People("quokkastack", 10)
	if err != nil {
		t.Fatalf("people search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 people, got %d: %+v", len(hits), hits)
	}
	// One entry per candidate, each with a snippet and a citable chunk.
	seen := map[uint]bool{}
	for _, h := range hits {
		if seen[h.Candidate.ID] {
			t.Fatalf("candidate %d appears twice", h.Candidate.ID)
		}
		seen[h.Candidate.ID] = true
		if h.Snippet == "" || h.ChunkID == 0 || h.Candidate.FullName == "" {
			t.Fatalf("incomplete hit: %+v", h)
		}
		if _, err := e.search.Cite(h.ChunkID); err != nil {
			t.Fatalf("citing %d: %v", h.ChunkID, err)
		}
	}
	if !seen[alice] || !seen[bob] {
		t.Fatalf("missing a candidate: %v", seen)
	}
}

func TestPeopleSearchWithNoMatchesIsEmptyNotAnError(t *testing.T) {
	e := newIndexEnv(t)
	hits, err := e.search.People("zyzzyva", 10)
	if err != nil || len(hits) != 0 {
		t.Fatalf("want empty, got %v / %v", hits, err)
	}
}
```

- [ ] **Step 2: Verify RED** — `undefined` on `People`.

- [ ] **Step 3: Implement**

Append to `searchservice.go`:

```go
// PersonHit is one candidate found through their evidence: who, and the best
// piece of why. Snippet is a slice of a document a stranger may have written,
// so it is displayed and nothing else.
type PersonHit struct {
	Candidate    models.Candidate `json:"candidate"`
	ChunkID      uint             `json:"chunkId"`
	ArtifactName string           `json:"artifactName"`
	Snippet      string           `json:"snippet"`
}

// People searches the whole talent pool: every chunk whose artifact is linked
// to a candidate, in any initiative or none. One entry per candidate, ranked by
// their best chunk, that chunk carried as the "why" and citable through Cite.
//
// ponytail: FTS only, best-chunk-wins per person. If ranking disappoints, the
// upgrade is a per-person index behind this same signature.
func (s *SearchService) People(query string, limit int) ([]PersonHit, error) {
	match := ftsAnyQuery(query)
	if match == "" {
		return []PersonHit{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = defaultSearchLimit
	}
	type row struct {
		CandidateID  uint
		ChunkID      uint
		ArtifactName string
		Text         string
	}
	rows := []row{}
	// Over-fetch chunks, then keep each candidate's best: a popular candidate
	// with many matching chunks must not crowd everyone else out.
	err := s.db.Raw(`
		SELECT l.target_id AS candidate_id, c.id AS chunk_id,
		       a.display_name AS artifact_name, c.text
		FROM chunks_fts
		JOIN chunks c ON c.id = chunks_fts.rowid
		JOIN artifacts a ON a.id = c.artifact_id
		JOIN artifact_links l ON l.artifact_id = c.artifact_id AND l.target_type = ?
		WHERE chunks_fts MATCH ?
		ORDER BY bm25(chunks_fts), c.id
		LIMIT ?`,
		models.LinkCandidate, match, limit*10).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("searching people: %w", err)
	}

	hits := []PersonHit{}
	seen := map[uint]bool{}
	ids := []uint{}
	for _, r := range rows {
		if seen[r.CandidateID] {
			continue
		}
		seen[r.CandidateID] = true
		ids = append(ids, r.CandidateID)
		hits = append(hits, PersonHit{
			Candidate:    models.Candidate{ID: r.CandidateID},
			ChunkID:      r.ChunkID,
			ArtifactName: r.ArtifactName,
			Snippet:      r.Text,
		})
		if len(hits) == limit {
			break
		}
	}
	if len(ids) == 0 {
		return hits, nil
	}
	people := []models.Candidate{}
	if err := s.db.Where("id IN ?", ids).Find(&people).Error; err != nil {
		return nil, fmt.Errorf("loading matched candidates: %w", err)
	}
	byID := map[uint]models.Candidate{}
	for _, p := range people {
		byID[p.ID] = p
	}
	for i := range hits {
		hits[i].Candidate = byID[hits[i].Candidate.ID]
	}
	return hits, nil
}
```

- [ ] **Step 4: Verify GREEN**, then `go test ./...`.

- [ ] **Step 5: Commit**

`git add searchservice.go searchservice_test.go`
Commit: `Search the talent pool through its evidence`

---

### Task 6: Bindings, CRM tab, and the left pane

**Files:**
- Regenerate: `frontend/bindings/` (never hand-edit)
- Modify: `frontend/src/components/TabBar.tsx` (TabId), `frontend/src/App.tsx`
- Create: `frontend/src/components/CrmPanel.tsx`
- Test: `frontend/src/components/CrmPanel.test.tsx`, adjust `frontend/src/App.test.tsx` if it asserts the utility-tab set

**Interfaces:**
- Consumes: `RecordService.SearchCandidates/SearchCompanies/SearchContacts/ListRoles/Create*/Update*`, `SearchService.People`, `InteractionService` bindings (Task 2/3), existing `RecordForm`, `createAction` (`../act`).
- Produces: `CrmPanel` default export with internal `selected` signal `{ type: "candidate"|"company"|"contact"|"role"; id: number } | null`; Task 7 fills the right pane inside this component.

- [ ] **Step 1: Regenerate bindings**

Run from repo root: `wails3 generate bindings -clean=true -ts -i`
Expected: `frontend/bindings/camstuart/talent-hound/` gains `interactionservice.ts` exports (`InteractionService`, `InteractionInput`, `TimelineEntry`) and the new `RecordService`/`SearchService` methods. Commit separately at the end of this task.

- [ ] **Step 2: Write the failing Vitest test**

```tsx
// frontend/src/components/CrmPanel.test.tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@solidjs/testing-library";
import CrmPanel from "./CrmPanel";

// The Go backend is not running: bindings are mocked. Every fixture is invented.
const { state, recordMocks, searchMocks, interactionMocks } = vi.hoisted(() => {
  const state = {
    candidates: [
      { id: 1, fullName: "Alice Amber", location: "Sydney", emails: [], phones: [], compensation: {} },
      { id: 2, fullName: "Bob Blue", location: "Melbourne", emails: [], phones: [], compensation: {} },
    ] as Record<string, unknown>[],
    people: [] as Record<string, unknown>[],
    timeline: [] as Record<string, unknown>[],
  };
  return {
    state,
    recordMocks: {
      SearchCandidates: vi.fn(async () => state.candidates),
      SearchCompanies: vi.fn(async () => []),
      SearchContacts: vi.fn(async () => []),
      ListRoles: vi.fn(async () => []),
      GetCandidate: vi.fn(async (id: number) => state.candidates.find((c) => c.id === id)),
      CreateCandidate: vi.fn(async (c: Record<string, unknown>) => ({ id: 9, ...c })),
      UpdateCandidate: vi.fn(async (c: Record<string, unknown>) => c),
    },
    searchMocks: {
      People: vi.fn(async () => state.people),
    },
    interactionMocks: {
      Timeline: vi.fn(async () => state.timeline),
      Log: vi.fn(async (i: Record<string, unknown>) => ({ id: 5, ...i })),
      Update: vi.fn(async (i: Record<string, unknown>) => i),
      Delete: vi.fn(async () => undefined),
    },
  };
});

vi.mock("../../bindings/camstuart/talent-hound", () => ({
  RecordService: recordMocks,
  SearchService: searchMocks,
  InteractionService: interactionMocks,
}));

beforeEach(() => {
  state.people = [];
  state.timeline = [];
  vi.clearAllMocks();
});

describe("CrmPanel", () => {
  it("lists candidates from the filter search by default", async () => {
    render(() => <CrmPanel />);
    await screen.findByText("Alice Amber");
    expect(screen.getByText("Bob Blue")).toBeTruthy();
    expect(recordMocks.SearchCandidates).toHaveBeenCalled();
  });

  it("passes the typed filter to the backend", async () => {
    render(() => <CrmPanel />);
    await screen.findByText("Alice Amber");
    fireEvent.input(screen.getByLabelText("Filter"), { target: { value: "sydney" } });
    fireEvent.submit(screen.getByLabelText("Filter form"));
    await waitFor(() =>
      expect(recordMocks.SearchCandidates).toHaveBeenLastCalledWith(
        expect.objectContaining({ text: "sydney" }),
      ),
    );
  });

  it("talent search shows ranked people with their why", async () => {
    state.people = [
      {
        candidate: { id: 1, fullName: "Alice Amber" },
        chunkId: 7,
        artifactName: "alice-resume",
        snippet: "Deep quokkastack experience.",
      },
    ];
    render(() => <CrmPanel />);
    fireEvent.input(await screen.findByLabelText("Talent search"), { target: { value: "quokkastack" } });
    fireEvent.submit(screen.getByLabelText("Talent search form"));
    await screen.findByText(/Deep quokkastack experience/);
    expect(searchMocks.People).toHaveBeenCalledWith("quokkastack", 20);
  });

  it("switching record type re-queries that type", async () => {
    render(() => <CrmPanel />);
    await screen.findByText("Alice Amber");
    fireEvent.click(screen.getByRole("tab", { name: "Companies" }));
    await waitFor(() => expect(recordMocks.SearchCompanies).toHaveBeenCalled());
  });
});
```

- [ ] **Step 3: Verify RED** — `cd frontend && bunx vitest run src/components/CrmPanel.test.tsx` fails (module not found).

- [ ] **Step 4: Implement the tab plumbing**

`TabBar.tsx`: `export type TabId = number | "settings" | "help" | "crm";`

`App.tsx`:
- `UTILITY_TITLES = { settings: "Settings", help: "Help", crm: "CRM" } as const;`
- Add to the titlebar actions (before Help): `<button aria-label="CRM" aria-pressed={activeId() === "crm"} onClick={() => openInitiative("crm")}>CRM</button>`
- Add beside the Help/Settings panels: `<Show when={!needsSetup() && activeId() === "crm"}><CrmPanel /></Show>` and import it.

- [ ] **Step 5: Implement CrmPanel (left pane + selection scaffold)**

```tsx
// frontend/src/components/CrmPanel.tsx
import { createResource, createSignal, For, Show } from "solid-js";
import { RecordService, SearchService } from "../../bindings/camstuart/talent-hound";
import type { PersonHit } from "../../bindings/camstuart/talent-hound";
import { createAction } from "../act";

// The recruiter's whole pool, cross-initiative. Two searches on purpose: the
// filter answers "who matches these facts", the talent search answers "whose
// evidence talks about this" — merging them would leave both unexplainable.

export type CrmKind = "candidate" | "company" | "contact" | "role";
const KINDS: { kind: CrmKind; label: string }[] = [
  { kind: "candidate", label: "Candidates" },
  { kind: "company", label: "Companies" },
  { kind: "contact", label: "Contacts" },
  { kind: "role", label: "Roles" },
];

type Row = { id: number; title: string; subtitle: string };

const rows = async (kind: CrmKind, text: string): Promise<Row[]> => {
  switch (kind) {
    case "candidate": {
      const cs = (await RecordService.SearchCandidates({ text, workRights: "", employmentType: "", arrangement: "", availableBy: "" })) ?? [];
      return cs.map((c) => ({ id: c.id, title: c.fullName, subtitle: c.location ?? "" }));
    }
    case "company": {
      const cs = (await RecordService.SearchCompanies(text)) ?? [];
      return cs.map((c) => ({ id: c.id, title: c.name, subtitle: "" }));
    }
    case "contact": {
      const cs = (await RecordService.SearchContacts(text)) ?? [];
      return cs.map((c) => ({ id: c.id, title: c.fullName, subtitle: c.email ?? "" }));
    }
    case "role": {
      const rs = (await RecordService.ListRoles()) ?? [];
      const t = text.trim().toLowerCase();
      return rs
        .filter((r) => !t || r.title.toLowerCase().includes(t))
        .map((r) => ({ id: r.id, title: r.title, subtitle: "" }));
    }
  }
};

export default function CrmPanel() {
  const [kind, setKind] = createSignal<CrmKind>("candidate");
  const [filter, setFilter] = createSignal("");
  const [applied, setApplied] = createSignal("");
  const [selected, setSelected] = createSignal<{ type: CrmKind; id: number } | null>(null);
  const [people, setPeople] = createSignal<PersonHit[] | null>(null);
  const [talentQuery, setTalentQuery] = createSignal("");
  const { act, error } = createAction();

  const [list, { refetch }] = createResource(
    () => ({ kind: kind(), text: applied() }),
    (q) => rows(q.kind, q.text),
  );

  const runTalent = (e: Event) => {
    e.preventDefault();
    void act(async () => {
      setPeople(((await SearchService.People(talentQuery(), 20)) ?? []) as PersonHit[]);
    });
  };

  return (
    <div class="container crm" aria-label="CRM">
      <aside class="crm-list">
        <div class="area-tabs" role="tablist" aria-label="Record types">
          <For each={KINDS}>
            {(k) => (
              <button
                class="area-tab"
                classList={{ active: kind() === k.kind }}
                role="tab"
                aria-selected={kind() === k.kind}
                onClick={() => {
                  setKind(k.kind);
                  setSelected(null);
                  setPeople(null);
                }}
              >
                {k.label}
              </button>
            )}
          </For>
        </div>

        <form
          aria-label="Filter form"
          onSubmit={(e) => {
            e.preventDefault();
            setApplied(filter());
            void refetch();
          }}
        >
          <input
            aria-label="Filter"
            placeholder="Filter by name, email, location…"
            value={filter()}
            onInput={(e) => setFilter(e.currentTarget.value)}
          />
        </form>

        <Show when={kind() === "candidate"}>
          <form aria-label="Talent search form" onSubmit={runTalent}>
            <input
              aria-label="Talent search"
              placeholder="Search the talent pool's evidence…"
              value={talentQuery()}
              onInput={(e) => setTalentQuery(e.currentTarget.value)}
            />
          </form>
        </Show>

        <Show when={error()}>
          <p class="modal-error" role="alert">{error()}</p>
        </Show>

        <Show
          when={people()}
          fallback={
            <ul class="record-list" aria-label="Records">
              <For each={list() ?? []}>
                {(r) => (
                  <li
                    class="search-hit"
                    classList={{ active: selected()?.id === r.id }}
                    onClick={() => setSelected({ type: kind(), id: r.id })}
                  >
                    <span class="artifact-name">{r.title}</span>
                    <span class="muted">{r.subtitle}</span>
                  </li>
                )}
              </For>
            </ul>
          }
        >
          {(hits) => (
            <ul class="record-list" aria-label="Talent search results">
              <For each={hits()}>
                {(h) => (
                  <li class="search-hit" onClick={() => setSelected({ type: "candidate", id: h.candidate.id })}>
                    <span class="artifact-name">{h.candidate.fullName}</span>
                    <span class="muted">{h.artifactName}</span>
                    <span class="shell-note">{h.snippet}</span>
                  </li>
                )}
              </For>
              <button class="muted" onClick={() => setPeople(null)}>
                Back to the list
              </button>
            </ul>
          )}
        </Show>
      </aside>

      <section class="crm-detail" aria-label="Record detail">
        <Show when={selected()} fallback={<p class="muted">Select a record to see its details and history.</p>}>
          {(sel) => <p class="muted">{/* Task 7 replaces this */}Selected {sel().type} #{sel().id}</p>}
        </Show>
      </section>
    </div>
  );
}
```

Type note: match the generated binding types — if `SearchCandidates` takes a `CandidateFilter` class instance, construct it as the bindings export it; the object literal shown compiles against Wails' generated `$Create`-style types in this repo (check a neighbouring panel's calls for the established idiom).

- [ ] **Step 6: Verify GREEN** — `bunx vitest run src/components/CrmPanel.test.tsx`, then the full `bun run test:unit` (App.test.tsx may need the new button expectation), then `cd frontend && bunx tsc --noEmit` (or `just qa` for the TypeScript check).

- [ ] **Step 7: Commit**

`git add frontend/bindings frontend/src/components/CrmPanel.tsx frontend/src/components/CrmPanel.test.tsx frontend/src/components/TabBar.tsx frontend/src/App.tsx frontend/src/App.test.tsx`
Commit: `Open a CRM tab with filtered and talent-pool search`

---

### Task 7: The right pane — details, artifacts, history, profile

**Files:**
- Modify: `frontend/src/components/CrmPanel.tsx`, `frontend/src/components/RecordForm.tsx` (optional `initial` prop), `frontend/src/components/ArtifactsPanel.tsx` (target prop), `frontend/src/components/WorkspaceAreas.tsx` (updated ArtifactsPanel call)
- Test: `frontend/src/components/CrmPanel.test.tsx`, `frontend/src/components/RecordForm.test.tsx`, `frontend/src/components/ArtifactsPanel.test.tsx`

**Interfaces:**
- Consumes: `InteractionService.Timeline/Log/Update/Delete`, `RecordService.Get*/Update*`, `CandidateProfileService.InUse(candidateId)`, `ArtifactService.ListForTarget/Create/Detach/Link`.
- Produces: `RecordForm` accepts `initial?: Record<string, string>`; `ArtifactsPanel` accepts `target?: { type: LinkTarget; id: number }` (defaults to the initiative when only `initiativeId` is given).

- [ ] **Step 1: Write the failing tests**

RecordForm — append to `RecordForm.test.tsx`:

```tsx
it("prefills from initial values for editing", async () => {
  const onSubmit = vi.fn(async () => undefined);
  render(() => (
    <RecordForm
      legend="Edit"
      fields={[{ key: "fullName", label: "Full name", required: true }]}
      submitLabel="Save"
      initial={{ fullName: "Alice Amber" }}
      onSubmit={onSubmit}
    />
  ));
  expect((screen.getByLabelText("Full name") as HTMLInputElement).value).toBe("Alice Amber");
});
```

ArtifactsPanel — append to `ArtifactsPanel.test.tsx` (mirror its existing mocks):

```tsx
it("scopes to an explicit target when one is given", async () => {
  render(() => <ArtifactsPanel target={{ type: LinkTarget.LinkCandidate, id: 7 }} />);
  await waitFor(() =>
    expect(artifactMocks.ListForTarget).toHaveBeenCalledWith(LinkTarget.LinkCandidate, 7),
  );
});
```

CrmPanel — append to `CrmPanel.test.tsx` (state.timeline fixtures as needed):

```tsx
it("shows the timeline for the selected record and logs a new interaction", async () => {
  state.timeline = [
    { id: 4, kind: "call", note: "Talked availability.", occurredAt: "2026-08-20", roleTitle: "", initiativeName: "" },
  ];
  render(() => <CrmPanel />);
  fireEvent.click(await screen.findByText("Alice Amber"));
  await screen.findByText(/Talked availability/);

  fireEvent.input(screen.getByLabelText("Interaction note"), { target: { value: "Sent the brief." } });
  fireEvent.submit(screen.getByLabelText("Log interaction form"));
  await waitFor(() =>
    expect(interactionMocks.Log).toHaveBeenCalledWith(
      expect.objectContaining({ targetType: "candidate", targetId: 1, note: "Sent the brief." }),
    ),
  );
  await waitFor(() => expect(interactionMocks.Timeline).toHaveBeenCalledTimes(2));
});

it("an outcome kind requires picking a role in the form", async () => {
  render(() => <CrmPanel />);
  fireEvent.click(await screen.findByText("Alice Amber"));
  await screen.findByLabelText("Interaction kind");
  fireEvent.change(screen.getByLabelText("Interaction kind"), { target: { value: "placement" } });
  await screen.findByLabelText("Interaction role");
});

it("edits the selected record through the details form", async () => {
  render(() => <CrmPanel />);
  fireEvent.click(await screen.findByText("Alice Amber"));
  const name = await screen.findByLabelText("Full name");
  fireEvent.input(name, { target: { value: "Alice A. Amber" } });
  fireEvent.submit(screen.getByLabelText("Details form"));
  await waitFor(() =>
    expect(recordMocks.UpdateCandidate).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1, fullName: "Alice A. Amber" }),
    ),
  );
});
```

Add `CandidateProfileService: { InUse: vi.fn(async () => null) }` and `ArtifactService` mocks to the CrmPanel `vi.mock` block as the implementation demands.

- [ ] **Step 2: Verify RED** — the three test files fail on missing props/labels.

- [ ] **Step 3: Implement**

`RecordForm.tsx`: add `initial?: Record<string, string>` to `Props`; change `initial(fields)` calls to merge: `{ ...initial(props.fields), ...(props.initial ?? {}) }` (both the signal seed and the post-submit reset).

`ArtifactsPanel.tsx`: add `target?: { type: LinkTarget; id: number }` to props; introduce `const target = () => props.target ?? { type: LinkTarget.LinkInitiative, id: props.initiativeId! };` and replace every `LinkTarget.LinkInitiative, props.initiativeId` pair with `target().type, target().id`. `ExtractService.Extract(a.id, ...)` keeps `props.initiativeId ?? 0` — extraction of CRM-target uploads runs jobless of an initiative. `WorkspaceAreas.tsx` call site stays `<ArtifactsPanel initiativeId={props.initiativeId} />` (unchanged behaviour).

`CrmPanel.tsx` right pane — replace the Task 6 placeholder with a `Detail` component in the same file:

- **Details:** `createResource(selected, load)` fetching via `GetCandidate`/`GetCompany`/`GetContact`/`GetRole`; a `RecordForm` per type with `initial` mapped from the record and `onSubmit` calling the matching `Update*` with `{ id: sel().id, ...values }` (numbers parsed where the model wants them — copy the field specs from `RecordsPanel.tsx` so create and edit stay identical).
- **Artifacts:** `<ArtifactsPanel target={{ type: sel().type as LinkTarget, id: sel().id }} />` (CrmKind values equal LinkTarget values by construction).
- **History:** list from `InteractionService.Timeline(sel().type, sel().id)`; each entry shows kind, occurredAt, note, and `roleTitle`/`initiativeName` when present, with Edit (prefills the form, submits `Update`) and Delete buttons. The log form: `select` `aria-label="Interaction kind"` over `["call","meeting","email","note","placement","application","rejection"]`, `textarea` `aria-label="Interaction note"`, `input type="date"` `aria-label="Interaction date"`, and — shown only for placement/application/rejection — a `select` `aria-label="Interaction role"` fed by `RecordService.ListRoles()`. Submit calls `InteractionService.Log({ targetType: sel().type, targetId: sel().id, kind, note, occurredAt, roleId, initiativeId: 0 })` then refetches the timeline. Errors land in the pane's `role="alert"`.
- **Profile** (candidates only): `CandidateProfileService.InUse(sel().id)`; when a profile exists, list `profile.aspects` read-only (`wording` + `type`); otherwise a muted "No approved profile yet." line.

- [ ] **Step 4: Verify GREEN** — the three files, then `bun run test:unit` and the tsc check.

- [ ] **Step 5: Commit**

`git add frontend/src/components/CrmPanel.tsx frontend/src/components/CrmPanel.test.tsx frontend/src/components/RecordForm.tsx frontend/src/components/RecordForm.test.tsx frontend/src/components/ArtifactsPanel.tsx frontend/src/components/ArtifactsPanel.test.tsx frontend/src/components/WorkspaceAreas.tsx`
Commit: `Show a record's details, evidence, and history in the CRM tab`

---

### Task 8: End-to-end proof

**Files:**
- Create: `frontend/e2e/crm.spec.ts`

**Interfaces:**
- Consumes: the whole stack via the server build (`wails3 task run:server DEV=true`, started by playwright.config.ts).

- [ ] **Step 1: Write the spec**

```ts
// frontend/e2e/crm.spec.ts
import { test, expect } from "@playwright/test";

// Specs run in parallel against one shared backend: names are per-run unique
// and locators are scoped to this spec's own rows.
const stamp = Date.now();

test("a logged call becomes findable evidence with a visible history", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("CRM").click();

  // Create the candidate through the CRM's own form.
  const name = `Casey Quokka ${stamp}`;
  await page.getByLabel("New candidate").click();
  await page.getByLabel("Full name").fill(name);
  await page.getByRole("button", { name: "Create candidate" }).click();
  await page.getByText(name).click();

  // Log a call whose wording is unique to this run.
  const phrase = `prefers wombatscale-${stamp} platforms`;
  await page.getByLabel("Interaction note").fill(`Casey ${phrase}.`);
  await page.getByLabel("Log interaction form").getByRole("button").click();
  await expect(page.getByText(phrase)).toBeVisible();

  // The note is talent-search evidence once the chunk job has run.
  await expect(async () => {
    await page.getByLabel("Talent search").fill(`wombatscale-${stamp}`);
    await page.getByLabel("Talent search form").press("Enter");
    await expect(page.getByLabel("Talent search results").getByText(name)).toBeVisible({ timeout: 2000 });
  }).toPass({ timeout: 20000 });
});

test("an outcome names its role in the timeline", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("CRM").click();

  const name = `Robin Wombat ${stamp}`;
  await page.getByLabel("New candidate").click();
  await page.getByLabel("Full name").fill(name);
  await page.getByRole("button", { name: "Create candidate" }).click();
  await page.getByText(name).click();

  await page.getByLabel("Interaction kind").selectOption("placement");
  // Role select appears for outcomes; needs at least one role to exist.
  // Create one through the CRM Roles form first if the select is empty —
  // adapt to the Roles create form's actual labels from RecordsPanel.
  await expect(page.getByLabel("Interaction role")).toBeVisible();
});
```

Adapt labels ("New candidate", "Create candidate") to the ones Task 6/7 actually rendered — the spec is written against the plan's labels; fix the spec, not the labels, if they already ship differently. The second test should create a role first via the Roles type tab (per-run-unique title), then complete the placement log and assert the role title shows in the timeline entry.

- [ ] **Step 2: Run it**

Run: `cd frontend && bunx playwright test e2e/crm.spec.ts`
Expected: PASS (first run downloads nothing; server build starts via config).

- [ ] **Step 3: Full check + commit**

Run: `just check` (qa + all three test layers).
`git add frontend/e2e/crm.spec.ts`
Commit: `Prove the CRM flow end to end`

---

## Self-review notes

- Spec coverage: model+migration (T1), evidence flow incl. edit/delete (T2/T3), structured search (T4), talent search (T5), CRM tab both panes (T6/T7), E2E (T8). Embedding of note chunks is deliberately lazy (initiative `EmbedAll` picks them up when linked); talent search is FTS — matches the spec's Approach A and its "later upgrade" clause.
- Type consistency: `InteractionInput`/`TimelineEntry`/`PersonHit`/`CandidateFilter` names match across tasks; `CrmKind` values equal `LinkTarget` string values.
- Known adaptation points are marked inline: `models.Role` required fields (T2), Wails binding call idiom (T6), E2E labels (T8).
