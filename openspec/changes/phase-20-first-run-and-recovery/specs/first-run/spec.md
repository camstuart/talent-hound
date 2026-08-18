## ADDED Requirements

### Requirement: First run is an ordered sequence
First run SHALL present, in order: choose the data folder, verify volume encryption, verify the bundled extraction sidecar and its pinned version, verify Ollama, show required models with download sizes and pull missing ones, acknowledge the data-handling preconditions, and create the first initiative. A later step SHALL NOT be reachable while an earlier one is unsatisfied.

#### Scenario: A fresh install starts at the data folder
- **WHEN** the application starts with no recorded data folder
- **THEN** the first-run state is the data-folder step

#### Scenario: The sidecar step is unreachable before a folder is chosen
- **WHEN** the sidecar check is requested with no data folder recorded
- **THEN** the state remains the data-folder step

#### Scenario: Setup is complete only after the first initiative
- **WHEN** every step is satisfied and at least one initiative exists
- **THEN** the first-run state is complete

### Requirement: The wizard's position is computed from what is true
The first-run state SHALL be derived from the current checks each time it is requested, and SHALL NOT be read from a stored step number.

#### Scenario: Ollama disappearing moves the state back
- **WHEN** Ollama was verified earlier and is unavailable now
- **THEN** the first-run state is the Ollama step again

#### Scenario: Cancelling at any step loses nothing but that step
- **WHEN** setup is abandoned at any step and the application is restarted
- **THEN** the state is the first unsatisfied step, and every earlier answer is still in force

### Requirement: A missing dependency blocks with its own reason
Each verification step SHALL report which dependency is missing and SHALL NOT report a generic failure.

#### Scenario: A missing sidecar names the sidecar
- **WHEN** the bundled extraction sidecar is absent
- **THEN** the step reports the sidecar and the path it looked in

#### Scenario: A sidecar of the wrong version is refused
- **WHEN** the sidecar reports a version other than the pinned one
- **THEN** the step reports both versions and does not pass

#### Scenario: A missing Ollama names Ollama
- **WHEN** Ollama is not reachable
- **THEN** the step reports Ollama and the endpoint it tried

### Requirement: Required models are shown with sizes and may be pulled
The models step SHALL list every required model role with its model name and approximate download size, and SHALL report which are already present.

#### Scenario: Missing models are listed with sizes
- **WHEN** the models step is shown and one required model is absent
- **THEN** it is listed as missing with its name and approximate size

#### Scenario: A declined pull leaves setup resumable
- **WHEN** the recruiter does not pull a missing model
- **THEN** setup remains at the models step and nothing is lost

#### Scenario: A failed pull leaves setup resumable
- **WHEN** a pull fails
- **THEN** the failure is reported, and the state remains the models step

### Requirement: The acknowledgement is required and recorded
Setup SHALL NOT complete until the recruiter acknowledges the data-handling preconditions, and the acknowledgement SHALL be recorded with the version of the text acknowledged.

#### Scenario: Setup cannot complete unacknowledged
- **WHEN** every other step is satisfied and the acknowledgement is absent
- **THEN** the first-run state is the acknowledgement step

#### Scenario: The acknowledgement survives a restart
- **WHEN** the acknowledgement is given and the application is restarted
- **THEN** it is still in force
