## ADDED Requirements

### Requirement: Role Profiles are created automatically without approval
A Role Profile SHALL be created from the role's current source content without requiring recruiter approval, and no part of the application SHALL require a Role Profile to be approved before use.

#### Scenario: Profiling a role needs no approval step
- **WHEN** a role with extracted source content is profiled
- **THEN** the resulting profile is immediately Ready, with no approval requested or recorded

#### Scenario: Roles and candidates differ deliberately
- **WHEN** a Role Profile and a Candidate Profile are both created
- **THEN** the Candidate Profile requires approval before use and the Role Profile does not

### Requirement: A Role Profile is Ready, Failed, or Stale
Every Role Profile SHALL report exactly one of three states. Ready means it was derived from the role's current source content and satisfied the classifier contract. Failed means the contract could not be satisfied. Stale means it was valid when made and the source content has since changed.

#### Scenario: A valid profile from current sources is Ready
- **WHEN** a role is profiled successfully from its current content
- **THEN** its state is Ready

#### Scenario: A source change makes it Stale
- **WHEN** the role's source content changes after profiling
- **THEN** the profile's state is Stale and it is not Ready

#### Scenario: A contract failure makes it Failed
- **WHEN** the classifier cannot produce a valid proposal for the role
- **THEN** the profile's state is Failed and carries a coded reason

#### Scenario: Reprofiling from changed sources restores Ready
- **WHEN** a stale role is profiled again from its current content
- **THEN** the new version is Ready

### Requirement: Failed and Stale profiles never disappear
A Failed or Stale Role Profile SHALL remain visible in the role listing with its state and reason, and SHALL offer retry. A Failed profile SHALL additionally offer manual entry. A role with no profile at all SHALL be shown as such rather than omitted.

#### Scenario: A failed profile stays on screen
- **WHEN** the roles are listed after a decomposition failure
- **THEN** that role appears with a Failed state and its reason

#### Scenario: A role with no profile is not hidden
- **WHEN** the roles are listed and one has never been profiled
- **THEN** it appears with a state saying so

#### Scenario: A failed profile offers retry and manual entry
- **WHEN** a Failed role profile is shown
- **THEN** the recruiter can retry the decomposition or enter aspects by hand

### Requirement: Role aspects carry priority only where the source supports it
A Role Profile aspect SHALL be must-have or nice-to-have only when the source wording supports it, and SHALL otherwise be unspecified. Ambiguous language SHALL NOT be resolved into a priority.

#### Scenario: Stated requirements keep their priority
- **WHEN** a listing says a requirement is essential
- **THEN** the aspect may be must-have

#### Scenario: Ambiguous wording stays unspecified
- **WHEN** a listing mentions a skill without indicating whether it is required
- **THEN** the aspect is unspecified

#### Scenario: An unstated attribute produces no aspect
- **WHEN** a listing says nothing about an attribute
- **THEN** no aspect asserts one

### Requirement: Recruiter edits version the profile without touching the source
Editing a Role Profile aspect SHALL create a new profile version and SHALL NOT modify the role's source artifact. The edited aspect SHALL carry recruiter supplied origin, and the original citation SHALL remain resolvable on the version that carried it.

#### Scenario: An edit creates a version
- **WHEN** the recruiter edits a requirement's wording
- **THEN** a new version exists and the previous one is unchanged

#### Scenario: The source artifact is untouched
- **WHEN** any Role Profile edit is made
- **THEN** the role's source artifact bytes and extracted content are unchanged

#### Scenario: An edited profile may disagree with its source
- **WHEN** the recruiter changes a requirement the listing states differently
- **THEN** the edit stands and the earlier version's citation still resolves to what the listing said
