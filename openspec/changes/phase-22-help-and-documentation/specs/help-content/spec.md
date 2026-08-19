## ADDED Requirements

### Requirement: Every topic is reachable without searching
Help SHALL present an index of every topic, grouped, so a recruiter who does not know the word for what they want can still find it.

#### Scenario: The index lists every article
- **WHEN** the help index is shown
- **THEN** every shipped article appears under a group

#### Scenario: An article is opened from the index
- **WHEN** a topic is chosen
- **THEN** its article is shown with its sections

### Requirement: A tutorial walks the flagship loop in order
Help SHALL include a tutorial covering, in the order a recruiter does them: creating an initiative, adding a candidate, attaching a document, extracting it, approving a profile, stating criteria, discovering roles, building a shortlist, reading an assessment, and copying out a draft.

#### Scenario: The tutorial covers each step
- **WHEN** the tutorial is read
- **THEN** each of those steps appears, in that order

#### Scenario: Each step says what to expect
- **WHEN** a tutorial step is read
- **THEN** it says what the application will do and what the recruiter has to decide

### Requirement: The articles cover the rules that surprise people
Help SHALL explain the behaviours a recruiter would otherwise discover by being refused: what is never sent, what blocks a deletion, what makes a profile stale, when real-data mode is unavailable, and what the application cannot do.

#### Scenario: The sending boundary is documented
- **WHEN** help is read on outreach
- **THEN** it states that the application drafts and copies out, and cannot send

#### Scenario: Deletion rules are documented
- **WHEN** help is read on deleting a candidate
- **THEN** it states that referencing initiatives block it, archived ones included

#### Scenario: The encryption gate is documented
- **WHEN** help is read on first run
- **THEN** it states that real candidate data needs an encrypted volume, and what happens when the check cannot be made

### Requirement: Help content carries no recruiter data
Help articles SHALL contain no candidate information, and no example drawn from a recruiter's records.

#### Scenario: Articles are inspected for content
- **WHEN** the shipped articles are inspected
- **THEN** every name, employer, and figure in them is invented
