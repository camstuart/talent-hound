## ADDED Requirements

### Requirement: A proposal is valid as a whole or not at all
The application SHALL validate a classifier proposal completely before persisting any part of it, and SHALL persist either every aspect of a valid proposal or none of an invalid one. It SHALL NOT filter, drop, or repair individual aspects in order to persist the remainder.

#### Scenario: One invalid aspect rejects the proposal
- **WHEN** a proposal contains many valid aspects and one that fails any validation rule
- **THEN** nothing is persisted, and the failure names what was wrong

#### Scenario: A valid proposal persists entirely
- **WHEN** every aspect of a proposal passes validation
- **THEN** all of them are persisted in one transaction

#### Scenario: Validation reports every problem, not the first
- **WHEN** a proposal violates several rules at once
- **THEN** the failure lists all of the violations, so the repair attempt is told everything

### Requirement: Every extracted aspect cites evidence that resolves
Every aspect with origin extracted SHALL carry at least one citation naming a source chunk the classifier was given, and quoting text that appears in that chunk. A citation naming an unknown chunk, or quoting text not present in the named chunk, SHALL fail validation.

#### Scenario: A missing citation fails the proposal
- **WHEN** an extracted aspect carries no citation
- **THEN** validation fails and nothing is persisted

#### Scenario: A citation to an unseen chunk fails
- **WHEN** a citation names a chunk that was not among those given to the classifier
- **THEN** validation fails, so a plausible but invented identifier cannot satisfy the rule

#### Scenario: A citation whose text is not in the source fails
- **WHEN** a citation names a real chunk but quotes text that does not appear in it
- **THEN** validation fails

#### Scenario: A resolving citation passes
- **WHEN** a citation names a chunk that was given and quotes text present in it
- **THEN** that aspect satisfies the citation rule

### Requirement: A recruiter supplied aspect cites its recruiter-authored record
An aspect with origin recruiter supplied SHALL cite the recruiter-authored record it came from rather than a source chunk, and SHALL NOT be required to resolve against document text. It SHALL still be refused if it cites nothing at all.

#### Scenario: A recruiter aspect cites its record
- **WHEN** the recruiter authors an aspect
- **THEN** it is stored citing the record they authored, and passes validation without a chunk

#### Scenario: A recruiter aspect with no evidence at all is refused
- **WHEN** an aspect claims recruiter supplied origin and cites nothing
- **THEN** validation fails

### Requirement: Duplicate and contradictory aspects fail validation
A proposal SHALL NOT contain two aspects of the same type with the same normalized meaning, nor two aspects of the same type whose structured values contradict each other.

#### Scenario: A duplicate fails the proposal
- **WHEN** a proposal contains two aspects of the same type with the same wording and structured value
- **THEN** validation fails

#### Scenario: A contradiction fails the proposal
- **WHEN** a proposal contains two aspects of the same type whose structured values cannot both be true
- **THEN** validation fails naming both

### Requirement: Invalid output receives exactly one repair retry
Invalid classifier output SHALL be sent back once, with the problems found, and SHALL be validated again. A second invalid response SHALL make the extraction Failed and retryable by the recruiter. No more than one repair call SHALL be made per attempt.

#### Scenario: A valid first response makes no repair call
- **WHEN** the first response validates
- **THEN** exactly one model call was made

#### Scenario: An invalid-then-valid response makes exactly one repair call
- **WHEN** the first response fails validation and the second passes
- **THEN** exactly two model calls were made and the profile is persisted

#### Scenario: Two invalid responses fail visibly
- **WHEN** both the first response and the repair fail validation
- **THEN** exactly two model calls were made, the extraction is Failed with a coded reason, and it can be retried

#### Scenario: A failure carries no source content
- **WHEN** a classification fails for any reason
- **THEN** the recorded reason is a short lowercase code containing no part of the source document

### Requirement: Instructions inside a source cannot widen the contract
Text within a source document SHALL NOT be able to add an aspect type outside the taxonomy, remove the citation requirement, raise a priority the source does not support, or introduce a structured field outside the defined set, regardless of how it is phrased.

#### Scenario: An injected instruction cannot add an unsupported type
- **WHEN** a source contains text instructing the classifier to emit an aspect of an invented type
- **THEN** the result is either a valid profile without that aspect, or a visible validation failure — never a stored aspect of that type

#### Scenario: An injected instruction cannot remove citations
- **WHEN** a source contains text instructing the classifier to omit citations
- **THEN** any uncited extracted aspect fails validation

#### Scenario: An injected instruction cannot raise priority
- **WHEN** a source contains text instructing the classifier to mark everything must-have
- **THEN** an aspect is must-have only if it carries a citation that resolves to wording supporting it, and is otherwise unspecified

#### Scenario: Injected text may be quoted as evidence
- **WHEN** an aspect is extracted whose citation resolves to the injected text itself
- **THEN** it is stored as an ordinary cited aspect, visibly quoting that text, and is exactly as trustworthy as the document it came from
