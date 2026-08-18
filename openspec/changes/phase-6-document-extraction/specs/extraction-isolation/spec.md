## ADDED Requirements

### Requirement: The sidecar is verified before any document is materialized
The application SHALL verify at startup that the configured sidecar path is absolute, exists, is a regular file, and reports the pinned version. An extraction that needs the sidecar SHALL confirm that verified path is still present before it writes any document byte to disk, and SHALL fail with a reason code when verification did not succeed.

#### Scenario: A relative path is rejected
- **WHEN** the sidecar is configured with a relative path
- **THEN** verification fails, and extraction of a sidecar-requiring artifact fails with `sidecar_missing` without creating a directory or writing a file

#### Scenario: A missing sidecar is rejected
- **WHEN** the configured sidecar path does not exist
- **THEN** verification fails and extraction of a sidecar-requiring artifact fails with `sidecar_missing`

#### Scenario: A version mismatch is rejected
- **WHEN** the sidecar reports a version other than the pinned one
- **THEN** verification fails and extraction of a sidecar-requiring artifact fails with `sidecar_version`

#### Scenario: A sidecar substituted after startup is rejected
- **WHEN** the verified sidecar is removed or replaced after startup and an extraction is requested
- **THEN** the extraction fails before the document is written to disk

#### Scenario: A broken sidecar does not stop the application
- **WHEN** verification fails at startup
- **THEN** the application still runs and every other capability is unaffected

#### Scenario: Native text extraction needs no sidecar
- **WHEN** verification fails at startup and a plain text artifact is extracted
- **THEN** the extraction succeeds, because it never needed the sidecar

### Requirement: The temporary copy is anonymous and current-user-only
The input handed to the sidecar SHALL be materialized in a randomly named directory inside the data folder, readable only by the current user. Neither the directory name nor the file name SHALL contain a candidate name, a display name, or the original filename.

#### Scenario: The staging path carries no identity
- **WHEN** an artifact with a candidate's name in its filename and display name is extracted
- **THEN** neither name appears anywhere in the path handed to the sidecar

#### Scenario: Two extractions do not collide
- **WHEN** the same artifact is extracted twice
- **THEN** each run uses its own directory

#### Scenario: The staging directory lives in the encrypted data folder
- **WHEN** a staging directory is created
- **THEN** it is inside the data folder that holds the database, not the system temporary folder

#### Scenario: Another local user cannot read the staging directory
- **WHEN** a staging directory exists during an extraction
- **THEN** an ordinary local user other than the current one cannot read it

### Requirement: Temporary plaintext does not outlive its run
A staging directory SHALL be removed when its extraction ends, whatever the outcome, and any staging directory found at startup SHALL be removed before extraction is offered.

#### Scenario: A successful run leaves nothing
- **WHEN** an extraction succeeds
- **THEN** its staging directory no longer exists

#### Scenario: A failed run leaves nothing
- **WHEN** an extraction fails, times out, or panics
- **THEN** its staging directory no longer exists

#### Scenario: A cancelled run leaves nothing
- **WHEN** an extraction is cancelled while the sidecar is running
- **THEN** its staging directory no longer exists

#### Scenario: A crash costs one restart
- **WHEN** the application starts and finds staging directories left by a previous process
- **THEN** all of them are removed, and nothing outside the staging root is touched

### Requirement: The sidecar runs contained, once per file
The sidecar SHALL be invoked by its verified absolute path with one input file per process, with plugins and network-dependent features disabled, under a wall-clock timeout, a process-tree memory limit, and an output limit. A violation of any limit SHALL terminate the process tree and record a retryable failure.

#### Scenario: A hanging sidecar is terminated
- **WHEN** the sidecar does not exit within its time limit
- **THEN** its process tree is terminated and the extraction fails with `extract_timeout`

#### Scenario: A flooding sidecar is terminated
- **WHEN** the sidecar writes more than the output limit to standard output
- **THEN** its process tree is terminated and the extraction fails with `extract_output`

#### Scenario: A memory-hungry sidecar is terminated
- **WHEN** the sidecar's process tree exceeds its memory limit
- **THEN** it is terminated and the extraction fails with `extract_memory`

#### Scenario: A spawned child does not outlive the run
- **WHEN** the sidecar spawns a child process and the run ends
- **THEN** no descendant of the sidecar is still running

#### Scenario: The parent survives every failure
- **WHEN** an extraction fails in any of these ways
- **THEN** the application remains healthy and the next extraction succeeds

#### Scenario: Plugins and network features stay off
- **WHEN** the packaged sidecar is invoked
- **THEN** its plugins are disabled and no network-dependent converter runs
