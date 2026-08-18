package fusion

import (
	"math"
	"testing"

	"camstuart/talent-hound/internal/profile"
)

// Every fixture here is invented. No real candidate information appears in this
// repository's tests, fixtures, or output.

// rrf is the hand-calculated contribution of one rank, so the assertions below
// are arithmetic a reader can check rather than a comparison with the code.
func rrf(ranks ...int) float64 {
	var total float64
	for _, r := range ranks {
		total += 1 / float64(K+r)
	}
	return total
}

func closeTo(t *testing.T, got, want float64, what string) {
	t.Helper()
	if math.Abs(got-want) > 1e-12 {
		t.Errorf("%s scored %v, want %v", what, got, want)
	}
}

func TestAKeyFoundByThreeListsOutranksOneFoundFirstByOne(t *testing.T) {
	fused := Fuse([]Ranked{
		{Source: "a", Method: "lexical", Keys: []uint{2, 1}},
		{Source: "b", Method: "lexical", Keys: []uint{3, 1}},
		{Source: "c", Method: "semantic", Keys: []uint{4, 1}},
	})
	if len(fused) != 4 {
		t.Fatalf("got %d keys", len(fused))
	}
	// Key 1 is second in all three lists; keys 2, 3, 4 are first in one each.
	if fused[0].Key != 1 {
		t.Fatalf("the key found by three lists ranked %d, want first: %+v", fused[0].Key, fused)
	}
	closeTo(t, fused[0].Score, rrf(2, 2, 2), "the three-list key")
	closeTo(t, fused[1].Score, rrf(1), "a one-list key")
}

func TestLexicalOnlyAndSemanticOnlyBothContribute(t *testing.T) {
	fused := Fuse([]Ranked{
		{Source: "criterion", Method: "lexical", Keys: []uint{1}},
		{Source: "criterion", Method: "semantic", Keys: []uint{2}},
	})
	if len(fused) != 2 {
		t.Fatalf("got %d keys, want both", len(fused))
	}
	// Equal scores, so the identifier breaks the tie.
	closeTo(t, fused[0].Score, rrf(1), "the lexical-only key")
	closeTo(t, fused[1].Score, rrf(1), "the semantic-only key")
	if fused[0].Key != 1 || fused[1].Key != 2 {
		t.Errorf("tied keys are out of identifier order: %+v", fused)
	}
}

func TestAnEmptyListContributesNothing(t *testing.T) {
	with := Fuse([]Ranked{
		{Source: "a", Method: "lexical", Keys: []uint{1, 2}},
		{Source: "b", Method: "semantic", Keys: nil},
	})
	without := Fuse([]Ranked{
		{Source: "a", Method: "lexical", Keys: []uint{1, 2}},
	})
	if len(with) != len(without) {
		t.Fatalf("an empty list changed the result: %d vs %d keys", len(with), len(without))
	}
	for i := range with {
		if with[i].Key != without[i].Key || with[i].Score != without[i].Score {
			t.Fatalf("an empty list changed position %d: %+v vs %+v", i, with[i], without[i])
		}
	}
}

func TestNoListsAtAllIsAnEmptyResult(t *testing.T) {
	if got := Fuse(nil); len(got) != 0 {
		t.Fatalf("fusing nothing gave %+v", got)
	}
	if got := Fuse([]Ranked{{Source: "a", Method: "lexical"}}); len(got) != 0 {
		t.Fatalf("fusing one empty list gave %+v", got)
	}
}

// Repetition within a list is a retrieval artifact, not evidence.
func TestADuplicateWithinOneListCountsOnceAtItsBestRank(t *testing.T) {
	fused := Fuse([]Ranked{
		{Source: "a", Method: "lexical", Keys: []uint{1, 2, 1, 1}},
	})
	if len(fused) != 2 {
		t.Fatalf("got %d keys, want 2: %+v", len(fused), fused)
	}
	var one Fused
	for _, f := range fused {
		if f.Key == 1 {
			one = f
		}
	}
	closeTo(t, one.Score, rrf(1), "the duplicated key")
	if len(one.Why) != 1 {
		t.Errorf("the duplicated key records %d contributions, want one: %+v", len(one.Why), one.Why)
	}
	if one.Why[0].Rank != 1 {
		t.Errorf("the duplicated key recorded rank %d, want its best", one.Why[0].Rank)
	}
}

func TestTiesOrderByIdentifierEveryTime(t *testing.T) {
	lists := []Ranked{
		{Source: "a", Method: "lexical", Keys: []uint{9, 3, 7, 1}},
		{Source: "b", Method: "semantic", Keys: []uint{1, 7, 3, 9}},
	}
	first := Fuse(lists)
	for run := range 50 {
		again := Fuse(lists)
		if len(again) != len(first) {
			t.Fatalf("run %d gave %d keys, first gave %d", run, len(again), len(first))
		}
		for i := range again {
			if again[i].Key != first[i].Key || again[i].Score != first[i].Score {
				t.Fatalf("run %d differs at %d: %+v vs %+v", run, i, again[i], first[i])
			}
		}
	}
	// Every key here appears once in each list at complementary ranks, so all
	// four scores are equal and the order is purely by identifier.
	for i := 1; i < len(first); i++ {
		if first[i].Score == first[i-1].Score && first[i].Key < first[i-1].Key {
			t.Fatalf("tied keys out of order: %d before %d", first[i-1].Key, first[i].Key)
		}
	}
}

func TestOverlappingListsAccumulate(t *testing.T) {
	fused := Fuse([]Ranked{
		{Source: "a", Method: "lexical", Keys: []uint{1, 2, 3}},
		{Source: "b", Method: "semantic", Keys: []uint{3, 2, 1}},
	})
	byKey := map[uint]Fused{}
	for _, f := range fused {
		byKey[f.Key] = f
	}
	closeTo(t, byKey[1].Score, rrf(1, 3), "key 1")
	closeTo(t, byKey[2].Score, rrf(2, 2), "key 2")
	closeTo(t, byKey[3].Score, rrf(3, 1), "key 3")
	// Keys 1 and 3 tie and both beat key 2, which is worth stating because it
	// is counter-intuitive: reciprocals are convex, so 1/61 + 1/63 is larger
	// than 1/62 + 1/62. A key that was first somewhere beats a key that was
	// never first anywhere, which is the behaviour a shortlist wants.
	if fused[0].Key != 1 || fused[1].Key != 3 || fused[2].Key != 2 {
		t.Errorf("the spread-rank keys did not beat the consistently-middle one: %+v", fused)
	}
	if fused[0].Score <= fused[2].Score {
		t.Errorf("a key that was first somewhere did not beat one that never was: %+v", fused)
	}
}

func TestProvenanceNamesEveryListThatContributed(t *testing.T) {
	fused := Fuse([]Ranked{
		{Source: "five years of Go", Method: "lexical", Keys: []uint{1}},
		{Source: "five years of Go", Method: "semantic", Keys: []uint{1}},
		{Source: "has led a team", Method: "semantic", Keys: []uint{1}},
	})
	if len(fused) != 1 {
		t.Fatalf("got %d keys", len(fused))
	}
	why := fused[0].Why
	if len(why) != 3 {
		t.Fatalf("recorded %d contributions, want 3: %+v", len(why), why)
	}
	// In list order, so provenance reads the same way twice.
	if why[0].Method != "lexical" || why[1].Method != "semantic" || why[2].Source != "has led a team" {
		t.Errorf("provenance is out of list order: %+v", why)
	}
}

func TestTopReturnsAtMostNAndAllWhenFewer(t *testing.T) {
	many := make([]Fused, 0, 30)
	for key := uint(1); key <= 30; key++ {
		many = append(many, Fused{Key: key})
	}
	if got := Top(many, 20); len(got) != 20 {
		t.Errorf("Top(30, 20) gave %d", len(got))
	}
	if got := Top(many[:7], 20); len(got) != 7 {
		t.Errorf("Top(7, 20) gave %d", len(got))
	}
	if got := Top(nil, 20); len(got) != 0 {
		t.Errorf("Top(nil, 20) gave %d", len(got))
	}
}

// The map exactly as the PRD writes it, transcribed here independently so the
// test disagrees with the code if either is edited alone.
var prdMap = map[profile.AspectType][]profile.AspectType{
	profile.Skill:          {profile.Skill, profile.Experience, profile.Responsibility},
	profile.Responsibility: {profile.Responsibility, profile.Experience, profile.Skill},
	profile.Experience:     {profile.Experience, profile.Responsibility},
	profile.Qualification:  {profile.Qualification},
	profile.Seniority:      {profile.Seniority, profile.Experience},
	profile.Other:          {profile.Other},
}

func TestEveryEdgeOfTheCompatibilityMapIsPresent(t *testing.T) {
	got := Map()
	if len(got) != len(prdMap) {
		t.Fatalf("the map has %d role types, the PRD states %d", len(got), len(prdMap))
	}
	for role, want := range prdMap {
		have := CandidateAspectsFor(role)
		if len(have) != len(want) {
			t.Errorf("%s searches %v, the PRD says %v", role, have, want)
			continue
		}
		for i := range want {
			if have[i] != want[i] {
				t.Errorf("%s searches %v, the PRD says %v", role, have, want)
				break
			}
		}
	}
}

// The absences matter as much as the entries.
func TestEveryDisallowedEdgeIsAbsent(t *testing.T) {
	permitted := map[string]bool{}
	for role, searched := range prdMap {
		for _, c := range searched {
			permitted[string(role)+"->"+string(c)] = true
		}
	}
	for _, role := range profile.AspectTypes {
		for _, candidate := range profile.AspectTypes {
			edge := string(role) + "->" + string(candidate)
			searched := CandidateAspectsFor(role)
			present := false
			for _, s := range searched {
				if s == candidate {
					present = true
				}
			}
			if present != permitted[edge] {
				t.Errorf("edge %s is %v, want %v", edge, present, permitted[edge])
			}
		}
	}
}

func TestAQualificationSearchesNothingElse(t *testing.T) {
	searched := CandidateAspectsFor(profile.Qualification)
	if len(searched) != 1 || searched[0] != profile.Qualification {
		t.Fatalf("a qualification searches %v", searched)
	}
	// And nothing but a qualification finds one.
	found := RoleAspectsFor(profile.Qualification)
	if len(found) != 1 || found[0] != profile.Qualification {
		t.Fatalf("a qualification is found by %v", found)
	}
}

// A hand-written inverse drifts the first time someone adds an edge, and the
// drift is invisible.
func TestTheInverseAgreesWithTheMap(t *testing.T) {
	for _, role := range profile.AspectTypes {
		for _, candidate := range CandidateAspectsFor(role) {
			back := RoleAspectsFor(candidate)
			found := false
			for _, r := range back {
				if r == role {
					found = true
				}
			}
			if !found {
				t.Errorf("%s searches %s, but %s is not found by %s", role, candidate, candidate, role)
			}
		}
	}
	for _, candidate := range profile.AspectTypes {
		for _, role := range RoleAspectsFor(candidate) {
			searched := CandidateAspectsFor(role)
			found := false
			for _, c := range searched {
				if c == candidate {
					found = true
				}
			}
			if !found {
				t.Errorf("%s is found by %s, but %s does not search %s", candidate, role, role, candidate)
			}
		}
	}
}

func TestTheInverseIsAsymmetricWhereThePRDIs(t *testing.T) {
	// experience searches responsibility …
	if got := CandidateAspectsFor(profile.Experience); len(got) != 2 {
		t.Fatalf("experience searches %v", got)
	}
	// … and is itself found by skill, responsibility, experience, and seniority.
	found := RoleAspectsFor(profile.Experience)
	want := map[profile.AspectType]bool{
		profile.Skill: true, profile.Responsibility: true,
		profile.Experience: true, profile.Seniority: true,
	}
	if len(found) != len(want) {
		t.Fatalf("experience is found by %v, want %d types", found, len(want))
	}
	for _, r := range found {
		if !want[r] {
			t.Errorf("experience is found by %s, which the map does not permit", r)
		}
	}
}

func TestStructuredTypesAreComparedRatherThanSearched(t *testing.T) {
	structured := []profile.AspectType{
		profile.Location, profile.WorkArrangement, profile.WorkRights,
		profile.EmploymentType, profile.Compensation,
	}
	for _, typ := range structured {
		if !IsStructured(typ) {
			t.Errorf("%s is not marked structured", typ)
		}
		if Searchable(typ) {
			t.Errorf("%s takes part in similarity retrieval", typ)
		}
		if got := CandidateAspectsFor(typ); len(got) != 0 {
			t.Errorf("%s searches %v — a structured type is compared, not searched", typ, got)
		}
	}
	for _, typ := range []profile.AspectType{
		profile.Skill, profile.Responsibility, profile.Experience,
		profile.Qualification, profile.Seniority, profile.Other,
	} {
		if IsStructured(typ) {
			t.Errorf("%s is marked structured", typ)
		}
		if !Searchable(typ) {
			t.Errorf("%s is not searchable", typ)
		}
	}
}
