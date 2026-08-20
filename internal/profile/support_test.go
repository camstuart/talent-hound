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

	dropped := DropUnsupportedStructured(&proposal, nil)

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

	if dropped := DropUnsupportedStructured(&proposal, nil); len(dropped) != 0 {
		t.Fatalf("dropped values the source states: %v", dropped)
	}
}

// A city the source never names is not the location, however plausible.
func TestAnInferredPlaceIsDropped(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne", "hiring a platform engineer in Melbourne",
			map[string]any{"city": "Melbourne", "country": "Australia", "remote_ok": false}),
	}}
	dropped := DropUnsupportedStructured(&proposal, nil)
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
	if dropped := DropUnsupportedStructured(&proposal, nil); len(dropped) != 0 {
		t.Fatalf("dropped values the source states: %v", dropped)
	}
}

// A number written with separators is the same number.
func TestASeparatedNumberIsSupported(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 180,000 base", "AUD 180,000 base",
			map[string]any{"minimum": float64(180000)}),
	}}
	if dropped := DropUnsupportedStructured(&proposal, nil); len(dropped) != 0 {
		t.Fatalf("a separated number was called unsupported: %v", dropped)
	}
}

// A number the source never mentions is not evidence-backed.
func TestAnInventedNumberIsDropped(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 180,000 base", "AUD 180,000 base",
			map[string]any{"minimum": float64(180000), "maximum": float64(220000)}),
	}}
	dropped := DropUnsupportedStructured(&proposal, nil)
	if _, ok := proposal.Aspects[0].Structured["maximum"]; ok {
		t.Fatal("an invented maximum survived")
	}
	if len(dropped) != 1 || !strings.Contains(dropped[0], "maximum") {
		t.Fatalf("dropped %v", dropped)
	}
}

// A capable model states "we do not sponsor" and "AUD 180,000 base" in its
// wording and then leaves sponsorship_required and basis out of the value
// beside it, on forty of a hundred constraints. Reading a word that is there is
// not a judgement about the role.
func TestAValueTheEvidenceStatesOutrightIsFilledIn(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "existing Australian work rights",
			"You must have existing Australian work rights; we do not sponsor.",
			map[string]any{"country": "Australia"}),
		aspectWith(Compensation, "AUD 180,000 base plus superannuation",
			"AUD 180,000 base plus superannuation.",
			map[string]any{"currency": "AUD", "minimum": float64(180000)}),
		aspectWith(WorkArrangement, "hybrid", "This is a hybrid role.", map[string]any{}),
		aspectWith(EmploymentType, "permanent", "offered as permanent work", map[string]any{}),
	}}

	DeriveStructured(&proposal, nil)

	if got := proposal.Aspects[0].Structured["sponsorship_required"]; got != false {
		t.Fatalf("sponsorship_required = %v, want false", got)
	}
	if got := proposal.Aspects[1].Structured["basis"]; got != "base" {
		t.Fatalf("basis = %v, want base", got)
	}
	if got := proposal.Aspects[2].Structured["arrangement"]; got != "hybrid" {
		t.Fatalf("arrangement = %v, want hybrid", got)
	}
	if got := proposal.Aspects[3].Structured["employment_type"]; got != "permanent" {
		t.Fatalf("employment_type = %v, want permanent", got)
	}
}

// A negation is handled, not guessed at: "we will sponsor" and "we do not
// sponsor" are opposite facts sharing a word.
func TestANegationIsReadCorrectly(t *testing.T) {
	for _, tc := range []struct {
		quote string
		want  any
	}{
		{"We do not sponsor visas.", false},
		{"We cannot sponsor.", false},
		{"Sponsorship available for the right candidate.", true},
		{"We will sponsor.", true},
	} {
		proposal := Proposal{Aspects: []Aspect{
			aspectWith(WorkRights, "work rights", tc.quote, map[string]any{}),
		}}
		DeriveStructured(&proposal, nil)
		if got := proposal.Aspects[0].Structured["sponsorship_required"]; got != tc.want {
			t.Fatalf("%q gave %v, want %v", tc.quote, got, tc.want)
		}
	}
}

// A value the model did state is never overwritten, and silence stays silent.
func TestDerivationNeitherOverwritesNorInvents(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 900 per day", "AUD 900 per day",
			map[string]any{"basis": "rate", "period": "day"}),
		aspectWith(Compensation, "AUD 150,000", "AUD 150,000", map[string]any{}),
		aspectWith(WorkRights, "work rights", "Australian work rights required.", map[string]any{}),
	}}

	DeriveStructured(&proposal, nil)

	if proposal.Aspects[0].Structured["basis"] != "rate" {
		t.Fatal("a stated basis was overwritten")
	}
	// Nothing in "AUD 150,000" says base, package, or a period.
	if len(proposal.Aspects[1].Structured) != 0 {
		t.Fatalf("a value was invented from silence: %+v", proposal.Aspects[1].Structured)
	}
	// The source is silent on sponsorship.
	if _, ok := proposal.Aspects[2].Structured["sponsorship_required"]; ok {
		t.Fatalf("sponsorship was inferred from silence: %+v", proposal.Aspects[2].Structured)
	}
}

// Derivation runs after the evidence check, so what it fills survives it.
func TestWhatIsDerivedIsAlsoSupported(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "work rights", "we do not sponsor", map[string]any{}),
		aspectWith(Compensation, "AUD 180,000 base", "AUD 180,000 base", map[string]any{}),
	}}
	DeriveStructured(&proposal, nil)
	if dropped := DropUnsupportedStructured(&proposal, nil); len(dropped) != 0 {
		t.Fatalf("the evidence check removed what was derived from evidence: %v", dropped)
	}
}

// "Australian work rights" states Australia. The model records the sponsorship
// and drops the country on six listings of twenty.
func TestACountryStatedByDemonymIsFilledIn(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "existing Australian work rights",
			"You must have existing Australian work rights; we do not sponsor.",
			map[string]any{"sponsorship_required": false}),
	}}
	DeriveStructured(&proposal, nil)
	if got := proposal.Aspects[0].Structured["country"]; got != "Australia" {
		t.Fatalf("country = %v, want Australia", got)
	}
}

// And a source that names no country still names none.
func TestNoCountryIsInventedFromSilence(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "work rights required", "You must have the right to work here.",
			map[string]any{}),
	}}
	DeriveStructured(&proposal, nil)
	if _, ok := proposal.Aspects[0].Structured["country"]; ok {
		t.Fatalf("a country was invented: %+v", proposal.Aspects[0].Structured)
	}
}

// Stripping the separators from a whole sentence and asking whether it
// contained the digits let a days_onsite of zero pass on a listing quoting
// "AUD 180,000": there is a zero in there, and it means nothing.
func TestANumberIsMatchedWholeNotAsDigitsInsideAnother(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkArrangement, "hybrid", "This is a hybrid role at AUD 180,000 base.",
			map[string]any{"arrangement": "hybrid", "days_onsite": float64(0)}),
	}}
	DropUnsupportedStructured(&proposal, nil)
	if _, ok := proposal.Aspects[0].Structured["days_onsite"]; ok {
		t.Fatalf("a zero read out of a salary survived: %+v", proposal.Aspects[0].Structured)
	}
	if proposal.Aspects[0].Structured["arrangement"] != "hybrid" {
		t.Fatal("the stated arrangement was dropped")
	}
}

// And the number a source does state, however it is punctuated, is supported.
func TestASeparatedNumberIsStillTheSameNumber(t *testing.T) {
	for _, quote := range []string{"AUD 180,000 base", "AUD 180000 base", "AUD 180,000.00 base"} {
		proposal := Proposal{Aspects: []Aspect{
			aspectWith(Compensation, "salary", quote, map[string]any{"minimum": float64(180000)}),
		}}
		if dropped := DropUnsupportedStructured(&proposal, nil); len(dropped) != 0 {
			t.Fatalf("%q: the stated number was called unsupported", quote)
		}
	}
}

// The wording is the model's own restatement and nothing checks that it appears
// in any source. Counting it as evidence let a model write "Annual base salary
// of AUD 180,000" and then support a period of a year with its own sentence.
func TestAModelCannotSupportAValueWithItsOwnWording(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "Annual base salary of AUD 180,000",
			"AUD 180,000 base plus superannuation.",
			map[string]any{"currency": "AUD", "minimum": float64(180000),
				"basis": "base", "period": "year"}),
	}}
	DropUnsupportedStructured(&proposal, nil)
	got := proposal.Aspects[0].Structured
	if _, ok := got["period"]; ok {
		t.Fatalf("a period supported only by the model's own wording survived: %+v", got)
	}
	if got["basis"] != "base" || got["minimum"] != float64(180000) {
		t.Fatalf("a cited value was dropped: %+v", got)
	}
}

// And a location's country is not read off a sentence about work rights.
func TestALocationCountryIsNotDerivedFromWorkRightsWording(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne",
			"hiring a senior platform engineer in Melbourne. You must have existing Australian work rights.",
			map[string]any{"city": "Melbourne"}),
	}}
	DeriveStructured(&proposal, nil)
	if _, ok := proposal.Aspects[0].Structured["country"]; ok {
		t.Fatalf("a country was read off the work-rights sentence: %+v", proposal.Aspects[0].Structured)
	}
	// The same words still state the country of the work rights.
	rights := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "Australian work rights",
			"You must have existing Australian work rights.", map[string]any{}),
	}}
	DeriveStructured(&rights, nil)
	if rights.Aspects[0].Structured["country"] != "Australia" {
		t.Fatalf("the work rights lost their country: %+v", rights.Aspects[0].Structured)
	}
}

// A rate quoted once states a floor, not a ceiling. Moving it is arithmetic:
// there is one figure and the source says what it is.
func TestALoneFigureRecordedAsAMaximumBecomesTheMinimum(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 900 per day", "offered at AUD 900 per day",
			map[string]any{"currency": "AUD", "maximum": float64(900), "period": "day", "basis": "rate"}),
	}}
	NormalizeStructured(&proposal)
	got := proposal.Aspects[0].Structured
	if got["minimum"] != float64(900) {
		t.Fatalf("the lone figure did not become the minimum: %+v", got)
	}
	if _, ok := got["maximum"]; ok {
		t.Fatalf("the maximum survived: %+v", got)
	}
}

// Two figures are a range, and which is which is the model's to say.
func TestARangeIsLeftAlone(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 150,000 to 180,000", "paying AUD 150,000 to 180,000",
			map[string]any{"maximum": float64(180000)}),
	}}
	NormalizeStructured(&proposal)
	if _, ok := proposal.Aspects[0].Structured["maximum"]; !ok {
		t.Fatalf("a range was rewritten: %+v", proposal.Aspects[0].Structured)
	}
}

// A stated minimum is never overwritten by a maximum.
func TestAStatedMinimumIsLeftAlone(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 900 per day", "AUD 900 per day",
			map[string]any{"minimum": float64(900), "maximum": float64(900)}),
	}}
	NormalizeStructured(&proposal)
	got := proposal.Aspects[0].Structured
	if got["minimum"] != float64(900) || got["maximum"] != float64(900) {
		t.Fatalf("a stated pair was rewritten: %+v", got)
	}
}

// A listing that says it is a remote role has said so of its location.
func TestARemoteRoleStatesItsLocationIsRemote(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne", "hiring a data engineer in Melbourne. This is a remote role.",
			map[string]any{"city": "Melbourne"}),
		aspectWith(Location, "Sydney", "hiring an engineer in Sydney. This is a hybrid role.",
			map[string]any{"city": "Sydney"}),
	}}
	DeriveStructured(&proposal, nil)
	if proposal.Aspects[0].Structured["remote_ok"] != true {
		t.Fatalf("a remote role did not state remote_ok: %+v", proposal.Aspects[0].Structured)
	}
	if _, ok := proposal.Aspects[1].Structured["remote_ok"]; ok {
		t.Fatalf("a hybrid role stated remote_ok: %+v", proposal.Aspects[1].Structured)
	}
}

// A location says its country in place phrasing — "Remote (Australia)" — and
// the adjective in "Australian work rights" is about the rights, not the place.
func TestALocationCountryComesFromPlacePhrasing(t *testing.T) {
	stated := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Remote (Australia)", "hiring a platform engineer in Remote (Australia)",
			map[string]any{"remote_ok": true}),
	}}
	DeriveStructured(&stated, nil)
	if stated.Aspects[0].Structured["country"] != "Australia" {
		t.Fatalf("a stated country was not read: %+v", stated.Aspects[0].Structured)
	}

	adjective := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne", "in Melbourne. You must have Australian work rights.",
			map[string]any{"city": "Melbourne"}),
	}}
	DeriveStructured(&adjective, nil)
	if _, ok := adjective.Aspects[0].Structured["country"]; ok {
		t.Fatalf("a country was read off the work-rights adjective: %+v", adjective.Aspects[0].Structured)
	}
}

// A listing whose arrangement is remote has said its location is
// remote-friendly: same profile, same document, already evidenced.
func TestARemoteArrangementSaysTheLocationIsRemoteFriendly(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne", "hiring a data engineer in Melbourne",
			map[string]any{"city": "Melbourne"}),
		aspectWith(WorkArrangement, "remote", "This is a remote role.",
			map[string]any{"arrangement": "remote"}),
	}}
	AlignAcrossAspects(&proposal)
	if proposal.Aspects[0].Structured["remote_ok"] != true {
		t.Fatalf("the location did not follow the arrangement: %+v", proposal.Aspects[0].Structured)
	}
	if proposal.Aspects[0].Structured["city"] != "Melbourne" {
		t.Fatal("the stated city was lost")
	}
}

// It runs one way: a location allowing remote work does not make the role a
// remote role.
func TestALocationSayingRemoteDoesNotSetTheArrangement(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne", "Melbourne, remote work considered",
			map[string]any{"city": "Melbourne", "remote_ok": true}),
		aspectWith(WorkArrangement, "hybrid", "This is a hybrid role.",
			map[string]any{"arrangement": "hybrid"}),
	}}
	AlignAcrossAspects(&proposal)
	if proposal.Aspects[1].Structured["arrangement"] != "hybrid" {
		t.Fatalf("the arrangement was rewritten: %+v", proposal.Aspects[1].Structured)
	}
}

// An onsite role says nothing about remote work.
func TestAnOnsiteArrangementFillsNothing(t *testing.T) {
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Perth", "in Perth", map[string]any{"city": "Perth"}),
		aspectWith(WorkArrangement, "onsite", "This is an onsite role.",
			map[string]any{"arrangement": "onsite"}),
	}}
	AlignAcrossAspects(&proposal)
	if _, ok := proposal.Aspects[0].Structured["remote_ok"]; ok {
		t.Fatalf("an onsite role set remote_ok: %+v", proposal.Aspects[0].Structured)
	}
}

// A citation is part of a sentence, and the sentence is what it was read from.
//
// The quote alone is too narrow: a model citing "Australian work rights" from a
// sentence continuing "; we do not sponsor" has quoted half of what it read.
func TestEvidenceIsTheCitedSentence(t *testing.T) {
	sources := []Source{{ChunkID: 1, Text: "## Requirements\n\n" +
		"You must have existing Australian work rights; we do not sponsor.\n" +
		"The role is based in Melbourne.\n"}}
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "Australian work rights", "existing Australian work rights",
			map[string]any{}),
	}}
	DeriveStructured(&proposal, sources)
	got := proposal.Aspects[0].Structured
	if got["country"] != "Australia" || got["sponsorship_required"] != false {
		t.Fatalf("the cited sentence was not read: %+v", got)
	}
}

// The chunk is not the sentence. A location citing its own sentence does not
// take a country from the work-rights sentence beside it — which is how eleven
// wrong countries got in.
func TestEvidenceStopsAtTheSentence(t *testing.T) {
	sources := []Source{{ChunkID: 1, Text: "Hiring a platform engineer in Melbourne. " +
		"You must have existing Australian work rights."}}
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Location, "Melbourne", "in Melbourne", map[string]any{"city": "Melbourne"}),
	}}
	DeriveStructured(&proposal, sources)
	if _, ok := proposal.Aspects[0].Structured["country"]; ok {
		t.Fatalf("a country crossed a sentence boundary: %+v", proposal.Aspects[0].Structured)
	}
}

// And a value stated nowhere in the cited sentence is still dropped.
func TestASentenceDoesNotSupportWhatItDoesNotSay(t *testing.T) {
	sources := []Source{{ChunkID: 1, Text: "Offered at AUD 180,000 base. The team works from Sydney."}}
	proposal := Proposal{Aspects: []Aspect{
		aspectWith(Compensation, "AUD 180,000 base", "AUD 180,000 base",
			map[string]any{"currency": "AUD", "minimum": float64(180000), "period": "year"}),
	}}
	DropUnsupportedStructured(&proposal, sources)
	if _, ok := proposal.Aspects[0].Structured["period"]; ok {
		t.Fatalf("a period nobody stated survived: %+v", proposal.Aspects[0].Structured)
	}
}

// Wording that appears verbatim in a source is quoting, whether or not the
// model labelled it a quote — and a paraphrase is not.
func TestVerbatimWordingCountsAsEvidenceAndAParaphraseDoesNot(t *testing.T) {
	sources := []Source{{ChunkID: 2, Text: "Kelpie Software is hiring a platform engineer in " +
		"Remote (Australia). This is a remote role, offered as permanent work."}}

	// The model wrote the document's own words and cited the sentence beside them.
	quoting := Proposal{Aspects: []Aspect{{
		Type: Location, Wording: "in Remote (Australia)",
		Structured: map[string]any{"remote_ok": true},
		Citations:  []Citation{{ChunkID: 2, Quote: "This is a remote role, offered as permanent work"}},
	}}}
	DeriveStructured(&quoting, sources)
	if quoting.Aspects[0].Structured["country"] != "Australia" {
		t.Fatalf("verbatim wording was not read: %+v", quoting.Aspects[0].Structured)
	}

	// A paraphrase appears nowhere and counts for nothing — cited, here, to a
	// sentence that says nothing about a country either.
	paraphrasing := Proposal{Aspects: []Aspect{{
		Type: Location, Wording: "The role is located in Melbourne, Australia.",
		Structured: map[string]any{"city": "Melbourne"},
		Citations:  []Citation{{ChunkID: 2, Quote: "This is a remote role, offered as permanent work"}},
	}}}
	DeriveStructured(&paraphrasing, sources)
	if _, ok := paraphrasing.Aspects[0].Structured["country"]; ok {
		t.Fatalf("a paraphrase was read as evidence: %+v", paraphrasing.Aspects[0].Structured)
	}
}

// And it does not let an unsupported value through either.
func TestVerbatimWordingDoesNotSupportWhatItDoesNotSay(t *testing.T) {
	sources := []Source{{ChunkID: 1, Text: "Offered at AUD 180,000 base plus superannuation."}}
	proposal := Proposal{Aspects: []Aspect{{
		Type: Compensation, Wording: "Offered at AUD 180,000 base plus superannuation.",
		Structured: map[string]any{"currency": "AUD", "minimum": float64(180000), "period": "year"},
		Citations:  []Citation{{ChunkID: 1, Quote: "AUD 180,000 base"}},
	}}}
	DropUnsupportedStructured(&proposal, sources)
	if _, ok := proposal.Aspects[0].Structured["period"]; ok {
		t.Fatalf("a period nobody stated survived: %+v", proposal.Aspects[0].Structured)
	}
}

// "We do not sponsor" contains the word sponsor and states the opposite of
// needing sponsorship. A value whose only evidence is a word inside its own
// negation is not evidenced.
func TestSponsorshipIsNotEvidencedByItsOwnNegation(t *testing.T) {
	refusing := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "work rights", "You must have existing work rights; we do not sponsor.",
			map[string]any{"status": "requires_sponsorship"}),
	}}
	DropUnsupportedStructured(&refusing, nil)
	if _, ok := refusing.Aspects[0].Structured["status"]; ok {
		t.Fatalf("a status read out of its own negation survived: %+v", refusing.Aspects[0].Structured)
	}

	// A source that does state it keeps it.
	offering := Proposal{Aspects: []Aspect{
		aspectWith(WorkRights, "work rights", "Sponsorship is required for this role.",
			map[string]any{"status": "requires_sponsorship"}),
	}}
	DropUnsupportedStructured(&offering, nil)
	if offering.Aspects[0].Structured["status"] != "requires_sponsorship" {
		t.Fatalf("a stated status was dropped: %+v", offering.Aspects[0].Structured)
	}
}
