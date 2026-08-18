## ADDED Requirements

### Requirement: Structured candidate record
The application SHALL persist a Candidate with full name, preferred name, email addresses, phone numbers, location, work-rights or visa details, availability, desired employment type, desired work arrangement, compensation or rate expectations, data-source or authority note, and last-confirmed date. Only full name SHALL be required. The application SHALL NOT store employment history, education, skills, achievements, or qualifications on this record.

#### Scenario: Candidate round-trips every defined field
- **WHEN** a candidate is created with all defined fields populated and then read back
- **THEN** every field returns the value that was stored, including multiple email addresses and multiple phone numbers

#### Scenario: Candidate with only a full name is valid
- **WHEN** a candidate is created with only a full name
- **THEN** it is persisted and every other field is empty rather than defaulted to a placeholder

#### Scenario: Missing full name is rejected
- **WHEN** a candidate is created with an empty or whitespace-only full name
- **THEN** creation fails with an error naming the field and nothing is persisted

### Requirement: Structured role record
The application SHALL persist a Role with title, company, location, work arrangement, employment type, compensation or rate when stated, published date, closing date, retrieved date, source identifier, canonical URL, source, recruiter-entered versus discovered origin, and lifecycle state. Responsibilities and requirements SHALL NOT be stored as free text on this record.

#### Scenario: Role round-trips every defined field
- **WHEN** a role is created with all defined fields populated and then read back
- **THEN** every field returns the value that was stored, including its origin and lifecycle state

#### Scenario: Recruiter-entered role needs no source metadata
- **WHEN** a role is created with recruiter-entered origin and no source identifier, canonical URL, or retrieved date
- **THEN** it is persisted with those fields empty

#### Scenario: Missing title is rejected
- **WHEN** a role is created with an empty or whitespace-only title
- **THEN** creation fails with an error naming the field and nothing is persisted

#### Scenario: Role may name a company with no company record
- **WHEN** a role is created naming a company that has no Company record
- **THEN** it is persisted with the company name retained and no company reference

### Requirement: Minimal company and contact records
The application SHALL persist a Company with name, website or domain, location, and source, and a Contact with full name, a Company reference, role or title, optional email, optional phone, and source. Relationship strength and interaction history SHALL NOT be stored.

#### Scenario: Company and contact round-trip
- **WHEN** a company and a contact linked to it are created and read back
- **THEN** every defined field returns the stored value and the contact resolves its company

#### Scenario: Contact requires a company
- **WHEN** a contact is created with no company reference, or with a reference to a company that does not exist
- **THEN** creation fails with an error and nothing is persisted

#### Scenario: Contact email and phone are optional
- **WHEN** a contact is created with neither email nor phone
- **THEN** it is persisted successfully

### Requirement: Structured field validation
The application SHALL validate structured field values at the service boundary, so that invalid values cannot be persisted by any caller. Validation SHALL trim surrounding whitespace and SHALL preserve Unicode content unchanged.

#### Scenario: Unicode values are preserved exactly
- **WHEN** a record is stored with names, locations, or notes containing non-Latin scripts, combining marks, or emoji
- **THEN** the values are returned byte-identical to what was submitted, after surrounding whitespace is trimmed

#### Scenario: Whitespace-only required values are rejected
- **WHEN** a required field is submitted as whitespace only
- **THEN** validation fails with an error naming that field

#### Scenario: Invalid dates are rejected
- **WHEN** a date field is submitted in an unparseable format, or a role's closing date precedes its published date
- **THEN** validation fails with an error naming the field and nothing is persisted

#### Scenario: Invalid URLs are rejected
- **WHEN** a canonical URL or company website is submitted that is not an absolute http or https URL
- **THEN** validation fails with an error stating what form is expected, and the value is not silently rewritten

#### Scenario: Compensation boundaries are enforced
- **WHEN** a compensation value is submitted with a negative amount, with a minimum greater than its maximum, with an unknown currency code, or with an unknown period
- **THEN** validation fails with an error naming the offending part

#### Scenario: Partially stated compensation is accepted
- **WHEN** a compensation value states only a minimum, or only a maximum, with a valid currency and period
- **THEN** it is persisted as stated with the absent bound left empty

#### Scenario: Optional contact details are validated only when present
- **WHEN** an optional email or phone is submitted as an empty value
- **THEN** it is stored as empty without a validation error, and when submitted with content it is validated

### Requirement: Records are shared by reference
Candidate, Role, Company, and Contact records SHALL be referencable by multiple initiatives without being copied, and SHALL survive the archiving or deletion of any initiative that references them.

#### Scenario: One candidate referenced by several initiatives
- **WHEN** two initiatives reference the same candidate and the candidate is updated
- **THEN** both initiatives resolve the updated record, and only one candidate row exists

#### Scenario: Archiving does not detach references
- **WHEN** an initiative referencing shared records is archived and then reopened
- **THEN** every reference resolves to the same records throughout

### Requirement: Contacts at a company
The application SHALL return the count and the listing of known contacts at a selected company, restricted to that company.

#### Scenario: Only that company's contacts are returned
- **WHEN** contacts-at-company is requested for a company that has contacts, while other companies also have contacts
- **THEN** only the selected company's contacts are returned, and the count equals the number returned

#### Scenario: Company with no contacts
- **WHEN** contacts-at-company is requested for a company with no contacts
- **THEN** an empty listing and a count of zero are returned, and this is not an error

#### Scenario: Unknown company is an error
- **WHEN** contacts-at-company is requested for a company that does not exist
- **THEN** an error is returned rather than an empty result, so a mistyped reference is not read as "no contacts"
