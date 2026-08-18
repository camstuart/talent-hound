## ADDED Requirements

### Requirement: The operating state stays visible
While working, the application SHALL keep visible the active initiative, the data scope, the selected model roles, whether a cloud override is in force, and whether it is online or offline.

#### Scenario: The state survives a tab change
- **WHEN** the recruiter moves between areas of a workspace
- **THEN** the initiative, scope, models, cloud override, and connectivity remain correct and visible

#### Scenario: A provider failure is reflected
- **WHEN** a provider becomes unreachable
- **THEN** the connectivity indication changes, and the rest of the state stays correct

#### Scenario: A cloud override is visible while in force
- **WHEN** a cloud override is approved for a task
- **THEN** the operating state shows that a cloud override is in force

#### Scenario: Demo scope is visible
- **WHEN** the scope is demo
- **THEN** the operating state says so, distinctly from real scope

### Requirement: The application version is displayed
The application SHALL display its version.

#### Scenario: The version is available in the interface
- **WHEN** the recruiter looks for the version
- **THEN** it is shown, and it matches the version in the diagnostic report
