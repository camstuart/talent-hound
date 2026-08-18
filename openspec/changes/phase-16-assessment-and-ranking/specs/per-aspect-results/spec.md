## ADDED Requirements

### Requirement: A per-aspect result is met, not met, or unknown
Every assessed requirement SHALL produce exactly one of three states: met, not met, or unknown. No other state SHALL be stored.

#### Scenario: Each state is producible
- **WHEN** requirements with supporting, contradicting, and absent evidence are assessed
- **THEN** they produce met, not met, and unknown respectively

#### Scenario: An unsupported state is refused
- **WHEN** the model returns any state outside the three
- **THEN** validation fails and no result is stored

### Requirement: A met result must cite evidence that resolves
A met result SHALL carry at least one citation, and every citation SHALL resolve to evidence that exists. A met result that cites nothing, or whose citation does not resolve, SHALL fail validation.

#### Scenario: An uncited met result fails
- **WHEN** the model returns met with no citation
- **THEN** validation fails and nothing is stored

#### Scenario: A met result citing something unavailable fails
- **WHEN** a met result cites evidence that cannot be resolved
- **THEN** validation fails

#### Scenario: An invalid met result is not quietly downgraded
- **WHEN** a met result fails validation
- **THEN** it is refused rather than being stored as unknown

#### Scenario: A resolving citation passes
- **WHEN** a met result cites evidence that resolves to text shown to the model
- **THEN** it is stored with that citation

### Requirement: A not-met result cites contrary evidence when there is any
A not met result SHALL cite contrary evidence when contrary evidence was available, and SHALL state explicitly when no evidence was found.

#### Scenario: Contrary evidence is cited
- **WHEN** the evidence contradicts the requirement
- **THEN** the not met result cites it

#### Scenario: Absence is stated rather than implied
- **WHEN** no evidence bears on the requirement
- **THEN** the result says so explicitly rather than being an uncited not met

### Requirement: Injected instructions in evidence cannot change a result
Text inside evidence SHALL NOT be able to change a result state, add a citation that does not resolve, or alter the required output shape.

#### Scenario: An injected instruction does not force met
- **WHEN** evidence contains text instructing the assessor to mark everything met
- **THEN** results are still determined by the contract, and any uncited met still fails validation

#### Scenario: An injected instruction cannot invent a citation
- **WHEN** evidence instructs the assessor to cite a fabricated source
- **THEN** the citation does not resolve and validation fails

### Requirement: A whole assessment is stored or none of it
A role's assessment SHALL be persisted only if every per-aspect result passes validation.

#### Scenario: One invalid result rejects the role's assessment
- **WHEN** one requirement's result fails validation
- **THEN** no result for that role is stored

#### Scenario: A failure is visible and retryable
- **WHEN** a role's assessment fails validation
- **THEN** the failure is recorded with a coded reason and can be retried
