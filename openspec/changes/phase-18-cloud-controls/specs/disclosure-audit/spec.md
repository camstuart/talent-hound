## MODIFIED Requirements

### Requirement: Every sent request creates one disclosure event
A request actually sent to a non-local provider SHALL create exactly one local disclosure audit event recording the timestamp, provider, task, information categories, initiative, and any relevant record references. This applies to every non-localhost request, including cloud model requests as well as searches.

#### Scenario: One request, one event
- **WHEN** a search is sent
- **THEN** exactly one disclosure event exists for it

#### Scenario: The event names the provider and task
- **WHEN** a disclosure event is inspected
- **THEN** it names the provider and the task the request was for

#### Scenario: The event names the initiative
- **WHEN** a disclosure event is inspected
- **THEN** it names the initiative the request was made within

#### Scenario: A failed request that was sent still creates an event
- **WHEN** a request is transmitted and the provider then fails
- **THEN** a disclosure event exists, because the information left the machine

#### Scenario: A cloud model request creates an event
- **WHEN** an approved cloud task sends a request
- **THEN** one disclosure event records it, naming the cloud provider and the task

#### Scenario: A local model request creates no event
- **WHEN** a task runs against the local endpoint
- **THEN** no disclosure event is created, because nothing left the machine

### Requirement: A disclosure event never records content
A disclosure audit event SHALL NOT contain the query text, any result content, any document content, any draft text, any payload, or any credential.

#### Scenario: The query is absent from the event
- **WHEN** a search is sent and its disclosure event is inspected
- **THEN** the query text appears nowhere in it

#### Scenario: Results are absent from the event
- **WHEN** results are returned and stored
- **THEN** no part of them appears in the disclosure event

#### Scenario: A cloud payload is absent from its event
- **WHEN** a cloud request is sent
- **THEN** its payload appears nowhere in the event

#### Scenario: A scan of stored events finds no content
- **WHEN** every stored disclosure event is scanned for the query, payload, and result text
- **THEN** none of them is found

### Requirement: Reproducibility lives with the initiative, not the audit log
The exact visible query SHALL be retained on the search record within its initiative, so a search can be reproduced without the audit log holding content.

#### Scenario: The search record reproduces the query
- **WHEN** a past search is inspected
- **THEN** the exact query it sent is available from its record

#### Scenario: The two records have different contents
- **WHEN** a search record and its disclosure event are compared
- **THEN** the query is in the search record and not in the event
