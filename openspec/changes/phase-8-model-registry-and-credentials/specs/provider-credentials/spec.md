## ADDED Requirements

### Requirement: Provider secrets live only in the operating system credential store
A provider secret SHALL be stored only in Windows Credential Manager or macOS Keychain. The application SHALL NOT provide any other storage for a secret — no file, no database column, no environment variable — on any platform.

#### Scenario: A secret is stored in the credential store
- **WHEN** a provider secret is stored
- **THEN** it is written to the operating system credential store and nowhere else

#### Scenario: An unsupported platform refuses rather than falling back
- **WHEN** a secret is stored on a platform with no supported credential store
- **THEN** the operation fails with a message saying the platform is unsupported, and no secret is written anywhere

#### Scenario: Revoking removes the secret without removing local data
- **WHEN** a provider's credential is revoked
- **THEN** the secret is removed from the credential store and no local records are deleted

#### Scenario: A missing credential is an ordinary answer
- **WHEN** a provider that has no stored credential is queried
- **THEN** the answer is that no credential exists, rather than an error the recruiter has to interpret

#### Scenario: Replacing a credential keeps only the new secret
- **WHEN** a provider's secret is stored again with a different value
- **THEN** the previous value is replaced and is no longer retrievable

#### Scenario: An empty secret is refused
- **WHEN** an empty secret is stored
- **THEN** it is refused rather than stored as a credential that exists but authorizes nothing

### Requirement: A stored secret is never returned to the interface
The application SHALL NOT expose any operation that returns a stored secret to the user interface. The interface SHALL be able to learn only whether a credential exists.

#### Scenario: The interface can ask only whether a credential exists
- **WHEN** the interface asks about a provider's credential
- **THEN** it receives whether one exists and never the value

#### Scenario: Entry is masked
- **WHEN** a secret is being entered
- **THEN** its characters are masked in the interface

### Requirement: A secret appears in no record, log, or error
A stored secret SHALL NOT appear in the database, in log output, in any error message, or in a copied recovery folder.

#### Scenario: The database contains no secret
- **WHEN** a secret has been stored and every credential operation has been exercised
- **THEN** the database file's contents do not contain the secret

#### Scenario: The logs contain no secret
- **WHEN** a secret has been stored and every credential operation has been exercised
- **THEN** the captured log output does not contain the secret

#### Scenario: An error naming a failed operation does not quote the secret
- **WHEN** storing a secret fails
- **THEN** the error names the provider and the failure without including the secret

#### Scenario: A copied data folder contains no secret
- **WHEN** the data folder is copied for recovery
- **THEN** nothing in the copy contains a provider secret
