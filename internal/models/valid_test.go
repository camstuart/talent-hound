package models

import "testing"

// The enumerations that decide what may be written to the database, checked
// against what they are supposed to admit and — more usefully — what they are
// supposed to refuse.
//
// These had no test. They are one line each, which is exactly why: a one-line
// membership check is obviously right until someone adds a value to the list
// and not to the constant, or the reverse, and then it is obviously wrong in a
// place nobody looks.
func TestTheEnumerationsAdmitTheirOwnValuesAndNothingElse(t *testing.T) {
	t.Run("initiative types", func(t *testing.T) {
		for _, ok := range []InitiativeType{
			InitiativeTypeJobSearch, InitiativeTypeTalentSearch, InitiativeTypeBusinessDevelopment,
		} {
			if !ok.Valid() {
				t.Errorf("%q is a declared initiative type and is refused", ok)
			}
		}
		for _, bad := range []InitiativeType{"", " ", "job_search ", "JobSearch", "jobsearch",
			"job-search", "recruitment", "0"} {
			if bad.Valid() {
				t.Errorf("%q is accepted as an initiative type", bad)
			}
		}
	})

	t.Run("job states", func(t *testing.T) {
		for _, ok := range JobStates {
			if !ok.Valid() {
				t.Errorf("%q is in JobStates and is refused", ok)
			}
		}
		for _, bad := range []JobState{"", "COMPLETED", "completed ", "done", "finished", "ok"} {
			if bad.Valid() {
				t.Errorf("%q is accepted as a job state", bad)
			}
		}
	})

	t.Run("model roles", func(t *testing.T) {
		for _, ok := range ModelRoles {
			if !ok.Valid() {
				t.Errorf("%q is in ModelRoles and is refused", ok)
			}
		}
		for _, bad := range []ModelRole{"", "Classify", "classify ", "chat", "rerank", "embedding"} {
			if bad.Valid() {
				t.Errorf("%q is accepted as a model role", bad)
			}
		}
	})

	t.Run("validation statuses", func(t *testing.T) {
		for _, ok := range []ValidationStatus{Unvalidated, Validated} {
			if !ok.Valid() {
				t.Errorf("%q is a declared status and is refused", ok)
			}
		}
		// "failed" is the one worth naming: a model that failed the benchmarks
		// is not a third state here, it is simply not validated, and a status
		// column that could say "failed" would let a caller treat it as a
		// judgement the application does not make.
		for _, bad := range []ValidationStatus{"", "failed", "Validated", "valid", "pending"} {
			if bad.Valid() {
				t.Errorf("%q is accepted as a validation status", bad)
			}
		}
	})
}
