## ADDED Requirements

### Requirement: Known structured identifiers become placeholders
Eligible assessment and drafting payloads SHALL replace the candidate's known structured direct identifiers — name, email, phone, and address — with placeholders before sending.

#### Scenario: A name becomes a placeholder
- **WHEN** a payload would contain the candidate's name
- **THEN** it contains a placeholder instead

#### Scenario: An email becomes a placeholder
- **WHEN** a payload would contain the candidate's email
- **THEN** it contains a placeholder instead

#### Scenario: A phone becomes a placeholder
- **WHEN** a payload would contain the candidate's phone number
- **THEN** it contains a placeholder instead

#### Scenario: An address becomes a placeholder
- **WHEN** a payload would contain the candidate's address
- **THEN** it contains a placeholder instead

#### Scenario: The professional content survives
- **WHEN** identifiers are replaced
- **THEN** the skills, experience, and requirements remain, so the request is still useful

### Requirement: Substitution happens before the preview
Placeholders SHALL be applied before the payload is shown, so the recruiter previews what will actually be sent.

#### Scenario: The preview shows placeholders
- **WHEN** an eligible payload is previewed
- **THEN** it contains placeholders rather than identifiers

#### Scenario: The sent payload matches the preview
- **WHEN** a previewed payload is sent
- **THEN** the endpoint receives the placeholders, not the identifiers

### Requirement: Substitution is not claimed to be complete
The application SHALL NOT represent placeholder substitution as removing every identifier, and the preview SHALL remain the recruiter's control over what leaves.

#### Scenario: An unknown identifier is not caught
- **WHEN** a document contains an identifier the record does not know
- **THEN** substitution may not replace it, and the preview is where the recruiter sees it

#### Scenario: The preview is available for every eligible send
- **WHEN** any eligible payload is about to be sent
- **THEN** it can be inspected first
