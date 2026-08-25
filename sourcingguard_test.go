package main

import (
	"errors"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/models"
)

// refusingGuard is a data guard that says no, as the setup service does in
// demo scope or on an unencrypted volume.
type refusingGuard struct{}

func (refusingGuard) AllowRealData() error {
	return errors.New("this installation cannot hold candidate data")
}

// Personal-data entry is refused at the write, not in the interface — and a
// promoted lead is a candidate typed in by another route.
func TestPromotionIsRefusedWhereCandidateDataIs(t *testing.T) {
	e := newSourcingEnv(t)
	roleID := e.readyRole(t)
	ids := e.leads(t, roleID, person("p1", "https://quokka.example.invalid/about", "Quokka", ""))
	e.sourcing.Guard = refusingGuard{}

	_, err := e.sourcing.Promote(ids[0], models.Candidate{FullName: "Quokka Stack"})
	if err == nil || !strings.Contains(err.Error(), "cannot hold candidate data") {
		t.Fatalf("err = %v", err)
	}
	var candidates, identities, artifacts int64
	e.db.Model(&models.Candidate{}).Count(&candidates)
	e.db.Model(&models.Identity{}).Count(&identities)
	e.db.Model(&models.Artifact{}).Where("source LIKE 'exa:%'").Count(&artifacts)
	if candidates+identities+artifacts != 0 {
		t.Fatalf("a refused promotion wrote %d candidates, %d identities, %d artifacts", candidates, identities, artifacts)
	}
	var lead models.Lead
	e.db.First(&lead, ids[0])
	if lead.State != models.LeadNew {
		t.Fatalf("the lead is %s after a refused promotion", lead.State)
	}
}

func TestAnIdentityIsRefusedWhereCandidateDataIs(t *testing.T) {
	e := newEnrichEnv(t)
	e.enrich.Guard = refusingGuard{}
	if _, err := e.enrich.AddIdentity(e.candidate, "", "https://github.com/wombatdev"); err == nil ||
		!strings.Contains(err.Error(), "cannot hold candidate data") {
		t.Fatalf("err = %v", err)
	}
	var identities int64
	e.db.Model(&models.Identity{}).Count(&identities)
	if identities != 0 {
		t.Fatalf("%d identities written past the guard", identities)
	}
}
