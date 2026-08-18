## ADDED Requirements

### Requirement: A draft is Active or Discarded
An Outreach Draft SHALL be in exactly one of two states. Editing SHALL keep it Active. Copying SHALL NOT change its state. Discarding SHALL make it Discarded and SHALL NOT delete its audit history.

#### Scenario: A new draft is Active
- **WHEN** a draft is generated
- **THEN** it is Active

#### Scenario: Editing keeps it Active
- **WHEN** the recruiter edits a draft
- **THEN** it remains Active with the edited text

#### Scenario: Copying is not a state change
- **WHEN** a draft is copied
- **THEN** it remains Active

#### Scenario: Discarding is terminal for the draft
- **WHEN** a draft is discarded
- **THEN** it is Discarded and is not offered for copying

#### Scenario: Discarding preserves audit history
- **WHEN** a draft with copy events is discarded
- **THEN** those events still exist

### Requirement: Every factual claim maps to evidence
A generated draft SHALL carry a mapping from each factual claim to the evidence supporting it, produced at generation time. Every mapped citation SHALL resolve.

#### Scenario: Claims are mapped
- **WHEN** a draft is generated
- **THEN** each factual claim it makes is listed with the evidence it rests on

#### Scenario: An unresolvable mapping is refused
- **WHEN** a generated draft maps a claim to evidence that does not resolve
- **THEN** the draft is refused rather than shown

#### Scenario: The mapping is not reconstructed on read
- **WHEN** a draft is read after editing
- **THEN** its mapping is the one recorded at generation

### Requirement: A draft can be edited and copied repeatedly
The recruiter SHALL be able to edit a draft and copy it any number of times. Each copy SHALL be recorded separately, and editing SHALL NOT destroy the draft or its history.

#### Scenario: Repeated copying records each one
- **WHEN** a draft is copied twice
- **THEN** two copy events exist

#### Scenario: Editing between copies is preserved
- **WHEN** a draft is copied, edited, and copied again
- **THEN** the draft holds the edited text and two copy events exist

#### Scenario: Editing does not create a copy event
- **WHEN** a draft is edited
- **THEN** no copy event is created
