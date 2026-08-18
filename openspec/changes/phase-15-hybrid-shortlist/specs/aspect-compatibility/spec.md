## ADDED Requirements

### Requirement: The aspect compatibility map is enforced exactly
Aspect retrieval SHALL follow the compatibility map: a skill role aspect searches skill, experience, and responsibility candidate aspects; responsibility searches responsibility, experience, and skill; experience searches experience and responsibility; qualification searches qualification; seniority searches seniority and experience; other searches other. No edge outside this map SHALL be used.

#### Scenario: Every permitted edge retrieves
- **WHEN** retrieval runs for each role aspect type in the map
- **THEN** each of its listed candidate aspect types is searched

#### Scenario: A disallowed edge retrieves nothing
- **WHEN** a qualification aspect is used for retrieval
- **THEN** no skill, responsibility, experience, seniority, or other aspect is retrieved by it

#### Scenario: The map is asymmetric where the PRD is asymmetric
- **WHEN** the edges are compared in both directions
- **THEN** experience searches responsibility while qualification searches nothing but qualification, exactly as stated

### Requirement: The map is stated once
The compatibility map SHALL exist in exactly one place, and any inverse required by retrieval SHALL be derived from it rather than written separately.

#### Scenario: Adding an edge changes both directions
- **WHEN** an edge is added to the map
- **THEN** retrieval in both directions reflects it without a second edit

#### Scenario: The derived inverse matches the map
- **WHEN** the inverse is computed and compared with the map
- **THEN** every pair present in one is present in the other, in the corresponding direction

### Requirement: Structured aspect types are compared deterministically
Location, work arrangement, work rights, employment type, and compensation SHALL be compared by their normalized structured values rather than retrieved by similarity.

#### Scenario: A structured type is not searched by similarity
- **WHEN** retrieval runs
- **THEN** location, work arrangement, work rights, employment type, and compensation aspects are not used as similarity queries

#### Scenario: A structured comparison is exact
- **WHEN** two structured values of the same field are compared
- **THEN** the result depends only on those values, with no model and no similarity involved
