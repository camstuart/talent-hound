## ADDED Requirements

### Requirement: An embedding space is a stored identity
The application SHALL record every embedding space as a row carrying the endpoint, model name, model digest, the embed role assignment revision that produced it, the dimension count, the similarity metric, and whether vectors are normalized. A space SHALL be uniquely identified by that combination, and every stored vector SHALL name exactly one space.

#### Scenario: Embedding under a configuration creates its space
- **WHEN** a retrieval unit is embedded and no space exists for the current embed assignment
- **THEN** a space is created recording the endpoint, model, digest, assignment revision, dimensions, metric, and normalization in force
- **AND** the vector is stored naming that space

#### Scenario: Embedding again reuses the same space
- **WHEN** further retrieval units are embedded with the embed assignment unchanged
- **THEN** no additional space is created and every vector names the existing one

#### Scenario: A missing digest does not prevent a space
- **WHEN** the endpoint reports no digest for the model
- **THEN** the space is created with an empty digest rather than refused
- **AND** the assignment revision still distinguishes it from any other configuration

### Requirement: A configuration change creates a new space
A change to the embed role's assignment SHALL produce a new embedding space rather than modify an existing one. Vectors produced under a previous space SHALL be retained unchanged and SHALL NOT be returned by retrieval in the current space.

#### Scenario: Changing the embed model creates a space
- **WHEN** the embed role is assigned a different model
- **THEN** the next embedding creates a new space naming the new assignment revision
- **AND** the previous space and its vectors still exist

#### Scenario: Older vectors leave current retrieval
- **WHEN** a corpus embedded under a previous space is searched after the embed model changes
- **THEN** no result comes from the previous space
- **AND** the count of retrieval units not yet embedded in the current space is reportable

#### Scenario: A parameter-only change is still a new space
- **WHEN** the embed assignment changes in a way that alters no visible name, producing a new revision
- **THEN** a new space is created, because the revision is part of the space's identity

### Requirement: Vectors from different spaces are never compared
The application SHALL NOT compute a similarity between vectors belonging to different embedding spaces. Retrieval SHALL restrict candidates to a single space before scoring, and no service method SHALL accept two vectors and return a similarity without an agreed space.

#### Scenario: A search scores only its own space
- **WHEN** a corpus contains vectors in two spaces and a semantic search is run
- **THEN** only vectors in the space of the current embed assignment are scored

#### Scenario: A cross-space comparison is unavailable
- **WHEN** the service surface is inspected for a way to compare two arbitrary vectors
- **THEN** no such method exists, and similarity is reachable only through a search that resolves a space first

### Requirement: A space reports its coverage
The application SHALL be able to report, for a given initiative and the current embedding space, how many retrieval units are embedded and how many are not.

#### Scenario: Coverage after a model change
- **WHEN** every chunk of an initiative is embedded and the embed model then changes
- **THEN** coverage in the new space reports zero embedded and every chunk outstanding
