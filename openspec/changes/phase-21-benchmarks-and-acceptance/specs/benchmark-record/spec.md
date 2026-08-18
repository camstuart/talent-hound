## ADDED Requirements

### Requirement: Every run records what it ran with
A benchmark run SHALL record the configuration, the model role assignments and their digests, the prompt and schema versions, the corpus hash, the measurements taken, and the results.

#### Scenario: A record carries its configuration
- **WHEN** a benchmark run completes
- **THEN** the record names every model role, its model, and its digest

#### Scenario: A record carries the corpus it ran against
- **WHEN** a benchmark run completes
- **THEN** the record carries the corpus hash and whether the corpus is synthetic

#### Scenario: A result without a record is not evidence
- **WHEN** a record is produced with no model assignment for a role the run used
- **THEN** the record says so rather than omitting the role

### Requirement: A record reports each condition separately
A classifier benchmark record SHALL report citation coverage, unsupported critical constraints, material-aspect capture, and structured-constraint reproduction as separate results alongside the overall pass.

#### Scenario: A failing run says which condition failed
- **WHEN** capture is below the threshold and every other condition passes
- **THEN** the record reports capture as the failing condition, and the others as passing

#### Scenario: An unsupported constraint is named
- **WHEN** the classifier introduces a must-have, location, work-rights, employment-type, or compensation value that no source supports
- **THEN** the record names it

### Requirement: Measurements are recorded with their conditions
Target-laptop measurements SHALL be recorded with what was measured and under what conditions, and SHALL NOT be reclassified after the fact.

#### Scenario: A measurement below a provisional target is recorded as measured
- **WHEN** a measurement misses a provisional performance target
- **THEN** it is recorded with its measured value and an explicit go/no-go decision, rather than being restated as a pass
