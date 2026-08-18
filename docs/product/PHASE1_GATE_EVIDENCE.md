# Phase 1 — Windows platform gate evidence

**Status:** NOT RUN — awaiting a run on the target Windows 11 x64 laptop.

Phase 2 does not start until every gate below reads PASS, or a failed gate has a
recorded replacement implementation choice inside the PRD, or the PRD is
explicitly reopened.

## How to produce this record

```
just sidecar                  # Windows: build the pinned MarkItDown package
set TH_SIDECAR_EXE=...\build\sidecar\dist\markitdown-sidecar\markitdown-sidecar.exe
just gate                     # Windows-native proofs
just gate-model               # live Ollama proofs (override TH_INSTRUCT_MODELS / TH_EMBED_MODELS)
```

Every measured value is printed by the tests on lines beginning `EVIDENCE`.
Copy them into the tables below.

## Machine

| Field | Value |
| --- | --- |
| Machine | _model / CPU / RAM / GPU_ |
| OS build | _`winver`_ |
| Go version | _`go version`_ |
| Ollama version | _`ollama --version`_ |
| Date | _run date_ |

## Gate results

| Gate | Result | Notes |
| --- | --- | --- |
| FTS5 lifecycle (create/insert/MATCH/delete/rebuild) | _PASS / FAIL_ | |
| Sidecar: PDF → Markdown | _PASS / FAIL_ | |
| Sidecar: DOCX → Markdown | _PASS / FAIL_ | |
| Sidecar: corrupt input fails cleanly | _PASS / FAIL_ | |
| Containment: timeout kills tree | _PASS / FAIL_ | |
| Containment: grandchild killed | _PASS / FAIL_ | |
| Containment: memory limit | _PASS / FAIL_ | |
| Containment: output limit | _PASS / FAIL_ | |
| Containment: parent healthy after each failure | _PASS / FAIL_ | |
| Volume encryption: encrypted / unencrypted | _PASS / FAIL_ | |
| Volume encryption: unavailable / permission-denied distinct, never `encrypted` | _PASS / FAIL_ | |
| Credential round-trip, secret absent from db and logs | _PASS / FAIL_ | |
| Ollama chat | _PASS / FAIL_ | |
| Ollama constrained JSON | _PASS / FAIL_ | |
| Ollama embeddings | _PASS / FAIL_ | |

## Phase 6 gate results

Added by Phase 6; same run, same command (`just gate`). Everything about
extraction that can be proven without Windows is a plain unit test and already
passes; these are the four that cannot be.

| Gate | Result | Notes |
| --- | --- | --- |
| Golden DOCX keeps headings, list, table, and Unicode | _PASS / FAIL_ | |
| Golden PDF extracts through the packaged sidecar | _PASS / FAIL_ | |
| Corrupt PDF becomes a retryable code carrying no document text | _PASS / FAIL_ | |
| Staging directory denies Everyone / Users / Authenticated Users | _PASS / FAIL_ | paste the `icacls` evidence line |
| Plugins flag exists and is never passed; no fetched markup in output | _PASS / FAIL_ | |

## Phase 8 gate results

Added by Phase 8; same run, same command (`just gate`). The service's own rules
are proven everywhere against an in-memory store; these two need the real
Credential Manager, because there is deliberately no other store to test.

| Gate | Result | Notes |
| --- | --- | --- |
| Credential service: store, replace, revoke, missing entry | _PASS / FAIL_ | |
| Stored credential absent from the database, data folder, and a folder copy | _PASS / FAIL_ | paste the `EVIDENCE` line |

## Model measurements

| Model | Role | Digest | Wall clock | Resident bytes | VRAM bytes | Embedding dims |
| --- | --- | --- | --- | --- | --- | --- |
| | instruct | | | | | n/a |
| | embedding | | | | | |

Conditions (other load, power profile, thermal state): _record here — these
numbers guide model selection, not acceptance thresholds; Phase 21 owns
benchmarks._

## Selected models

| Role | Model | Digest | Why |
| --- | --- | --- | --- |
| instruct | | | |
| embedding | | | |

## Sidecar pin

See `build/sidecar/PIN.md` — MarkItDown version, Python version, package
SHA-256.

## Failed gates and consequences

_For each failure: the gate, what failed, and either the replacement
implementation choice within the PRD or the explicit PRD reopen request._

None recorded yet.
