## ADDED Requirements

### Requirement: Ranked lists are combined by reciprocal-rank fusion
The application SHALL combine ranked lists by summing `1 / (k + rank)` for each list a role appears in, with a fixed constant k. Raw scores from the contributing systems SHALL NOT be compared, normalized, or averaged.

#### Scenario: A role found by several lists outranks one found by one
- **WHEN** one role appears in three lists at middling ranks and another appears first in one list only
- **THEN** the role found by three lists ranks higher

#### Scenario: Lexical-only and semantic-only lists both contribute
- **WHEN** a role appears only in the lexical list and another only in the semantic list
- **THEN** both appear in the fused list, ranked by their reciprocal-rank contributions

#### Scenario: An empty list contributes nothing
- **WHEN** one of the lists is empty
- **THEN** the fused result is the same as if that list had not been run

#### Scenario: A duplicate within one list counts once
- **WHEN** a list contains the same role twice
- **THEN** it contributes its best rank only

#### Scenario: Scores from the two systems never meet
- **WHEN** fusion is computed
- **THEN** no lexical score is compared with, added to, or scaled against a semantic score

### Requirement: Results are grouped by role
Multiple matching chunks or aspects of one role SHALL occupy one shortlist position, not several.

#### Scenario: Many matching chunks yield one entry
- **WHEN** five chunks of one role match
- **THEN** that role occupies exactly one shortlist position

#### Scenario: Grouping preserves the best contribution
- **WHEN** a role matches at several ranks within one list
- **THEN** its contribution from that list is computed from its best rank

### Requirement: The shortlist is a stable top twenty
The shortlist SHALL contain at most twenty roles, ordered by fused score descending and then by role identifier ascending. Repeated runs over an unchanged corpus SHALL return identical results in identical order.

#### Scenario: More than twenty eligible roles returns exactly twenty
- **WHEN** thirty eligible roles match
- **THEN** exactly twenty are returned

#### Scenario: Fewer than twenty returns all of them
- **WHEN** seven eligible roles match
- **THEN** all seven are returned

#### Scenario: Ties order by identifier
- **WHEN** two roles have identical fused scores
- **THEN** the lower role identifier comes first, on every run

#### Scenario: Repeated runs are identical
- **WHEN** the shortlist is computed twice over an unchanged corpus
- **THEN** the two results are identical in content and order

#### Scenario: Nothing matching returns an empty shortlist
- **WHEN** no eligible role matches anything
- **THEN** an empty shortlist is returned rather than an error
