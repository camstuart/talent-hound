## ADDED Requirements

### Requirement: Explicit protected criteria are blocked deterministically
The application SHALL refuse to store a Search Criterion that explicitly names a protected attribute, using a fixed list and a deterministic match that does not consult a model. The list SHALL cover age, sex, gender identity, sexual orientation, race or national origin, religion, disability, family or carer status, pregnancy, marital status, political opinion, and union membership.

#### Scenario: Every protected category is blocked
- **WHEN** a criterion explicitly naming any category on the list is offered
- **THEN** it is refused, naming the category

#### Scenario: Blocking does not call a model
- **WHEN** a protected criterion is refused
- **THEN** no model call was made, so the refusal cannot depend on a model's availability or judgement

#### Scenario: Case, punctuation, and spacing do not evade the block
- **WHEN** a protected term is written in a different case, with punctuation, hyphenated, or with extra spacing
- **THEN** it is refused the same way

#### Scenario: A blocked criterion is not stored at all
- **WHEN** a criterion is refused
- **THEN** no criterion row exists and the criteria version is unchanged

### Requirement: A deterministic block cannot be overridden
There SHALL be no path — no flag, confirmation, or setting — by which a criterion refused by the protected-term check is stored anyway.

#### Scenario: No override exists
- **WHEN** the service surface is inspected for a way to store a refused criterion
- **THEN** no such method or parameter exists

#### Scenario: Retrying does not succeed
- **WHEN** a refused criterion is offered again unchanged
- **THEN** it is refused again

### Requirement: Ambiguous proxies warn without blocking
The local `classify` role MAY flag a criterion as a possible proxy for a protected attribute. Such a flag SHALL attach a visible warning and SHALL NOT prevent the criterion from being stored or used.

#### Scenario: A flagged proxy is stored with a warning
- **WHEN** a criterion is flagged as a possible proxy
- **THEN** it is stored, and its warning is available wherever the criterion is shown

#### Scenario: A warning can be dismissed but the criterion stands either way
- **WHEN** the recruiter proceeds despite a warning
- **THEN** the criterion is used normally

#### Scenario: A clearly lawful criterion produces neither block nor warning
- **WHEN** a criterion describes a skill, responsibility, or qualification the work requires
- **THEN** it is stored with no block and no warning

#### Scenario: An unavailable model does not become a block
- **WHEN** no classify model is available and a criterion is added
- **THEN** it is stored with no warning rather than refused

### Requirement: A stored warning does not change under the reader
A warning SHALL be recorded when the criterion is created or edited and SHALL NOT be recalculated on read.

#### Scenario: A warning persists across reads
- **WHEN** a criterion with a warning is listed repeatedly
- **THEN** the warning is the same every time and no model is called

#### Scenario: Changing the model does not change existing warnings
- **WHEN** the classify model changes after a criterion was stored
- **THEN** that criterion's warning is unchanged

### Requirement: Work-rights criteria remain available
A criterion requiring a right to work SHALL be permitted. It SHALL NOT be treated as, or inferred to imply, nationality, citizenship, or national origin.

#### Scenario: A work-rights requirement is accepted
- **WHEN** a criterion requires an existing right to work in a country
- **THEN** it is stored without a block

#### Scenario: A nationality requirement is blocked
- **WHEN** a criterion requires citizenship of, or origin in, a country
- **THEN** it is refused under national origin

#### Scenario: The two are distinguished by their terms
- **WHEN** both criteria are offered in one session
- **THEN** the work-rights one is stored and the nationality one is refused
