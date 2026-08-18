## ADDED Requirements

### Requirement: The registry holds three local model roles
The application SHALL record assignments for the `embed`, `classify`, and `generate` roles. Each assignment SHALL record the endpoint, the model name, its immutable digest when the endpoint reports one, its parameters, and a validation status.

#### Scenario: An assignment records the model's identity
- **WHEN** a model is assigned to a role
- **THEN** the assignment records the endpoint, model name, digest, parameters, and validation status

#### Scenario: A missing required role is reported
- **WHEN** the registry is read and no model has been assigned to a required role
- **THEN** that role is reported as unassigned rather than silently defaulted to a model name

#### Scenario: An unknown role is refused
- **WHEN** an assignment names a role that is not one of the three
- **THEN** it is refused

### Requirement: Registry assignments are local only
A registry assignment SHALL name the local model endpoint. An assignment naming any other endpoint SHALL be refused, because a cloud endpoint is a task-level override in a later phase and never a required role here.

#### Scenario: A cloud endpoint is refused for a required role
- **WHEN** an assignment names an endpoint that is not the local one
- **THEN** it is refused and the previous assignment is unchanged

#### Scenario: An invalid endpoint URL is refused
- **WHEN** an assignment names an endpoint that is not an absolute http or https URL
- **THEN** it is refused

#### Scenario: Unsupported parameters are refused
- **WHEN** an assignment carries parameters that are not valid JSON or contain a parameter the role does not support
- **THEN** it is refused

### Requirement: Configuration changes create a new revision
An assignment SHALL be immutable once written. Changing the endpoint, model, digest, or parameters SHALL create a new revision, and the current assignment for a role SHALL be its highest revision. Assigning a configuration identical to the current one SHALL NOT create a revision.

#### Scenario: Changing the model creates a revision
- **WHEN** a role is assigned a different model
- **THEN** a new revision is created and the previous revision is still recorded

#### Scenario: Changing the parameters creates a revision
- **WHEN** a role is assigned the same model with different parameters
- **THEN** a new revision is created

#### Scenario: Changing the digest creates a revision
- **WHEN** the same model name is assigned with a different immutable digest
- **THEN** a new revision is created, because the model behind the name has changed

#### Scenario: Re-assigning the same configuration is not a change
- **WHEN** a role is assigned exactly the configuration it already has
- **THEN** no new revision is created

#### Scenario: A revision is never edited
- **WHEN** any assignment changes
- **THEN** the earlier revision's endpoint, model, digest, and parameters are unchanged

### Requirement: Classify follows generate until it is assigned
The `classify` role SHALL default to the `generate` assignment without duplicating its configuration. Resolving `classify` while it is unassigned SHALL return the current `generate` assignment together with the fact that it was inherited.

#### Scenario: Classify inherits the generate assignment
- **WHEN** `generate` is assigned and `classify` is not
- **THEN** resolving `classify` returns the `generate` model and reports that it was inherited

#### Scenario: Classify keeps following when generate changes
- **WHEN** `generate` is assigned a different model while `classify` is unassigned
- **THEN** resolving `classify` returns the new model

#### Scenario: An explicit assignment stops the inheritance
- **WHEN** `classify` is assigned a model of its own and `generate` is later changed
- **THEN** resolving `classify` returns its own model and does not report inheritance

#### Scenario: Classify cannot be resolved before generate exists
- **WHEN** neither `generate` nor `classify` has been assigned
- **THEN** resolving `classify` reports that the role is unassigned

### Requirement: A model is Unvalidated until a benchmark says otherwise
Every assignment SHALL start with validation status Unvalidated. Validation status SHALL only become Validated when a benchmark record is supplied.

#### Scenario: A newly assigned model is Unvalidated
- **WHEN** any model is assigned to any role
- **THEN** its validation status is Unvalidated

#### Scenario: Validation without a benchmark reference is refused
- **WHEN** a role is marked Validated with no benchmark reference
- **THEN** the request is refused and the status stays Unvalidated

#### Scenario: A new revision starts Unvalidated again
- **WHEN** a validated assignment is replaced by a new revision
- **THEN** the new revision's validation status is Unvalidated
