## MODIFIED Requirements

### Requirement: Extraction state and provenance are recorded on the artifact
An artifact SHALL record its extraction state as exactly one of pending, extracted, or failed, together with the name and version of the extractor that produced its Markdown and, when it failed, a reason code. Extracted Markdown SHALL be limited to 10 MB. Rows derived from an artifact's Markdown SHALL be discarded in the same transaction that writes new Markdown, so nothing derived outlives the text it came from.

#### Scenario: A newly ingested artifact is pending
- **WHEN** an artifact is ingested
- **THEN** its extraction state is pending, its Markdown is empty, and it records no extractor

#### Scenario: A successful extraction records its extractor
- **WHEN** an artifact is extracted
- **THEN** its state is extracted, its Markdown is stored, and the extractor name and version that produced it are recorded

#### Scenario: A native text extraction names itself
- **WHEN** a plain text artifact is extracted without the sidecar
- **THEN** the recorded extractor identifies the native text path rather than the sidecar

#### Scenario: Output over the cap is a failure, not a truncation
- **WHEN** an extraction would produce more than 10 MB of Markdown
- **THEN** the extraction fails with the code `extract_output` and no partial Markdown is stored

#### Scenario: A retry clears the previous outcome
- **WHEN** extraction is retried for an artifact that previously failed
- **THEN** the artifact returns to pending with no reason code before the new attempt records its result

#### Scenario: Re-extraction discards the chunks of the previous Markdown
- **WHEN** an artifact that has been chunked is extracted again
- **THEN** its existing chunks are removed as part of recording the new extraction state, and none of them can be searched or cited afterwards
