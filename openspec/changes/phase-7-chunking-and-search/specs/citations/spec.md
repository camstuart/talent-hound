## ADDED Requirements

### Requirement: A chunk's offsets select its text in the source
A chunk's stored start and end offsets SHALL select exactly its stored text within the Markdown of the artifact it came from.

#### Scenario: Every offset selects the cited text
- **WHEN** any stored chunk is compared with the artifact's Markdown at its offsets
- **THEN** the selected text is byte-for-byte the chunk's stored text

#### Scenario: The content hash matches the text
- **WHEN** a chunk's content hash is recomputed from its stored text
- **THEN** it equals the stored hash

### Requirement: A citation resolves to an artifact and a human-readable location
A citation SHALL resolve a chunk to the artifact it came from and to a location a person can use to find the same words in the original document, including the artifact's name, the heading path, and the chunk's position.

#### Scenario: A citation names its artifact and section
- **WHEN** a chunk retrieved by search is resolved
- **THEN** the result names the artifact and gives the heading path leading to that chunk

#### Scenario: A citation resolves against the source rather than being trusted
- **WHEN** a citation is resolved
- **THEN** the stored text is verified against the artifact's Markdown at the stored offsets before the citation is returned

#### Scenario: A stale citation fails rather than misleading
- **WHEN** a chunk's offsets no longer select its stored text in the artifact's Markdown
- **THEN** resolving that citation fails with an error instead of returning text from the wrong place

#### Scenario: A heading path with no headings still resolves
- **WHEN** a chunk lies in a document with no headings at all
- **THEN** its citation resolves with an empty heading path and still names the artifact and position

### Requirement: Cited text is displayed as untrusted text
Chunk text and search snippets SHALL be displayed as literal text. They SHALL NOT be rendered as markup or interpreted as instructions.

#### Scenario: Markup in a snippet is not rendered
- **WHEN** a search result's text contains script tags or HTML
- **THEN** it is displayed as literal characters
