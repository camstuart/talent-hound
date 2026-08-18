## ADDED Requirements

### Requirement: A reclassification is proposed against the approved version
Reclassifying a candidate with an approved profile SHALL create a Proposed version and SHALL NOT modify or replace the approved one. The approved version SHALL remain the version in use until the new one is approved.

#### Scenario: Reclassification leaves the approved version in place
- **WHEN** a candidate with an approved profile is reclassified
- **THEN** a Proposed version exists, the approved version is unchanged, and it is still the one in use

#### Scenario: An approved aspect is never silently overwritten
- **WHEN** a reclassification produces an aspect contradicting an approved one
- **THEN** the approved aspect is unchanged and the difference is presented as a conflict

### Requirement: A diff presents additions, removals, and conflicts
Comparing a proposed version against an approved one SHALL produce three groups: aspects present only in the proposal, aspects present only in the approved version, and aspects that correspond but differ.

#### Scenario: A new aspect is an addition
- **WHEN** the proposal contains an aspect with no counterpart in the approved version
- **THEN** it appears as an addition

#### Scenario: A vanished aspect is a removal
- **WHEN** the approved version contains an aspect with no counterpart in the proposal
- **THEN** it appears as a removal

#### Scenario: A corresponding aspect that differs is a conflict
- **WHEN** both versions contain an aspect of the same type describing the same thing, with different content
- **THEN** it appears as a conflict showing both sides

#### Scenario: An unchanged aspect appears in none of the three
- **WHEN** an aspect is identical in both versions
- **THEN** it is neither an addition, a removal, nor a conflict

#### Scenario: A diff is a pure comparison of two versions
- **WHEN** the same two versions are compared twice
- **THEN** the result is identical, and no model is called

### Requirement: Resolving a conflict is an explicit choice
The recruiter SHALL be able to resolve each conflict by keeping the approved aspect or taking the proposed one, and the resolution SHALL produce a new version rather than editing either compared version.

#### Scenario: Keeping the approved side
- **WHEN** the recruiter keeps the approved aspect for a conflict
- **THEN** the resulting version contains the approved aspect

#### Scenario: Taking the proposed side
- **WHEN** the recruiter takes the proposed aspect for a conflict
- **THEN** the resulting version contains the proposed aspect

#### Scenario: Neither compared version is modified
- **WHEN** any conflict is resolved
- **THEN** both the approved and the proposed versions are unchanged
