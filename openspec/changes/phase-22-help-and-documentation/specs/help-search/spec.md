## ADDED Requirements

### Requirement: Help answers without a model, a network, or a database
Help search SHALL work with no model assigned, no data folder chosen, no corpus indexed, and no network available.

#### Scenario: Searching on a fresh install
- **WHEN** help is searched before first run has completed
- **THEN** results are returned

#### Scenario: Searching with no model assigned
- **WHEN** help is searched with no generate model
- **THEN** results are returned, and the absence of a written answer is explained rather than shown as an empty answer

#### Scenario: Searching offline
- **WHEN** help is searched with no network
- **THEN** results are returned, and no request is made

### Requirement: A search returns the sections that answer, with their context
A help search SHALL return ranked sections, each naming its article, its heading, and enough text to recognise the answer.

#### Scenario: A term that appears in one section
- **WHEN** a term appears in one section only
- **THEN** that section ranks first

#### Scenario: A section about a term beats one that mentions it
- **WHEN** one section is about a subject and another mentions it in passing
- **THEN** the section about it ranks higher

#### Scenario: A word form is matched
- **WHEN** the query says "deleting" and the article says "delete"
- **THEN** the section is found

#### Scenario: Nothing matches
- **WHEN** no section matches the query
- **THEN** the result says so, and does not return unrelated sections as though they answered

### Requirement: An answer is cited or it is not given
When a generate model is assigned, help MAY compose an answer from the retrieved sections. The answer SHALL cite the sections it used, and SHALL NOT be shown when it cites none.

#### Scenario: An answer names its sections
- **WHEN** an answer is composed from two sections
- **THEN** both are cited and reachable from the answer

#### Scenario: The sections do not cover the question
- **WHEN** the retrieved sections do not answer the question
- **THEN** help says so plainly rather than composing an answer

#### Scenario: An uncited answer is withheld
- **WHEN** a model returns an answer citing no section
- **THEN** it is not shown, and the search results are shown instead

### Requirement: A help question never leaves the machine
Help SHALL send no query, telemetry, or feedback anywhere, and SHALL have no endpoint to configure.

#### Scenario: No endpoint exists
- **WHEN** the service surface and settings are inspected
- **THEN** no help endpoint, analytics, or feedback control exists

#### Scenario: A composed answer uses the local model
- **WHEN** an answer is composed
- **THEN** it is composed by the assigned local model, under the same rules as every other generation
