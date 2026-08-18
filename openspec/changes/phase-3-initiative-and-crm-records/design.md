## Context

`Initiative` today is `{ID, Name, Type, CreatedAt, UpdatedAt}` with `Create` and `List`. The PRD names four shared records, their exact structured fields, one hard cardinality rule (a Job Search Initiative has exactly one Candidate), and one warm-path query (contacts at a company). The lifecycle table gives initiatives Active ⇄ Archived and defers every deletion rule to the deletion-invariants table, which Phase 19 implements in full.

The temptation in this phase is to build the relationship model P1 will want. The PRD explicitly defers Interaction, Relationship, Signal, and Opportunity, and defers relationship strength and interaction history on Contact. This design builds the named fields and stops.

## Goals / Non-Goals

**Goals:**
- FR-01 and FR-02 working end to end against the real backend.
- Records shared by reference, never copied, so archiving or deleting an initiative cannot lose talent-pool knowledge.
- Field validation that holds at the service boundary, where the frontend cannot bypass it.
- A workspace shell that later phases fill in without being rearranged.

**Non-Goals:**
- No profile aspects, embeddings, artifacts, matching, or drafting. Employment history, education, skills, and qualifications become aspects in Phase 5+ and are absent here.
- No Candidate, Role, Company, or Contact deletion — Phase 19 owns every deletion invariant, including the blocked-by-referencing-initiative rule.
- No Talent Search or Business Development pipeline. Their workspaces render the shell and say so.
- No chat, no search, no jobs. The four areas are navigation, not features.
- No CRM richness: no relationship strength, no interaction history, no tags, no pipeline stages.

## Decisions

**Job Search cardinality is a nullable `candidate_id` column on `initiatives`, not a join table.**
"Exactly one Candidate" is a to-one relationship; a join table would model it as many and then need a constraint to forbid what it just allowed. The column is nullable because Talent Search and Business Development have no candidate. The service rejects creating a Job Search without a candidate and rejects a second one because the field simply cannot hold two.
`ponytail:` a to-one column; upgrade path is a join table if Talent Search ever needs many candidates per initiative, which is P1.

**The cardinality rule is enforced in the service; the column shape is the backstop for "at most one".**
"At most one" is the column: it cannot physically hold two. "At least one for Job Search" is a table-level `CHECK`, which SQLite can only add by rebuilding the table — and the rebuild would fail on any pre-Phase-3 Job Search row, since none has a candidate. Rebuilding the table to add a constraint that then forces us to mangle or drop existing rows costs more than it protects, so the rule lives in the service, which is the only writer and which can name the field in its error. `initiatives.status` does get a single-column `CHECK`, because `ALTER TABLE ADD COLUMN` can carry one and existing rows default to `active` without conflict.
`ponytail:` service-enforced; add the table-level `CHECK` in the rebuild that some later migration needs anyway.

**Multi-valued candidate emails and phones are JSON arrays in TEXT columns.**
The PRD says "email addresses" and "phone numbers", plural, and says nothing about per-address metadata. Child tables would add two tables, two services, and two forms to store a list of strings. SQLite's JSON1 can query them if that ever matters.
`ponytail:` JSON array in one column; upgrade to a child table when an address needs its own attributes (primary, verified, source).

**Compensation is stored as `min`, `max`, `currency`, `period` — four columns, integers for the amounts.**
Floats for money are a 3am page. Amounts are whole currency units (not minor units — the PRD's expectations are salaries and rates, not cents), currency is a 3-letter ISO 4217 code, period is one of `hour`, `day`, `week`, `month`, `year`. Either bound may be absent; when both are present, `min <= max`. Negative is rejected; an absurd upper bound is rejected as a typo guard.

**Dates that mean "a day" are stored as dates; timestamps stay timestamps.**
Availability, last-confirmed, published, closing, and retrieved are calendar facts about the world, not events in this database — storing them as instants invites timezone drift when the recruiter's laptop travels. `CreatedAt`/`UpdatedAt` remain timestamps.

**URLs are validated with `net/url` for an absolute `http`/`https` URL, and nothing more.**
No reachability check (offline-first), no canonicalisation, no scheme guessing. A bare domain is rejected with a message saying so rather than silently prefixed.

**Whitespace is trimmed and Unicode is preserved as-is; only required-field emptiness is a validation error.**
Trim then check empty, so a name of spaces fails. No normalisation form conversion, no ASCII folding, no length-in-bytes limits — a name is text, and mangling it is a data-quality bug that is invisible until a recruiter sees their own name wrong.

**Archived initiatives keep every reference and stay readable.**
Archive is a status change, nothing more: no cascade, no detach, no hiding of referenced records. Reopen is the same change back. The list defaults to Active with Archived available, so the sidebar does not grow forever.

**Initiative deletion removes the initiative row and its owned rows, and is a no-op for shared records.**
In this phase an initiative owns nothing yet, so deletion is one row — but the test that shared records survive is written now, because every later phase adds owned rows to that cascade and this is the test that catches the day someone adds a cascade to `candidates`.

**One `RecordService` for all four record types, not four services.**
Four near-identical services would be four registrations, four binding files, and four import lines to save nothing. One service with `CreateCandidate`, `UpdateRole`, `ContactsAtCompany`, and so on.
`ponytail:` split it when one record type grows behaviour the others do not share.

**The four workspace areas are a tab strip inside the initiative panel rendering four empty panels.**
No routing library, no lazy loading, no per-area state machine. Each panel names what will live there. Talent Search and Business Development render the identical shell with a line saying their pipeline is out of PoC scope.

## Risks / Trade-offs

- JSON-in-a-column cannot be constrained by the database; a malformed array reaching the column would be a service bug. The service is the only writer and validates on the way in, and a round-trip test covers it.
- The `candidate_id` column means a Job Search initiative cannot be created before its candidate exists. That is the intended order (the PRD's resume drag-in creates the candidate first), but the UI must create the candidate in the same flow or the modal is a dead end.
- Deferring record deletion to Phase 19 means the PoC can accumulate records with no way to remove them for several phases. Deliberate: a half-enforced deletion rule is worse than none, and the data folder can be deleted wholesale.
- The structured-field list is taken verbatim from the PRD. If the PRD's field list is wrong, this phase makes it expensive to change later; the fields are therefore added exactly as named, with no "while we're here" extras.

## Migration Plan

Migration 2 adds `status` (defaulting to `active`) and `candidate_id` to `initiatives`. Migration 3 adds `candidates`, `roles`, `companies`, `contacts` and their indexes. Existing rows are all Active by default and carry no candidate; existing Job Search rows created before this phase therefore violate the new cardinality rule and are backfilled to be readable but flagged, or — since the only such rows are development and E2E fixtures — recreated. The decision is recorded in tasks 1.4. Rollback is snapshot restore, which Phase 2 provides.

## Open Questions

- Are pre-Phase-3 Job Search initiatives backfilled with a placeholder candidate, or is the constraint applied only to new rows? Currently: no placeholder — the check constraint applies to new writes, and the E2E fixture is recreated.
- Does the Role record's "company" field reference the Company record or store a free-text name? Currently a nullable Company reference plus the source's own company text, because a discovered role names a company that may not have a record yet.
- Should the archived initiative list be a filter on the sidebar or a separate view? Currently a filter.
