## ADDED Requirements

### Requirement: The benchmark corpus is frozen before any run
The benchmark corpus SHALL consist of five matching scenarios and twenty classifier role listings, stored as data, with every label applied before any model runs against it.

#### Scenario: The corpus is complete
- **WHEN** the corpus is loaded
- **THEN** it holds five scenarios and twenty role listings, and every listing carries its material aspects and structured constraints

#### Scenario: Every scenario carries its ratings
- **WHEN** a matching scenario is loaded
- **THEN** it carries the recruiter's plausibility rating for each of its candidate roles

#### Scenario: A corpus missing labels is refused
- **WHEN** a listing has no material aspects
- **THEN** loading the corpus fails, naming the listing

### Requirement: The corpus is identified by a hash over its bytes
The corpus SHALL be identified by a single hash over every corpus file, computed so that it depends on the content and not on the order the files were read.

#### Scenario: The same corpus hashes the same
- **WHEN** the corpus is hashed twice, including across separate processes
- **THEN** the hashes are identical

#### Scenario: A changed corpus hashes differently
- **WHEN** any corpus file changes by one byte
- **THEN** the hash changes

#### Scenario: A changed hash is reported, not failed
- **WHEN** a run is recorded against a corpus whose hash differs from an earlier run's
- **THEN** the record carries the new hash, and the run is not failed for it

### Requirement: The corpus states that its content is invented
The corpus SHALL state that its content is synthetic, so no run is mistaken for one against the recruiter's real placements.

#### Scenario: The provenance is in the corpus and in the record
- **WHEN** a benchmark record is produced
- **THEN** it states whether the corpus is the synthetic one or the recruiter's
