## ADDED Requirements

### Requirement: The classifier benchmark passes on four conditions
The classifier benchmark SHALL pass only when every extracted aspect has a valid citation, no unsupported must-have, location, work-rights, employment-type, or compensation value is introduced, at least 80% of labelled material aspects are captured, and explicit structured constraints are reproduced correctly.

#### Scenario: All four conditions met
- **WHEN** every condition is satisfied
- **THEN** the benchmark passes

#### Scenario: An uncited aspect fails the benchmark
- **WHEN** one extracted aspect has no valid citation
- **THEN** the benchmark fails, however high the capture rate

#### Scenario: Capture just below the threshold fails
- **WHEN** 79% of material aspects are captured and every other condition holds
- **THEN** the benchmark fails

#### Scenario: A misreproduced structured constraint fails
- **WHEN** a stated compensation or location value is reproduced incorrectly
- **THEN** the benchmark fails, and the record names the constraint

### Requirement: The matching benchmark passes on the PRD's rule
The matching benchmark SHALL pass when at least three of the top five roles are rated plausible in at least four of the five scenarios. Duplicate roles SHALL collapse before rating, and an absent slot SHALL count as not plausible.

#### Scenario: Four of five scenarios reach three plausible
- **WHEN** four scenarios have three or more plausible in the top five
- **THEN** the benchmark passes

#### Scenario: Three of five is not enough
- **WHEN** only three scenarios reach three plausible
- **THEN** the benchmark fails

#### Scenario: Duplicates collapse before counting
- **WHEN** the top five contains the same role twice
- **THEN** it counts once, and the freed slot is not counted as plausible

#### Scenario: A short list is not padded
- **WHEN** a scenario returns three roles
- **THEN** the two absent slots count as not plausible

### Requirement: A thin live run is inconclusive, not a result
A live acceptance run that finds fewer than ten eligible roles SHALL be reported as source-coverage inconclusive, and SHALL NOT be reported as a pass or a failure.

#### Scenario: Nine eligible roles
- **WHEN** a live run finds nine eligible roles
- **THEN** the outcome is inconclusive, and the count is recorded

#### Scenario: Ten eligible roles is a result
- **WHEN** a live run finds ten eligible roles
- **THEN** the run is scored normally

### Requirement: A cloud-assisted run cannot pass the PoC
A run that used any cloud override SHALL be recorded separately and SHALL NOT satisfy a PoC acceptance gate.

#### Scenario: A cloud-assisted run is marked
- **WHEN** a benchmark run used a cloud override for any task
- **THEN** the record marks it cloud-assisted, and its outcome cannot be a PoC pass

#### Scenario: Local-only is what passes
- **WHEN** the acceptance record is read
- **THEN** the passing runs are the local-only ones
