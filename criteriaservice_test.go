package main

import (
	"strings"
	"testing"

	"camstuart/talent-hound/internal/criteria"
	"camstuart/talent-hound/internal/models"
	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// criteriaEnv is a profileEnv with criteria wired in.
type criteriaEnv struct {
	*profileEnv
	criteria *CriteriaService
}

func newCriteriaEnv(t *testing.T) *criteriaEnv {
	t.Helper()
	base := newProfileEnv(t)
	return &criteriaEnv{
		profileEnv: base,
		criteria:   NewCriteriaService(base.db, base.registry, base.model, base.profiles),
	}
}

func (e *criteriaEnv) add(t *testing.T, text, priority string) *models.SearchCriterion {
	t.Helper()
	row, err := e.criteria.Add(CriterionInput{
		InitiativeID: e.initiative, Text: text, Priority: priority,
	})
	if err != nil {
		t.Fatalf("adding %q: %v", text, err)
	}
	return row
}

func (e *criteriaEnv) version(t *testing.T) int {
	t.Helper()
	v, err := e.criteria.Version(e.initiative)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	return v
}

func TestCriteriaAreStoredWithTheirPriorityAndOrder(t *testing.T) {
	e := newCriteriaEnv(t)
	first := e.add(t, "five years of production Go", models.CriterionMustHave)
	second := e.add(t, "has led a platform team", models.CriterionNiceToHave)

	list, err := e.criteria.List(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d criteria", len(list))
	}
	if list[0].ID != first.ID || list[1].ID != second.ID {
		t.Errorf("criteria came back in the wrong order")
	}
	if list[0].Priority != models.CriterionMustHave || list[1].Priority != models.CriterionNiceToHave {
		t.Errorf("priorities are %q and %q", list[0].Priority, list[1].Priority)
	}
}

func TestAnUnsupportedPriorityIsRefused(t *testing.T) {
	e := newCriteriaEnv(t)
	for _, priority := range []string{"", "unspecified", "critical", "MUST_HAVE"} {
		_, err := e.criteria.Add(CriterionInput{
			InitiativeID: e.initiative, Text: "five years of Go", Priority: priority,
		})
		if err == nil {
			t.Errorf("priority %q was accepted", priority)
		}
	}
}

// The whole provisional list, refused, with nothing stored and no version move.
func TestExplicitProtectedCriteriaAreBlockedAndStoreNothing(t *testing.T) {
	blocked := []string{
		"must be under 35",
		"looking for a male engineer",
		"prefer a nonbinary candidate",
		"must be heterosexual",
		"must be an Australian citizen",
		"prefer a Christian candidate",
		"must be able bodied",
		"no children please",
		"must not be pregnant",
		"prefer someone married",
		"must be politically conservative",
		"must not be a union member",
	}
	for _, text := range blocked {
		t.Run(text, func(t *testing.T) {
			e := newCriteriaEnv(t)
			before := e.version(t)

			_, err := e.criteria.Add(CriterionInput{
				InitiativeID: e.initiative, Text: text, Priority: models.CriterionMustHave,
			})
			if err == nil {
				t.Fatalf("%q was accepted", text)
			}
			var stored int64
			if err := e.db.Model(&models.SearchCriterion{}).Count(&stored).Error; err != nil {
				t.Fatalf("counting: %v", err)
			}
			if stored != 0 {
				t.Errorf("a refused criterion left %d rows", stored)
			}
			if after := e.version(t); after != before {
				t.Errorf("a refused criterion moved the version from %d to %d", before, after)
			}
		})
	}
}

// A refusal must not depend on an endpoint being up, so it happens before any
// model is consulted.
func TestBlockingCallsNoModel(t *testing.T) {
	e := newCriteriaEnv(t)
	e.assignClassify(t, "synthetic-classify")
	before := e.model.callCount()

	_, err := e.criteria.Add(CriterionInput{
		InitiativeID: e.initiative, Text: "must be under 35", Priority: models.CriterionMustHave,
	})
	if err == nil {
		t.Fatal("a protected criterion was accepted")
	}
	if e.model.callCount() != before {
		t.Fatal("the deterministic block consulted a model")
	}
}

// An advisory block is a checkbox on a discrimination claim.
func TestARefusedCriterionCannotBeStoredByAnyRoute(t *testing.T) {
	e := newCriteriaEnv(t)
	const text = "must be under 35"

	// Retrying unchanged fails the same way.
	for range 3 {
		if _, err := e.criteria.Add(CriterionInput{
			InitiativeID: e.initiative, Text: text, Priority: models.CriterionMustHave,
		}); err == nil {
			t.Fatal("a retry stored a refused criterion")
		}
	}
	// Editing an existing criterion into a refused one is also refused.
	row := e.add(t, "five years of production Go", models.CriterionMustHave)
	if _, err := e.criteria.Edit(row.ID, text, models.CriterionMustHave); err == nil {
		t.Fatal("editing smuggled in a refused criterion")
	}
	// And applying it as a proposal is refused too — Apply goes through Add.
	if _, err := e.criteria.Apply(e.initiative, []Proposal{
		{Text: text, Priority: models.CriterionMustHave},
	}); err == nil {
		t.Fatal("applying a proposal stored a refused criterion")
	}
	var stored int64
	if err := e.db.Model(&models.SearchCriterion{}).Where("text = ?", text).Count(&stored).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if stored != 0 {
		t.Fatalf("%d refused criteria were stored", stored)
	}
}

func TestWorkRightsCriteriaAreAvailableAndNationalityIsNot(t *testing.T) {
	e := newCriteriaEnv(t)
	// Lawful and necessary — this must keep working.
	for _, text := range []string{
		"must have Australian work rights",
		"must have the right to work in Australia",
		"eligible to work in New Zealand without sponsorship",
	} {
		if _, err := e.criteria.Add(CriterionInput{
			InitiativeID: e.initiative, Text: text, Priority: models.CriterionMustHave,
		}); err != nil {
			t.Errorf("a lawful work-rights criterion was refused: %q — %v", text, err)
		}
	}
	// And the thing it must not be confused with.
	for _, text := range []string{
		"must be an Australian citizen",
		"Australian citizenship required",
	} {
		_, err := e.criteria.Add(CriterionInput{
			InitiativeID: e.initiative, Text: text, Priority: models.CriterionMustHave,
		})
		if err == nil {
			t.Errorf("a nationality criterion was accepted: %q", text)
			continue
		}
		if !strings.Contains(err.Error(), "national origin") {
			t.Errorf("%q was refused as %v", text, err)
		}
	}
}

// The model is allowed to be wrong here, which is why it only warns.
func TestAnAmbiguousProxyWarnsWithoutBlocking(t *testing.T) {
	e := newCriteriaEnv(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{
		`{"proxy":true,"reason":"\"recent graduate\" tends to select for age"}`,
	}

	row, err := e.criteria.Add(CriterionInput{
		InitiativeID: e.initiative, Text: "recent graduate preferred",
		Priority: models.CriterionNiceToHave,
	})
	if err != nil {
		t.Fatalf("a flagged proxy was refused rather than warned about: %v", err)
	}
	if row.Warning == "" {
		t.Fatal("a flagged proxy carries no warning")
	}
	if !strings.Contains(row.Warning, "age") {
		t.Errorf("the warning does not say why: %q", row.Warning)
	}

	// It is a real criterion: listed, usable, and counted.
	list, err := e.criteria.List(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(list) != 1 || list[0].Warning == "" {
		t.Fatalf("the warned criterion is not in the list with its warning: %+v", list)
	}
}

func TestAClearlyLawfulCriterionGetsNoBlockAndNoWarning(t *testing.T) {
	e := newCriteriaEnv(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{`{"proxy":false,"reason":"an ordinary professional requirement"}`}

	row, err := e.criteria.Add(CriterionInput{
		InitiativeID: e.initiative, Text: "five years of production Go",
		Priority: models.CriterionMustHave,
	})
	if err != nil {
		t.Fatalf("a lawful criterion was refused: %v", err)
	}
	if row.Warning != "" {
		t.Errorf("a lawful criterion carries a warning: %q", row.Warning)
	}
}

// Blocks are deterministic; a block that depends on an endpoint being up is not.
func TestAnUnavailableModelDoesNotBecomeABlock(t *testing.T) {
	e := newCriteriaEnv(t)
	// No classify model assigned at all.
	row, err := e.criteria.Add(CriterionInput{
		InitiativeID: e.initiative, Text: "recent graduate preferred",
		Priority: models.CriterionNiceToHave,
	})
	if err != nil {
		t.Fatalf("a criterion was refused because no model was available: %v", err)
	}
	if row.Warning != "" {
		t.Errorf("a warning appeared with no model: %q", row.Warning)
	}
}

// A warning that appears and disappears as the loaded model changes is a
// warning nobody will trust.
func TestAWarningIsRecordedOnceAndDoesNotMoveUnderTheReader(t *testing.T) {
	e := newCriteriaEnv(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{`{"proxy":true,"reason":"may select for age"}`}
	row := e.add(t, "digital native preferred", models.CriterionNiceToHave)
	stored := row.Warning
	if stored == "" {
		t.Fatal("no warning was recorded")
	}

	// The model now says the opposite, and the classify assignment changes.
	e.model.responses = []string{`{"proxy":false,"reason":"fine"}`}
	e.assignClassify(t, "a-different-model")

	before := e.model.callCount()
	for range 3 {
		list, err := e.criteria.List(e.initiative)
		if err != nil {
			t.Fatalf("listing: %v", err)
		}
		if list[0].Warning != stored {
			t.Fatalf("the warning changed on read: %q then %q", stored, list[0].Warning)
		}
	}
	if e.model.callCount() != before {
		t.Fatal("listing criteria called a model")
	}
}

func TestContentChangesTheVersionAndOrderingDoesNot(t *testing.T) {
	e := newCriteriaEnv(t)
	start := e.version(t)

	first := e.add(t, "five years of production Go", models.CriterionMustHave)
	afterAdd := e.version(t)
	if afterAdd == start {
		t.Fatal("adding a criterion did not change the version")
	}
	second := e.add(t, "has led a platform team", models.CriterionNiceToHave)
	afterSecond := e.version(t)
	if afterSecond == afterAdd {
		t.Fatal("adding a second criterion did not change the version")
	}

	// Reordering is presentation only.
	if err := e.criteria.Reorder(e.initiative, []uint{second.ID, first.ID}); err != nil {
		t.Fatalf("reordering: %v", err)
	}
	if v := e.version(t); v != afterSecond {
		t.Fatalf("reordering moved the version from %d to %d", afterSecond, v)
	}
	list, err := e.criteria.List(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Error("the new order was not preserved")
	}

	// Editing and removing are content changes.
	edited, err := e.criteria.Edit(first.ID, "six years of production Go", models.CriterionMustHave)
	if err != nil {
		t.Fatalf("editing: %v", err)
	}
	if edited == nil || edited.Text != "six years of production Go" {
		t.Fatalf("editing returned %+v", edited)
	}
	afterEdit := e.version(t)
	if afterEdit == afterSecond {
		t.Fatal("editing did not change the version")
	}
	if err := e.criteria.Remove(second.ID); err != nil {
		t.Fatalf("removing: %v", err)
	}
	if v := e.version(t); v == afterEdit {
		t.Fatal("removing did not change the version")
	}
}

func TestAnInitiativeWithNoCriteriaStillHasAVersion(t *testing.T) {
	e := newCriteriaEnv(t)
	if v := e.version(t); v == 0 {
		t.Fatal("an initiative with no criteria has no version")
	}
}

// Criteria and profiles are different records with different versions, and
// touching one must not touch the other.
func TestCriteriaAndCandidateFactsAreSeparatelyVersioned(t *testing.T) {
	e := newCriteriaEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{e.answer(t,
		profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production"})}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}

	criteriaBefore := e.version(t)
	profileVersionsBefore := countProfiles(t, e)

	// A criterion change moves the criteria version and no profile.
	e.add(t, "five years of production Go", models.CriterionMustHave)
	if e.version(t) == criteriaBefore {
		t.Error("adding a criterion did not move the criteria version")
	}
	if countProfiles(t, e) != profileVersionsBefore {
		t.Error("adding a criterion created a profile version")
	}

	// A profile change moves the profile and no criteria version.
	criteriaAfter := e.version(t)
	if _, err := e.profiles.EditAspect(id, 0, "Go, SQLite, and PostgreSQL", nil); err != nil {
		t.Fatalf("editing the profile: %v", err)
	}
	if countProfiles(t, e) == profileVersionsBefore {
		t.Error("editing an aspect created no profile version")
	}
	if e.version(t) != criteriaAfter {
		t.Error("editing a profile moved the criteria version")
	}
	_ = p
}

func countProfiles(t *testing.T, e *criteriaEnv) int64 {
	t.Helper()
	var n int64
	if err := e.db.Model(&models.Profile{}).Count(&n).Error; err != nil {
		t.Fatalf("counting profiles: %v", err)
	}
	return n
}

// The structural form of "preferences are never inferred from resume history"
// is that the proposer never sees the aspects that carry it.
func TestHistoryNeverBecomesAProposal(t *testing.T) {
	e := newCriteriaEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")

	quote := skillQuote
	chunk := e.chunkQuoting(t)
	cite := []profile.Citation{{ChunkID: chunk, Quote: quote}}
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		// Proposable: what the person can do.
		{Type: profile.Skill, Wording: "Go and SQLite in production", Citations: cite},
		{Type: profile.Seniority, Wording: "Senior platform engineer", Citations: cite},
		// History: where they have been, what they were paid, where they lived.
		{Type: profile.Experience, Wording: "Senior platform engineer at Northwind", Citations: cite},
		{Type: profile.Location, Wording: "Melbourne", Citations: cite},
		{Type: profile.Compensation, Wording: "was on AUD 175,000", Citations: cite},
		{Type: profile.Qualification, Wording: "BSc, University of Melbourne", Citations: cite},
	})}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}

	proposals, err := e.criteria.Propose(e.initiative, id)
	if err != nil {
		t.Fatalf("proposing: %v", err)
	}
	for _, prop := range proposals {
		switch prop.From {
		case string(profile.Location), string(profile.Compensation), string(profile.Experience):
			t.Errorf("a %s aspect became a proposal: %q", prop.From, prop.Text)
		}
		if strings.Contains(prop.Text, "Northwind") {
			t.Errorf("a prior employer became a proposal: %q", prop.Text)
		}
		if strings.Contains(prop.Text, "Melbourne") {
			t.Errorf("a past location became a proposal: %q", prop.Text)
		}
		if strings.Contains(prop.Text, "175,000") {
			t.Errorf("past compensation became a proposal: %q", prop.Text)
		}
	}
	if len(proposals) == 0 {
		t.Fatal("nothing was proposed at all — the exclusion is too broad to be useful")
	}

	// A recruiter may still type any of them, because a person decided.
	if _, err := e.criteria.Add(CriterionInput{
		InitiativeID: e.initiative, Text: "open to hybrid work in Melbourne",
		Priority: models.CriterionNiceToHave,
	}); err != nil {
		t.Errorf("a recruiter-typed location criterion was refused: %v", err)
	}
}

// The qualification case is subtle: a degree is a qualification the work may
// need, but a named school is history. Naming the school must not carry over.
func TestANamedSchoolIsNotProposedAsAPreference(t *testing.T) {
	e := newCriteriaEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	cite := []profile.Citation{{ChunkID: e.chunkQuoting(t), Quote: skillQuote}}
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production", Citations: cite},
	})}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}
	proposals, err := e.criteria.Propose(e.initiative, id)
	if err != nil {
		t.Fatalf("proposing: %v", err)
	}
	for _, prop := range proposals {
		if strings.Contains(strings.ToLower(prop.Text), "university") {
			t.Errorf("a named school became a proposal: %q", prop.Text)
		}
	}
}

func TestProposingWritesNothingAndApplyingTakesOnlyWhatWasChosen(t *testing.T) {
	e := newCriteriaEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	cite := []profile.Citation{{ChunkID: e.chunkQuoting(t), Quote: skillQuote}}
	e.model.responses = []string{jsonProposal(t, []profile.Aspect{
		{Type: profile.Skill, Wording: "Go and SQLite in production", Citations: cite},
		{Type: profile.Seniority, Wording: "Senior platform engineer", Citations: cite},
		{Type: profile.WorkArrangement, Wording: "hybrid", Citations: cite},
	})}
	p, err := e.profiles.Classify(id)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if _, err := e.profiles.Approve(p.ID); err != nil {
		t.Fatalf("approving: %v", err)
	}

	before := e.version(t)
	proposals, err := e.criteria.Propose(e.initiative, id)
	if err != nil {
		t.Fatalf("proposing: %v", err)
	}
	if len(proposals) < 3 {
		t.Fatalf("got %d proposals", len(proposals))
	}
	// Proposing writes nothing at all.
	var stored int64
	if err := e.db.Model(&models.SearchCriterion{}).Count(&stored).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if stored != 0 {
		t.Fatalf("proposing created %d criteria", stored)
	}
	if e.version(t) != before {
		t.Fatal("proposing moved the criteria version")
	}

	// Applying takes exactly what the recruiter chose.
	chosen := []Proposal{proposals[0], proposals[2]}
	applied, err := e.criteria.Apply(e.initiative, chosen)
	if err != nil {
		t.Fatalf("applying: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("applied %d criteria", len(applied))
	}
	list, err := e.criteria.List(e.initiative)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("%d criteria exist after applying two of three", len(list))
	}
	texts := []string{list[0].Text, list[1].Text}
	if !slicesContains(texts, proposals[0].Text) || !slicesContains(texts, proposals[2].Text) {
		t.Errorf("the applied criteria are %v", texts)
	}
	if slicesContains(texts, proposals[1].Text) {
		t.Errorf("an unchosen proposal was applied: %q", proposals[1].Text)
	}
}

func slicesContains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestProposingNeedsAnApprovedProfile(t *testing.T) {
	e := newCriteriaEnv(t)
	id := e.candidateWithResume(t)
	e.assignClassify(t, "synthetic-classify")
	e.model.responses = []string{e.answer(t,
		profile.Aspect{Type: profile.Skill, Wording: "Go and SQLite in production"})}
	if _, err := e.profiles.Classify(id); err != nil {
		t.Fatalf("classifying: %v", err)
	}

	// Proposed but not approved: no proposals, and the reason says why.
	_, err := e.criteria.Propose(e.initiative, id)
	if err == nil {
		t.Fatal("proposals came from an unapproved profile")
	}
	if !strings.Contains(err.Error(), "approved") {
		t.Errorf("the refusal does not name approval: %v", err)
	}
}

// Reordering with an id from another initiative must not silently succeed.
func TestReorderingRefusesForeignCriteria(t *testing.T) {
	e := newCriteriaEnv(t)
	mine := e.add(t, "five years of production Go", models.CriterionMustHave)

	inits := NewInitiativeService(e.db)
	other, err := inits.Create("Other "+t.Name(), models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("creating the other initiative: %v", err)
	}
	theirs, err := e.criteria.Add(CriterionInput{
		InitiativeID: other.ID, Text: "has run a data platform", Priority: models.CriterionMustHave,
	})
	if err != nil {
		t.Fatalf("adding to the other initiative: %v", err)
	}

	if err := e.criteria.Reorder(e.initiative, []uint{theirs.ID, mine.ID}); err == nil {
		t.Fatal("reordering accepted another initiative's criterion")
	}
}

// What the screen is told cannot be a criterion, and what the service actually
// refuses, have to be the same list.
//
// Blocked exists so a screen can say what is forbidden rather than only that
// something was refused. It had no test, and a list that drifted from the
// enforcement would either promise a protection nothing applies or surprise the
// recruiter with a refusal they were never warned about.
func TestWhatIsListedAsBlockedIsWhatIsActuallyRefused(t *testing.T) {
	e := newCriteriaEnv(t)

	listed := e.criteria.Blocked()
	if len(listed) == 0 {
		t.Fatal("the screen would say nothing is protected")
	}
	want := criteria.Categories()
	if len(listed) != len(want) {
		t.Fatalf("the service lists %d categories and the rules define %d", len(listed), len(want))
	}
	have := map[string]bool{}
	for _, c := range listed {
		have[c] = true
	}
	for _, c := range want {
		if !have[string(c)] {
			t.Fatalf("%q is enforced and the screen would not list it", c)
		}
	}

	// And a criterion drawn from a listed ground is refused, with the ground
	// named rather than a bare rejection.
	// Grounds the hard list refuses outright. It is deliberately narrow — the
	// list refuses and the model warns — so "no visa holders" is not here:
	// "visa" is masked as lawful work-rights phrasing on purpose, and whether
	// that phrasing should be refused outright is a product decision rather
	// than something a test should settle.
	for _, text := range []string{
		"must be under 35", "prefer a male candidate", "must be an Australian citizen",
	} {
		_, err := e.criteria.Add(CriterionInput{
			InitiativeID: e.initiative, Text: text, Priority: string(models.CriterionMustHave),
		})
		if err == nil {
			t.Fatalf("%q was accepted as a criterion", text)
		}
		named := false
		for _, c := range listed {
			if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c)) {
				named = true
			}
		}
		if !named {
			t.Fatalf("%q was refused without naming a ground: %v", text, err)
		}
	}
}
