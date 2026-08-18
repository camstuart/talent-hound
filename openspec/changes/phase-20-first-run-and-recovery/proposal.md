## Why

Every phase so far assumed the application was already running on a machine that was already set up. This one removes that assumption. The recruiter this product is for works alone, on one Windows laptop, with no IT support: whatever cannot be done self-service cannot be done at all.

Three things follow from that. Setup has to be ordered and resumable, because a wizard that loses its place after a failed model pull is a wizard that gets abandoned halfway. The encryption gate has to be checked at every startup rather than once at install, because a data folder can move to an unencrypted volume long after first run. And recovery has to refuse loudly, because a partially recovered database that opens anyway destroys the recruiter's only copy — the failure mode that matters is not "recovery didn't work", it is "recovery appeared to work".

Diagnostics are the same problem in miniature. A diagnostic report is the one artifact deliberately built to be shown to someone else, which makes it the one place where candidate details would leak by design rather than by accident. It is built from state the application already knows — versions, availability, counts — and never from content.

## What Changes

- Add an ordered, resumable first-run flow: data folder, encryption, sidecar, Ollama, models, acknowledgement, first initiative.
- Check volume encryption at every startup, not only at first run, and block real-data mode whenever it is not encrypted, unknown, or unavailable.
- Add an optional demo scope that works on an unencrypted volume and refuses artifacts and personal-data entry in code.
- Report required models with their download sizes and let missing ones be pulled, with a declined or failed pull leaving setup resumable.
- Add redacted local diagnostics, an open-logs-folder action, and the application version.
- Add a delete-all action that names the exact resolved folder and requires confirmation.
- Document and verify the closed-app folder copy, integrity check, snapshot, migration, restore-on-failure, credential re-entry, and model re-download path.
- Keep the active initiative, data scope, selected models, cloud override, and connectivity visible while working.
- Ship no telemetry endpoint, SDK, background reporter, or opt-in control.

## Capabilities

### New Capabilities
- `first-run`: the ordered steps, what each one blocks, and what resuming means.
- `data-scope`: real versus demo, the startup encryption gate, and what demo refuses.
- `diagnostics`: what a diagnostic report contains, what it can never contain, and the absence of telemetry.
- `folder-recovery`: opening a copied data folder, and the failures that must never open it.
- `operating-state`: what stays visible while working.

## Impact

- New `internal/setup/` — the step order, the gate results, and the required-model table, all pure so the wizard state is a function of what is true rather than a stored cursor.
- New `setupservice.go` — the folder choice, the checks, the pull, the acknowledgement, and the scope.
- New `diagnosticsservice.go` — the redacted report, the logs folder, the version, and delete-all.
- `internal/db/` — recovery of a copied folder over the existing integrity, snapshot, and restore behaviour.
- `frontend/src/components/FirstRunWizard.tsx` and a status strip in the shell.
- Fixtures containing secrets, candidate details, queries, payloads, draft content, and control characters, asserted absent from every diagnostic report.
- Windows-only installer, Defender/SmartScreen, and packaged-sidecar checks are recorded as gate evidence; they cannot run on this development machine.
