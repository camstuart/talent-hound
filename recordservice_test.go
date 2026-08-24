package main

import (
	"reflect"
	"strings"
	"testing"

	"camstuart/talent-hound/internal/models"
)

// Every value below is invented. Real candidate information never enters this
// repository, its logs, or its test output.

func newRecordService(t *testing.T) *RecordService {
	t.Helper()
	return NewRecordService(newTestDB(t))
}

func amount(v int64) *int64 { return &v }

func TestCandidateRoundTripsEveryField(t *testing.T) {
	r := newRecordService(t)

	in := models.Candidate{
		FullName:               "Priya Raman",
		PreferredName:          "Pri",
		Emails:                 models.StringList{"pri@example.test", "priya@example.test"},
		Phones:                 models.StringList{"+64 21 555 0100", "+64 9 555 0111"},
		Location:               "Wellington, New Zealand",
		WorkRights:             "Citizen — no visa required",
		Availability:           "2026-09-01",
		DesiredEmploymentType:  "Permanent",
		DesiredWorkArrangement: "Hybrid, two days on site",
		Compensation: models.Compensation{
			Min: amount(160000), Max: amount(185000), Currency: "NZD", Period: models.PeriodYear,
		},
		SourceNote:    "Stated by the candidate on a call",
		LastConfirmed: "2026-08-14",
	}
	created, err := r.CreateCandidate(in)
	if err != nil {
		t.Fatalf("CreateCandidate: %v", err)
	}

	got, err := r.GetCandidate(created.ID)
	if err != nil {
		t.Fatalf("GetCandidate: %v", err)
	}
	in.ID, in.CreatedAt, in.UpdatedAt = got.ID, got.CreatedAt, got.UpdatedAt
	if !reflect.DeepEqual(*got, in) {
		t.Errorf("round trip changed the candidate:\n got %+v\nwant %+v", *got, in)
	}
	if len(got.Emails) != 2 || got.Emails[1] != "priya@example.test" {
		t.Errorf("email list did not round trip: %v", got.Emails)
	}
	if len(got.Phones) != 2 {
		t.Errorf("phone list did not round trip: %v", got.Phones)
	}
}

func TestCandidateNeedsOnlyAFullName(t *testing.T) {
	r := newRecordService(t)

	got, err := r.CreateCandidate(models.Candidate{FullName: "Priya Raman"})
	if err != nil {
		t.Fatalf("CreateCandidate: %v", err)
	}
	if got.PreferredName != "" || got.Location != "" || got.Availability != "" || got.Compensation.Stated() {
		t.Errorf("unstated fields were defaulted to placeholders: %+v", got)
	}
	if len(got.Emails) != 0 || len(got.Phones) != 0 {
		t.Errorf("unstated lists are not empty: %v %v", got.Emails, got.Phones)
	}
}

func TestRecordValidation(t *testing.T) {
	r := newRecordService(t)
	company, err := r.CreateCompany(models.Company{Name: "Northwind Robotics"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	missingCompany := uint(9999)

	cases := []struct {
		name    string
		create  func() error
		wantErr string
	}{
		{
			name:    "candidate without a name",
			create:  func() error { _, err := r.CreateCandidate(models.Candidate{FullName: "  \t "}); return err },
			wantErr: "candidate full name must not be empty",
		},
		{
			name: "candidate with an unparseable date",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", Availability: "next Tuesday"})
				return err
			},
			wantErr: "YYYY-MM-DD",
		},
		{
			name: "candidate with an impossible date",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", LastConfirmed: "2026-02-30"})
				return err
			},
			wantErr: "YYYY-MM-DD",
		},
		{
			name: "negative compensation",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", Compensation: models.Compensation{
					Min: amount(-1), Currency: "NZD", Period: models.PeriodYear}})
				return err
			},
			wantErr: "must not be negative",
		},
		{
			name: "minimum above maximum",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", Compensation: models.Compensation{
					Min: amount(200000), Max: amount(100000), Currency: "NZD", Period: models.PeriodYear}})
				return err
			},
			wantErr: "greater than maximum",
		},
		{
			name: "absurd amount",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", Compensation: models.Compensation{
					Min: amount(9_000_000_000), Currency: "NZD", Period: models.PeriodYear}})
				return err
			},
			wantErr: "typo",
		},
		{
			name: "unknown currency",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", Compensation: models.Compensation{
					Min: amount(100), Currency: "dollars", Period: models.PeriodHour}})
				return err
			},
			wantErr: "ISO 4217",
		},
		{
			name: "unknown period",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", Compensation: models.Compensation{
					Min: amount(100), Currency: "NZD", Period: models.CompensationPeriod("fortnight")}})
				return err
			},
			wantErr: "period must be one of",
		},
		{
			name: "compensation with no currency at all",
			create: func() error {
				_, err := r.CreateCandidate(models.Candidate{FullName: "A", Compensation: models.Compensation{Min: amount(100)}})
				return err
			},
			wantErr: "ISO 4217",
		},
		{
			name: "company website that is a bare domain",
			create: func() error {
				_, err := r.CreateCompany(models.Company{Name: "N", Website: "northwind.test"})
				return err
			},
			wantErr: "absolute http or https URL",
		},
		{
			name: "company website with the wrong scheme",
			create: func() error {
				_, err := r.CreateCompany(models.Company{Name: "N", Website: "ftp://northwind.test"})
				return err
			},
			wantErr: "absolute http or https URL",
		},
		{
			name:    "company without a name",
			create:  func() error { _, err := r.CreateCompany(models.Company{Name: " "}); return err },
			wantErr: "company name must not be empty",
		},
		{
			name:    "contact without a company",
			create:  func() error { _, err := r.CreateContact(models.Contact{FullName: "Dana Okafor"}); return err },
			wantErr: "must belong to a company",
		},
		{
			name: "contact at a company that does not exist",
			create: func() error {
				_, err := r.CreateContact(models.Contact{FullName: "Dana Okafor", CompanyID: missingCompany})
				return err
			},
			wantErr: "loading company",
		},
		{
			name:    "contact without a name",
			create:  func() error { _, err := r.CreateContact(models.Contact{CompanyID: company.ID}); return err },
			wantErr: "contact full name must not be empty",
		},
		{
			name: "role without a title",
			create: func() error {
				_, err := r.CreateRole(models.Role{Origin: models.RoleOriginDiscovered, LifecycleState: models.RoleActive})
				return err
			},
			wantErr: "role title must not be empty",
		},
		{
			name: "role with an unknown origin",
			create: func() error {
				_, err := r.CreateRole(models.Role{Title: "T", Origin: "imagined", LifecycleState: models.RoleActive})
				return err
			},
			wantErr: "unknown role origin",
		},
		{
			name: "discovered role in a recruiter-entered state",
			create: func() error {
				_, err := r.CreateRole(models.Role{Title: "T", Origin: models.RoleOriginDiscovered, LifecycleState: models.RoleFilled})
				return err
			},
			wantErr: "not valid for a discovered role",
		},
		{
			name: "recruiter-entered role in a discovered state",
			create: func() error {
				_, err := r.CreateRole(models.Role{Title: "T", Origin: models.RoleOriginRecruiterEntered, LifecycleState: models.RoleStale})
				return err
			},
			wantErr: "not valid for a recruiter_entered role",
		},
		{
			name: "role closing before it was published",
			create: func() error {
				_, err := r.CreateRole(models.Role{Title: "T", Origin: models.RoleOriginDiscovered, LifecycleState: models.RoleActive,
					PublishedOn: "2026-08-10", ClosingOn: "2026-08-01"})
				return err
			},
			wantErr: "precedes its published date",
		},
		{
			name: "role with a relative canonical URL",
			create: func() error {
				_, err := r.CreateRole(models.Role{Title: "T", Origin: models.RoleOriginDiscovered, LifecycleState: models.RoleActive,
					CanonicalURL: "/jobs/123"})
				return err
			},
			wantErr: "absolute http or https URL",
		},
		{
			name: "role at a company that does not exist",
			create: func() error {
				_, err := r.CreateRole(models.Role{Title: "T", Origin: models.RoleOriginDiscovered, LifecycleState: models.RoleActive,
					CompanyID: &missingCompany})
				return err
			},
			wantErr: "loading company",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.create()
			if err == nil {
				t.Fatalf("accepted an invalid record, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}

	// Nothing invalid was persisted: only the one company from the setup.
	companies, err := r.ListCompanies()
	if err != nil {
		t.Fatalf("ListCompanies: %v", err)
	}
	if len(companies) != 1 {
		t.Errorf("invalid records were persisted: %d companies", len(companies))
	}
	candidates, err := r.ListCandidates()
	if err != nil {
		t.Fatalf("ListCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("invalid candidates were persisted: %d", len(candidates))
	}
	roles, err := r.ListRoles()
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("invalid roles were persisted: %d", len(roles))
	}
}

func TestValidationAcceptsTheAwkwardButValid(t *testing.T) {
	r := newRecordService(t)

	// Unicode survives untouched; only surrounding whitespace is trimmed.
	unicode := "  Zoë  Ólafsdóttir-李  "
	got, err := r.CreateCandidate(models.Candidate{
		FullName:   unicode,
		Location:   "Reykjavík 🇮🇸",
		SourceNote: "combining marks: é",
		Emails:     models.StringList{"  spaced@example.test  ", "   ", ""},
	})
	if err != nil {
		t.Fatalf("CreateCandidate: %v", err)
	}
	if got.FullName != "Zoë  Ólafsdóttir-李" {
		t.Errorf("full name was mangled: %q", got.FullName)
	}
	if got.Location != "Reykjavík 🇮🇸" || got.SourceNote != "combining marks: é" {
		t.Errorf("unicode content was rewritten: %+v", got)
	}
	if len(got.Emails) != 1 || got.Emails[0] != "spaced@example.test" {
		t.Errorf("email list was not cleaned to one trimmed entry: %v", got.Emails)
	}

	// Partially stated compensation: only a minimum, only a maximum.
	for _, comp := range []models.Compensation{
		{Min: amount(95), Currency: "aud", Period: models.PeriodHour},
		{Max: amount(220000), Currency: "USD", Period: "YEAR"},
		{Min: amount(0), Max: amount(0), Currency: "NZD", Period: models.PeriodDay},
	} {
		if _, err := r.CreateCandidate(models.Candidate{FullName: "A", Compensation: comp}); err != nil {
			t.Errorf("rejected valid compensation %+v: %v", comp, err)
		}
	}

	// A recruiter-entered role needs no source metadata at all.
	role, err := r.CreateRole(models.Role{
		Title:          "  Staff Engineer  ",
		CompanyName:    "Northwind Robotics",
		Origin:         models.RoleOriginRecruiterEntered,
		LifecycleState: models.RoleOpen,
	})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if role.Title != "Staff Engineer" {
		t.Errorf("role title was not trimmed: %q", role.Title)
	}
	if role.SourceID != "" || role.CanonicalURL != "" || role.RetrievedOn != "" || role.CompanyID != nil {
		t.Errorf("absent source metadata was defaulted: %+v", role)
	}

	// A contact with neither email nor phone is fine.
	company, err := r.CreateCompany(models.Company{Name: "Northwind Robotics", Website: "https://northwind.test"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	if _, err := r.CreateContact(models.Contact{CompanyID: company.ID, FullName: "Dana Okafor"}); err != nil {
		t.Errorf("rejected a contact with no email or phone: %v", err)
	}
}

func TestRoleRoundTripsEveryField(t *testing.T) {
	r := newRecordService(t)
	company, err := r.CreateCompany(models.Company{Name: "Northwind Robotics", Website: "https://northwind.test",
		Location: "Auckland", Source: "Manually entered"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}

	in := models.Role{
		Title:           "Staff Platform Engineer",
		CompanyName:     "Northwind Robotics",
		CompanyID:       &company.ID,
		Location:        "Auckland, New Zealand",
		WorkArrangement: "Remote within NZ",
		EmploymentType:  "Permanent full-time",
		Compensation:    models.Compensation{Min: amount(180000), Max: amount(210000), Currency: "NZD", Period: models.PeriodYear},
		PublishedOn:     "2026-08-01",
		ClosingOn:       "2026-08-31",
		RetrievedOn:     "2026-08-16",
		SourceID:        "nwr-2026-118",
		CanonicalURL:    "https://northwind.test/jobs/118",
		Source:          "Company careers page",
		Origin:          models.RoleOriginDiscovered,
		LifecycleState:  models.RoleActive,
	}
	created, err := r.CreateRole(in)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	got, err := r.GetRole(created.ID)
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}
	in.ID, in.CreatedAt, in.UpdatedAt = got.ID, got.CreatedAt, got.UpdatedAt
	if got.CompanyID == nil || *got.CompanyID != company.ID {
		t.Fatalf("company reference lost: %v", got.CompanyID)
	}
	in.CompanyID = got.CompanyID
	if !reflect.DeepEqual(*got, in) {
		t.Errorf("round trip changed the role:\n got %+v\nwant %+v", *got, in)
	}
}

func TestRoleMayNameACompanyWithNoRecord(t *testing.T) {
	r := newRecordService(t)

	got, err := r.CreateRole(models.Role{
		Title:          "Data Engineer",
		CompanyName:    "A company we have never met",
		Origin:         models.RoleOriginDiscovered,
		LifecycleState: models.RoleActive,
	})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if got.CompanyName != "A company we have never met" || got.CompanyID != nil {
		t.Errorf("company name and reference are not independent: %+v", got)
	}
}

func TestCompanyAndContactRoundTrip(t *testing.T) {
	r := newRecordService(t)

	company, err := r.CreateCompany(models.Company{
		Name: "Northwind Robotics", Website: "https://northwind.test",
		Location: "Auckland", Source: "Introduced by a client",
	})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	contact, err := r.CreateContact(models.Contact{
		CompanyID: company.ID, FullName: "Dana Okafor", Title: "Head of Engineering",
		Email: "dana@northwind.test", Phone: "+64 9 555 0199", Source: "Met at a meetup",
	})
	if err != nil {
		t.Fatalf("CreateContact: %v", err)
	}

	gotCompany, err := r.GetCompany(company.ID)
	if err != nil {
		t.Fatalf("GetCompany: %v", err)
	}
	if gotCompany.Website != "https://northwind.test" || gotCompany.Location != "Auckland" || gotCompany.Source != "Introduced by a client" {
		t.Errorf("company did not round trip: %+v", gotCompany)
	}
	gotContact, err := r.GetContact(contact.ID)
	if err != nil {
		t.Fatalf("GetContact: %v", err)
	}
	if gotContact.CompanyID != company.ID || gotContact.Title != "Head of Engineering" ||
		gotContact.Email != "dana@northwind.test" || gotContact.Phone != "+64 9 555 0199" || gotContact.Source != "Met at a meetup" {
		t.Errorf("contact did not round trip: %+v", gotContact)
	}
}

func TestContactsAtCompany(t *testing.T) {
	r := newRecordService(t)

	northwind, err := r.CreateCompany(models.Company{Name: "Northwind Robotics"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	southerly, err := r.CreateCompany(models.Company{Name: "Southerly Systems"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}
	empty, err := r.CreateCompany(models.Company{Name: "Nobody We Know"})
	if err != nil {
		t.Fatalf("CreateCompany: %v", err)
	}

	for _, seed := range []struct {
		company uint
		name    string
	}{
		{northwind.ID, "Dana Okafor"},
		{northwind.ID, "Alex Whitcombe"},
		{southerly.ID, "Jordan Meiring"},
	} {
		if _, err := r.CreateContact(models.Contact{CompanyID: seed.company, FullName: seed.name}); err != nil {
			t.Fatalf("CreateContact: %v", err)
		}
	}

	at, err := r.ContactsAtCompany(northwind.ID)
	if err != nil {
		t.Fatalf("ContactsAtCompany: %v", err)
	}
	if at.Count != 2 || len(at.Contacts) != 2 {
		t.Fatalf("count %d, listing %d, want 2 and 2", at.Count, len(at.Contacts))
	}
	if at.Company.Name != "Northwind Robotics" {
		t.Errorf("result names the wrong company: %q", at.Company.Name)
	}
	for _, c := range at.Contacts {
		if c.CompanyID != northwind.ID {
			t.Errorf("contact %q belongs to company %d, not the selected one", c.FullName, c.CompanyID)
		}
	}

	none, err := r.ContactsAtCompany(empty.ID)
	if err != nil {
		t.Fatalf("a company with no contacts is not an error: %v", err)
	}
	if none.Count != 0 || len(none.Contacts) != 0 {
		t.Errorf("empty company returned %d contacts", none.Count)
	}

	if _, err := r.ContactsAtCompany(9999); err == nil {
		t.Error("an unknown company returned an empty result instead of an error")
	}
}

func TestSharedRoleIsReferencedNotCopied(t *testing.T) {
	s, r := newServices(t)
	role, err := r.CreateRole(models.Role{Title: "Staff Engineer",
		Origin: models.RoleOriginRecruiterEntered, LifecycleState: models.RoleOpen})
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	first, err := s.Create("Search A", models.InitiativeTypeTalentSearch, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Archiving and reopening an initiative never touches a shared record.
	if _, err := s.Archive(first.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if _, err := r.GetRole(role.ID); err != nil {
		t.Fatalf("archiving removed the role: %v", err)
	}
	if _, err := s.Reopen(first.ID); err != nil {
		t.Fatalf("Reopen: %v", err)
	}

	role.LifecycleState = models.RoleFilled
	saved, err := r.UpdateRole(*role)
	if err != nil {
		t.Fatalf("UpdateRole: %v", err)
	}
	if saved.LifecycleState != models.RoleFilled {
		t.Errorf("UpdateRole returned %q", saved.LifecycleState)
	}
	roles, err := r.ListRoles()
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("role was copied: %d rows", len(roles))
	}
	if roles[0].LifecycleState != models.RoleFilled {
		t.Errorf("update did not stick: %q", roles[0].LifecycleState)
	}
}

func TestUpdateRejectsUnknownRecords(t *testing.T) {
	r := newRecordService(t)

	if _, err := r.UpdateCandidate(models.Candidate{ID: 9999, FullName: "Ghost"}); err == nil {
		t.Error("UpdateCandidate accepted an unknown candidate")
	}
	if _, err := r.UpdateCompany(models.Company{ID: 9999, Name: "Ghost"}); err == nil {
		t.Error("UpdateCompany accepted an unknown company")
	}
	if _, err := r.UpdateContact(models.Contact{ID: 9999, CompanyID: 1, FullName: "Ghost"}); err == nil {
		t.Error("UpdateContact accepted an unknown contact")
	}
	if _, err := r.UpdateRole(models.Role{ID: 9999, Title: "Ghost",
		Origin: models.RoleOriginRecruiterEntered, LifecycleState: models.RoleOpen}); err == nil {
		t.Error("UpdateRole accepted an unknown role")
	}
}

func TestSearchCandidatesFiltersByTextAndFields(t *testing.T) {
	gdb := newTestDB(t)
	s := NewRecordService(gdb)
	mk := func(name, location, rights, employment string, available models.Date) {
		t.Helper()
		_, err := s.CreateCandidate(models.Candidate{
			FullName: name, Location: location, WorkRights: rights,
			DesiredEmploymentType: employment, Availability: available,
		})
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
	}
	mk("Alice Amber", "Sydney", "citizen", "permanent", "2026-09-01")
	mk("Bob Blue", "Melbourne", "visa", "contract", "2026-12-01")
	mk("Cara Crimson", "Sydney", "citizen", "contract", "")

	got, err := s.SearchCandidates(CandidateFilter{Text: "sydney"})
	if err != nil {
		t.Fatalf("searching: %v", err)
	}
	if len(got) != 2 || got[0].FullName != "Alice Amber" || got[1].FullName != "Cara Crimson" {
		t.Fatalf("text filter wrong: %+v", names(got))
	}

	got, _ = s.SearchCandidates(CandidateFilter{WorkRights: "citizen", EmploymentType: "contract"})
	if len(got) != 1 || got[0].FullName != "Cara Crimson" {
		t.Fatalf("field filters wrong: %+v", names(got))
	}

	// AvailableBy keeps people available on or before the date; an empty
	// availability means unknown and is kept.
	got, _ = s.SearchCandidates(CandidateFilter{AvailableBy: "2026-10-01"})
	if len(got) != 2 {
		t.Fatalf("availability filter wrong: %+v", names(got))
	}

	// No filters returns everyone, by name.
	got, _ = s.SearchCandidates(CandidateFilter{})
	if len(got) != 3 {
		t.Fatalf("unfiltered wrong: %+v", names(got))
	}
}

func names(cs []models.Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.FullName
	}
	return out
}

func TestSearchCompaniesAndContactsMatchByName(t *testing.T) {
	gdb := newTestDB(t)
	s := NewRecordService(gdb)
	co, err := s.CreateCompany(models.Company{Name: "Northwind Industries"})
	if err != nil {
		t.Fatalf("company: %v", err)
	}
	if _, err := s.CreateCompany(models.Company{Name: "Contoso"}); err != nil {
		t.Fatalf("company: %v", err)
	}
	if _, err := s.CreateContact(models.Contact{CompanyID: co.ID, FullName: "Dana Doe", Email: "dana@northwind.test"}); err != nil {
		t.Fatalf("contact: %v", err)
	}

	cos, err := s.SearchCompanies("north")
	if err != nil || len(cos) != 1 || cos[0].Name != "Northwind Industries" {
		t.Fatalf("company search wrong: %v %+v", err, cos)
	}
	people, err := s.SearchContacts("northwind.test")
	if err != nil || len(people) != 1 || people[0].FullName != "Dana Doe" {
		t.Fatalf("contact search wrong: %v %+v", err, people)
	}
}
