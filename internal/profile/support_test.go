package profile

import (
	"strings"
	"testing"
)

// Every fixture here is invented.

func aspectWith(kind AspectType, wording, quote string, structured map[string]any) Aspect {
	return Aspect{
		Type: kind, Wording: wording, Structured: structured,
		Citations: []Citation{{ChunkID: 1, Quote: quote}},
	}
}

// The two mistakes that accounted for forty-two of fifty-eight introduced
// values against the frozen corpus: a status nobody stated, and a period read
// out of a salary quoted as base.
func TestAValueTheCitationDoesNotSupportIsDropped(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "existing Australian work rights",
			"You must have existing Australian work rights; we do not sponsor.",
			map[string]any{"country": "Australia", "status": "citizen", "sponsorship_required": false}),
		aspectWith(Compensation, "AUD 180,000 base plus superannuation",
			"AUD 180,000 base plus superannuation.",
			map[string]any{"currency": "AUD", "minimum": float64(180000), "period": "year", "basis": "base"}),
	}}

	dropped := DropUnsupportedStructured(&proposal)

	rights := proposal.Aspects[0].Structured
	if _, ok := rights["status"]; ok {
		t.Fatalf("a status nobody stated survived: %+v", rights)
	}
	// "we do not sponsor" does address sponsorship, and Australia is in the words.
	if rights["sponsorship_required"] != false || rights["country"] != "Australia" {
		t.Fatalf("a stated value was dropped: %+v", rights)
	}

	pay := proposal.Aspects[1].Structured
	if _, ok := pay["period"]; ok {
		t.Fatalf("a period nobody stated survived: %+v", pay)
	}
	if pay["basis"] != "base" || pay["currency"] != "AUD" || pay["minimum"] != float64(180000) {
		t.Fatalf("a stated value was dropped: %+v", pay)
	}
	if len(dropped) != 2 {
		t.Fatalf("dropped %v, want the status and the period", dropped)
	}
}

func TestAStatedValueSurvives(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkArrangement, "hybrid, three days onsite",
			"This is a hybrid role, three days onsite.",
			map[string]any{"arrangement": "hybrid", "days_onsite": float64(3)}),
		aspectWith(Location, "Melbourne", "hiring a platform engineer in Melbourne",
			map[string]any{"city": "Melbourne"}),
		aspectWith(EmploymentType, "permanent", "offered as permanent work",
			map[string]any{"employment_type": "permanent"}),
	}}

	if dropped := DropUnsupportedStructured(&proposal); len(dropped) != 0 {
		t.Fatalf("dropped values the source states: %v", dropped)
	}
}

// A city the source never names is not the location, however plausible.
func TestAnInferredPlaceIsDropped(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne", "hiring a platform engineer in Melbourne",
			map[string]any{"city": "Melbourne", "country": "Australia", "remote_ok": false}),
	}}
	dropped := DropUnsupportedStructured(&proposal)
	got := proposal.Aspects[0].Structured
	if got["city"] != "Melbourne" {
		t.Fatalf("the stated city was dropped: %+v", got)
	}
	for _, gone := range []string{"country", "remote_ok"} {
		if _, ok := got[gone]; ok {
			t.Fatalf("%q survived without evidence: %+v", gone, got)
		}
	}
	if len(dropped) != 2 {
		t.Fatalf("dropped %v", dropped)
	}
}

// A remote role does state remote_ok, and a day rate does state its period.
func TestEvidenceIsReadFromTheWordsNotTheFieldName(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "remote within Australia", "This is a remote role, offered anywhere in Australia",
			map[string]any{"country": "Australia", "remote_ok": true}),
		aspectWith(Compensation, "AUD 900 per day", "AUD 900 per day",
			map[string]any{"currency": "AUD", "minimum": float64(900), "period": "day", "basis": "rate"}),
	}}
	if dropped := DropUnsupportedStructured(&proposal); len(dropped) != 0 {
		t.Fatalf("dropped values the source states: %v", dropped)
	}
}

// A number written with separators is the same number.
func TestASeparatedNumberIsSupported(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 180,000 base", "AUD 180,000 base",
			map[string]any{"minimum": float64(180000)}),
	}}
	if dropped := DropUnsupportedStructured(&proposal); len(dropped) != 0 {
		t.Fatalf("a separated number was called unsupported: %v", dropped)
	}
}

// A number the source never mentions is not evidence-backed.
func TestAnInventedNumberIsDropped(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 180,000 base", "AUD 180,000 base",
			map[string]any{"minimum": float64(180000), "maximum": float64(220000)}),
	}}
	dropped := DropUnsupportedStructured(&proposal)
	if _, ok := proposal.Aspects[0].Structured["maximum"]; ok {
		t.Fatal("an invented maximum survived")
	}
	if len(dropped) != 1 || !strings.Contains(dropped[0], "maximum") {
		t.Fatalf("dropped %v", dropped)
	}
}
