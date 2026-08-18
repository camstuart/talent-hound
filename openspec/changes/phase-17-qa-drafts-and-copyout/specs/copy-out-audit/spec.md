## ADDED Requirements

### Requirement: Copying records a metadata-only event
Each copy SHALL create one audit event recording the time, the initiative, and the draft it was about.

#### Scenario: One copy, one event
- **WHEN** a draft is copied
- **THEN** exactly one copy event exists for it

#### Scenario: The event names the initiative and the draft
- **WHEN** a copy event is inspected
- **THEN** it names the initiative and the draft

### Requirement: A copy event never contains content
A copy event SHALL NOT contain the draft text, any payload, any query, or any document content.

#### Scenario: The draft text is absent
- **WHEN** a draft with distinctive text is copied and its event is inspected
- **THEN** that text appears nowhere in the event

#### Scenario: A scan of stored events finds no content
- **WHEN** every stored copy event is scanned for the draft text
- **THEN** it is not found

### Requirement: Discarding does not record a copy or a send
Discarding a draft SHALL create no copy event, and no event of any kind SHALL suggest that a message was sent.

#### Scenario: Discarding creates no copy event
- **WHEN** a draft is discarded
- **THEN** the number of copy events is unchanged

#### Scenario: No event records a send
- **WHEN** every audit event kind is inspected
- **THEN** none of them means a message was sent
