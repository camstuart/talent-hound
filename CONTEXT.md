# Talent Hound

Talent Hound supports an Australian independent recruiter with evidence-backed role discovery and matching while keeping the recruiter in control of personal information, decisions, and outreach. Technology recruitment is the initial validation market, but the language is industry-agnostic.

## Work organization

**Initiative**:
A bounded unit of recruiter work directed at one placement or business objective. It owns its criteria, research, matches, drafts, and decisions while referencing reusable records and evidence.
_Avoid_: Project, agent session

**Job Search Initiative**:
An initiative that begins with exactly one known candidate and seeks suitable roles for that candidate.
_Avoid_: Candidate search

**Talent Search Initiative**:
An initiative that begins with a known or prospective role and seeks suitable candidates.
_Avoid_: Job search

**Business Development Initiative**:
An initiative that seeks prospective clients or placement needs using company signals, recruiter knowledge, and relationships.
_Avoid_: Opportunity search

**Search Criteria**:
The recruiter-approved must-have and nice-to-have requirements or preferences for one initiative. Search Criteria describes the desired outcome and is separate from extracted Candidate Profile facts.
_Avoid_: Candidate profile, role profile, filters

## People and organizations

**Recruiter**:
The Australian independent recruitment professional who owns the workspace, evaluates recommendations, and controls disclosures, changes, deletion, and outreach.
_Avoid_: Operator, agent

**Candidate**:
A person who may be considered for roles and whose information the recruiter is permitted to use.
_Avoid_: Talent, applicant

**Talent Pool**:
The recruiter's reusable collection of Candidate records and associated knowledge.
_Avoid_: Candidate database, resume library

**Company**:
An organization that may employ candidates, engage the recruiter, or otherwise be relevant to an initiative.
_Avoid_: Account

**Client**:
A Company that has engaged the recruiter for recruitment work.
_Avoid_: Customer, account

**Prospective Client**:
A Company that has not engaged the recruiter but has evidence of a possible recruitment need.
_Avoid_: Lead

**Contact**:
A person known in relation to a Company, Role, or introduction path.
_Avoid_: Lead

**Interaction**:
A recorded conversation, meeting, message, or other recruiter touchpoint involving a Candidate or Contact.
_Avoid_: Artifact, relationship

## Profiles and matching

**Candidate Profile**:
The recruiter-approved, evidence-backed decomposition of a Candidate's facts used for retrieval and matching. It does not contain inferred preferences.
_Avoid_: Resume, candidate summary, search criteria

**Role Profile**:
The evidence-backed decomposition of a Role's responsibilities, requirements, and structured constraints.
_Avoid_: Job description, search criteria

**Profile Aspect**:
One typed statement in a Candidate Profile or Role Profile, retaining its source wording and Evidence.
_Avoid_: Tag, keyword, score

**Role Fit**:
The assessment of how a Role satisfies the Search Criteria and preferences of a Job Search Initiative.
_Avoid_: Candidate fit

**Candidate Fit**:
The assessment of how a Candidate satisfies the requirements in a Role Profile.
_Avoid_: Role fit

**Match**:
A two-directional assessment between one Candidate and one Role, containing Role Fit, Candidate Fit, Evidence, gaps, and unknowns.
_Avoid_: Recommendation, score

## Demand and relationships

**Role**:
A hiring requirement or vacancy with a known origin and lifecycle state. A recruiter-entered Role is distinct from a public Role discovered through Exa.
_Avoid_: Opportunity, job search

**Discovered Role**:
A Role created from a permitted public source and governed by retrieval-date staleness and explicit Purge.
_Avoid_: Recruiter-entered role, scraped job

**Opportunity**:
An evidence-backed possibility of placement work that has not been confirmed as a Role.
_Avoid_: Vacancy, role

**Signal**:
A dated fact that may indicate organizational change or recruitment demand.
_Avoid_: Opportunity

**Relationship**:
A known connection between people or organizations that may support a credible introduction.
_Avoid_: Contact

**Warm Introduction Path**:
A chain of known Relationships through which the recruiter may request an introduction to a target person.
_Avoid_: Dotted-line connection

## Evidence and action

**Artifact**:
One immutable stored occurrence of a file, note, pasted text, or captured public source used as reusable context and Evidence.
_Avoid_: Attachment, document, blob

**Evidence**:
A traceable Artifact, recruiter-authored record, or permitted public source that supports a profile fact, Match result, or draft claim.
_Avoid_: AI reasoning, confidence

**Purge**:
Explicitly delete a Discovered Role, all of its current and historical source Artifacts, derived profiles and retrieval data, Matches, and active drafts. Separately owned recruiter notes and metadata-only audit history may survive with unavailable references.
_Avoid_: Archive, hide, expunge

**Outreach Draft**:
An unsent, editable message prepared for recruiter review and copy-out.
_Avoid_: Outreach, sent message

**CopiedOut**:
A metadata-only Audit Event recording that the recruiter copied an Outreach Draft. It is repeatable and is not a draft state.
_Avoid_: Sent, approved

**Audit Event**:
Content-free metadata recording an external disclosure, copy-out, or other auditable action within an Initiative.
_Avoid_: Activity log, payload log
