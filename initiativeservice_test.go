package main

import (
	"strings"
	"testing"

	"gorm.io/gorm"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
)

// All fixtures below are invented. Real candidate information never enters this
// repository, its logs, or its test output.

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	return gdb
}

func newTestService(t *testing.T) *InitiativeService {
	t.Helper()
	return NewInitiativeService(newTestDB(t))
}

// newServices returns both services over one database, for the tests that need
// an initiative and the records it references.
func newServices(t *testing.T) (*InitiativeService, *RecordService) {
	t.Helper()
	gdb := newTestDB(t)
	return NewInitiativeService(gdb), NewRecordService(gdb)
}

// aCandidate creates a throwaway candidate and returns its ID.
func aCandidate(t *testing.T, r *RecordService) uint {
	t.Helper()
	c, err := r.CreateCandidate(models.Candidate{FullName: "Priya Raman"})
	if err != nil {
		t.Fatalf("creating candidate: %v", err)
	}
	return c.ID
}

func TestInitiativeCreateAcceptsEveryValidTypeAndRejectsTheRest(t *testing.T) {
	s, r := newServices(t)
	candidateID := aCandidate(t, r)

	cases := []struct {
		name       string
		initiative string
		typ        models.InitiativeType
		candidates []uint
		wantErr    string
	}{
		{name: "job search with one candidate", initiative: "Find a Go role", typ: models.InitiativeTypeJobSearch, candidates: []uint{candidateID}},
		{name: "talent search", initiative: "Hire a designer", typ: models.InitiativeTypeTalentSearch},
		{name: "business development", initiative: "Partnerships", typ: models.InitiativeTypeBusinessDevelopment},
		{name: "unknown type", initiative: "Mystery", typ: models.InitiativeType("bogus"), wantErr: "unknown initiative type"},
		{name: "empty type", initiative: "Mystery", typ: models.InitiativeType(""), wantErr: "unknown initiative type"},
		{name: "empty name", initiative: "", typ: models.InitiativeTypeTalentSearch, wantErr: "must not be empty"},
		{name: "whitespace name", initiative: "   \t ", typ: models.InitiativeTypeTalentSearch, wantErr: "must not be empty"},
		{name: "job search with no candidate", initiative: "No candidate", typ: models.InitiativeTypeJobSearch, wantErr: "exactly one candidate"},
		{name: "job search with two candidates", initiative: "Two candidates", typ: models.InitiativeTypeJobSearch, candidates: []uint{candidateID, candidateID}, wantErr: "exactly one candidate"},
		{name: "job search with a missing candidate", initiative: "Ghost", typ: models.InitiativeTypeJobSearch, candidates: []uint{9999}, wantErr: "does not exist"},
		{name: "talent search with a candidate", initiative: "Unwanted candidate", typ: models.InitiativeTypeTalentSearch, candidates: []uint{candidateID}, wantErr: "does not take a candidate"},
	}

	created := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.Create(tc.initiative, tc.typ, tc.candidates)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Create(%q, %q) succeeded, want error containing %q", tc.initiative, tc.typ, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}
			created++
			if got.ID == 0 {
				t.Error("Create did not assign an ID")
			}
			if got.Status != models.InitiativeActive {
				t.Errorf("new initiative status is %q, want active", got.Status)
			}
			if got.CreatedAt.IsZero() {
				t.Error("Create did not set a creation time")
			}
			if tc.typ == models.InitiativeTypeJobSearch {
				if got.CandidateID == nil || *got.CandidateID != candidateID {
					t.Errorf("job search candidate is %v, want %d", got.CandidateID, candidateID)
				}
			} else if got.CandidateID != nil {
				t.Errorf("%s initiative got candidate %v", tc.typ, *got.CandidateID)
			}
		})
	}

	list, err := s.List(true)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != created {
		t.Errorf("database holds %d initiatives, want the %d that were accepted", len(list), created)
	}
}

func TestInitiativeCreateTrimsName(t *testing.T) {
	s := newTestService(t)

	got, err := s.Create("  Partnerships  ", models.InitiativeTypeBusinessDevelopment, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got.Name != "Partnerships" {
		t.Errorf("Create did not trim name: %q", got.Name)
	}
}

func TestInitiativeCreateReferencesCandidateWithoutCopying(t *testing.T) {
	s, r := newServices(t)
	candidateID := aCandidate(t, r)

	one, err := s.Create("Search A", models.InitiativeTypeJobSearch, []uint{candidateID})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	two, err := s.Create("Search B", models.InitiativeTypeJobSearch, []uint{candidateID})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if *one.CandidateID != *two.CandidateID {
		t.Fatal("two initiatives referencing one candidate hold different IDs")
	}

	// Update through the record, and both initiatives see it: one row, shared.
	updated, err := r.GetCandidate(candidateID)
	if err != nil {
		t.Fatalf("GetCandidate: %v", err)
	}
	updated.Location = "Wellington"
	saved, err := r.UpdateCandidate(*updated)
	if err != nil {
		t.Fatalf("UpdateCandidate: %v", err)
	}
	if saved.Location != "Wellington" {
		t.Errorf("UpdateCandidate returned %q", saved.Location)
	}
	candidates, err := r.ListCandidates()
	if err != nil {
		t.Fatalf("ListCandidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidate was copied: %d rows", len(candidates))
	}
	if candidates[0].Location != "Wellington" {
		t.Errorf("candidate location is %q, want Wellington", candidates[0].Location)
	}
}

func TestInitiativeRename(t *testing.T) {
	s := newTestService(t)
	created, err := s.Create("Original", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	other, err := s.Create("Taken", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	renamed, err := s.Rename(created.ID, "  Renamed  ")
	if err != nil {
		t.Fatalf("Rename returned error: %v", err)
	}
	if renamed.Name != "Renamed" {
		t.Errorf("Rename stored %q", renamed.Name)
	}
	if renamed.Type != created.Type || renamed.Status != created.Status || !renamed.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("Rename changed more than the name: %+v", renamed)
	}

	if _, err := s.Rename(created.ID, "   "); err == nil {
		t.Error("Rename accepted a blank name")
	}
	if reloaded, err := s.Get(created.ID); err != nil || reloaded.Name != "Renamed" {
		t.Errorf("rejected rename changed the stored name: %v, %v", reloaded, err)
	}

	// A name is a label, not an identifier: duplicates are fine.
	if _, err := s.Rename(created.ID, other.Name); err != nil {
		t.Errorf("Rename rejected a duplicate name: %v", err)
	}
	if _, err := s.Rename(9999, "Nowhere"); err == nil {
		t.Error("Rename accepted an unknown initiative")
	}
}

func TestInitiativeLifecycleTransitions(t *testing.T) {
	s, r := newServices(t)
	candidateID := aCandidate(t, r)
	created, err := s.Create("Find a Go role", models.InitiativeTypeJobSearch, []uint{candidateID})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	archived, err := s.Archive(created.ID)
	if err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}
	if archived.Status != models.InitiativeArchived {
		t.Errorf("status after archive is %q", archived.Status)
	}
	if archived.CandidateID == nil || *archived.CandidateID != candidateID {
		t.Error("archiving dropped the candidate reference")
	}
	if _, err := r.GetCandidate(candidateID); err != nil {
		t.Errorf("archiving removed the referenced candidate: %v", err)
	}

	if _, err := s.Archive(created.ID); err == nil {
		t.Error("Archive accepted an already-archived initiative")
	}

	reopened, err := s.Reopen(created.ID)
	if err != nil {
		t.Fatalf("Reopen returned error: %v", err)
	}
	if reopened.Status != models.InitiativeActive {
		t.Errorf("status after reopen is %q", reopened.Status)
	}
	if reopened.CandidateID == nil || *reopened.CandidateID != candidateID {
		t.Error("reopening dropped the candidate reference")
	}
	if _, err := s.Reopen(created.ID); err == nil {
		t.Error("Reopen accepted an already-active initiative")
	}
	if _, err := s.Archive(9999); err == nil {
		t.Error("Archive accepted an unknown initiative")
	}
}

func TestInitiativeListSeparatesActiveFromArchived(t *testing.T) {
	s := newTestService(t)
	keep, err := s.Create("Active one", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	gone, err := s.Create("Archived one", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := s.Archive(gone.ID); err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	active, err := s.List(false)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(active) != 1 || active[0].ID != keep.ID {
		t.Errorf("default listing returned %+v, want only the active initiative", active)
	}

	all, err := s.List(true)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("listing with archived returned %d, want 2", len(all))
	}
	if all[0].ID != keep.ID || all[1].ID != gone.ID {
		t.Error("listing is not oldest-first")
	}
}

func TestInitiativeDeleteLeavesSharedRecords(t *testing.T) {
	s, r := newServices(t)
	candidateID := aCandidate(t, r)
	company, err := r.CreateCompany(models.Company{Name: "Northwind Robotics"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	contact, err := r.CreateContact(models.Contact{CompanyID: company.ID, FullName: "Dana Okafor"})
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}
	role, err := r.CreateRole(models.Role{
		Title:          "Staff Engineer",
		CompanyID:      &company.ID,
		Origin:         models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}

	first, err := s.Create("Search A", models.InitiativeTypeJobSearch, []uint{candidateID})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	second, err := s.Create("Search B", models.InitiativeTypeJobSearch, []uint{candidateID})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := s.Archive(second.ID); err != nil {
		t.Fatalf("Archive returned error: %v", err)
	}

	if err := s.Delete(first.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := s.Get(first.ID); err == nil {
		t.Error("deleted initiative is still readable")
	}

	// Every shared record survives, and the other initiative still resolves it.
	if _, err := r.GetCandidate(candidateID); err != nil {
		t.Errorf("deleting an initiative removed the candidate: %v", err)
	}
	if _, err := r.GetCompany(company.ID); err != nil {
		t.Errorf("deleting an initiative removed the company: %v", err)
	}
	if _, err := r.GetContact(contact.ID); err != nil {
		t.Errorf("deleting an initiative removed the contact: %v", err)
	}
	if _, err := r.GetRole(role.ID); err != nil {
		t.Errorf("deleting an initiative removed the role: %v", err)
	}
	survivor, err := s.Get(second.ID)
	if err != nil {
		t.Fatalf("the other initiative is gone: %v", err)
	}
	if survivor.CandidateID == nil || *survivor.CandidateID != candidateID {
		t.Error("the surviving initiative lost its candidate reference")
	}

	// An archived initiative deletes under the same rules.
	if err := s.Delete(second.ID); err != nil {
		t.Fatalf("Delete rejected an archived initiative: %v", err)
	}
	if _, err := r.GetCandidate(candidateID); err != nil {
		t.Errorf("deleting the last referencing initiative removed the candidate: %v", err)
	}
	if err := s.Delete(9999); err == nil {
		t.Error("Delete accepted an unknown initiative")
	}
}
