# Talent Hound PoC Project Plan

**Status:** Approved implementation plan for PRD v0.5

**Last updated:** 2026-08-16

**Product contract:** [Talent Hound Product Requirements Document](PRD.md)

## Goal

Deliver the Final v0.5 proof of concept as a sequence of small, independently testable phases. Every phase must leave the application in a releasable state, make failures visible, and close with automated tests plus any required Windows-native evidence.

This plan deliberately does not assign calendar estimates. Throughput is not yet measured, and the Windows sidecar, local-model, and Exa feasibility gates may change the effort without changing the approved product scope.

## Delivery rules

1. One phase is one reviewable pull request. Split a phase if it cannot be understood and tested in one review.
2. A phase is complete only when its exit gate passes. Code complete is not phase complete.
3. Add the smallest implementation that satisfies the current phase. Do not pre-build P1 extension points.
4. Test behavior at the lowest useful layer, then keep one real-backend end-to-end test for each critical user journey.
5. Use deterministic fakes for Ollama, Exa, cloud endpoints, time, and process failures in routine tests. Live services are reserved for explicit integration and acceptance runs.
6. Automated fixtures contain synthetic or anonymized data only. Real candidate information never enters source control, logs, screenshots, or test reports.
7. Windows-specific phases do not close on mocks alone. Their native checks must pass on the target Windows 11 x64 laptop or an equivalent Windows test machine.
8. `just check` is the minimum regression gate after every phase. A phase also runs the focused tests named below.
9. A failing migration, extraction, model call, external request, cancellation, or deletion must leave data consistent and expose a useful retryable or terminal state.
10. Update this plan's traceability table if delivery reveals a requirement is covered by a different phase. Product scope changes require a PRD revision.

## Test architecture

The existing three test layers remain the default:

| Layer | Purpose | Normal command |
| --- | --- | --- |
| Go tests | Domain rules, services, SQL migrations, retrieval, process control, provider clients, hashing, and failure paths | `just test-go` |
| Vitest | SolidJS state and components with mocked generated bindings | `just test-unit` |
| Playwright | Critical journeys through the Wails server build and the real Go backend | `just test-e2e` |
| Static and security checks | Go and TypeScript linting, type checks, security scanning, vulnerability scanning, and duplication checks | `just qa` |
| Full regression | All static checks and all three test layers | `just check` |

Additional suites introduced by this plan:

- **disk-backed SQLite integration tests:** migrations, FTS5, triggers, transactions, snapshots, restoration, and recovery;
- **provider contract tests:** local HTTP fakes that implement the exact OpenAI-compatible and Exa response shapes used by the application;
- **sidecar contract tests:** a controllable fake executable for routine failure tests plus the packaged MarkItDown binary on Windows;
- **golden document tests:** small PDF, DOCX, Markdown, plain-text, scanned-PDF, Unicode, malformed, and oversized-output fixtures;
- **retrieval oracle tests:** tiny corpora whose FTS, cosine, compatibility, fusion, and ranking results are known exactly;
- **security fixtures:** prompt injection, path manipulation, direct identifiers, prohibited criteria, malformed structured output, secret-shaped strings, and untrusted Markdown;
- **held-out benchmarks:** the frozen classifier and matching corpora defined by the PRD; and
- **target-laptop acceptance:** native Windows, BitLocker, Credential Manager, Job Object, Ollama, Exa, performance, offline, recovery, and installer checks.

Tests should use fake clocks, fixed model responses, stable IDs, and isolated temporary data folders. Network access is disabled by default. No routine test may depend on live Exa, Ollama, a cloud model, SEEK, LinkedIn, or another public website.

## Phase map

| Phase | Outcome | Primary requirements |
| --- | --- | --- |
| 0 | Trusted baseline and fixture kit | Delivery prerequisite |
| 1 | Windows platform risks proven | FR-04, FR-10, FR-11, FR-13 |
| 2 | Explicit migrations and safe database opening | FR-04, FR-13 |
| 3 | Initiative and minimal CRM records | FR-01, FR-02 |
| 4 | Immutable artifacts and link semantics | FR-03 |
| 5 | Durable, cancellable background jobs | FR-04, FR-05, FR-08 |
| 6 | Failure-isolated PDF and DOCX extraction | FR-04 |
| 7 | Deterministic chunking, citations, and FTS5 | FR-04 |
| 8 | Local model registry and credential foundation | FR-10, FR-11 |
| 9 | Local embeddings and exact-cosine retrieval | FR-04, FR-08 |
| 10 | Versioned profile and classifier contract | FR-05 |
| 11 | Approved Candidate Profiles | FR-05 |
| 12 | Automatic, editable Role Profiles | FR-05 |
| 13 | Search Criteria and prohibited-criteria controls | FR-08 |
| 14 | Exa role discovery and source lifecycle | FR-07 |
| 15 | Hybrid top-20 shortlist | FR-08 |
| 16 | Two-directional assessment and ranking | FR-08 |
| 17 | Scoped Q&A, drafts, and copy-out | FR-06, FR-09 |
| 18 | Optional cloud controls and disclosure audit | FR-10, FR-11 |
| 19 | Complete deletion invariants | FR-12 |
| 20 | First run, recovery, diagnostics, and packaging | FR-13 |
| 21 | Benchmarks and PoC acceptance | All P0 requirements |

## Phase 0 — Trusted baseline and fixture kit

**Outcome:** Preserve the working initiative shell and make every later phase cheap to test.

**Build:**

- Record a clean `just check` baseline.
- Add shared helpers for isolated disk-backed test databases and temporary data folders only where repeated setup already warrants them.
- Add deterministic fake-clock and local HTTP-provider fixtures.
- Add synthetic document, role, candidate, malicious-input, and error fixtures.
- Define test labels or commands for routine, Windows-native, live-provider, benchmark, and acceptance suites.

**Tests:**

- Existing Go, Vitest, and Playwright suites pass unchanged.
- Two simultaneous test runs use different databases and data folders.
- Test teardown proves no fixture data enters the normal application data location.
- Provider fakes reject unexpected requests and record only redacted diagnostic metadata.
- Fixture licensing and anonymization are documented.

**Exit gate:** `just check` passes twice from a clean checkout, and the test suite can run without network access.

## Phase 1 — Windows platform risk gates

**Outcome:** Prove the native mechanisms on which the PoC depends before building product flows around them.

**Build:**

- Create minimal executable proofs for FTS5 support in the resolved modernc SQLite build.
- Package and invoke the pinned MarkItDown PyInstaller one-dir sidecar on Windows.
- Prove Windows Job Object timeout, memory, and process-tree termination.
- Prove detection of BitLocker or Windows Device Encryption for the selected volume.
- Prove create/read/remove behavior in Windows Credential Manager.
- Prove the selected Ollama models expose the required OpenAI-compatible chat and embeddings behavior on the target laptop.

Do not build general platform abstractions in this phase. Keep only code or recorded decisions that the production path will use.

**Tests:**

- FTS5 table create, insert, query, delete, and rebuild pass on Windows.
- A valid PDF and DOCX produce non-empty Markdown through the packaged sidecar.
- Hanging, child-spawning, memory-hungry, and oversized-output fake sidecars are terminated and leave the parent healthy.
- Encrypted, unencrypted, unavailable, and permission-denied volume checks return distinct results.
- Credential round-trip succeeds and neither the database nor logs contain the secret.
- Chat, constrained JSON, and embedding calls succeed against Ollama; dimensions and model revision are captured.
- Target-laptop memory use and wall-clock results are recorded for initial model selection.

**Exit gate:** Every proof passes on Windows 11 x64. Any failed dependency is resolved by changing the implementation choice within the PRD, or the PRD is explicitly reopened before feature work continues.

## Phase 2 — Explicit migrations and safe database opening

**Outcome:** Replace `AutoMigrate` with a versioned, recoverable schema foundation.

**Build:**

- Add an ordered explicit SQL migration runner and schema-version table.
- Create a snapshot before each pending migration.
- Run SQLite integrity checks before recovery or migration.
- Restore the snapshot and refuse to open the folder when a migration fails.
- Add the startup FTS5 smoke test.
- Resolve the selected data-folder database path without changing the current database unexpectedly.

**Tests:**

- A new database migrates from zero to current.
- A database at each historical version migrates to current with its records intact.
- Reopening a current database is idempotent.
- Unknown future versions are rejected without writes.
- A deliberately failing migration restores a byte-valid, integrity-checked snapshot and leaves no partially applied schema.
- Snapshot creation failure, read-only folder, full-disk simulation, corrupt database, and interrupted migration produce safe errors.
- FTS5-unavailable startup fails visibly before personal data is accepted.
- Disk-backed concurrent-open behavior is defined and tested.

**Exit gate:** Migration and restore tests pass against real files, not only `:memory:`, and the existing initiative records survive an upgrade fixture.

## Phase 3 — Initiative and minimal CRM records

**Outcome:** Establish the shared records and exact initiative cardinality needed by the flagship loop.

**Build:**

- Complete initiative rename, archive, reopen, and delete behavior.
- Add Candidate, Role, Company, and Contact records with exactly the structured fields in the PRD.
- Require exactly one Candidate for a Job Search Initiative.
- Provide the minimal contacts-at-company count and listing.
- Keep Talent Search and Business Development as workspace shells.
- Add the Context, Research, Matches, and Drafts navigation skeleton without speculative pipeline behavior.

**Tests:**

- Table-driven Go tests cover every valid and invalid initiative type and lifecycle transition.
- Job Search creation rejects zero or multiple candidates.
- Structured field validation covers missing required values, Unicode, whitespace, dates, URLs, compensation boundaries, and optional contact details.
- Shared Candidate and Role records can be referenced from multiple initiatives without copying.
- Contacts-at-company returns only contacts linked to the selected Company and handles an empty result.
- Archiving and reopening preserve all references.
- Vitest covers form validation, keyboard operation, error display, and lifecycle labels.
- Playwright creates each initiative type, persists records through restart, archives and reopens a workspace, and confirms the four-area shell.

**Exit gate:** FR-01 and FR-02 work end to end with the real backend, excluding deletion rules deliberately completed in Phase 19.

## Phase 4 — Immutable artifacts and link semantics

**Outcome:** Store one evidence record per ingestion and expose safe link, rename, detach, and orphan behavior.

**Build:**

- Persist original bytes and immutable provenance metadata in SQLite.
- Keep editable display name separate from immutable original filename.
- Link artifacts to initiatives and records without copying bytes.
- Retain SHA-256 for integrity while intentionally not deduplicating equal bytes.
- Enforce the 25 MB input limit.
- Add the visible orphaned-artifact library.

**Tests:**

- Round-trip exact bytes for every supported native text type and arbitrary binary bytes.
- Two identical ingestions create two Artifact IDs with independent provenance.
- Boundary tests cover zero bytes, exactly 25 MB, and one byte over the limit.
- MIME/extension disagreement, Unicode filenames, path-like filenames, duplicate display names, and invalid metadata are handled safely.
- Editing display name never changes original filename, hash, source, or capture time.
- Detaching one link preserves bytes and other links; removing the last link produces a visible orphan.
- Transaction rollback leaves neither a half-created artifact nor a dangling link.
- Playwright uploads, views, renames, detaches, and finds an orphan through the real backend.

**Exit gate:** FR-03's storage and link behavior passes; global deletion remains disabled until Phase 19 can enforce every invariant.

## Phase 5 — Durable, cancellable background jobs

**Outcome:** Give every long-running pipeline one consistent lifecycle before those pipelines are introduced.

**Build:**

- Persist Queued, Running, Completed, Failed, and Cancelled jobs.
- Record total and completed item counts plus a redacted failure reason.
- Add cancellation and manual retry.
- Mark jobs found Running after restart as Failed with reason `interrupted`.
- Define per-item transaction boundaries so completed batch items survive cancellation.

**Tests:**

- A state-transition table rejects every illegal transition.
- Cancellation before start, during one item, between items, and after completion is deterministic.
- Completed per-item results survive batch cancellation; the active item rolls back.
- Restart converts Running to Failed/interrupted exactly once and permits retry.
- Repeated cancellation and retry requests are idempotent.
- Worker panic, process error, database error, and application shutdown do not leave Running jobs or partial current-item data.
- Vitest covers progress, completed/total counts, cancel, failure, and retry states.
- Playwright starts, cancels, restarts, and retries a controllable fake job.

**Exit gate:** All later slow operations can use this lifecycle without inventing another queue or partial state.

## Phase 6 — Failure-isolated PDF and DOCX extraction

**Outcome:** Convert supported documents through the pinned sidecar while protecting application availability and temporary data.

**Build:**

- Verify the sidecar path and pinned version at startup.
- Invoke only the verified absolute executable path from the install directory, with plugins and network-dependent features disabled.
- Materialize one input into a random current-user-only directory inside the encrypted data folder.
- Invoke one process under the Windows Job Object limits proven in Phase 1.
- Capture Markdown up to 10 MB and structured errors without logging content.
- Clean the input and temporary directory after every run and sweep abandoned directories at startup.
- Bypass the sidecar for plain text, Markdown, and pasted text.

**Tests:**

- Golden PDF and DOCX fixtures preserve expected headings, lists, tables, and Unicode text.
- Relative, missing, version-mismatched, or substituted sidecar paths are rejected before document bytes are materialized.
- Plain text, Markdown, and pasted content never invoke the sidecar.
- Scanned PDF, corrupt PDF/DOCX, empty output, malformed stderr, non-zero exit, timeout, memory cap, child process, and output cap become clear retryable failures.
- Temporary paths contain no candidate name or original filename.
- Permission checks prove another ordinary local user cannot read newly created temporary directories.
- Success, failure, cancellation, parent crash simulation, and next-startup sweep leave no abandoned plaintext inputs.
- Markdown output containing scripts, prompt injection, terminal control characters, and hostile links is stored as untrusted text, not executed.
- Native configuration inspection and a network-attempt fixture prove sidecar plugins and network-dependent features remain disabled.
- Native Windows integration runs against the packaged sidecar, not only the fake process.

**Exit gate:** Supported documents reliably become Markdown on Windows; all resource-limit and cleanup tests pass after forced failures.

## Phase 7 — Deterministic chunking, citations, and FTS5

**Outcome:** Produce stable, human-resolvable evidence chunks and lexical retrieval.

**Build:**

- Add fixed heading/list/paragraph then sentence chunking.
- Persist ordinals, character offsets, heading paths, token counts, hashes, and chunker version/parameters.
- Add the external-content FTS5 table, triggers, and explicit rebuild path.
- Resolve citations back to artifact and human-readable location.
- Run chunking and indexing through background jobs.

**Tests:**

- Golden tests cover headings, nested lists, tables, long paragraphs, abbreviations, Unicode, blank sections, and exact target-size boundaries.
- Repeated chunking with identical input and parameters produces identical chunks and hashes.
- Every stored offset selects the cited source text; every heading path resolves.
- FTS insert, update of derived text, delete, rollback, and rebuild remain synchronized with chunks.
- Queries cover punctuation, quoting, Unicode, empty input, common terms, and injection-shaped strings.
- Cancelling indexing removes the current attempt's chunks and records retryable Extraction-failed while prior committed items remain.
- Artifact changes invalidate old derived rows rather than mixing chunker versions.
- Disk-backed tests compare FTS results before and after rebuild.

**Exit gate:** A supported artifact can be extracted, chunked, searched, and cited end to end, with FTS integrity surviving rebuild and cancellation.

## Phase 8 — Local model registry and credential foundation

**Outcome:** Configure the required local roles and securely hold external-provider credentials.

**Build:**

- Persist `embed`, `classify`, and `generate` assignments and endpoint configuration revisions.
- Default `classify` to the local `generate` model without duplicating configuration.
- Record model name, immutable digest when available, parameters, and Validated/Unvalidated status.
- Verify Ollama and required models; expose missing-model information and pull prompts.
- Store Exa and optional cloud secrets only in Windows Credential Manager.
- Redact secrets and sensitive content from errors and diagnostics.

**Tests:**

- Registry validation rejects missing required roles, invalid URLs, unsupported parameters, and cloud-only candidate processing.
- Changing endpoint, model digest, or parameters increments the configuration revision.
- `classify` follows `generate` until explicitly assigned and continues to behave predictably after either assignment changes.
- Default models remain Unvalidated until benchmark records prove otherwise.
- Ollama unavailable, model missing, pull declined, pull failed, timeout, malformed response, and memory error are distinct visible states.
- Credential create, replace, retrieve, revoke, and missing-entry paths pass natively on Windows.
- Database, selected data folder, copied recovery folder, logs, crash errors, and UI masks contain no secret values.
- Fake OpenAI-compatible endpoints assert the exact payload shape used by each role.

**Exit gate:** All required local roles can be configured and called without storing credentials in application data.

## Phase 9 — Local embeddings and exact-cosine retrieval

**Outcome:** Add portable semantic retrieval without a vector extension.

**Build:**

- Embed source chunks and Profile Aspects as distinct retrieval units.
- Store little-endian float32 vectors with embedding-space identity, dimensions, metric, model digest, and endpoint revision.
- Implement exact cosine scan in Go and prohibit comparisons across embedding spaces.
- Keep all candidate-content embedding calls local.

**Tests:**

- Float32 serialization round-trips known bit patterns and rejects wrong byte lengths or dimensions.
- Cosine results match a small trusted numerical oracle, including equal, orthogonal, opposite, zero, NaN, and malformed vectors.
- Stable tie-breaking returns deterministic order.
- Different embedding spaces are never merged or compared.
- Model or endpoint revision changes create a new space and mark older derived data unavailable for current retrieval.
- A fake cloud endpoint receives zero candidate embedding calls under all configurations.
- Cancellation and provider failure leave no partial vector for the current item.
- Representative exact-scan measurements are recorded at increasing corpus sizes; no vector extension is added unless the PRD threshold is missed.

**Exit gate:** Semantic lookup is correct, deterministic, local for candidate information, and measurable at the representative scale.

## Phase 10 — Versioned profile and classifier contract

**Outcome:** Establish the reusable typed, citable decomposition shared by Candidate and Role Profiles.

**Build:**

- Persist profile versions and Profile Aspects under the exact taxonomy.
- Version the constrained JSON schema and classifier prompt.
- Validate type, priority, origin, source wording, normalized structured values, and citations.
- Apply one repair retry, then return a visible retryable failure.
- Preserve extracted versus Recruiter supplied origin.

**Tests:**

- Schema fixtures cover every aspect type and must-have, nice-to-have, and unspecified role priority.
- Unsupported types, unsupported priorities, missing citations, citation outside source, invented structured fields, duplicates, and contradictory values fail validation.
- Unclear source wording remains absent, unknown, or unspecified as defined; it is never promoted to must-have.
- Valid first response uses no repair call; invalid-then-valid uses exactly one; invalid twice fails visibly.
- Prompt-injection text inside a source cannot add instructions, unsupported aspects, or uncited facts.
- Recruiter supplied aspects remain distinct and cite their recruiter-authored record.
- Schema and prompt version changes alter derived-profile identity.
- Contract tests run against the selected local model on Windows in addition to deterministic fake responses.

**Exit gate:** The classifier contract either returns a fully valid profile proposal or a visible failure; it cannot persist partially valid output.

## Phase 11 — Approved Candidate Profiles

**Outcome:** Turn candidate records and artifacts into recruiter-approved evidence used by search and matching.

**Build:**

- Create Candidate Profiles as Proposed and block search/matching until initial approval.
- Support resume drag-in that attaches to a selected Candidate or creates the Candidate for a new Job Search Initiative.
- Present source citations and extracted versus Recruiter supplied origin.
- Support recruiter edits, additions, removals, and approval.
- On source change, preserve the approved version as Stale and create a proposed additions/removals/conflicts diff.
- Permit manual profile construction after extraction failure.

**Tests:**

- Initial classification combines structured candidate data and linked artifacts without losing provenance.
- Dragging a resume into a new Job Search Initiative creates exactly one Candidate and one linked Artifact; cancellation creates neither.
- Search and matching remain blocked for missing, Proposed, or failed initial profiles.
- Approval freezes a version and all cited evidence remains resolvable.
- Source addition, replacement, edit, detach, and deletion create the correct stale state and proposed diff.
- Recruiter-approved aspects are never silently overwritten by reclassification.
- The stale approved version remains usable only with the specified warning until reapproval.
- Manual profile construction works when a scanned or corrupt resume cannot be extracted.
- Vitest covers diff review, citation navigation, edit/approve, conflict resolution, and keyboard operation.
- Playwright drops a resume, reviews the proposal, edits one aspect, approves it, changes a source, and observes staleness.

**Exit gate:** A recruiter can create and maintain an approved Candidate Profile with complete provenance and human control.

## Phase 12 — Automatic, editable Role Profiles

**Outcome:** Decompose discovered or entered roles without forcing approval of every listing.

**Build:**

- Automatically create a Role Profile from current role content.
- Expose extracted requirements, priority, constraints, citations, and manual edits.
- Keep Failed profiles visible with retry and manual-entry actions.
- Mark profiles Stale after current source content changes.
- Make only Ready current versions eligible for automatic assessment.

**Tests:**

- Representative role fixtures cover stated and unstated skills, responsibilities, experience, qualifications, seniority, location, work arrangement, work rights, employment type, and compensation.
- Must-have and nice-to-have are assigned only when the source supports them; ambiguous language stays unspecified.
- Explicit normalized constraints reproduce the source exactly while preserving original wording.
- Failure, retry, manual completion, source change, and stale-to-Ready transitions are correct.
- Failed and Stale Role Profiles never disappear and never enter automatic assessment.
- Recruiter edits create a new evidence-aware version without mutating the source artifact.
- Vitest covers failure cards, retry/manual entry, citations, edits, and lifecycle labels.
- Playwright creates one Ready and one Failed profile and confirms only the Ready role is assessable.

**Exit gate:** Role profiling is automatic, transparent, editable, and safe to feed into matching.

## Phase 13 — Search Criteria and prohibited-criteria controls

**Outcome:** Capture recruiter-approved search intent separately from candidate facts.

**Build:**

- Add ordered must-have and nice-to-have Search Criteria per initiative.
- Propose criteria from approved professional aspects while requiring recruiter application.
- Never infer preferences from employment or education history alone.
- Block explicit protected criteria deterministically.
- Use local `classify` only to warn on ambiguous potential proxies.
- Version criteria for assessment invalidation.

**Tests:**

- Candidate facts and Search Criteria remain separately editable and separately versioned.
- Proposal fixtures prove prior employer, school, location history, and compensation history do not silently become preferences.
- The complete provisional protected list is blocked across case, punctuation, and straightforward wording variants.
- Ambiguous proxy fixtures warn but do not hard-block; clearly lawful criteria do not generate deterministic blocks.
- Work-rights criteria remain available without nationality inference.
- Chat or classifier proposals cannot mutate criteria without explicit recruiter action.
- Reordering changes presentation only; changing content or priority changes the criteria version.
- Vitest and Playwright cover block, warning, recruiter override where permitted, human apply, and accessibility.

**Exit gate:** Only recruiter-approved lawful search intent can drive discovery and matching.

## Phase 14 — Exa role discovery and source lifecycle

**Outcome:** Find public role listings without silently disclosing direct candidate identity or bypassing source controls.

**Build:**

- Generate queries from approved professional aspects and Search Criteria.
- Remove direct identifiers and generalize exact employers, clients, projects, and schools by default.
- Show the exact editable query and require confirmation for every Exa search.
- Cache roles and permitted Exa content with provenance.
- Persist the visible Exa Search query for reproducibility and create one metadata-only disclosure audit event for each request actually sent.
- Resolve identity by source ID, then canonical URL, then content fingerprint.
- Implement unchanged and changed source observations, historical evidence, staleness, and manual paste/attach fallback.
- Keep direct fetching deny-by-default. Ship an empty allowlist unless a source has completed the PRD's access review.

**Tests:**

- Query fixtures remove name, email, phone, street address, and known structured identifiers.
- Organization names are generalized by default; deliberately re-added specific terms trigger the additional warning.
- The exact previewed query equals the sent query byte for byte.
- Cancelled preview sends no request and creates no disclosure audit event.
- A successful request stores the exact visible query on the Search record and the required metadata, but no query content, in its audit event.
- Fake Exa responses cover pagination, duplicates, missing fields, malformed records, rate limits, timeout, offline, retry, and partial results.
- Role identity resolution follows the required precedence deterministically.
- Same hash updates `retrieved_at`; changed content creates a new current source artifact and excludes the historical artifact from current retrieval.
- Thirty-day and closing-date boundaries use a fake clock; rediscovery returns Stale to Active.
- Denied, unlisted, authenticated, robots-disallowed, anti-bot, SEEK, and LinkedIn direct-fetch attempts remain blocked.
- Manual paste/attach can complete insufficient Exa content without claiming automated provenance.
- Playwright previews, edits, cancels, sends, caches, revisits, and stales role results through a fake Exa server.

**Exit gate:** Role discovery is reproducible, auditable, source-safe, and produces current evidence for profiling without browser automation.

## Phase 15 — Hybrid top-20 shortlist

**Outcome:** Select assessment candidates cheaply with structured scope, FTS, exact cosine KNN, and reciprocal-rank fusion.

**Build:**

- Exclude only out-of-scope, deleted, and Stale roles.
- Search each approved criterion and compatible Candidate Profile aspect against role chunks and aspects.
- Enforce the PRD's aspect compatibility map.
- Fuse ranked lists with reciprocal-rank fusion, group by Role, and return a stable top 20.
- Preserve potential structured must-have failures for assessment rather than hiding them.

**Tests:**

- A hand-calculated corpus verifies every compatibility-map edge and excludes every disallowed edge.
- FTS-only, vector-only, overlapping, empty, duplicate, and tied lists produce the expected fused order.
- Role grouping prevents multiple chunks from occupying multiple shortlist slots.
- Scope, deleted, and Stale filters are applied before retrieval and never leak excluded roles.
- Known location, work-rights, or arrangement conflicts remain visible when otherwise retrieved.
- Fewer than 20 roles returns all eligible roles; more than 20 returns exactly 20 with stable ties.
- Repeated runs and database rebuilds return identical ordering.
- Performance measurements record P50 and P95 at representative sizes.
- Playwright exposes the shortlist provenance and explains why each role entered the candidate set.

**Exit gate:** The shortlist is deterministic, explainable, and fast enough to feed the expensive assessment stage.

## Phase 16 — Two-directional assessment and ranking

**Outcome:** Produce evidence-backed match results with complete invalidation and deterministic ordering.

**Build:**

- Compare Role Profile against Search Criteria and Candidate Profile against Role Profile separately.
- Use deterministic comparison for structured constraints.
- Retrieve compatible candidate evidence by exact-cosine KNN for semantic requirements.
- Ask `generate` for met, not met, or unknown with the required citation behavior.
- Implement the exact ranking order from the PRD.
- Compute one canonical `assessment_input_hash` over every listed input.
- Run assessment as cancellable background jobs and reuse only valid cached results.

**Tests:**

- Per-aspect fixtures cover met with citation, not met with contrary evidence, not met without contrary evidence, unknown, and no evidence.
- Structured comparison covers normalized location, work arrangement, work rights, employment type, compensation, and unknown values.
- KNN only selects evidence; adversarial similarity scores cannot directly change the final result state.
- Output validation rejects uncited met results, invalid states, unavailable citations, and prompt-injected instructions.
- A table-driven ranking oracle tests each tie-break independently and in combination, ending with stable Role ID.
- Every assessment hash input is changed one at a time and must invalidate the cache; presentation-only changes must not.
- Canonical serialization produces the same hash across process restarts and map iteration order.
- Batch cancellation keeps completed per-role results, rolls back the current item, and records counts.
- Reassessment reuses unchanged valid results and recomputes only stale ones.
- Vitest covers two-direction labels, gaps, unknowns, evidence navigation, progress, cancel, and stale state.
- Playwright runs and reruns a deterministic ten-role assessment and verifies ranking and cache reuse.

**Exit gate:** FR-08 is complete: every assessed result is explainable, correctly ordered, cancellable, and invalidated by all decision-relevant changes.

## Phase 17 — Scoped Q&A, drafts, and copy-out

**Outcome:** Complete the local recruiter interaction loop without adding message transport.

**Build:**

- Add initiative-scoped Q&A and summaries over approved local context.
- Require citations for factual answers.
- Let chat propose, but never silently apply, structured changes.
- Generate editable evidence-backed pitch and outreach drafts.
- Record each copy action as a metadata-only CopiedOut audit event.
- Keep Outreach Draft lifecycle Active or Discarded.

**Tests:**

- Retrieval scope excludes other initiatives and unapproved or unavailable evidence.
- Factual answers cite resolvable evidence; unsupported questions return unknown rather than invention.
- Prompt injection in artifacts cannot change scope, apply state changes, call Exa/cloud, delete data, or copy content.
- Every factual draft claim maps to evidence; invalid model output is rejected or clearly marked for correction.
- Editing and repeated copying preserve the draft and create separate metadata-only events.
- CopiedOut events contain no draft text, payload, query, or document content.
- Discarding a draft does not falsely record a copy or send action.
- A repository-level transport check and runtime test prove no email, SMS, LinkedIn, or other outreach sender exists.
- Vitest covers citation navigation, apply-confirmation UI, draft editing, copy feedback, discard, and keyboard operation.
- Playwright completes local Q&A, draft generation, edit, copy twice, and discard through the real backend.

**Exit gate:** The recruiter can ask, draft, edit, and copy locally; the application cannot send outreach.

## Phase 18 — Optional cloud controls and disclosure audit

**Outcome:** Provide the approved diagnostic escape hatch without weakening the local-first boundary.

**Build:**

- Add one optional cloud endpoint and task-level override.
- Bind consent to initiative, endpoint revision, and eligible task type.
- Preview the first actual payload, retain later preview access, support revocation, and reset consent on endpoint change.
- Enforce local-only raw candidate artifacts, Candidate Profile extraction, and embeddings.
- Replace known structured direct identifiers with placeholders in eligible assessment and drafting payloads.
- Require explicit payload selection and preview for every cloud-chat send.
- Store metadata-only audit events for every non-localhost request.

**Tests:**

- A complete allow/deny matrix covers role extraction, assessment, drafting, chat, candidate extraction, and embeddings.
- No configuration can send raw candidate artifacts or candidate embeddings to the cloud endpoint.
- Consent for one initiative, endpoint revision, or task does not authorize another.
- Endpoint change and revocation take effect before the next request.
- First-use preview exactly matches the sent payload; cloud chat always requires per-send preview.
- Known names, emails, phones, and other structured identifiers become placeholders in eligible payloads.
- Cancelled previews, denied tasks, offline state, timeout, provider error, and credential removal send nothing unexpected.
- Audit events contain required metadata and no payload, query, document, draft, or credential content.
- Playwright exercises approve, reuse, revoke, endpoint-change reset, and denial with a fake cloud endpoint.

**Exit gate:** Cloud use is explicit, narrowly scoped, locally audited, and incapable of becoming the default runtime.

## Phase 19 — Complete deletion invariants

**Outcome:** Make every destructive action conform exactly to the PRD and prove derived data removal.

**Build:**

- Implement initiative, candidate, artifact, draft, and discovered-role deletion rules.
- Block candidate deletion for every Active or Archived initiative reference.
- Resolve shared candidate artifacts through explicit global-delete or retain-under-other-links choices.
- Keep Exa source artifacts read-only outside role purge.
- Preserve recruiter notes and metadata-only audit events with cleared unavailable references where required.
- Run cascades transactionally and execute scoped verification queries before reporting success.

**Tests:**

- A table-driven invariant suite covers every row in the PRD deletion table.
- Candidate deletion is blocked by both Active and Archived initiatives and succeeds only after all references are deleted.
- Candidate-only and shared-artifact branches remove or retain exactly the intended evidence.
- Link-only detach and global artifact deletion have distinct confirmations and effects.
- Exa source artifact detach/delete is rejected; role purge removes current and historical sources, profile, aspects, embeddings, matches, and active drafts.
- Purge-all-stale applies the same invariant independently to every selected Stale role and reports any role that could not be purged without partial deletion.
- Recruiter-authored notes survive role purge with unavailable references; CopiedOut events survive with cleared role or draft references as specified.
- Injected failure at every cascade step rolls the entire transaction back.
- Verification queries check chunks, FTS, embeddings, profiles, matches, and exclusively owned artifacts while tolerating intentionally shared evidence.
- Repeated deletion requests are safe and do not damage unrelated records.
- Vitest verifies consequence previews list the exact links and records affected.
- Playwright runs the additional deletion acceptance scenarios for stale role purge and candidate deletion.

**Exit gate:** FR-12 and every deletion acceptance condition pass with transaction-failure coverage.

## Phase 20 — First run, recovery, diagnostics, and packaging

**Outcome:** Make the PoC self-service and safe on the supported Windows laptop.

**Build:**

- Complete the ordered first-run wizard and data-handling acknowledgement.
- Check selected-volume encryption at every startup and block real-data mode when unavailable.
- Verify sidecar and Ollama, show model sizes, and prompt to pull missing models.
- Add redacted local diagnostics and open-logs-folder action.
- Document closed-app folder copy, reinstall, integrity check, snapshot, migration, restore-on-failure, credential re-entry, and model re-download.
- Add application version, manual installer path, uninstall documentation, and exact-folder delete-all confirmation.
- Keep the active initiative, data scope, selected models, cloud override, and online/offline state visible.
- Ship no telemetry endpoint, SDK, background reporter, or opt-in control in the PoC.
- Package the Windows application and PyInstaller one-dir sidecar together.

**Tests:**

- Wizard state tests cover fresh install, cancellation at each step, restart/resume, missing sidecar, missing Ollama, declined pull, failed pull, and acknowledgement requirement.
- Encrypted volume permits real data; unencrypted, unknown, or check-failed volume blocks it at first run and later startup.
- Optional empty demo mode rejects artifacts and personal-data entry.
- Diagnostic fixtures containing secrets, candidate details, queries, payloads, draft content, and control characters remain redacted and safe to display.
- UI tests keep initiative, scope, model, provider override, and connectivity state correct through tab changes and provider failures.
- Network observation during offline and normal local workflows finds no telemetry request.
- A copied data folder opens after integrity check and migration; missing credentials/models produce guided recovery without data loss.
- Corrupt copy, failed integrity, failing migration, read-only folder, and missing snapshot never open or overwrite the only copy.
- Delete-all names the exact resolved folder, requires confirmation, and is tested only against isolated temporary folders.
- Installer smoke tests verify launch, sidecar path/version, WebView2 behavior, upgrade over previous build, uninstall without silent data deletion, and reinstall/recovery.
- Windows Defender/SmartScreen and installed-file checks record whether the packaged one-dir sidecar launches cleanly and whether code signing is required.
- Offline native test covers CRM, artifacts, profiles, retrieval, Q&A, and local generation after models are installed.

**Exit gate:** A non-technical recruiter can install, start securely, recover a copied folder, work offline, inspect diagnostics, upgrade, and uninstall without hidden data loss.

## Phase 21 — Benchmarks and PoC acceptance

**Outcome:** Prove the product contract on frozen data and the real target laptop.

**Build:**

- Freeze five anonymized past-placement scenarios and twenty representative classifier role listings before tuning.
- Add a repeatable benchmark runner that records configuration, model digests, prompt/schema versions, corpus hashes, measurements, and results.
- Tune only on separate non-held-out data.
- Run the local-only flagship, additional gates, performance measurements, and recovery exercise.
- Fix only acceptance-blocking defects; new capabilities return to the backlog or require a PRD change.

**Tests and evidence:**

- Classifier benchmark: every aspect cited; no unsupported critical constraint; at least 80% material-aspect capture; exact structured constraints reproduced.
- Matching benchmark: at least three plausible top-five roles in at least four of five frozen scenarios.
- Live local-only acceptance: at least ten eligible roles, ten Ready profiles and assessments, at least three plausible top-five roles, and one usable evidence-backed draft.
- A result below ten live roles is recorded as source-coverage inconclusive, not silently passed or failed.
- Cloud diagnostic runs are reported separately and cannot pass the PoC.
- Additional PRD gates cover offline work, no sending capability, role purge, candidate deletion, external-disclosure approval, encryption refusal, and recovery.
- Target-laptop measurements cover cold start, indexing, decomposition, retrieval P95, assessment, end-to-end twenty-role time, database size, and overnight corpus indexing.
- Accessibility walkthrough covers keyboard operation and visible distinction between source, recruiter-authored, and AI content.
- Final `just check`, Windows installer smoke test, security scan, dependency vulnerability scan, and redacted-log inspection pass.

**Exit gate:** Every functional, security, deletion, offline, recovery, classifier, and matching gate in the PRD passes. Provisional performance misses are recorded with measured evidence and an explicit go/no-go decision; they are not silently reclassified.

## Milestones

| Milestone | Included phases | Demonstrable result |
| --- | --- | --- |
| A — Safe local evidence | 0–7 | A recruiter record can hold an immutable document that is safely extracted, indexed, searched, and cited. |
| B — Structured intelligence | 8–13 | Local models produce recruiter-controlled Candidate and Role Profiles plus lawful Search Criteria. |
| C — Flagship loop | 14–17 | Exa roles become a deterministic evidence-backed shortlist, assessment, Q&A result, and copied draft. |
| D — Trusted PoC | 18–21 | Cloud is controlled, deletion and recovery are proven, Windows packaging works, and acceptance is measured. |

## Requirement traceability

| Requirement | Primary phase | Verification summary |
| --- | --- | --- |
| FR-01 | 3 | Go lifecycle tests, Vitest interaction tests, Playwright initiative journeys |
| FR-02 | 3 | Field validation, persistence, shared-reference, and contacts-at-company tests |
| FR-03 | 4, 19 | Byte/provenance/link tests plus full invariant and deletion verification |
| FR-04 | 1, 6, 7, 9 | Native sidecar limits, golden chunking, FTS rebuild, vector oracle, scale measurements |
| FR-05 | 10–12 | Schema/citation contract, candidate approval/diff, automatic role-profile lifecycle, classifier benchmark |
| FR-06 | 17 | Scope, citation, unknown-answer, prompt-injection, and end-to-end Q&A tests |
| FR-07 | 14 | Identifier-safe query fixtures, exact preview, fake Exa contract, source observation and staleness tests |
| FR-08 | 5, 13, 15, 16 | Job lifecycle, criteria controls, RRF oracle, two-direction assessment, ranking and hash tests |
| FR-09 | 17 | Evidence-backed drafts, edit/copy/discard, metadata-only audit, no-transport proof |
| FR-10 | 8, 18 | Local role registry, Ollama contracts, cloud allow/deny and consent matrix |
| FR-11 | 8, 18 | Windows credential lifecycle, revocation, secret-absence, and provider-setting tests |
| FR-12 | 19 | Complete invariant table, rollback injection, scoped removal verification |
| FR-13 | 1, 2, 20 | Encryption, migrations/snapshots, first run, recovery, diagnostics, installer and offline tests |

## Definition of done for every phase

A phase closes only when:

- its build items and named tests are complete;
- `just check` passes with no quarantined or newly skipped tests;
- generated Wails bindings are regenerated when service contracts change and are never hand-edited;
- failure, empty, loading, cancellation, retry, offline, and permission states relevant to the phase are visible;
- security and personal-data boundaries relevant to the phase have negative tests;
- new migrations pass fresh-install, upgrade, repeat-run, and failure-restore tests;
- Windows-native behavior has passed on Windows when the phase depends on Windows APIs or packaging;
- automated fixtures and logs contain no real personal information or credentials;
- the implementation and test names use the canonical language from [CONTEXT.md](../../CONTEXT.md); and
- the phase can be demonstrated without unfinished behavior from a later phase being presented as complete.

## Explicitly deferred from this plan

The PoC does not pre-build CSV/ZIP import, batch extraction, OCR, a dedicated classifier model, embedding-similarity chunking, a vector extension, company/profile/news search, browser or Chrome automation, automated SEEK or LinkedIn fetching, broader free-text PII redaction, cross-initiative Q&A, full CRM, Talent Search or Business Development pipelines, message sending, hosted or multi-user features, macOS packaging, or encrypted backup.

Those items begin only after Phase 21, in recruiter-priority order, with encrypted authenticated backup and restore first.
