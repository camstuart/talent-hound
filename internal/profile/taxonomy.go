// Package profile is the contract every derived profile is held to: what an
// aspect may be, what the classifier is asked for, and what must be true before
// any of it is written down.
//
// It has no database and no service dependency on purpose. The rules are pure
// functions over a proposal, which is what lets them be tested exhaustively
// without a model, a transaction, or a fixture directory.
package profile

import "slices"

// AspectType is the controlled, industry-neutral type of one statement.
//
// The list is closed, and the closure is the feature. Matching compares a
// role's aspects against a candidate's by type; a model free to invent
// `culture_fit` on one side and `team_values` on the other has produced two
// aspects that will never meet, with no error anywhere.
type AspectType string

// The controlled aspect types, in the PRD's order.
const (
	Skill           AspectType = "skill"
	Responsibility  AspectType = "responsibility"
	Experience      AspectType = "experience"
	Qualification   AspectType = "qualification"
	Seniority       AspectType = "seniority"
	Location        AspectType = "location"
	WorkArrangement AspectType = "work_arrangement"
	WorkRights      AspectType = "work_rights"
	EmploymentType  AspectType = "employment_type"
	Compensation    AspectType = "compensation"
	// Other is the honest overflow, and it is deliberately useless for
	// matching — which is the correct incentive.
	Other AspectType = "other"
)

// AspectTypes is the whole taxonomy, in the PRD's order.
var AspectTypes = []AspectType{
	Skill, Responsibility, Experience, Qualification, Seniority,
	Location, WorkArrangement, WorkRights, EmploymentType, Compensation, Other,
}

// Valid reports whether t is in the taxonomy.
func (t AspectType) Valid() bool { return slices.Contains(AspectTypes, t) }

// Priority is an employer's weighting of a role requirement.
type Priority string

// The three permitted priorities.
const (
	MustHave   Priority = "must_have"
	NiceToHave Priority = "nice_to_have"
	// Unspecified is a terminal value, not a gap waiting to be filled. The
	// classifier must not invent priority, and nothing downstream promotes it.
	Unspecified Priority = "unspecified"
)

// Priorities is the closed list.
var Priorities = []Priority{MustHave, NiceToHave, Unspecified}

// Valid reports whether p is a permitted priority.
func (p Priority) Valid() bool { return slices.Contains(Priorities, p) }

// Origin says whether a person or a document asserted this.
type Origin string

// The two permitted origins.
const (
	Extracted Origin = "extracted"
	// RecruiterSupplied aspects may exist without an artifact, and are visibly
	// labelled wherever they are used as evidence.
	RecruiterSupplied Origin = "recruiter_supplied"
)

// Origins is the closed list.
var Origins = []Origin{Extracted, RecruiterSupplied}

// Valid reports whether o is a permitted origin.
func (o Origin) Valid() bool { return slices.Contains(Origins, o) }

// SubjectKind is what a profile describes.
type SubjectKind string

// The two things a profile can describe.
const (
	SubjectCandidate SubjectKind = "candidate"
	SubjectRole      SubjectKind = "role"
)

// SubjectKinds is the closed list.
var SubjectKinds = []SubjectKind{SubjectCandidate, SubjectRole}

// Valid reports whether k is a permitted subject kind.
func (k SubjectKind) Valid() bool { return slices.Contains(SubjectKinds, k) }

// structuredFields is the complete set of normalized field names each type may
// carry. A type absent from this map may carry no structured value at all.
//
// An unknown field is a failure rather than something to ignore: the model is
// being asked to normalize, and a normalization nobody consumes is a
// normalization nobody notices is wrong.
var structuredFields = map[AspectType][]string{
	Location:        {"city", "region", "country", "remote_ok"},
	WorkArrangement: {"arrangement", "days_onsite"},
	WorkRights:      {"country", "status", "sponsorship_required"},
	EmploymentType:  {"employment_type"},
	Compensation:    {"currency", "minimum", "maximum", "period", "basis"},
}

// StructuredFields returns the field names t may normalize, and whether t may
// carry a structured value at all.
func StructuredFields(t AspectType) ([]string, bool) {
	fields, ok := structuredFields[t]
	return fields, ok
}

// Enumerated values for the structured fields whose meaning has to be
// comparable rather than merely present. Everything else is free text or a
// number, because normalizing a city name is a different problem than this
// phase is solving.
//
// "unknown" is legal everywhere: a source that does not say is a fact, and
// inventing a value in its place is the failure this whole contract exists to
// prevent.
var structuredEnums = map[string][]string{
	"arrangement":     {"onsite", "hybrid", "remote", "unknown"},
	"employment_type": {"permanent", "contract", "casual", "internship", "unknown"},
	"status":          {"citizen", "permanent_resident", "visa_holder", "requires_sponsorship", "unknown"},
	"period":          {"hour", "day", "week", "month", "year", "unknown"},
	"basis":           {"base", "total_package", "rate", "unknown"},
}

// StructuredEnum returns the permitted values for a field, when it is
// enumerated.
func StructuredEnum(field string) ([]string, bool) {
	values, ok := structuredEnums[field]
	return values, ok
}
