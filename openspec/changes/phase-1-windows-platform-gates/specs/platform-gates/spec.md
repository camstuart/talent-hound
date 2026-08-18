## ADDED Requirements

### Requirement: FTS5 availability in the CGO-free SQLite build
The application SHALL prove that the resolved pure-Go SQLite build supports FTS5 create, insert, query, delete, and rebuild against a disk-backed database on Windows 11 x64.

#### Scenario: FTS5 lifecycle succeeds
- **WHEN** the FTS5 probe creates a virtual table, inserts rows, runs a MATCH query, deletes a row, and runs `rebuild`
- **THEN** every statement succeeds and the MATCH results reflect the inserts and the deletion

#### Scenario: FTS5 unavailable
- **WHEN** the SQLite build does not provide FTS5
- **THEN** the probe fails with an error naming FTS5 and the driver build, and the gate is reported as failed rather than skipped

### Requirement: Packaged MarkItDown sidecar extraction
The application SHALL invoke the pinned MarkItDown PyInstaller one-dir sidecar by verified absolute path, one file per process, with plugins and network features disabled, and obtain Markdown for PDF and DOCX inputs.

#### Scenario: PDF and DOCX convert to Markdown
- **WHEN** the packaged sidecar is invoked on a synthetic PDF and a synthetic DOCX fixture
- **THEN** each invocation exits zero and emits non-empty Markdown containing the fixture's known text

#### Scenario: Sidecar version is pinned and recorded
- **WHEN** the sidecar package is built
- **THEN** the MarkItDown version, Python version, and package digest are recorded in the repository

#### Scenario: Unsupported or corrupt input
- **WHEN** the sidecar is invoked on a corrupt file
- **THEN** the invocation returns a non-zero exit and a diagnostic message, and the parent process remains healthy

### Requirement: Windows Job Object containment of the sidecar
The application SHALL run each sidecar invocation inside a Windows Job Object that enforces a wall-clock timeout, a memory limit, an output-size limit, and kill-on-close process-tree termination.

#### Scenario: Hanging process is terminated on timeout
- **WHEN** a fake sidecar sleeps past the configured timeout
- **THEN** the process tree is terminated, a retryable timeout failure is returned, and no orphan process remains

#### Scenario: Child processes are killed with the parent
- **WHEN** a fake sidecar spawns a long-lived grandchild process and the job is terminated
- **THEN** both the child and the grandchild are gone after termination

#### Scenario: Memory limit is enforced
- **WHEN** a fake sidecar allocates beyond the configured memory limit
- **THEN** the process tree is terminated and a memory-limit failure is returned

#### Scenario: Oversized output is bounded
- **WHEN** a fake sidecar writes more than the configured output limit to stdout
- **THEN** reading stops at the limit, the process tree is terminated, and an output-limit failure is returned

#### Scenario: Parent survives every containment failure
- **WHEN** any containment failure above occurs
- **THEN** the parent process continues running and can perform a subsequent successful invocation

### Requirement: Volume encryption detection
The application SHALL determine whether the volume holding a given path is protected by BitLocker or Windows Device Encryption, and SHALL distinguish encrypted, unencrypted, unavailable, and permission-denied outcomes.

#### Scenario: Encrypted volume
- **WHEN** the probe inspects a volume with BitLocker or Device Encryption enabled
- **THEN** it reports `encrypted`

#### Scenario: Unencrypted volume
- **WHEN** the probe inspects a volume with protection off
- **THEN** it reports `unencrypted`

#### Scenario: Status unavailable
- **WHEN** the encryption status cannot be determined (service or API unavailable)
- **THEN** it reports `unavailable` and never reports `encrypted`

#### Scenario: Permission denied
- **WHEN** the query is rejected for lack of privilege
- **THEN** it reports `permission-denied` distinctly from `unavailable`, and never reports `encrypted`

### Requirement: Windows Credential Manager secret storage
The application SHALL create, read, and remove a secret in Windows Credential Manager under an application-scoped target name, and SHALL keep the secret out of the database, logs, and diagnostics.

#### Scenario: Credential round-trip
- **WHEN** the probe writes a secret, reads it back, then deletes it
- **THEN** the read returns the exact secret and the subsequent read reports not-found

#### Scenario: Secret never leaks
- **WHEN** the round-trip completes
- **THEN** neither the database file, the log output, nor the recorded evidence contains the secret value

### Requirement: Local Ollama model capability
The application SHALL prove that the selected local Ollama models serve OpenAI-compatible chat, constrained-JSON chat, and embeddings at `http://localhost:11434/v1` on the target laptop.

#### Scenario: Chat completion
- **WHEN** the probe issues a chat completion to the selected instruct model
- **THEN** a non-empty assistant message is returned

#### Scenario: Constrained JSON output
- **WHEN** the probe requests a response constrained to a JSON schema
- **THEN** the response parses as JSON and validates against that schema

#### Scenario: Embeddings
- **WHEN** the probe embeds a fixture string with the selected embedding model
- **THEN** a float32 vector is returned and its dimension count is recorded

#### Scenario: Model identity is captured
- **WHEN** any model probe succeeds
- **THEN** the model name, digest or immutable revision, and endpoint configuration revision are recorded with the result

### Requirement: Recorded gate evidence
The phase SHALL produce a committed evidence record stating, for each gate, pass or fail, the machine and OS build, measured wall-clock and peak memory for model probes, and the resulting model selection.

#### Scenario: Evidence produced on a full Windows run
- **WHEN** the Windows-native and live-Ollama suites are run on the target laptop
- **THEN** the evidence record is updated with per-gate results, timings, memory, and selected models

#### Scenario: A gate fails
- **WHEN** any gate fails
- **THEN** the evidence record names the failed gate and either the replacement implementation choice within the PRD or an explicit request to reopen the PRD

### Requirement: Gate suites are separable from routine tests
Native and live-service proofs SHALL be excluded from the default test run so that routine suites pass on any platform without network access.

#### Scenario: Routine run on a non-Windows machine without network
- **WHEN** `just check` runs on macOS or Linux with no network access
- **THEN** it passes and the Windows-native and live-Ollama proofs are not executed

#### Scenario: Gate run is explicitly invoked
- **WHEN** the Windows-native and live-provider labels are invoked on the target laptop
- **THEN** all gate proofs execute and report per-gate pass or fail
