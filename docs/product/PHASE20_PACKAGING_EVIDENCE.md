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

## What is checked from here

These run in `just check` on any machine, so the Windows-only code cannot break
silently between now and the day the laptop is available.

| Check | Result |
| --- | --- |
| Every package vets for `GOOS=windows` | PASS — in `just vet-gates` |
| The desktop binary cross-compiles for Windows | PASS — 26 MB |
| The server binary cross-compiles for Windows | PASS — 26 MB |
| Windows-only sources compile: credential store, BitLocker check, job objects | PASS — they are skipped by filename on this host, which is why the cross-build is in the routine run |
| The installer's identity is the product's own | PASS — it shipped as "My Product" by "My Company" at version 0.0.1 until this was checked |
| The installer's version matches what the application reports | PASS — a diagnostic report that disagrees with Add/Remove Programs is one nobody can act on |
| The uninstaller removes no AppData folder but the WebView2 one | PASS — on Windows the data folder is `%AppData%\talent-hound` and the WebView2 directory is `%AppData%\talent-hound.exe`, four characters apart in the same parent |
| The uninstaller says where the data folder is | PASS — printed to the log, and shown when the uninstall is not silent, along with the credential store |
| The sidecar pin matches what the application demands | PASS — `requirements.txt`, `PIN.md`, and `PinnedSidecarVersion` are one version; a drift would have the packaged reader refused on first run |
| Every `just` recipe the pin record names exists | PASS — it named `just sidecar-digest`, which does not exist |

What this does not check is anything about running: linking is not launching,
and every row below still needs the machine.

## Installer

| Check | Result | Notes |
| --- | --- | --- |
| Installs without administrator rights, or states what it needs | NOT RUN | |
| Launches from the Start menu after install | NOT RUN | |
| WebView2 present, or the installer provisions it | NOT RUN | |
| Sidecar is installed beside the application at the expected path | NOT RUN | |
| Sidecar reports the pinned version from the packaged build | NOT RUN | |
| Upgrade over the previous build keeps the data folder | NOT RUN | |
| Uninstall removes the application and leaves the data folder | NOT RUN — the script is checked, the behaviour is not |
| Uninstall documents where the data folder is | NOT RUN — the message exists and is pinned by a test; nobody has read it on Windows |
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
