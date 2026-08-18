## ADDED Requirements

### Requirement: Similarity is exact cosine
The application SHALL score candidates by cosine similarity computed exactly over every stored dimension, with no approximation, sampling, or early termination. Accumulation SHALL use higher precision than the stored element type. Scores SHALL be clamped to the range -1 to 1 inclusive.

#### Scenario: Scores match a trusted oracle
- **WHEN** cosine similarity is computed for hand-chosen vector pairs
- **THEN** each score equals the independently computed value within a small numerical tolerance

#### Scenario: Identical vectors score one
- **WHEN** a vector is compared with itself
- **THEN** the score is 1, and never a value above 1

#### Scenario: Orthogonal and opposite vectors score zero and minus one
- **WHEN** two orthogonal vectors are compared
- **THEN** the score is 0
- **WHEN** a vector is compared with its negation
- **THEN** the score is -1, and never a value below -1

#### Scenario: A zero vector has no defined similarity
- **WHEN** a similarity is requested involving a vector with zero magnitude
- **THEN** it is refused rather than answered with a number

#### Scenario: A non-finite vector is refused
- **WHEN** a similarity is requested involving a vector containing NaN or infinity
- **THEN** it is refused, and no result set containing a NaN score is produced

### Requirement: Retrieval order is deterministic
Semantic results SHALL be ordered by score descending and, where scores are equal, by a stable identifier ascending. Repeating a search over an unchanged corpus SHALL return the same results in the same order.

#### Scenario: Equal scores order by identifier
- **WHEN** two retrieval units have identical vectors and therefore identical scores
- **THEN** they appear in ascending identifier order, on every run

#### Scenario: A repeated search is identical
- **WHEN** the same query is run twice against an unchanged corpus
- **THEN** the two result lists are identical in content and order

### Requirement: Retrieval is scoped to the initiative that asked
A semantic search SHALL return only retrieval units derived from artifacts linked to the initiative being searched, and only vectors belonging to the current embedding space.

#### Scenario: Another initiative's evidence is not returned
- **WHEN** two initiatives each have embedded evidence and one is searched with a query matching the other's text
- **THEN** no result comes from the other initiative

#### Scenario: A search with no current space returns nothing
- **WHEN** a semantic search runs while no embed model is assigned
- **THEN** it reports that no embedding space is configured rather than returning unscoped results

### Requirement: A semantic result resolves to a citation
Every semantic result SHALL carry enough to resolve back to its source: the artifact, the retrieval unit, its location, and the text that was embedded. Resolution SHALL verify the stored offsets still select the stored text before returning it.

#### Scenario: A result resolves to its source text
- **WHEN** a semantic result is resolved to a citation
- **THEN** the citation names the artifact and a human-readable location, and quotes the text the offsets select

#### Scenario: A stale result fails rather than misleads
- **WHEN** a result is resolved and its stored offsets no longer select its stored text
- **THEN** resolution fails with an error rather than quoting different text

### Requirement: Candidate content is embedded locally under every configuration
The application SHALL send candidate content only to the local model endpoint for embedding, under every configuration it permits. No cloud endpoint SHALL receive candidate content for embedding, whether or not cloud features are otherwise enabled.

#### Scenario: A cloud endpoint receives nothing
- **WHEN** the whole embedding path is exercised over candidate content while a cloud endpoint is reachable and configured wherever configuration permits
- **THEN** the cloud endpoint records zero requests

#### Scenario: The embed role cannot name a cloud endpoint
- **WHEN** the embed role is assigned an endpoint that is not local
- **THEN** the assignment is refused, so no configuration exists in which embedding leaves the machine

### Requirement: Exact-scan cost is measured, not assumed
The application's exact-scan retrieval SHALL have representative timings recorded at increasing corpus sizes. No approximate index or vector extension SHALL be introduced unless a recorded measurement misses the stated threshold.

#### Scenario: Timings are recorded at increasing sizes
- **WHEN** the scan benchmark runs
- **THEN** it reports a per-query wall-clock figure at several corpus sizes, in a form that can be pasted into the gate evidence
