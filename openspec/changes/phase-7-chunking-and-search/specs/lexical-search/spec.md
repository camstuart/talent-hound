## ADDED Requirements

### Requirement: Chunks are indexed for lexical search
Chunk text SHALL be indexed in an SQLite FTS5 table created by an explicit migration. The index SHALL be kept synchronized with the chunk rows by database triggers, so that it is correct regardless of which code path wrote the chunks.

#### Scenario: A new chunk becomes searchable
- **WHEN** a chunk is inserted
- **THEN** a search for a term in its text finds it

#### Scenario: A deleted chunk stops being searchable
- **WHEN** a chunk is deleted
- **THEN** a search for a term in its text no longer finds it

#### Scenario: Changed text is searchable by its new words only
- **WHEN** a chunk's text is updated
- **THEN** searches match the new text and no longer match words that were only in the old text

#### Scenario: A rolled-back transaction leaves no index entries
- **WHEN** chunks are inserted in a transaction that is then rolled back
- **THEN** no search finds them, and the index contains nothing referring to them

#### Scenario: Re-extraction removes an artifact's chunks from the index
- **WHEN** an artifact is extracted again and its chunks are discarded
- **THEN** searches no longer return those chunks

### Requirement: The index can be rebuilt on demand
The application SHALL expose an explicit rebuild of the lexical index that restores it from the chunk rows.

#### Scenario: Results are unchanged by a rebuild
- **WHEN** the index is rebuilt on a database holding chunks
- **THEN** the same query returns the same chunks in the same order as it did before the rebuild

#### Scenario: A rebuild repairs a damaged index
- **WHEN** the index has lost entries that the chunk rows still contain
- **THEN** rebuilding restores those entries and the query finds them again

### Requirement: A search query is treated as text, never as index syntax
A search query SHALL be interpreted as a set of terms, all of which must appear. Query text SHALL NOT be passed through as full-text search expression syntax, so that punctuation, quoting, and operator-shaped input are searched for rather than executed.

#### Scenario: Punctuation and quotes are searched for
- **WHEN** a query contains parentheses, quotation marks, hyphens, or apostrophes
- **THEN** the search runs and returns matches rather than failing with a syntax error

#### Scenario: Operator-shaped input is not an operator
- **WHEN** a query contains words that are full-text search operators
- **THEN** they are searched for as ordinary terms

#### Scenario: An empty query returns nothing
- **WHEN** a query is empty or contains only whitespace
- **THEN** no results are returned and no error is raised

#### Scenario: Unicode terms are searchable
- **WHEN** a query contains non-Latin script or accented characters present in a chunk
- **THEN** that chunk is returned

#### Scenario: A common term returns results without failing
- **WHEN** a query is a very common word appearing in most chunks
- **THEN** results are returned up to the requested limit

### Requirement: Search is scoped to the workspace that asked
A search SHALL return only chunks belonging to artifacts linked to the initiative the search was made from.

#### Scenario: Another initiative's evidence is not returned
- **WHEN** a search is run from one initiative and a matching chunk belongs to an artifact linked only to another
- **THEN** that chunk is not returned
