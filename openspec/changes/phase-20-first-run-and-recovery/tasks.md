## 1. Setup state

- [x] 1.1 `internal/setup/`: the ordered steps and the computed state
- [x] 1.2 The remembered pointer — data folder, acknowledgement, scope — outside the data folder
- [x] 1.3 Required-model table with names and approximate download sizes
- [x] 1.4 `setupservice.go`: folder choice, checks, pull, acknowledgement, scope

## 2. Gates

- [x] 2.1 Encryption checked at every startup, not only at first run
- [x] 2.2 Unencrypted, unavailable, and permission-denied each block real scope with their own reason
- [x] 2.3 Demo scope refusing artifacts and candidates at the service boundary
- [x] 2.4 Sidecar presence and pinned version; Ollama reachability, each named on failure

## 3. Diagnostics

- [x] 3.1 `diagnosticsservice.go`: report from versions, paths, availability, counts, and job codes
- [x] 3.2 Open-logs-folder action reporting the resolved path
- [x] 3.3 Application version, matching the report
- [x] 3.4 Delete-all naming the exact resolved folder and requiring its confirmation

## 4. Recovery

- [x] 4.1 Pre-checks before any write: folder exists, is writable, holds a database
- [x] 4.2 Recovery over the existing integrity, snapshot, migration, and restore path
- [x] 4.3 In-app recovery documentation naming the resolved folder

## 5. Frontend

- [x] 5.1 `FirstRunWizard.tsx`: the ordered steps, each blocking, each resumable
- [x] 5.2 A status strip: initiative, scope, models, cloud override, connectivity
- [x] 5.3 Diagnostics, version, logs folder, recovery documentation, and delete-all in settings

## 6. Tests

- [x] 6.1 Wizard state over fresh install, cancellation at each step, restart, missing sidecar, missing Ollama, declined pull, failed pull, and the acknowledgement
- [x] 6.2 Encrypted permits; unencrypted, unknown, and check-failed block at first run and at later startup
- [x] 6.3 Demo scope rejects artifacts and personal-data entry
- [x] 6.4 Diagnostic fixtures with secrets, candidate details, queries, payloads, draft content, and control characters stay absent
- [x] 6.5 Recovery: healthy copy opens; corrupt, failed-integrity, failing-migration, read-only, no-database, and future-schema never open or overwrite
- [x] 6.6 Delete-all against temporary folders only, matched and mismatched confirmation
- [x] 6.7 No telemetry endpoint, SDK, reporter, or control anywhere in the repository
- [x] 6.8 Vitest over the wizard and the status strip
- [x] 6.9 Playwright over first run, blocked real scope, diagnostics, and delete-all
- [x] 6.10 Fixtures are synthetic only — no real candidate information anywhere

## 7. Windows-only gates

- [x] 7.1 (recorded NOT RUN — needs the Windows laptop) Installer smoke: launch, sidecar path and version, WebView2, upgrade, uninstall, reinstall — recorded as evidence
- [x] 7.2 (recorded NOT RUN — needs the Windows laptop) Defender/SmartScreen and packaged one-dir sidecar — recorded as evidence
- [x] 7.3 (recorded NOT RUN — needs the Windows laptop) Offline native run over CRM, artifacts, profiles, retrieval, Q&A, and local generation — recorded as evidence

## 8. Exit gate

- [x] 8.1 (packaging and installer parts recorded NOT RUN) A recruiter can install, start securely, recover a copied folder, work offline, inspect diagnostics, and uninstall without hidden data loss
- [x] 8.2 `just check` passes
