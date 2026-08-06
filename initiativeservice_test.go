package main

import (
	"testing"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/models"
)

func newTestService(t *testing.T) *InitiativeService {
	t.Helper()
	gdb, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	return NewInitiativeService(gdb)
}

func TestInitiativeServiceCreateAndList(t *testing.T) {
	s := newTestService(t)

	created, err := s.Create("Find a Go role", models.InitiativeTypeJobSearch)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == 0 {
		t.Error("Create did not assign an ID")
	}
	if created.Name != "Find a Go role" || created.Type != models.InitiativeTypeJobSearch {
		t.Errorf("Create returned %+v", created)
	}

	if _, err := s.Create("Hire a designer", models.InitiativeTypeTalentSearch); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d initiatives, want 2", len(list))
	}
	if list[0].Name != "Find a Go role" || list[1].Name != "Hire a designer" {
		t.Errorf("List order wrong: %q, %q", list[0].Name, list[1].Name)
	}
}

func TestInitiativeServiceCreateValidation(t *testing.T) {
	s := newTestService(t)

	if _, err := s.Create("   ", models.InitiativeTypeJobSearch); err == nil {
		t.Error("Create accepted a blank name")
	}
	if _, err := s.Create("Valid name", models.InitiativeType("bogus")); err == nil {
		t.Error("Create accepted an unknown type")
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("invalid creates were persisted: %d rows", len(list))
	}
}

func TestInitiativeServiceCreateTrimsName(t *testing.T) {
	s := newTestService(t)

	created, err := s.Create("  Partnerships  ", models.InitiativeTypeBusinessDevelopment)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Name != "Partnerships" {
		t.Errorf("Create did not trim name: %q", created.Name)
	}
}
