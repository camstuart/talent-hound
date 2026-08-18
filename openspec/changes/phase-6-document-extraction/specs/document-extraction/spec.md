## ADDED Requirements

### Requirement: Supported artifacts are converted to Markdown
An artifact whose media type is PDF or DOCX SHALL be converted to Markdown by the bundled sidecar, one file per process. An artifact whose media type is plain text or Markdown SHALL be converted without invoking the sidecar. Any other media type SHALL fail extraction with a code saying the type is unsupported.

#### Scenario: A PDF is converted through the sidecar
- **WHEN** extraction runs for an artifact whose bytes are a PDF
- **THEN** the sidecar is invoked once for that one file and its Markdown is stored against the artifact

#### Scenario: Plain text is converted without a subprocess
- **WHEN** extraction runs for an artifact whose media type is plain text
- **THEN** its own bytes become its Markdown, no process is started, and no file is written outside the database

#### Scenario: Pasted Markdown is converted without a subprocess
- **WHEN** extraction runs for an artifact that was pasted as Markdown
- **THEN** its own bytes become its Markdown and no process is started

#### Scenario: An unsupported type is refused
- **WHEN** extraction runs for an artifact whose media type is neither text, Markdown, PDF, nor DOCX
- **THEN** the artifact's extraction fails with the code `unsupported_type` and no process is started

### Requirement: Extraction state and provenance are recorded on the artifact
An artifact SHALL record its extraction state as exactly one of pending, extracted, or failed, together with the name and version of the extractor that produced its Markdown and, when it failed, a reason code. Extracted Markdown SHALL be limited to 10 MB.

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

### Requirement: Extraction failures carry no document content
An extraction failure SHALL be recorded as a short lowercase code from a fixed vocabulary. The sidecar's own error output SHALL NOT be stored on the artifact, written to the job record, or logged, because a document parser's errors quote the document.

#### Scenario: A timeout is recorded as a code
- **WHEN** the sidecar exceeds its time limit
- **THEN** the artifact records `extract_timeout` and the sidecar's error output is not stored anywhere

#### Scenario: A non-zero exit is recorded as a code
- **WHEN** the sidecar exits non-zero with a message on standard error naming text from the document
- **THEN** the artifact records `extract_failed` and no part of that message is stored or logged

#### Scenario: Empty output is distinguishable
- **WHEN** the sidecar succeeds but produces no Markdown, as a scanned or image-only PDF does
- **THEN** the artifact records `extract_empty`, which the recruiter can act on and retry

#### Scenario: Free text is refused as a reason
- **WHEN** an extraction reason that is not a short lowercase code is written to an artifact
- **THEN** the write is refused by the database rather than stored

### Requirement: Extracted Markdown is treated as untrusted text
Extracted Markdown SHALL be stored and displayed as text. It SHALL NOT be rendered as markup, executed, or interpreted as an instruction by any part of this phase.

#### Scenario: Markup in a document is not rendered
- **WHEN** an extraction contains script tags, HTML, or terminal control characters
- **THEN** they are stored exactly as extracted and displayed as literal text

#### Scenario: An instruction in a document is only text
- **WHEN** an extraction contains text addressed to a language model
- **THEN** it is stored as text and this phase gives it to no model

### Requirement: Extraction runs as a background job
Extraction SHALL run through the background job lifecycle, one artifact per job, so that it is cancellable, retryable, and honest after a crash without a second mechanism.

#### Scenario: Extraction is enqueued rather than run inline
- **WHEN** extraction is requested for an artifact
- **THEN** a job is enqueued for it and the request returns without waiting for the conversion

#### Scenario: A cancelled extraction stores nothing
- **WHEN** an extraction job is cancelled while the conversion is in progress
- **THEN** the artifact is left pending with no Markdown and no reason code

#### Scenario: An extraction interrupted by a crash can be retried
- **WHEN** the application restarts while an extraction job is running
- **THEN** that job is failed as interrupted, the artifact is still pending, and extraction can be requested again

#### Scenario: The recruiter can see where an extraction got to
- **WHEN** an artifact has been extracted, has failed, or has not been extracted
- **THEN** its state is visible in the workspace, with the reason code when it failed
