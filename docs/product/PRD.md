# Talent Hound Product Requirements Document

**Status:** Final v0.5 — Proof of Concept product contract

**Last updated:** 2026-08-16

**Change summary from v0.4:** Finalized after product-definition review and a decision interview. Candidate deletion is now blocked by every referencing initiative, including archived initiatives. Role purge, audit retention, artifact provenance, temporary extraction files, recovery, BitLocker enforcement, source acquisition, and local-versus-cloud acceptance are fully defined. Candidate Profile, Role Profile, Profile Aspect, and Search Criteria are explicit domain concepts. The **classify** role decomposes resumes, candidate details, and job descriptions into evidence-backed aspects. Matching is two-directional and uses aspect-level FTS/KNN retrieval, deterministic ranking, and one complete assessment input hash. The five-scenario matching benchmark is joined by a held-out profile-extraction benchmark. This document is the approved PoC product contract; remaining implementation inputs and validation tasks do not reopen its scope.

## Summary

Talent Hound is a local-first Windows desktop workbench for one independent recruiter. It combines a private talent CRM, local retrieval-augmented generation over recruiter-owned information, and public-web role discovery through Exa.

The PoC validates one flagship loop:

> Turn one known candidate's information into an approved candidate profile, find live roles, decompose each role into comparable requirements, produce an evidence-backed ranked shortlist, and prepare a recruiter-editable pitch.

Role-to-candidate search and business-development workflows follow in P1. The recruiter remains the decision-maker. Recommendations are evidence-backed, confidential data stays on the device by default, and the application cannot send outreach.

## Problem

Recruiters repeatedly move information between resumes, notes, job boards, public profiles, CRM records, email, and AI tools. Finding and comparing opportunities requires manual research; knowledge in the existing talent pool is hard to reuse; and pasting resumes, conversations, and commercial terms into cloud AI tools creates confidentiality risk.

The result is slower shortlisting, underused candidate relationships, inconsistent comparison of candidates and roles, and avoidable disclosure of sensitive information.

## Primary user

One independent technology recruiter operating in Australia, working alone on a Windows laptop with CPU-only inference, 16 GB RAM, and no IT support. Setup and recovery must be self-service. Team workflows and shared databases are out of scope.

Technology recruitment supplies the first validation scenarios, but records, aspect types, criteria, prompts, and workflows must not hard-code technology-specific assumptions.

This PoC validates usefulness for this single known recruiter. Commercial validation such as pricing, willingness to pay, retention, support, and distribution is explicitly outside this document.

## Product principles

1. **Local by default:** Candidate records, artifacts, profiles, embeddings, retrieval, and model processing stay on the device unless a specifically permitted task is deliberately sent to an external provider.
2. **Human controlled:** AI proposes; the recruiter reviews, edits, approves, and decides. Key structured changes, disclosures, deletions, and outreach actions always require recruiter action.
3. **Evidence before assertion:** Every material claim in a profile, match, or draft links to a local artifact, recruiter-authored record, or permitted public source.
4. **The model is the last, smallest step:** Structured comparison, FTS, and vector retrieval select and organize evidence before generation.
5. **Portable, not hosted:** All recruiter content and application database data lives in one selected data folder. Credentials, application binaries, and Ollama models have explicitly documented external locations.
6. **Compliant discovery:** Talent Hound uses lawfully accessible public sources and never bypasses authentication, platform restrictions, robots controls, or anti-bot measures.
7. **Industry agnostic:** Technology recruitment validates the PoC but does not constrain its domain model.
8. **Failure is visible:** Missing evidence, failed extraction, unavailable sources, interrupted jobs, and unsupported model behavior are shown rather than silently hidden or guessed.

## Decisions locked

| Area | Final PoC decision |
| --- | --- |
| Release framing | This is the final product contract for a PoC with one known recruiter. P1 is the path from PoC to V1. |
| Platform | Windows 11 x64 only. The Go application remains CGO-free and portable, but macOS is not a PoC concern. |
| At-rest security | The selected data-folder volume must have BitLocker or Windows Device Encryption enabled. The app checks at every startup and blocks real personal-data use on an unencrypted volume. SQLite stores plain BLOBs protected by the encrypted volume. |
| Backup | Encrypted backup is the first P1 feature. PoC safety is a closed-app directory copy plus an automatic database snapshot before every schema migration. |
| Import | CSV and ZIP import are P1. PoC ingestion is manual records, pasted text, and resume drag-in. |
| AI protocol | All model calls use OpenAI-compatible chat and embeddings APIs. Required local assignments use Ollama at http://localhost:11434/v1. One optional cloud endpoint is available only for permitted, explicitly approved tasks. |
| Model roles | PoC roles are **embed**, **generate**, and **classify**. **Classify** is a first-class structured-decomposition role and defaults to the same local model as **generate** until a dedicated model proves useful. |
| Interaction model | Structured panels are primary. Initiative-scoped chat may propose changes but cannot silently mutate structured state or bypass confirmations. |
| Cardinality | A Job Search Initiative references exactly one candidate. |
| Profiles | Candidate Profile and Role Profile are persisted, evidence-backed derived records composed of typed Profile Aspects. |
| Search intent | Search Criteria belongs to an initiative and remains separate from extracted candidate facts. Candidate preferences are never inferred from resume history alone. |
| Matching | Matching evaluates both role fit for the candidate and candidate fit for the role. Aspect-level KNN retrieves evidence; it never independently declares a match. |
| Outreach | Draft-and-copy only. The application has no sending integration. Copying creates a metadata-only audit event. |
| PoC search scope | Exa searches role listings only. Company, profile, and news searches are P1. |
| Source acquisition | Exa content is the default. Direct automated fetch is allowed only for developer-maintained, reviewed public-source allowlists. SEEK, LinkedIn, authenticated pages, blocked pages, anti-bot challenges, and Chrome automation are never automated in the PoC. |
| Discovered roles | A 30-day staleness window plus explicit purge. No revalidation state machine or automatic purge. |
| Artifacts | One immutable artifact is created per ingestion. The PoC does not deduplicate artifact bytes. Original filename and provenance are immutable; display name is editable. |
| Text extraction | A bundled Microsoft MarkItDown PyInstaller sidecar converts PDF and DOCX to Markdown, one file per process. It provides failure isolation, not an exploit sandbox. |
| Chunking | Fixed structural and sentence chunking. Embedding-similarity boundary detection is P1 unless evidence demonstrates a need. |
| Assessment caching | One **assessment_input_hash** over every decision-relevant input governs validity, caching, and staleness. |
| Deletion | Governed solely by the deletion-invariants table. Candidate deletion is blocked while any initiative, active or archived, references the candidate. |
| Updates | Manual installer. Migrations run after an automatic snapshot and restore the snapshot on failure. |

## Flagship workflow: find roles for an existing candidate

1. The recruiter creates a **Job Search Initiative** and selects or creates its single candidate. Dropping a resume may create the candidate directly.
2. Talent Hound extracts, chunks, and indexes candidate artifacts locally.
3. The **classify** role proposes an evidence-backed Candidate Profile. The recruiter reviews and approves it before the first search or match.
4. Talent Hound proposes separate initiative Search Criteria from the approved profile and recruiter input. Criteria describe the role the candidate seeks; they do not redefine candidate facts.
5. Talent Hound constructs an Exa role-listing query from approved professional aspects and criteria. Direct identifiers are excluded, and exact employers, clients, projects, and schools are generalized by default. The recruiter sees and edits every query before sending.
6. Exa results are cached with source provenance. Permitted direct fetching follows the source-acquisition policy; otherwise the recruiter may paste or attach page content manually.
7. The **classify** role automatically creates an evidence-backed Role Profile for each result. Failed profiles remain visible and retryable.
8. Structured scope filters, FTS, and exact-cosine aspect KNN produce a 20-role assessment shortlist.
9. Talent Hound assesses role fit for the candidate and candidate fit for the role, showing per-aspect results, gaps, missing information, and citations.
10. Results are ordered by the deterministic ranking contract in this document.
11. The recruiter selects a target. Talent Hound produces an editable, evidence-backed pitch, which the recruiter may copy out.
12. Initiative chat may answer scoped questions and propose criteria or profile changes. The recruiter must apply structured changes; Exa, cloud, and destructive actions retain their own confirmations.

Talent Search and Business Development initiatives remain available as existing workspace shells with Context, artifacts, and chat, but their discovery and matching pipelines are P1. Legal-clause analysis, candidate reuse, and cross-initiative Q&A are also P1.

## Domain model

The canonical language is maintained in [CONTEXT.md](../../CONTEXT.md).

| Concept | Purpose |
| --- | --- |
| Initiative | A bounded objective and workspace. It owns criteria, searches, matches, drafts, jobs, and audit events while referencing shared records and artifacts. |
| Candidate | A person in the recruiter's talent pool. A candidate may be referenced by multiple initiatives. |
| Candidate Profile | The approved, evidence-backed decomposition of candidate facts used by retrieval and matching. |
| Role Profile | The evidence-backed decomposition of a role's responsibilities, requirements, and structured constraints. |
| Profile Aspect | One typed, citable statement within a Candidate Profile or Role Profile. |
| Search Criteria | Recruiter-approved must-have and nice-to-have requirements or preferences for one initiative. |
| Company | A minimal PoC record created manually or from a discovered role. |
| Contact | A person at a company. PoC warm-path support is the count and listing of known contacts at that company. |
| Role | A recruiter-entered or publicly discovered hiring requirement with origin, structured fields, and lifecycle state. |
| Artifact | One immutable ingestion occurrence containing a file, note, pasted text, or captured source used as evidence. |
| Match | A two-directional assessment between one candidate and one role, including per-aspect results, evidence, gaps, and validity hash. |
| Outreach Draft | An active or discarded unsent message. Copying it is an audit event, not a state transition. |
| Audit Event | Metadata recording an external disclosure, copy-out, or other auditable action without storing payload or draft content. |

Deferred P1 concepts include Interaction, Relationship, Signal, Opportunity, and formal artifact-version records.

### Profile-aspect taxonomy

The controlled, industry-neutral aspect types are:

- **skill**
- **responsibility**
- **experience**
- **qualification**
- **seniority**
- **location**
- **work arrangement**
- **work rights**
- **employment type**
- **compensation**
- **other**

Every aspect preserves its source wording and citations. A Role Profile aspect has priority **must-have**, **nice-to-have**, or **unspecified**. The classifier must not invent priority when the source is unclear. Candidate Profile aspects do not carry employer priority. Search Criteria carry recruiter-selected must-have or nice-to-have priority.

Location, work arrangement, work rights, employment type, and compensation may include normalized structured values for deterministic comparison. The original source wording remains available.

### Structured candidate and role fields

The Candidate record stores full and preferred name, email addresses, phone numbers, location, work-rights or visa details, availability, desired employment and work arrangement, compensation or rate expectations, data-source or authority note, and last-confirmed date. Employment history, education, skills, achievements, and qualifications are represented as evidence-backed profile aspects. Notes and artifacts retain everything else.

The Role record stores title, company, location, work arrangement, employment type, compensation or rate when stated, published date, closing date, retrieved date, source ID, canonical URL, source, recruiter-entered versus discovered origin, and lifecycle state. Responsibilities and requirements are Profile Aspects.

The minimal Company record stores name, website or domain, location, and source. The minimal Contact record stores full name, Company, role or title, optional email and phone, and source. Relationship strength, interaction history, and richer CRM behavior are P1.

### Lifecycle states

| Entity | States and transitions |
| --- | --- |
| Initiative | Active ⇄ Archived. Deletion follows the deletion invariants. |
| Candidate Profile | Proposed → Approved. A source change creates a proposed diff and marks the approved version Stale; the stale approved version remains usable with a warning until a new version is approved. Initial extraction failure is retryable or replaceable by manual profile construction. |
| Role Profile | Extracting → Ready or Failed. A source change marks it Stale; re-extraction or manual editing produces a new Ready version. Only Ready role profiles are automatically assessed. |
| Discovered Role | Active → Stale after 30 days since retrieval or when a stated closing date passes → Purged by explicit recruiter action. Rediscovery returns Stale to Active. |
| Recruiter-entered Role | Open → Filled or Closed; manually reopenable. |
| Match | Pending → Assessed. A mismatched assessment input hash makes it Stale until reassessed. |
| Artifact | Extracting → Indexed or Extraction-failed. Failure is retryable. |
| Outreach Draft | Active or Discarded. Copying creates repeatable **CopiedOut** audit events. |
| Background Job | Queued → Running → Completed, Failed, or Cancelled. Cancellation stores completed and total item counts; it is not a separate partial state. A job found Running after restart becomes Failed with reason **interrupted** and may be retried manually. |

Cancelling an assessment batch retains independently completed per-role results. Cancelling a single artifact's indexing removes that attempt's derived data and records a retryable extraction failure.

### Deletion invariants

| Action | Final rule |
| --- | --- |
| Delete initiative | Deletes its criteria, matches, drafts, jobs, and audit events. It never deletes shared candidates, roles, companies, contacts, or recruiter-added artifacts. |
| Delete candidate | Blocked while any Active or Archived initiative references the candidate. Referencing initiatives must be deleted first. Deletion then removes the Candidate Profile, structured candidate data, candidate-only artifacts, aspects, embeddings, and derived retrieval data. |
| Candidate artifact shared elsewhere | Candidate deletion is blocked until the recruiter chooses global artifact deletion or explicitly retains the artifact under its other links after being warned it may contain candidate information. |
| Detach recruiter-added artifact | Removes one link only. Bytes and all other links remain untouched. |
| Delete recruiter-added artifact globally | Lists every existing link before confirmation, then removes every link and all derived data. |
| Exa source artifact | Role-owned and read-only. It cannot be independently detached or globally deleted; purge the role instead. |
| Orphaned recruiter-added artifact | Retained visibly in the artifact library until explicitly deleted. |
| Purge discovered role | Global. Lists referencing initiatives, then deletes the role, current and historical source artifacts, Role Profile, aspects, embeddings, matches, and active drafts. Recruiter-authored notes survive with an unavailable role reference. Metadata-only CopiedOut events survive within their initiative with the role reference cleared. |
| Delete draft | Deletes draft content. Existing CopiedOut audit events survive within the initiative with their draft reference cleared. |
| Every deletion | Is blocked, link-only, or transactional and cascading. A scoped verification query proves the deleted entity and exclusively owned evidence no longer appear in retrieval or matching. |

## Artifact storage and extraction

SQLite using the existing CGO-free glebarez/modernc stack is the system of record for CRM data and original artifact bytes.

### Artifact records

Each ingestion creates one Artifact record. PoC artifacts are not deduplicated, even when SHA-256 hashes match, because filename, source, and capture time are independent evidence provenance.

Artifacts store:

- original bytes as a SQLite BLOB;
- immutable original filename, detected media type, byte length, SHA-256, source, and capture time;
- editable display name;
- extraction state and error;
- extractor and chunker names, versions, and parameters; and
- links to initiatives or records through **artifact_links** without copying bytes.

The default per-file limit is 25 MB.

### Extraction sidecar

PDF and DOCX artifacts are converted to Markdown by a bundled Microsoft MarkItDown PyInstaller one-dir sidecar, one file per subprocess. Plain text, Markdown, and pasted text bypass the sidecar.

The contract is:

- absolute input path;
- Markdown on stdout;
- structured errors on stderr;
- non-zero exit codes for failure;
- one file per process; and
- a maximum of 10 MB extracted Markdown per artifact.

The app creates a randomly named, current-user-only temporary directory inside the selected data folder. Candidate names and other identifiers never appear in temporary paths. The input file is removed after each run, and abandoned extraction directories are swept at every startup.

The subprocess runs under a Windows Job Object with timeout, memory, and process-tree limits. Timeout, memory-limit, or output-limit violations terminate the process tree and record a retryable failure. This is failure isolation, not an exploit sandbox: the sidecar runs with the recruiter's user permissions. Plugins and MarkItDown network-dependent features are disabled, invocation uses only the verified absolute path in the install directory, and output is treated as untrusted text.

Scanned or image-only PDFs fail with a clear retryable message. OCR and batch extraction are P1. The PoC contract does not reserve or pre-design batch mode.

### Source observations

When Exa rediscovers a role:

- an unchanged content hash updates **retrieved_at** without creating an artifact;
- changed content creates a new immutable current source artifact;
- the previous source link becomes historical;
- historical source artifacts remain visible for provenance but are excluded from current retrieval and matching; and
- the Role Profile and affected matches become Stale.

Purging the role deletes all current and historical role-owned source artifacts.

## Profiles, indexing, and retrieval

### Structured decomposition

The **classify** role produces constrained JSON profiles from candidate artifacts, candidate structured data, and role source content.

Classifier output rules:

- every extracted aspect must include supporting source citations;
- unclear values remain absent or unknown;
- role priority is unspecified unless the source supports must-have or nice-to-have;
- unsupported aspects fail validation;
- output must match the versioned profile schema; and
- invalid output receives one repair retry, after which extraction becomes Failed and retryable.

A Candidate Profile begins Proposed and must be recruiter-approved before first search or matching. Recruiter-approved aspects are never silently overwritten. Re-extraction produces a proposed diff of additions, removals, and conflicts. Each aspect records whether it is extracted or recruiter-authored.

A recruiter-authored fact may exist without an artifact but is visibly labeled **Recruiter supplied**. It may be cited by matches and drafts as a recruiter-authored record rather than document-verified evidence.

Role Profiles are created automatically because per-role approval would defeat the workflow. They remain editable. A failed Role Profile stays visible in Research, supports retry or manual entry, and is excluded from automatic assessment until Ready. A failed Candidate Profile may be retried or built manually; search and matching remain blocked until an initial profile is Approved.

### Chunking

Extracted Markdown is chunked with a fixed algorithm: headings, lists, and paragraphs first, followed by sentence segmentation to a target size. Chunks record artifact ID, ordinal, text, character offsets, heading-path locator, token count, content hash, and chunker version and parameters.

### Lexical and semantic indexes

- SQLite FTS5 indexes source chunks and Profile Aspect text through explicit SQL migrations, triggers, and a rebuild path.
- A startup smoke test creates and queries an FTS5 table.
- Source chunks and semantic Profile Aspects are embedded as separate retrieval units.
- Embeddings are portable little-endian IEEE-754 float32 BLOBs keyed by source identity, content hash, and embedding space.
- An embedding space records endpoint configuration revision, model digest or immutable revision, dimensions, and metric.
- Candidate-content embeddings are always generated locally.
- The PoC uses exact cosine scanning in Go. A packaged vector extension is considered only if the representative benchmark proves the scan inadequate.
- Scores are never compared across embedding spaces.

Aspect KNN follows this compatibility map:

| Role aspect | Candidate aspects searched |
| --- | --- |
| skill | skill, experience, responsibility |
| responsibility | responsibility, experience, skill |
| experience | experience, responsibility |
| qualification | qualification |
| seniority | seniority, experience |
| other | other |

Location, work arrangement, work rights, employment type, and compensation use deterministic structured comparison against candidate facts or initiative Search Criteria.

### Hybrid shortlist

1. Exclude only out-of-scope, deleted, and Stale roles.
2. Run FTS5 and exact-cosine top-K searches for each approved Search Criterion and compatible Candidate Profile aspect against role chunks and aspects.
3. Fuse the ranked lists using reciprocal-rank fusion and group results by role.
4. Send the top 20 roles to assessment.

Must-have failures such as location or work-rights conflicts are assessed and shown; they do not silently filter an otherwise retrieved role out.

Deletion transactions remove the affected chunks, FTS rows, aspect embeddings, source embeddings, profiles, and cached matches. Verification queries are scoped to the deleted entity so intentionally shared evidence is not mistaken for deletion failure.

Virtual tables and triggers require explicit SQL migrations. The current model-only AutoMigrate behavior is insufficient and must be replaced by a migration step. The research basis lives in [docs/research/recruiting-data-and-sqlite-rag.md](../research/recruiting-data-and-sqlite-rag.md).

## Role discovery and source policy

Exa searches target role listings only. A generated query excludes candidate name, email, phone, street address, and other direct identifiers. Exact employers, clients, projects, and schools are generalized by default. The recruiter may deliberately add a specific organization in the visible query editor; direct-identifier detection produces an additional warning before sending.

Talent Hound creates or updates a source-attributed Role using source ID, then canonical URL, then content fingerprint for identity. It caches the source fields and content supplied by Exa when permitted.

Direct automated page fetching is available only for a developer-maintained allowlist of publicly accessible sources whose applicable access rules have been reviewed. It is rate-limited and cannot be user-overridden. SEEK, LinkedIn, authenticated pages, blocked pages, robots-disallowed paths, anti-bot challenges, and browser-control workflows are not automated. When Exa lacks sufficient text, the recruiter may navigate to the source and manually paste or attach content.

A discovered role becomes Stale after 30 days without retrieval or when a stated closing date passes. Stale roles are visibly labeled and excluded from matching. A recruiter may purge one role or all Stale roles under the deletion invariants.

## Matching

Matching has two distinct directions:

1. **Role fit for candidate:** the Role Profile is assessed against the initiative's Search Criteria and candidate preferences.
2. **Candidate fit for role:** the approved Candidate Profile is assessed against the Role Profile's extracted requirements.

For each semantic role requirement, exact-cosine KNN retrieves compatible Candidate Profile aspects and underlying evidence chunks. Structured constraints are compared deterministically. The **generate** role receives the requirement, retrieved evidence, and source citations and returns **met**, **not met**, or **unknown**. It must cite evidence for **met**, cite contrary evidence for **not met** when available, and explicitly state when no evidence was found.

KNN similarity is evidence selection, not a decision or final score. Fit reasons, gaps, and missing information derive from per-aspect results. Unspecified role requirements are assessed and displayed but do not count as must-have or nice-to-have in ranking unless the recruiter changes their priority.

### Deterministic ranking

Assessed matches are ordered by:

1. no unmet must-haves on either fit direction;
2. fewer total unmet must-haves across both directions;
3. fewer total unknown must-haves;
4. more total met nice-to-haves;
5. higher reciprocal-rank-fusion retrieval position; and
6. stable Role ID.

Search Criterion ordering controls presentation, not weighting. Explicit weights are deferred until recruiter testing demonstrates a need. Must-have failures sort a match down but never hide it.

### Assessment validity

Each assessment stores one **assessment_input_hash** computed over:

- approved Candidate Profile version and lifecycle state;
- Role Profile version and lifecycle state;
- Search Criteria version;
- exact evidence chunk and aspect content hashes;
- structured-comparison and ranking-rule version;
- generation endpoint configuration revision;
- actual model digest or immutable revision;
- prompt-template and output-schema versions;
- generation parameters; and
- role staleness state.

The stored assessment is valid only while recomputation matches. This is the sole caching and staleness rule for matches.

Assessments run as cancellable background jobs with visible progress. Completed per-role results survive batch cancellation because each is independently valid under its hash.

### Prohibited criteria

The structured taxonomy contains no protected-attribute fields. A deterministic category and term check blocks explicit protected criteria. The local **classify** role flags ambiguous or potential proxy criteria for recruiter review but does not hard-block them because model judgments may be wrong. Generation is instructed never to infer protected attributes. Blocking and warning behavior is covered by small acceptance fixtures.

The provisional protected-criteria list covers age, sex, gender identity, sexual orientation, race or national origin, religion, disability, family or carer status, pregnancy, marital status, political opinion, and union membership. It requires specialist confirmation before public release.

Work rights are supported as a first-class operational field because recruiters need to establish whether a candidate may perform a role. The field must not be used as a proxy for nationality or national origin. The permitted wording, handling of visa categories, and jurisdictional boundaries require specialist confirmation before public release.

Because free text and indirect proxies cannot be completely policed by software, lawful criteria selection remains the recruiter's responsibility. The criteria editor states this once, plainly.

## AI and provider behavior

### Model registry

The registry provides required local assignments for:

- **embed:** source chunk and Profile Aspect embeddings;
- **classify:** Candidate Profile and Role Profile decomposition, plus prohibited-criteria warnings; and
- **generate:** assessment, summaries, drafts, and chat.

**Classify** defaults to the local **generate** model. A dedicated smaller classifier is a P1 tuning option, not a PoC requirement. Each assignment records endpoint configuration revision, model name and immutable digest when available, parameters, validation status, and supported output behavior.

Default models are labeled **Validated** only after passing the held-out profile and matching benchmarks on the target laptop. Custom models are labeled **Unvalidated**. The app does not claim to detect generic model reliability at runtime; schema errors, memory failures, and timeouts are reported directly.

### Local and cloud boundaries

Local Ollama assignments are always present. The optional cloud endpoint is a task-level override, never a replacement for the required local configuration.

- Raw candidate artifacts, Candidate Profile extraction, and embeddings are local-only.
- Public Role Profile extraction may use an approved cloud override.
- Cloud assessment and drafting receive only approved Candidate Profile aspects and selected evidence snippets, with known structured direct identifiers replaced by placeholders.
- Cloud chat requires explicit payload selection and preview for each send.
- Broader free-text PII detection and redaction is P1.

Cloud consent is bound to the exact initiative, endpoint, and eligible task type: role extraction, assessment, drafting, or chat. First use shows the actual payload. Later payloads remain previewable. Changing the endpoint resets approvals, and the recruiter can revoke any approval. Exa retains per-query preview and confirmation.

Every non-localhost request creates a local audit event containing timestamp, provider, task, information categories, initiative, and nullable record references. Audit events never store request payload, query content, document content, or draft text. Exa Search records may retain their visible query within the owning initiative for reproducibility.

Provider keys live in Windows Credential Manager or macOS Keychain, are masked after entry, and never enter application data, logs, diagnostics, or directory-copy recovery. Removing a credential disables the provider without deleting local information.

External failure never blocks CRM, artifacts, profiles, local retrieval, or local generation.

## Security, privacy, and data protection

### PoC release gates

- The selected data-folder volume is encrypted and checked at every startup. Real personal-data workflows remain unavailable otherwise.
- Credentials live only in Windows Credential Manager or macOS Keychain.
- No candidate artifact, profile, or CRM content leaves the device merely because it was added.
- Exa and cloud disclosures follow the provider rules and are locally audited.
- Candidate data can be inspected, corrected, and deleted together with its profiles and derived retrieval data.
- Logs and diagnostics exclude document contents, candidate details, queries, payloads, draft contents, and credentials.
- Extraction temporary files stay within the selected encrypted data folder and are cleaned after use or at the next startup.
- The application contains no outbound-message transport.

An optional empty demo workspace may be used on an unencrypted volume, but it must reject real candidate artifacts and personal-data entry and is not a PoC acceptance environment.

### PoC data-handling preconditions

At first run the recruiter acknowledges:

- their authority to hold and use candidate data they load;
- their retention and deletion responsibilities;
- that public information is used for evaluation and recruiter-controlled outreach, not republication;
- that Exa queries may disclose generalized professional information and are always previewed;
- that optional cloud tasks have separate payload and consent controls; and
- the prohibited-criteria boundary and recruiter responsibility.

A full specialist review of Australian privacy, employment, discrimination, and electronic-messaging obligations is required before any public release. It is not a gate for this one-recruiter PoC, but the PoC does not claim exemption from applicable obligations.

## Product operations

### Data locations

The selected data folder contains all recruiter content, CRM records, artifacts, profiles, aspects, indexes, audit events, background-job state, migration snapshots, and redacted logs.

Application binaries and the extraction sidecar live in the install directory. Credentials live in Windows Credential Manager or macOS Keychain. Ollama and downloaded models live in Ollama-managed storage.

Only the selected data folder must be copied for data recovery. Credentials must be re-entered and missing models re-downloaded.

### First run

1. Choose the data folder.
2. Verify encryption on its volume; block real-data mode until enabled.
3. Verify the bundled extraction sidecar and pinned version.
4. Verify Ollama.
5. Show required model names and download sizes, approximately 4–8 GB total, and pull missing models.
6. Acknowledge PoC data-handling preconditions.
7. Create the first Job Search Initiative.

### Interim data safety and recovery

The app documents how to copy the selected data folder while Talent Hound is fully closed. Before every schema migration it creates a database snapshot in that folder.

Self-service recovery before P1 backup is:

1. reinstall Talent Hound and Ollama;
2. select the previously copied data folder;
3. run SQLite integrity and schema-version checks;
4. snapshot the recovered database before applying migrations;
5. restore the snapshot and leave the folder unopened if migration fails; and
6. re-enter provider credentials and re-download missing models.

A failed integrity check or migration never opens a partially recovered database or overwrites the recruiter's only copy.

### Updates, uninstall, and diagnostics

- Updates use a developer-supplied installer.
- The application displays its version.
- Migrations run automatically after snapshot and roll back to that snapshot on failure.
- Uninstall uses the standard uninstaller and documents the data-folder location.
- An in-app **delete all data** action lists the exact selected data folder and requires confirmation.
- Diagnostics are local and redacted, with an **open logs folder** action.
- No telemetry is enabled.

## UX requirements

- The left sidebar lists initiatives; opening one creates a browser-style tab.
- Each initiative tab has **Context**, **Research**, **Matches**, and **Drafts**, plus initiative-scoped chat.
- Context exposes the Candidate Profile, Role Profiles, Search Criteria, records, and artifacts with their approval, source, and staleness state.
- The active initiative, data scope, selected local models, any cloud override, and online/offline state are always visible.
- Every profile aspect, match result, and factual draft claim can be traced to evidence without leaving the screen.
- AI-generated, recruiter-authored, and source-derived content are visually distinct.
- Chat proposes structured changes; the recruiter explicitly applies them.
- Destructive actions follow the deletion-invariants table and state every affected link or record before confirmation.
- Long-running extraction, profile creation, indexing, search, and assessment show job state, progress, completed-item counts, cancellation, failure, and retry.
- Failed Role Profiles remain visible rather than disappearing from results.

## PoC scope

### P0 — required

| ID | Requirement |
| --- | --- |
| FR-01 | Create, rename, archive, reopen, and delete initiatives of all three existing types. Talent Search and Business Development remain workspace shells without their P1 pipelines. |
| FR-02 | Create and maintain the defined structured Candidate and Role fields plus minimal Company and Contact records sufficient for contacts-at-company lookup. Resume drag-in may create a Candidate. |
| FR-03 | Attach PDF, DOCX, plain-text, Markdown, and pasted-text artifacts; store one immutable Artifact per ingestion as a SQLite BLOB; view, edit display name, detach, and delete under the deletion invariants. |
| FR-04 | Extract PDF and DOCX through the single-file MarkItDown sidecar contract; perform fixed structural chunking; index chunks and aspects with FTS5 and local float32 embeddings; provide exact-cosine and hybrid retrieval. |
| FR-05 | Use the **classify** role to create versioned Candidate and Role Profiles under the aspect taxonomy, evidence rules, approval rules, failure behavior, and classifier validation contract. |
| FR-06 | Provide initiative-scoped Q&A and summaries over approved local context with citations. |
| FR-07 | Run recruiter-initiated Exa role searches with identity-safe generated queries, per-query preview, provenance-stamped cache, source-acquisition policy, staleness, source observations, and purge. |
| FR-08 | Produce a top-20 hybrid shortlist and two-directional criteria assessment using the KNN evidence, deterministic ranking, background-job lifecycle, and assessment input hash defined here. |
| FR-09 | Create editable evidence-backed pitches and outreach drafts. Copy-out creates metadata-only audit events; the application cannot send messages. |
| FR-10 | Provide required local **embed**, **classify**, and **generate** assignments plus one optional cloud endpoint under the validation, disclosure, and task-boundary rules. |
| FR-11 | Store Exa and cloud credentials in Windows Credential Manager or macOS Keychain and provide visible provider configuration and revocation. |
| FR-12 | Enforce every deletion invariant and verify scoped removal from profiles, retrieval, matching, and candidate-owned evidence. |
| FR-13 | Provide the BitLocker gate, first-run flow, migration snapshots, closed-app recovery procedure, manual updates, uninstall information, and redacted diagnostics. |

### P1 — after the PoC proves the loop

- encrypted, authenticated backup export and restore as the first item;
- CSV import with mapping and resume folder or ZIP linking after auditing the recruiter's data;
- batch extraction designed with that import workflow;
- OCR for scanned or image-only resumes;
- a dedicated classifier model if benchmarks justify it;
- embedding-similarity chunk boundaries if retrieval testing justifies them;
- company, profile, and news search types;
- browser or Chrome-assisted capture and any additional reviewed direct-fetch sources;
- broader automatic free-text PII detection and cloud-payload redaction;
- global cross-initiative Q&A;
- full Company and Contact CRM behavior;
- Talent Search and Business Development pipelines;
- candidate reuse from closed searches;
- richer relationship and interaction records;
- outreach templates;
- Talent Hound ZIP and HR Open TCP 4.5 import/export;
- legal and commercial document analysis;
- macOS packaging; and
- multiple cloud endpoints.

### Out of scope

- autonomous or bulk outreach;
- authenticated-page scraping or bypassing access controls, robots rules, or anti-bot systems;
- automated SEEK or LinkedIn page fetching;
- Chrome control or browser automation in the PoC;
- access to the recruiter's private LinkedIn connection graph;
- hosted, multi-user, or shared CRM;
- full ATS workflows such as scheduling, offers, and onboarding;
- legal advice;
- autonomous placement decisions; and
- cloud processing of raw candidate artifacts or candidate embeddings.

## Success criteria

### Flagship acceptance

The chosen validation candidate must have a market in which Exa returns at least ten eligible live roles. If fewer than ten are returned, the scenario is **inconclusive due to source coverage**, not a product pass or failure.

Using local Ollama models only, the recruiter:

1. drops the candidate's real resume into a Job Search Initiative;
2. reviews and approves the Candidate Profile and Search Criteria;
3. previews and sends an Exa role query;
4. receives at least ten Active roles with Ready Role Profiles;
5. receives assessments for those ten roles, with per-aspect results and a citation for every result marked met;
6. rates at least three of the top five roles plausible; and
7. produces at least one **usable draft**: a pitch they state they would send after their edits, containing no fabricated factual claims.

A cloud-assisted run may diagnose whether a failure arises from local model quality or the wider pipeline. It can establish **product loop validated; local-runtime hypothesis failed**, but it does not pass the PoC.

### Additional gates

The PoC also passes when the recruiter can:

1. disconnect from the internet and continue using CRM, artifacts, approved profiles, local retrieval, Q&A, and generation;
2. show that the application has no message-sending capability;
3. purge a Stale discovered role and verify its source-derived content, profiles, matches, and active drafts are absent from retrieval;
4. delete every initiative referencing a candidate, delete that candidate, resolve any shared artifacts, and verify candidate-owned information is absent from retrieval;
5. confirm that every non-local request during acceptance had the required preview or task approval;
6. pass the held-out matching benchmark;
7. pass the held-out classifier benchmark;
8. reject real-data mode on an unencrypted volume; and
9. recover a copied data folder without corrupting or partially migrating it.

### Matching benchmark

Five scenarios come from the recruiter's actual past placements. Each uses an anonymized resume and a frozen role-listing corpus stored as artifacts. Scenarios are held out of prompt and model tuning. The recruiter rates the top five roles plausible or not plausible. Duplicate roles collapse before rating, and absent slots are not plausible.

Pass: at least three of the top five are plausible in at least four of five scenarios.

### Classifier benchmark

The held-out classifier corpus uses five resumes and twenty representative role listings from the frozen scenarios. Before model execution, the recruiter labels material aspects.

Pass:

- every extracted aspect has a valid citation;
- no unsupported must-have, location, work-rights, employment-type, or compensation value is introduced;
- at least 80% of recruiter-labeled material aspects are captured; and
- explicit structured constraints are reproduced correctly.

### Indicative measurements

These inform future prioritization but do not determine PoC acceptance:

- time from initiative start to shortlist and draft compared with the recruiter performing the same frozen scenario manually, using three timed trials each; and
- the proportion of generated drafts the recruiter chooses to use.

## Non-functional requirements

Functional quality, security, offline behavior, deletion, and recovery are acceptance gates. Timing and volume targets are provisional measurements on the target laptop.

- **Platform:** Windows 11 x64, CPU-only inference, 16 GB RAM.
- **Search:** hybrid retrieval P95 below 2 seconds on a representative derived corpus from approximately 1,000 candidates and 1,000 Active roles.
- **Assessment:** one match below 60 seconds; twenty-role assessment below 20 minutes as a background batch.
- **Profile decomposition, indicative only:** Candidate Profile below 3 minutes; one Role Profile below 30 seconds; twenty Role Profiles below 10 minutes; role profiling plus assessment of twenty roles below 30 minutes.
- **Indexing:** one resume extracted and indexed below 60 seconds; a synthetic or migrated 1,000-resume index completes overnight, below 8 hours.
- **Application:** cold start below 5 seconds excluding Ollama; database below 5 GB at the representative corpus; artifact input at most 25 MB; extracted Markdown at most 10 MB.
- **Recovery:** interrupted jobs become explicitly retryable or roll back their current item; migration snapshots restore cleanly.
- **Offline:** CRM, artifacts, profiles, retrieval, Q&A, and local generation work without network after models are installed.
- **Accessibility:** core flows are keyboard-operable; a full WCAG audit is beyond the PoC.
- **Explainability:** source facts, recruiter-authored facts, and AI inference are always distinguishable.

## Delivery sequence

1. **Secure retrieval foundation:** explicit SQL migrations and snapshots, BitLocker gate, artifact ingestion, sidecar contract, fixed chunking, FTS5, local embeddings, exact cosine retrieval, Ollama roles, and data-folder recovery.
2. **Structured profiles:** Candidate Profile, Role Profile, Profile Aspects, approval and stale-profile behavior, classifier fixtures, and held-out classifier benchmark.
3. **Flagship loop:** Search Criteria, identity-safe Exa role search, source observations, aspect retrieval, two-directional matching, deterministic ranking, and evidence-backed drafts.
4. **Hardening and acceptance:** deletion invariants, audit history, first-run flow, frozen matching benchmark, source-coverage scenario, offline test, and target-laptop measurements.
5. **P1:** recruiter-prioritized work beginning with encrypted backup export and restore.

## Risks and accepted trade-offs

| Risk | Product response |
| --- | --- |
| CPU-only models are too slow or weak. | Benchmark early on the actual laptop, keep generation evidence small, cache by complete input hash, and use cloud only as an explicitly approved diagnostic escape hatch. Local-only quality remains a PoC gate. |
| No PoC backup facility means disk failure can lose data. | Documented closed-app folder copy and automatic pre-migration snapshots; encrypted backup is first in P1. |
| Exa coverage is insufficient or stale. | Treat low-yield live acceptance as source-coverage inconclusive, use frozen quality benchmarks, retain broader permitted public sources, and never depend on authenticated data. |
| A source forbids bots or automated access. | Use Exa content, a reviewed developer allowlist, or recruiter-supplied pasted content. Never automate SEEK, LinkedIn, blocked pages, or anti-bot challenges. |
| Profile classification omits or invents requirements. | Constrained schema, mandatory citations, one repair retry, visible failures, recruiter approval for candidates, editable role profiles, and a held-out classifier benchmark. |
| Prompt injection appears in documents or web content. | Treat retrieved content as quoted evidence only, separate it from instructions, validate structured output, and retain human confirmation for state changes and external actions. |
| FTS5 or migrations fail in the CGO-free SQLite stack. | Startup FTS5 smoke test, explicit migrations, pre-migration snapshots, and a prototype before any vector extension. |
| The PyInstaller sidecar triggers antivirus or extracts poorly. | One-dir build, pinned versions, verified path and version, clear retryable failures, and code-signing if needed. |
| A malicious document exploits MarkItDown. | The sidecar provides failure isolation only. Plugins and network features are disabled, but it runs with user permissions; this risk is accepted for the one-user PoC. Hardened sandboxing is a P1 consideration. |
| Ranking amplifies missing data or unlawful criteria. | Two-directional per-aspect evidence, explicit unknowns, deterministic ranking, blocked explicit protected criteria, classifier warnings, and recruiter responsibility. |
| Optional cloud use discloses personal information. | Raw candidate files, candidate extraction, and embeddings remain local; task-specific approval, payload preview, identifier placeholders, audit metadata, and revocation constrain eligible cloud tasks. |
| An unencrypted data volume exposes plain SQLite BLOBs. | BitLocker or Windows Device Encryption is a hard gate for real-data mode and is checked every startup. |

## Implementation inputs and validation tasks

These items must be resolved during delivery but do not change the final PoC scope:

1. Audit the recruiter's current ATS, spreadsheets, resume folders, candidate count, and data quality to design P1 import.
2. Select and pin the local embedding and 3–8B instruct models after target-laptop benchmarks.
3. Agree the monthly Exa budget and cache policy within provider terms.
4. Seed the default Search Criteria template from the recruiter's Australian technology-role practice.
5. Select and freeze the five past-placement benchmark scenarios and twenty classifier role listings before tuning.
6. Measure how often scanned or image-only resumes occur to prioritize OCR.
7. Obtain specialist confirmation of prohibited-criteria, work-rights, privacy, retention, and messaging wording before public release.
8. Review access rules and caching rights before adding any source to the direct-fetch allowlist.

## Existing product foundation

The repository already provides a Wails desktop application with a SolidJS interface, a local SQLite database using GORM and glebarez/modernc, creation and navigation of Job Search, Talent Search, and Business Development initiatives, and a placeholder workspace for each initiative.

The initiative workspace remains the seam for the flagship loop. Candidate, Role, Company, Contact, and recruiter-added Artifact records are shared objects that initiatives reference, so deleting an initiative does not silently discard reusable talent-pool knowledge.
