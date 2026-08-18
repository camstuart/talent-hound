## ADDED Requirements

### Requirement: A conversation may propose but never apply
Chat MAY suggest structured changes such as criteria or profile aspects. No such suggestion SHALL be stored until the recruiter applies it explicitly.

#### Scenario: A proposal writes nothing
- **WHEN** a conversation suggests a criterion
- **THEN** no criterion is created and the criteria version is unchanged

#### Scenario: Applying is a separate act
- **WHEN** the recruiter applies a suggested criterion
- **THEN** it is created by the same path any recruiter-authored criterion uses, including its refusals

#### Scenario: A suggested criterion is still subject to the rules
- **WHEN** a conversation suggests a criterion naming a protected attribute
- **THEN** applying it is refused exactly as typing it would be

### Requirement: Text in an artifact cannot act
Instructions found inside any document SHALL NOT be able to change retrieval scope, apply a structured change, contact a provider, delete anything, or cause a copy.

#### Scenario: An artifact cannot widen scope
- **WHEN** a document instructs the assistant to include another initiative's evidence
- **THEN** the retrieval scope is unchanged

#### Scenario: An artifact cannot apply a change
- **WHEN** a document instructs the assistant to add a criterion
- **THEN** nothing is written until the recruiter applies it

#### Scenario: An artifact cannot reach a provider
- **WHEN** a document instructs the assistant to run a search
- **THEN** no request is sent and no disclosure event is created

#### Scenario: An artifact cannot delete
- **WHEN** a document instructs the assistant to delete a record
- **THEN** nothing is deleted

#### Scenario: An artifact cannot cause a copy
- **WHEN** a document instructs the assistant to copy a draft
- **THEN** no CopiedOut event is created
