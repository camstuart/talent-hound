## ADDED Requirements

### Requirement: Volume encryption is checked at every startup
The application SHALL check the encryption status of the volume holding the selected data folder at every startup, not only during first run.

#### Scenario: An encrypted volume permits real data
- **WHEN** the volume reports as encrypted
- **THEN** real-data scope is available

#### Scenario: A volume that becomes unencrypted blocks real data later
- **WHEN** the volume was encrypted at first run and reports unencrypted at a later startup
- **THEN** real-data scope is blocked from that startup onward

#### Scenario: An unknown result never permits real data
- **WHEN** the encryption check is unavailable or permission-denied
- **THEN** real-data scope is blocked, and the reason distinguishes "could not check" from "not encrypted"

### Requirement: Demo scope refuses real content
An optional demo scope SHALL be available on any volume, and SHALL refuse candidate artifacts and personal-data entry at the service boundary.

#### Scenario: Demo scope refuses an artifact
- **WHEN** an artifact is created in demo scope
- **THEN** it is refused, and nothing is stored

#### Scenario: Demo scope refuses a candidate
- **WHEN** a candidate is created in demo scope
- **THEN** it is refused, and nothing is stored

#### Scenario: The refusal is at the boundary, not the interface
- **WHEN** the create call is made directly rather than through the interface
- **THEN** it is refused identically

#### Scenario: Demo scope is not an acceptance environment
- **WHEN** the scope is demo
- **THEN** the operating state shows it, and the acceptance run does not treat it as a pass

### Requirement: Scope changes are explicit
The scope SHALL change only by an explicit choice, and SHALL NOT change as a side effect of an encryption result.

#### Scenario: A blocked real scope does not silently become demo
- **WHEN** real scope is selected and the volume is not encrypted
- **THEN** the application blocks personal-data work and reports why, rather than switching scope
