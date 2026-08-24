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
