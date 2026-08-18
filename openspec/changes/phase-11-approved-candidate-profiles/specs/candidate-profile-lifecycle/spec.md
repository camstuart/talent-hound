## ADDED Requirements

### Requirement: A Candidate Profile begins Proposed
A newly classified Candidate Profile SHALL be created in the Proposed state. It SHALL NOT become Approved without an explicit recruiter action.

#### Scenario: Classification produces a Proposed version
- **WHEN** a candidate is classified for the first time
- **THEN** the resulting profile version is Proposed

#### Scenario: Nothing approves a profile automatically
- **WHEN** a profile is classified, reclassified, or edited
- **THEN** it remains unapproved until the recruiter approves it

### Requirement: Approval freezes a version
Approving a profile version SHALL record that it was approved and SHALL make it the version search and matching use. The approved version's aspects and citations SHALL remain resolvable for as long as it is the approved version.

#### Scenario: An approved version becomes the one in use
- **WHEN** the recruiter approves a Proposed version
- **THEN** that version is the candidate's approved profile

#### Scenario: Cited evidence stays resolvable after approval
- **WHEN** an approved version's aspects are inspected
- **THEN** every extracted aspect's citation still resolves to its source text

#### Scenario: A later version does not displace the approved one
- **WHEN** a new version is created after an approval
- **THEN** the approved version remains the one in use until the new one is approved

### Requirement: A source change makes an approved profile Stale
When the evidence a profile was approved from changes, the approved version SHALL be preserved and reported as Stale. It SHALL remain usable, with a warning, until a newer version is approved.

#### Scenario: Adding a source makes the approved profile stale
- **WHEN** an artifact is linked to a candidate whose profile is approved
- **THEN** the approved version is reported Stale and is still the version in use

#### Scenario: Replacing a source makes the approved profile stale
- **WHEN** an artifact's extracted content is replaced
- **THEN** the approved profile of a candidate that cites it is reported Stale

#### Scenario: Detaching a source makes the approved profile stale
- **WHEN** an artifact is unlinked from the candidate
- **THEN** the approved profile is reported Stale

#### Scenario: A stale approved profile is still usable with a warning
- **WHEN** a stale approved profile is used
- **THEN** it is permitted and the staleness is reported alongside it

#### Scenario: Approving again clears staleness
- **WHEN** a version derived from the current sources is approved
- **THEN** the profile is no longer Stale

### Requirement: A failed classification leaves a visible, retryable profile
A classification that cannot produce a valid proposal SHALL leave a Failed profile version that is visible and retryable, and SHALL NOT block the recruiter from building a profile by hand.

#### Scenario: A failure is visible
- **WHEN** classification fails for a candidate
- **THEN** a Failed version exists carrying a coded reason

#### Scenario: A failed profile can be built by hand
- **WHEN** the recruiter adds aspects to a candidate whose classification failed
- **THEN** those aspects form a version that can be approved like any other

#### Scenario: A hand-built profile is visibly recruiter supplied
- **WHEN** a profile is built entirely by hand
- **THEN** every aspect has recruiter supplied origin and cites the record it was authored into

### Requirement: Recruiter edits create versions rather than mutating
Editing or removing an aspect SHALL produce a new profile version. No existing version's aspects SHALL be modified in place, and an edited aspect SHALL carry recruiter supplied origin.

#### Scenario: An edit creates a version
- **WHEN** the recruiter edits an aspect's wording
- **THEN** a new version exists containing the edit and the previous version is unchanged

#### Scenario: A removal creates a version
- **WHEN** the recruiter removes an aspect
- **THEN** a new version exists without it and the previous version still has it

#### Scenario: An edited aspect changes origin
- **WHEN** the recruiter edits an extracted aspect
- **THEN** the resulting aspect is recruiter supplied, because a person now asserts it
