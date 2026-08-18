## ADDED Requirements

### Requirement: Extracted Markdown is cut by a fixed structural algorithm
Extracted Markdown SHALL be divided into chunks by a fixed algorithm that identifies headings, list items, tables, code blocks, and paragraphs first, and segments into sentences only when a single structural block exceeds the maximum chunk size. No boundary decision SHALL depend on anything other than the text and the recorded chunker parameters.

#### Scenario: A heading stays with the text it introduces
- **WHEN** Markdown containing a heading followed by a short paragraph is chunked
- **THEN** the heading and the paragraph occupy the same chunk rather than the heading forming a chunk of its own

#### Scenario: A section boundary ends a chunk
- **WHEN** two sections short enough to fit together are separated by a heading
- **THEN** they are placed in different chunks, because the heading path changed

#### Scenario: A long paragraph is segmented into sentences
- **WHEN** a single paragraph exceeds the maximum chunk size
- **THEN** it is divided at sentence boundaries and no chunk begins or ends inside a sentence

#### Scenario: An abbreviation is not a sentence boundary
- **WHEN** a paragraph being segmented contains abbreviations such as titles, initials, or decimal numbers
- **THEN** no chunk boundary falls at those points

#### Scenario: A table is not divided
- **WHEN** Markdown containing a table is chunked
- **THEN** the table's rows remain in one chunk together with its header row

#### Scenario: Nested lists keep their items whole
- **WHEN** Markdown containing a nested list is chunked
- **THEN** each list item, including its continuation lines, lies wholly within one chunk

#### Scenario: Blank sections produce no chunks
- **WHEN** Markdown contains blank lines, trailing whitespace, or a heading with no body
- **THEN** no empty chunk is produced

#### Scenario: Unicode text is chunked without corruption
- **WHEN** Markdown containing non-Latin scripts, accented characters, and emoji is chunked
- **THEN** every chunk's text is valid and the concatenation of the chunked regions reproduces those characters unchanged

#### Scenario: Content exactly at the target size
- **WHEN** a block's size is exactly the target chunk size
- **THEN** it is emitted as one chunk rather than being split

### Requirement: Chunking is deterministic
Chunking the same Markdown with the same chunker version and parameters SHALL produce identical chunks: the same count, the same boundaries, the same offsets, and the same content hashes.

#### Scenario: Repeated chunking is identical
- **WHEN** the same Markdown is chunked twice
- **THEN** both runs produce the same number of chunks with pairwise identical text, offsets, token counts, and hashes

#### Scenario: Re-chunking an artifact replaces its chunks wholly
- **WHEN** an artifact that already has chunks is chunked again
- **THEN** its previous chunks are removed and replaced, so all of its chunks come from one chunker version and one set of parameters

### Requirement: A chunk records where it came from
Each chunk SHALL record the artifact it came from, its ordinal within that artifact, its text, its start and end character offsets, its heading path, its token count, a content hash of its text, and the name, version, and parameters of the chunker that produced it.

#### Scenario: Ordinals are contiguous and ordered
- **WHEN** an artifact is chunked
- **THEN** its chunks are numbered from zero without gaps, in the order they appear in the Markdown

#### Scenario: The heading path names the section
- **WHEN** a chunk lies beneath nested headings
- **THEN** its heading path lists those headings from the outermost to the one directly above it

#### Scenario: The chunker identifies itself
- **WHEN** a chunk is stored
- **THEN** it records the chunker name, its version, and the parameters in force, so a later phase can tell which chunks a new version has made stale

#### Scenario: The token count states what it counts
- **WHEN** a chunk records a token count
- **THEN** the counting method is recorded in the chunker parameters rather than left to be inferred

### Requirement: Chunking runs as a background job
Chunking SHALL run through the background job lifecycle, one artifact per item, so that a cancelled run keeps the artifacts it completed and leaves nothing behind from the one it did not.

#### Scenario: Chunking is enqueued rather than run inline
- **WHEN** chunking is requested for one or more artifacts
- **THEN** a job is enqueued with one item per artifact and the request returns without waiting

#### Scenario: Cancellation keeps committed items and discards the current one
- **WHEN** a chunking job over several artifacts is cancelled part way through
- **THEN** the artifacts already chunked keep their chunks, the artifact in flight has none, and the remaining artifacts are untouched

#### Scenario: An artifact that has not been extracted cannot be chunked
- **WHEN** chunking runs for an artifact whose extraction has not succeeded
- **THEN** the item fails with a short reason code and no chunks are written

#### Scenario: A chunking failure carries no document content
- **WHEN** a chunking item fails
- **THEN** the recorded reason is a short lowercase code and no part of the document appears in the job record or the logs
