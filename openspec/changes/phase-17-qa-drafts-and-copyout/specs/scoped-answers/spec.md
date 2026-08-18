## ADDED Requirements

### Requirement: Answers see only the asking initiative's approved evidence
Question answering SHALL retrieve only from evidence linked to the initiative the question was asked in, and only from approved or Ready derived data. Scope SHALL be applied when selecting evidence, not by filtering an answer afterwards.

#### Scenario: Another initiative's evidence is unreachable
- **WHEN** a question is asked whose answer exists only in another initiative's documents
- **THEN** the answer does not contain it and reports that the evidence here does not say

#### Scenario: Unapproved evidence is unreachable
- **WHEN** a candidate's profile is not approved
- **THEN** its aspects are not used to answer questions

#### Scenario: Scope is part of the retrieval
- **WHEN** an answer is produced
- **THEN** out-of-scope evidence was never retrieved, rather than retrieved and discarded

### Requirement: A factual answer cites evidence that resolves
An answer asserting a fact SHALL carry at least one citation, and every citation SHALL resolve to evidence in scope. An answer claiming support with no resolving citation SHALL be refused.

#### Scenario: A supported answer cites
- **WHEN** the evidence supports an answer
- **THEN** the answer carries citations that resolve to the evidence

#### Scenario: An answer claiming support with no citation is refused
- **WHEN** the model returns a supported answer with no citations
- **THEN** it is refused rather than shown

#### Scenario: A citation that does not resolve is refused
- **WHEN** an answer cites evidence that is not in scope
- **THEN** it is refused

### Requirement: An unsupported question returns unknown rather than invention
When the evidence does not support an answer, the application SHALL say so explicitly and SHALL NOT produce a plausible answer in its place.

#### Scenario: Nothing relevant returns an explicit unknown
- **WHEN** a question is asked that the evidence does not address
- **THEN** the answer says the evidence does not say

#### Scenario: An unsupported answer carries no invented prose
- **WHEN** an answer is unsupported
- **THEN** it contains no factual assertion

#### Scenario: An empty corpus is not an error
- **WHEN** a question is asked in an initiative with no evidence
- **THEN** the answer reports having nothing to draw on rather than failing
