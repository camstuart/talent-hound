# Phase 20 — Windows packaging, installer, and offline evidence

**Status:** NOT RUN — every check below needs the target Windows 11 x64 laptop.

These are the parts of Phase 20 that cannot be produced on a development
machine. A green suite on macOS is not evidence about Windows, and is not
recorded here as if it were. Everything else in Phase 20 — the wizard state, the
scope gate, the diagnostics content, the recovery pre-checks, delete-all — is
covered by the Go, Vitest, and Playwright suites and runs everywhere.

## How to produce this record

```
just sidecar                   # build the pinned MarkItDown one-dir sidecar
wails3 task package            # build the Windows application and installer
just gate                      # Windows-native platform proofs
just check                     # the full suite on the target machine
```

Then work through the tables below on the packaged build, not on `just dev`.

## Installer

| Check | Result | Notes |
| --- | --- | --- |
| Installs without administrator rights, or states what it needs | NOT RUN | |
| Launches from the Start menu after install | NOT RUN | |
| WebView2 present, or the installer provisions it | NOT RUN | |
| Sidecar is installed beside the application at the expected path | NOT RUN | |
| Sidecar reports the pinned version from the packaged build | NOT RUN | |
| Upgrade over the previous build keeps the data folder | NOT RUN | |
| Uninstall removes the application and leaves the data folder | NOT RUN | |
| Uninstall documents where the data folder is | NOT RUN | |
| Reinstall over a retained data folder opens it | NOT RUN | |

## Defender and SmartScreen

| Check | Result | Notes |
| --- | --- | --- |
| Defender does not quarantine the application or the sidecar | NOT RUN | |
| SmartScreen behaviour on first launch of an unsigned build | NOT RUN | |
| Whether code signing is required for a usable first run | NOT RUN | |
| The packaged one-dir sidecar launches from its installed path | NOT RUN | |

## First run on a clean machine

| Check | Result | Notes |
| --- | --- | --- |
| BitLocker on: real scope is available | NOT RUN | |
| BitLocker off: real scope is blocked, and says so | NOT RUN | |
| `manage-bde` unavailable: the CIM fallback answers | NOT RUN | |
| Neither available: blocked as "could not check", not as encrypted | NOT RUN | |
| Missing Ollama is named at its own step | NOT RUN | |
| Model pull runs, and a declined pull leaves setup resumable | NOT RUN | |
| The provider key is stored in the Windows credential store only | NOT RUN | |

## Offline native run

Aeroplane mode, after the models are installed.

| Check | Result | Notes |
| --- | --- | --- |
| CRM records create, edit, and list | NOT RUN | |
| Artifacts ingest and extract | NOT RUN | |
| Profiles classify and approve | NOT RUN | |
| Retrieval and Q&A answer with citations | NOT RUN | |
| Local generation writes a draft | NOT RUN | |
| No outbound request is observed | NOT RUN | |
| No telemetry request is observed | NOT RUN | |

## Recovery on a second machine

| Check | Result | Notes |
| --- | --- | --- |
| Folder copied while the application was fully closed opens | NOT RUN | |
| Integrity and schema-version checks run before it opens | NOT RUN | |
| A migration takes a snapshot first | NOT RUN | |
| Credentials are re-entered, and the data is intact without them | NOT RUN | |
| Missing models are reported as a recovery step, not data loss | NOT RUN | |

## Failed gates and consequences

_For each failure: the gate, what failed, and either the replacement
implementation choice within the PRD or the explicit PRD reopen request._

None recorded yet.
