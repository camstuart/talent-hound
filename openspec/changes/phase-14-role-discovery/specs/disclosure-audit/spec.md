## ADDED Requirements

### Requirement: Every sent request creates one disclosure event
A request actually sent to a non-local provider SHALL create exactly one local disclosure audit event recording the timestamp, provider, task, information categories, initiative, and any relevant record references.

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

### Requirement: A disclosure event never records content
A disclosure audit event SHALL NOT contain the query text, any result content, any document content, or any draft text.

#### Scenario: The query is absent from the event
- **WHEN** a search is sent and its disclosure event is inspected
- **THEN** the query text appears nowhere in it

#### Scenario: Results are absent from the event
- **WHEN** results are returned and stored
- **THEN** no part of them appears in the disclosure event

#### Scenario: A scan of stored events finds no content
- **WHEN** every stored disclosure event is scanned for the query and result text
- **THEN** neither is found

### Requirement: Reproducibility lives with the initiative, not the audit log
The exact visible query SHALL be retained on the search record within its initiative, so a search can be reproduced without the audit log holding content.

#### Scenario: The search record reproduces the query
- **WHEN** a past search is inspected
- **THEN** the exact query it sent is available from its record

#### Scenario: The two records have different contents
- **WHEN** a search record and its disclosure event are compared
- **THEN** the query is in the search record and not in the event
