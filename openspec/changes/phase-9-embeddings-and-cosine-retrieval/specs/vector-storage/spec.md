## ADDED Requirements

### Requirement: Vectors are stored as little-endian float32
The application SHALL serialize embedding vectors as little-endian IEEE-754 float32 values with no header, framing, or padding, so that a vector of N dimensions occupies exactly 4N bytes. Deserialization SHALL reproduce the original values bit for bit.

#### Scenario: Known bit patterns round-trip
- **WHEN** a vector containing zero, negative zero, the smallest normal, the largest finite float32, and ordinary values is serialized and deserialized
- **THEN** every element is identical to the original, bit for bit

#### Scenario: Byte length matches the dimension count
- **WHEN** a vector of N dimensions is serialized
- **THEN** the resulting blob is exactly 4N bytes long

### Requirement: Malformed or mismatched vectors are refused
Deserialization SHALL refuse a blob whose length is not a multiple of four, and SHALL refuse a blob whose element count differs from the dimension count of the embedding space it is being read for. Storage SHALL refuse a vector whose element count differs from its space's dimensions.

#### Scenario: A truncated blob is refused
- **WHEN** a blob whose length is not a multiple of four is deserialized
- **THEN** it is refused with an error and no vector is produced

#### Scenario: A wrong-dimension vector is refused on read
- **WHEN** a blob of the wrong element count is read for a space of a given dimension count
- **THEN** it is refused with an error naming both counts

#### Scenario: A wrong-dimension vector is refused on write
- **WHEN** a vector whose length differs from its space's dimensions is offered for storage
- **THEN** it is refused and nothing is written

### Requirement: Degenerate vectors are refused at storage
The application SHALL refuse to store a vector containing a NaN or infinite component, and SHALL refuse to store a vector whose components are all zero. Such a response from an endpoint SHALL be treated as a failure of that embedding rather than as a vector.

#### Scenario: A non-finite component is refused
- **WHEN** an endpoint returns a vector containing NaN or infinity
- **THEN** the embedding fails with a coded reason and no vector is stored

#### Scenario: An all-zero vector is refused
- **WHEN** an endpoint returns a vector whose components are all zero
- **THEN** the embedding fails with a coded reason and no vector is stored

### Requirement: A retrieval unit is a kind and an identifier
The application SHALL store each vector against a retrieval unit identified by a kind and an identifier, so that source chunks and other embeddable kinds share one storage shape. At most one vector SHALL exist for a given space, kind, and identifier.

#### Scenario: Re-embedding replaces rather than duplicates
- **WHEN** a retrieval unit already embedded in a space is embedded again
- **THEN** the existing vector is replaced and exactly one vector exists for that space, kind, and identifier

#### Scenario: Distinct kinds do not collide
- **WHEN** two retrieval units of different kinds share an identifier value
- **THEN** both vectors are stored and neither replaces the other

### Requirement: Vectors do not outlive their source
When the retrieval units derived from an artifact are replaced or removed, the application SHALL delete the vectors of the removed units in the same transaction.

#### Scenario: Re-chunking discards the old vectors
- **WHEN** an artifact whose chunks are embedded is chunked again
- **THEN** the vectors of the replaced chunks are deleted in the same transaction that replaces them
- **AND** no vector remains that names a chunk that no longer exists

#### Scenario: Re-extraction discards derived vectors
- **WHEN** an artifact's Markdown is replaced by a new extraction
- **THEN** its chunks and their vectors are both gone once the transaction commits

### Requirement: An interrupted embedding leaves nothing behind
A cancelled or failed embedding SHALL leave no vector for the retrieval unit it was working on. Failure reasons SHALL be short lowercase codes carrying no document content.

#### Scenario: Cancellation leaves completed units only
- **WHEN** a batch embedding job over several units is cancelled partway
- **THEN** the units already committed have vectors, the remaining units have none, and the unit in flight has none

#### Scenario: A provider failure writes nothing
- **WHEN** the endpoint fails while embedding a unit
- **THEN** the job records a coded reason and no vector exists for that unit

#### Scenario: A failure reason carries no content
- **WHEN** an embedding fails for any reason
- **THEN** the recorded reason is a short lowercase code and contains no part of the embedded text
