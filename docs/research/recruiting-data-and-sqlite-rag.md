# Recruiting data import and SQLite RAG

Decision note, researched 2026-08-15. “Standard” below means a published specification; “design inference” means a Talent Hound product/engineering choice.

**Scope note:** This research predates the Final v0.5 product contract. Its import recommendations now apply to P1, not the PoC, and [the PRD](../product/PRD.md) is authoritative wherever delivery scope or terminology differs.

## Recommended decisions

1. **V1 portable import: UTF-8 CSV with a header and interactive column mapping, plus an optional ZIP/folder of resumes referenced by relative path.** This is a practical interchange baseline, not a claim that CSV has a universal recruiting schema. Bullhorn's official importer accepts Candidates, Contacts, and Leads from CSV, while Workable accepts a CSV template and associates resumes supplied by URL or a matching file in a ZIP/folder ([Bullhorn](https://kb.bullhorn.com/ats/Content/BHATS/Topics/importingDataToBullhorn.htm), [Workable](https://help.workable.com/hc/en-us/articles/115012497707-Importing-candidate-data-from-spreadsheet)).
2. **Use HR Open Standards as the canonical vocabulary and a future JSON interchange option, not as the only V1 importer.** HR Open publishes free global HR vocabularies; its recruiting material includes Candidate, Position Opening, and Search Document schemas, and its current 4.5 Trusted Career Profile covers portable employment, skills, education, and credential data ([downloads](https://www.hropenstandards.org/standards-downloads), [Recruiting schemas](https://www.hropenstandards.org/products/HR-XML-Recruiting-Course), [Trusted Career Profile](https://www.hropenstandards.org/news/official-release-of-the-trusted-career-profile-tcp)). The official sources establish a standard but do **not** establish that Australian ATS/CRM products commonly export it; the downloadable artifacts also require an account and acceptance of HR Open's IP agreement. Supporting HR Open files is therefore a post-V1 interoperability feature, after reviewing the actual 4.5 package and licence.
3. **Keep imported source data separate from derived RAG data.** Candidate fields and original documents are durable records; extracted text, chunks, FTS entries, and embeddings are reproducible indexes. Do not put chunks or embeddings in the CSV contract.
4. **Store original documents in SQLite BLOBs and extracted/chunk text as TEXT.** SQLite defines BLOB as bytes stored exactly as supplied, but this is storage rather than encryption ([SQLite datatypes](https://www.sqlite.org/datatype3.html)). Enforce an application upload limit well below SQLite's default 1,000,000,000-byte string/BLOB limit ([SQLite limits](https://www.sqlite.org/limits.html)).
5. **Require FTS5 for lexical retrieval; do not require a vector extension for V1 correctness.** Use an application-owned, little-endian IEEE-754 float32 BLOB format and an exact cosine scan in Go for the first independent-recruiter-sized corpus. Add a SQLite vector index only when a corpus/latency benchmark shows the scan is inadequate. This preserves the current CGO-free Windows/macOS build.
6. **Do not adopt official `vec1` as a production dependency yet.** It is the preferred extension to reassess because it is maintained by SQLite, supports cosine/L2 and exact/ANN modes, and maps vectors to SQLite rowids. As of this note its current release is 0.7, its own roadmap says testing is insufficient, and it must be built as a native shared library with CPU/platform considerations ([vec1 overview and roadmap](https://sqlite.org/vec1/doc/trunk/doc/vec1.md), [vec1 manual](https://sqlite.org/vec1/doc/trunk/doc/vec1intro.md)).

## Import contract

### CSV rules

RFC 4180 documents the common CSV shape and registers `text/csv`, but it is explicitly Informational rather than an Internet Standard. It specifies optional headers, consistent field counts, comma separation, double-quoting for commas/newlines/quotes, and doubled quotes inside quoted fields ([RFC 4180](https://www.rfc-editor.org/info/rfc4180/)). The W3C CSVW Recommendation likewise notes that no single CSV standard exists and recommends `text/csv`, UTF-8, a header row, consistent row widths, and RFC-style escaping ([W3C tabular-data model, §7](https://www.w3.org/TR/tabular-data-model/#best-practice-csv)).

**Design inference:** emit RFC-4180-compatible UTF-8 CSV; accept CRLF or LF and an optional UTF-8 BOM. Use Go's standard `encoding/csv`; do not add XLSX support until real imports require it. A user-editable column mapper is more portable than vendor-specific hard-coded templates because Bullhorn maps CSV fields while Workable requires its own exact headings ([Bullhorn](https://kb.bullhorn.com/ats/Content/BHATS/Topics/importingDataToBullhorn.htm), [Workable](https://help.workable.com/hc/en-us/articles/115012497707-Importing-candidate-data-from-spreadsheet)). W3C CSVW defines JSON metadata for column types, validation, and mapping, but implementing the full vocabulary is unnecessary for V1 ([CSVW metadata vocabulary](https://www.w3.org/TR/tabular-metadata/)).

### Canonical candidate mapping

This is a **Talent Hound design inference**, informed by HR Open's Candidate/TCP scope rather than copied from its gated schemas:

| Group | Canonical targets | Import behaviour |
| --- | --- | --- |
| Provenance | `source_system`, `source_record_id`, `source_url`, `imported_at` | Preserve when supplied; generate an internal ID regardless. |
| Identity | `first_name`, `last_name`, `preferred_name` or `full_name` | Accept a row when it has a usable identity, contact method, profile URL, or attached resume; flag incomplete records instead of discarding them. |
| Contact | primary `email`, primary `phone` | Optional; never use email alone as the database primary key. |
| Professional | `current_title`, `current_employer`, `location`, `skills`, `tags` | Optional and searchable. Keep the untouched source row for audit/remapping rather than inventing a nested CSV convention for multi-value fields. |
| CRM | `status`, `owner_notes`, `last_contacted_at` | Optional; dates are ISO 8601 on export. |
| Document link | `resume_path` | Optional relative path into the selected ZIP/folder; reject absolute paths and traversal. V1 supports one primary resume per row; additional artifacts can be attached after import. |

The original CSV and an import report should be retained as artifacts so failed or partially mapped imports are explainable. Deduplication should propose matches from source ID, contact details, profile URL, and name/employer, but require confirmation before merging; this is a safety-oriented design choice, not an industry standard.

## Minimum SQLite RAG structure

There is no published RAG database schema standard in the sources reviewed. The following is a **Talent Hound design inference** that keeps source evidence, citations, re-indexing, and provider changes recoverable:

| Record | Minimum fields | Why retained |
| --- | --- | --- |
| `artifacts` | `id`, `original_name`, `media_type`, `byte_length`, `sha256`, `content_blob`, `created_at`, extraction status/error, extractor name/version, extracted text | Original evidence plus enough provenance to detect duplicate/changed content and rerun extraction. BLOB and TEXT are distinct SQLite storage classes ([SQLite datatypes](https://www.sqlite.org/datatype3.html)). |
| `artifact_links` | `artifact_id`, linked entity kind/id | A resume or terms document may contextualize a candidate, job, company, or initiative without duplicating bytes. |
| `chunks` | `id` (integer rowid), `artifact_id`, `ordinal`, `text`, page and/or character offsets, `token_count`, `content_sha256`, chunker name/version and chunk-size/overlap parameters | Stable ordering and source offsets support citations; hashes and algorithm parameters make stale derived data detectable. |
| `embedding_spaces` | `id`, provider configuration ID, model ID, dimensions, distance metric | OpenAI-compatible endpoints/models can produce incompatible vector spaces. `vec1` likewise fixes a table's vector byte length after the first insert ([vec1 reference](https://sqlite.org/vec1/doc/trunk/doc/vec1ref.md#the-virtual-table)). |
| `chunk_embeddings` | `chunk_id`, `embedding_space_id`, little-endian IEEE-754 float32 BLOB, embedded chunk hash, `created_at`; unique on `(chunk_id, embedding_space_id)` | Allows re-embedding without overwriting another model's index and proves which chunk version was embedded. |

Record chunking parameters rather than assuming a universal tokenizer. As one primary-source precedent, OpenAI vector-store files record a chunking strategy and currently default to 800-token chunks with 400-token overlap; that is a provider default, not a RAG standard ([OpenAI vector-store file API](https://platform.openai.com/docs/api-reference/vector-stores-files/createFile)).

Use an external-content FTS5 table over `chunks.text`, keyed by chunk rowid, with database triggers for insert/update/delete and an explicit rebuild after backfilling existing chunks. SQLite's documentation says external-content indexes avoid copying the source text, but the application is responsible for consistency and recommends triggers/rebuilds ([FTS5 external-content tables](https://www.sqlite.org/fts5.html#external_content_tables), [consistency pitfalls](https://www.sqlite.org/fts5.html#external_content_table_pitfalls)).

Retrieval should filter candidates/artifacts by ordinary relational metadata, retrieve lexical candidates via FTS5 and semantic candidates from one selected embedding space, then merge/rerank them. **Design inference:** never compare or combine raw distance scores from different embedding spaces.

## Current Go/SQLite compatibility

- The repository currently uses GORM with `github.com/glebarez/sqlite` and resolves `modernc.org/sqlite` v1.44.3; database setup only auto-migrates the Initiative model ([`go.mod`](../../go.mod), [`internal/db/db.go`](../../internal/db/db.go)). The GORM driver is an embedded, CGO-free SQLite implementation tested by its maintainer on Windows and macOS ([glebarez/sqlite documentation](https://pkg.go.dev/github.com/glebarez/sqlite)).
- The resolved modernc generated sources enable `SQLITE_ENABLE_FTS5` for both Darwin and Windows targets ([Darwin arm64 source](https://gitlab.com/cznic/sqlite/-/blob/v1.44.3/lib/sqlite_darwin_arm64.go), [Darwin amd64 source](https://gitlab.com/cznic/sqlite/-/blob/v1.44.3/lib/sqlite_darwin_amd64.go), [Windows source](https://gitlab.com/cznic/sqlite/-/blob/v1.44.3/lib/sqlite_windows.go)). **Implementation gate:** add a small runtime test that creates and queries an FTS5 virtual table on both release platforms; virtual tables and their triggers require explicit SQL migrations rather than the current model-only `AutoMigrate` call.
- Official vec1 is a standalone C extension, so integrating it would introduce native binaries/builds beyond the current pure-Go setup ([vec1 build instructions](https://sqlite.org/vec1/doc/trunk/doc/vec1.md#building-the-extension)).
- `sqlite-vec` is a separate third-party extension and explicitly remains pre-v1 ([maintainer repository](https://github.com/asg017/sqlite-vec)). Modernc added a CGO-free transpiled copy in v1.47.0, later than this repo's v1.44.3 ([modernc changelog](https://gitlab.com/cznic/sqlite/-/blob/master/CHANGELOG.md)). If benchmark data forces a vector extension before vec1 is ready, spike a modernc upgrade plus `sqlite-vec` registration through the existing glebarez/GORM stack; do not assume compatibility from the transitive version alone.

## Acceptance checks implied by these decisions

1. Import a quoted UTF-8 candidate CSV with embedded commas/newlines and attach a matching resume from a ZIP without path traversal.
2. Round-trip source bytes from SQLite and verify the stored SHA-256.
3. Re-extract/re-chunk a document and prove stale embeddings are excluded by chunk hash/embedding space.
4. Query the same chunks by FTS5 and exact cosine search, with results resolving to source page/offset metadata.
5. Run the FTS5 smoke test and document round-trip on signed Windows and macOS builds before release.
