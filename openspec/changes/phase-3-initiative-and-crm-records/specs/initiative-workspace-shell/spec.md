## ADDED Requirements

### Requirement: Four-area workspace skeleton
Every open initiative SHALL present Context, Research, Matches, and Drafts as navigable areas. Each area SHALL state what it will hold rather than presenting speculative pipeline behaviour.

#### Scenario: All four areas are present for every type
- **WHEN** an initiative of any of the three types is opened
- **THEN** Context, Research, Matches, and Drafts are all reachable within that initiative's tab

#### Scenario: Switching areas keeps the initiative active
- **WHEN** an area is selected within an open initiative
- **THEN** only that area's panel is shown, the initiative remains the active tab, and no other initiative's state changes

#### Scenario: Empty areas are explicit
- **WHEN** an area with no content yet is opened
- **THEN** it states what will appear there, rather than showing an error, a spinner, or a blank panel

### Requirement: Non–Job Search types are workspace shells
Talent Search and Business Development initiatives SHALL render the workspace shell and SHALL NOT offer pipeline actions.

#### Scenario: Talent Search offers no pipeline
- **WHEN** a Talent Search initiative is opened
- **THEN** the four areas render and the workspace states that its pipeline is outside PoC scope

#### Scenario: Business Development offers no pipeline
- **WHEN** a Business Development initiative is opened
- **THEN** the four areas render and the workspace states that its pipeline is outside PoC scope

### Requirement: Lifecycle state is visible and operable
The interface SHALL show each initiative's lifecycle state and SHALL offer rename, archive, and reopen from the workspace.

#### Scenario: Archived initiatives are labelled
- **WHEN** an archived initiative appears in the sidebar or as an open tab
- **THEN** its archived state is visible without opening it

#### Scenario: Archive and reopen are available in place
- **WHEN** an initiative is open
- **THEN** rename is available, and exactly one of archive or reopen is offered according to its current state

### Requirement: Record forms are keyboard-operable and report errors in place
Record creation and editing forms SHALL be completable by keyboard alone and SHALL display validation errors against the field that failed.

#### Scenario: Form is completable by keyboard
- **WHEN** a record form is filled in and submitted using only keyboard navigation
- **THEN** every field is reachable in a sensible order and the record is created

#### Scenario: Validation errors are attached to their field
- **WHEN** a submitted form fails validation
- **THEN** the error is displayed against the offending field, the form keeps the values already entered, and nothing is persisted

#### Scenario: Backend rejection surfaces to the user
- **WHEN** the backend rejects a value the frontend allowed
- **THEN** the backend's message is shown to the user rather than being swallowed or replaced by a generic failure
