## ADDED Requirements

### Requirement: Matching has two separate directions
The application SHALL assess role fit for the candidate — the Role Profile against the initiative's Search Criteria — separately from candidate fit for the role — the approved Candidate Profile against the Role Profile's requirements. The two SHALL be stored and displayed as distinct results.

#### Scenario: Both directions are produced
- **WHEN** a role is assessed against a candidate
- **THEN** results exist for role-fit-for-candidate and for candidate-fit-for-role

#### Scenario: The directions may disagree
- **WHEN** a role meets the recruiter's criteria while the candidate does not meet the role's requirements
- **THEN** one direction reports met results and the other reports unmet ones, without either overriding the other

#### Scenario: Each direction cites its own evidence
- **WHEN** a per-aspect result is inspected
- **THEN** its evidence comes from the side being assessed, not from the other direction

### Requirement: Assessment requires an approved candidate and a Ready role
Assessment SHALL only run for a candidate whose profile is approved and a role whose profile is Ready, using the existing readiness and eligibility checks.

#### Scenario: An unapproved candidate blocks assessment
- **WHEN** assessment is requested for a candidate whose profile is not approved
- **THEN** it is refused with the readiness reason

#### Scenario: A stale role is not assessed
- **WHEN** assessment is requested for a role whose profile is Stale
- **THEN** it is refused with the eligibility reason

### Requirement: Similarity selects evidence and never decides a result
Exact-cosine retrieval SHALL be used only to choose which candidate aspects and evidence chunks are shown to the model. The similarity score SHALL NOT appear in, or influence, the recorded result state.

#### Scenario: A high score does not create a met result
- **WHEN** evidence scores very highly for a requirement but its text does not support it
- **THEN** the result is not met or unknown, never met on the strength of the score

#### Scenario: The score is absent from the stored result
- **WHEN** a per-aspect result is inspected
- **THEN** no similarity score is stored on it

#### Scenario: Evidence with no similarity can still be assessed
- **WHEN** no candidate aspect is similar to a requirement
- **THEN** the requirement is assessed as unknown with an explicit statement that no evidence was found
