package db

// A migration is one schema version: an ordered batch of SQL statements applied
// atomically together with the `PRAGMA user_version` bump to Version.
//
// Rules for adding one:
//   - append only, never edit a released migration;
//   - Version is the previous Version + 1;
//   - plain `CREATE`/`ALTER` — the `IF NOT EXISTS` in migration 1 is a one-off
//     for databases created by the old AutoMigrate.
type migration struct {
	Version int
	Name    string
	SQL     []string
}

// migrations is the schema, in order. It is the only source of truth: the
// structs in internal/models are for querying, not for creating tables.
var migrations = []migration{
	{
		Version: 1,
		Name:    "baseline_initiatives",
		// ponytail: byte-for-byte what AutoMigrate produced, with IF NOT
		// EXISTS so existing dev/E2E databases are adopted at v1 instead of
		// needing a detection branch. Migration 2 onward is plain DDL.
		SQL: []string{
			"CREATE TABLE IF NOT EXISTS `initiatives` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`name` text NOT NULL," +
				"`type` text NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX IF NOT EXISTS `idx_initiatives_type` ON `initiatives`(`type`)",
		},
	},
	{
		Version: 2,
		Name:    "crm_records",
		// Exactly the structured fields the PRD names, and no others.
		// Calendar facts (availability, published, closing, retrieved,
		// last confirmed) are YYYY-MM-DD text, not timestamps: they describe
		// days in the world, not events in this database. The column type is
		// `text`, not `date`, so the driver hands them back as strings.
		// Empty string means "not stated" throughout, so no column is nullable
		// except the two optional foreign keys and the compensation amounts.
		SQL: []string{
			"CREATE TABLE `candidates` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`full_name` text NOT NULL," +
				"`preferred_name` text NOT NULL DEFAULT ''," +
				// ponytail: JSON arrays of strings; a child table when an
				// address needs its own attributes (primary, verified, source).
				"`emails` text NOT NULL DEFAULT '[]'," +
				"`phones` text NOT NULL DEFAULT '[]'," +
				"`location` text NOT NULL DEFAULT ''," +
				"`work_rights` text NOT NULL DEFAULT ''," +
				"`availability` text NOT NULL DEFAULT ''," +
				"`desired_employment_type` text NOT NULL DEFAULT ''," +
				"`desired_work_arrangement` text NOT NULL DEFAULT ''," +
				"`comp_min` integer," +
				"`comp_max` integer," +
				"`comp_currency` text NOT NULL DEFAULT ''," +
				"`comp_period` text NOT NULL DEFAULT ''," +
				"`source_note` text NOT NULL DEFAULT ''," +
				"`last_confirmed` text NOT NULL DEFAULT ''," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE TABLE `companies` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`name` text NOT NULL," +
				"`website` text NOT NULL DEFAULT ''," +
				"`location` text NOT NULL DEFAULT ''," +
				"`source` text NOT NULL DEFAULT ''," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE TABLE `contacts` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`company_id` integer NOT NULL REFERENCES `companies`(`id`)," +
				"`full_name` text NOT NULL," +
				"`title` text NOT NULL DEFAULT ''," +
				"`email` text NOT NULL DEFAULT ''," +
				"`phone` text NOT NULL DEFAULT ''," +
				"`source` text NOT NULL DEFAULT ''," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			// contacts-at-company is the whole point of the contact record.
			"CREATE INDEX `idx_contacts_company_id` ON `contacts`(`company_id`)",
			"CREATE TABLE `roles` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`title` text NOT NULL," +
				// A discovered role names a company that may have no record yet,
				// so the text stands alone and the reference is optional.
				"`company_name` text NOT NULL DEFAULT ''," +
				"`company_id` integer REFERENCES `companies`(`id`)," +
				"`location` text NOT NULL DEFAULT ''," +
				"`work_arrangement` text NOT NULL DEFAULT ''," +
				"`employment_type` text NOT NULL DEFAULT ''," +
				"`comp_min` integer," +
				"`comp_max` integer," +
				"`comp_currency` text NOT NULL DEFAULT ''," +
				"`comp_period` text NOT NULL DEFAULT ''," +
				"`published_on` text NOT NULL DEFAULT ''," +
				"`closing_on` text NOT NULL DEFAULT ''," +
				"`retrieved_on` text NOT NULL DEFAULT ''," +
				"`source_id` text NOT NULL DEFAULT ''," +
				"`canonical_url` text NOT NULL DEFAULT ''," +
				"`source` text NOT NULL DEFAULT ''," +
				"`origin` text NOT NULL," +
				"`lifecycle_state` text NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX `idx_roles_company_id` ON `roles`(`company_id`)",
		},
	},
	{
		Version: 3,
		Name:    "initiative_lifecycle_and_candidate",
		// candidate_id is the whole of "a Job Search Initiative has exactly one
		// Candidate": a column cannot hold two. The "at least one" half lives in
		// InitiativeService, because a table-level CHECK needs a table rebuild
		// that pre-Phase-3 rows would fail — see the change's design.md.
		SQL: []string{
			"ALTER TABLE `initiatives` ADD COLUMN `status` text NOT NULL DEFAULT 'active' " +
				"CHECK (`status` IN ('active','archived'))",
			"ALTER TABLE `initiatives` ADD COLUMN `candidate_id` integer REFERENCES `candidates`(`id`)",
			"CREATE INDEX `idx_initiatives_status` ON `initiatives`(`status`)",
		},
	},
	{
		Version: 4,
		Name:    "artifacts_and_links",
		// One artifact is one ingestion occurrence, bytes and all. Nothing here
		// is ever updated except display_name: filename, media type, size, hash,
		// source, and capture time are the provenance that makes it evidence.
		// Extraction columns arrive in Phase 6, with the code that writes them.
		SQL: []string{
			"CREATE TABLE `artifacts` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`display_name` text NOT NULL," +
				"`original_filename` text NOT NULL DEFAULT ''," +
				"`media_type` text NOT NULL," +
				"`byte_length` integer NOT NULL," +
				"`sha256` text NOT NULL," +
				"`source` text NOT NULL DEFAULT ''," +
				"`captured_at` datetime NOT NULL," +
				"`bytes` blob NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			// Deliberately NOT unique: equal bytes are not the same artifact,
			// because filename, source, and capture time are evidence too. The
			// index is for integrity lookups only.
			"CREATE INDEX `idx_artifacts_sha256` ON `artifacts`(`sha256`)",
			// ponytail: polymorphic target, so one table covers initiatives,
			// candidates, roles, companies, and contacts. The service checks the
			// target exists; per-type tables would buy real foreign keys if the
			// target set ever stops growing.
			"CREATE TABLE `artifact_links` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`artifact_id` integer NOT NULL REFERENCES `artifacts`(`id`)," +
				"`target_type` text NOT NULL," +
				"`target_id` integer NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE UNIQUE INDEX `idx_artifact_links_unique` ON " +
				"`artifact_links`(`artifact_id`,`target_type`,`target_id`)",
			"CREATE INDEX `idx_artifact_links_target` ON `artifact_links`(`target_type`,`target_id`)",
		},
	},
	{
		Version: 5,
		Name:    "background_jobs",
		// One row per job, counts rather than item rows: nothing in the PoC
		// resumes from an item, it re-runs.
		//
		// failure_reason is CHECKed to a short lowercase code so no free text —
		// and therefore no candidate content — can reach a job record.
		SQL: []string{
			"CREATE TABLE `jobs` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`kind` text NOT NULL," +
				"`initiative_id` integer REFERENCES `initiatives`(`id`)," +
				"`params` text NOT NULL DEFAULT '{}'," +
				"`state` text NOT NULL CHECK (`state` IN ('queued','running','completed','failed','cancelled'))," +
				"`total_items` integer NOT NULL DEFAULT 0," +
				"`completed_items` integer NOT NULL DEFAULT 0," +
				"`failure_reason` text NOT NULL DEFAULT '' " +
				"CHECK (`failure_reason` = '' OR `failure_reason` GLOB '[a-z][a-z0-9_]*')," +
				"`started_at` datetime," +
				"`finished_at` datetime," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			// The two questions asked of this table: what is unfinished, and
			// what has this initiative been doing.
			"CREATE INDEX `idx_jobs_state` ON `jobs`(`state`)",
			"CREATE INDEX `idx_jobs_initiative_id` ON `jobs`(`initiative_id`)",
		},
	},
	{
		Version: 6,
		Name:    "artifact_extraction",
		// What an artifact says once something has read it. There is no
		// 'running' state: in-progress belongs to the job, and a second place
		// to be in-progress is a second place to be wrong after a crash.
		//
		// extraction_error follows the jobs rule — a short lowercase code, so a
		// parser error quoting the document cannot become a stored field.
		//
		// Enforced by triggers rather than CHECK: SQLite accepts a CHECK on a
		// column added by ALTER TABLE and then never evaluates it. Rebuilding
		// the table to get a real CHECK would mean recreating its indexes and
		// its foreign key for a constraint two triggers state just as plainly.
		SQL: []string{
			"ALTER TABLE `artifacts` ADD COLUMN `extraction_state` text NOT NULL DEFAULT 'pending'",
			"ALTER TABLE `artifacts` ADD COLUMN `extraction_error` text NOT NULL DEFAULT ''",
			"ALTER TABLE `artifacts` ADD COLUMN `extractor` text NOT NULL DEFAULT ''",
			"ALTER TABLE `artifacts` ADD COLUMN `extractor_version` text NOT NULL DEFAULT ''",
			"ALTER TABLE `artifacts` ADD COLUMN `markdown` text NOT NULL DEFAULT ''",
			"CREATE TRIGGER `artifacts_extraction_insert` BEFORE INSERT ON `artifacts` " +
				"FOR EACH ROW WHEN " + extractionGuard("NEW") +
				" BEGIN SELECT RAISE(ABORT, 'invalid extraction state or reason'); END",
			"CREATE TRIGGER `artifacts_extraction_update` BEFORE UPDATE ON `artifacts` " +
				"FOR EACH ROW WHEN " + extractionGuard("NEW") +
				" BEGIN SELECT RAISE(ABORT, 'invalid extraction state or reason'); END",
			// The one question asked of these columns: what still needs reading.
			"CREATE INDEX `idx_artifacts_extraction_state` ON `artifacts`(`extraction_state`)",
			// The same guard for jobs. Migration 5's CHECK on failure_reason is
			// real but too weak to be worth trusting — see badReason — and it
			// cannot be tightened in place without rebuilding the table, so the
			// trigger is what actually enforces the rule from here.
			"CREATE TRIGGER `jobs_reason_insert` BEFORE INSERT ON `jobs` " +
				"FOR EACH ROW WHEN " + badReason("NEW.`failure_reason`") +
				" BEGIN SELECT RAISE(ABORT, 'invalid failure reason'); END",
			"CREATE TRIGGER `jobs_reason_update` BEFORE UPDATE ON `jobs` " +
				"FOR EACH ROW WHEN " + badReason("NEW.`failure_reason`") +
				" BEGIN SELECT RAISE(ABORT, 'invalid failure reason'); END",
		},
	},
	{
		Version: 7,
		Name:    "chunks_and_fts",
		// Chunks are derived data with a provenance chain: bytes → markdown →
		// chunks. start_offset and end_offset select exactly `text` in the
		// artifact's markdown, which is the whole of what makes a citation
		// resolvable rather than merely plausible.
		SQL: []string{
			"CREATE TABLE `chunks` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`artifact_id` integer NOT NULL REFERENCES `artifacts`(`id`)," +
				"`ordinal` integer NOT NULL," +
				"`text` text NOT NULL," +
				"`start_offset` integer NOT NULL," +
				"`end_offset` integer NOT NULL," +
				// JSON array of the headings above this chunk, outermost first.
				"`heading_path` text NOT NULL DEFAULT '[]'," +
				"`token_count` integer NOT NULL," +
				"`hash` text NOT NULL," +
				"`chunker` text NOT NULL," +
				"`chunker_version` text NOT NULL," +
				"`chunker_params` text NOT NULL DEFAULT '{}'," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			// Ordinals are the citation's address within an artifact, so two
			// chunks cannot claim the same one.
			"CREATE UNIQUE INDEX `idx_chunks_artifact_ordinal` ON `chunks`(`artifact_id`,`ordinal`)",
			// Not unique: two CVs may legitimately share a paragraph, and they
			// are two pieces of evidence with two provenances.
			"CREATE INDEX `idx_chunks_hash` ON `chunks`(`hash`)",
			// External content: the text lives once, in `chunks`. The triggers
			// are what make the index correct through code paths that never go
			// near the search service — including a rollback, since they run
			// inside the same transaction as the rows they follow.
			"CREATE VIRTUAL TABLE `chunks_fts` USING fts5(" +
				"text, content='chunks', content_rowid='id')",
			"CREATE TRIGGER `chunks_fts_insert` AFTER INSERT ON `chunks` BEGIN " +
				"INSERT INTO `chunks_fts`(rowid, text) VALUES (new.`id`, new.`text`); END",
			"CREATE TRIGGER `chunks_fts_delete` AFTER DELETE ON `chunks` BEGIN " +
				"INSERT INTO `chunks_fts`(`chunks_fts`, rowid, text) VALUES ('delete', old.`id`, old.`text`); END",
			"CREATE TRIGGER `chunks_fts_update` AFTER UPDATE ON `chunks` BEGIN " +
				"INSERT INTO `chunks_fts`(`chunks_fts`, rowid, text) VALUES ('delete', old.`id`, old.`text`); " +
				"INSERT INTO `chunks_fts`(rowid, text) VALUES (new.`id`, new.`text`); END",
		},
	},
	{
		Version: 8,
		Name:    "model_assignments",
		// Append-only: one row per configuration a role has ever had, and the
		// current one is its highest revision. Phase 9 identifies an embedding
		// space by endpoint revision and model digest, so these rows have to be
		// immutable — an identifier pointing at an editable row is not one.
		SQL: []string{
			"CREATE TABLE `model_assignments` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`role` text NOT NULL CHECK (`role` IN ('embed','classify','generate'))," +
				"`revision` integer NOT NULL," +
				"`endpoint` text NOT NULL," +
				"`model` text NOT NULL," +
				// Empty is honest: the endpoint did not report a digest.
				"`digest` text NOT NULL DEFAULT ''," +
				"`params` text NOT NULL DEFAULT '{}'," +
				"`validation` text NOT NULL DEFAULT 'unvalidated' " +
				"CHECK (`validation` IN ('unvalidated','validated'))," +
				// What proved the model good enough, when anything did.
				"`benchmark_ref` text NOT NULL DEFAULT ''," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE UNIQUE INDEX `idx_model_assignments_role_revision` ON " +
				"`model_assignments`(`role`,`revision`)",
			// Validated without evidence is the state this guard exists to
			// refuse: Phase 21 owns benchmarks, and until one exists there is
			// nothing that could make the claim true.
			"CREATE TRIGGER `model_assignments_validation_insert` BEFORE INSERT ON `model_assignments` " +
				"FOR EACH ROW WHEN " + unevidencedValidation("NEW") +
				" BEGIN SELECT RAISE(ABORT, 'validated needs a benchmark reference'); END",
			"CREATE TRIGGER `model_assignments_validation_update` BEFORE UPDATE ON `model_assignments` " +
				"FOR EACH ROW WHEN " + unevidencedValidation("NEW") +
				" BEGIN SELECT RAISE(ABORT, 'validated needs a benchmark reference'); END",
		},
	},
}

// unevidencedValidation is the condition an assignment is refused on: claiming
// Validated with nothing to point at.
func unevidencedValidation(row string) string {
	return "(" + row + ".`validation` = 'validated' AND " + row + ".`benchmark_ref` = '')"
}

// badReason is the condition a reason-code column is refused on: anything that
// is not empty and not a short lowercase code.
//
// The obvious `GLOB '[a-z][a-z0-9_]*'` does not do this — GLOB's `*` matches
// any character at all, so it constrains only the first letter and a sentence
// beginning with a lowercase word slips straight through. Refusing the
// characters explicitly is the version that actually holds.
func badReason(col string) string {
	return "NOT (" + col + " = '' OR (" + col + " GLOB '[a-z]*'" +
		" AND NOT " + col + " GLOB '*[^a-z0-9_]*'" +
		" AND length(" + col + ") <= 40))"
}

// extractionGuard is the condition the extraction triggers abort on: an unknown
// state, or a reason that is not a code.
func extractionGuard(row string) string {
	return "(" + row + ".`extraction_state` NOT IN ('pending','extracted','failed')" +
		" OR " + badReason(row+".`extraction_error`") + ")"
}

// latestVersion is the newest schema version this build knows.
func latestVersion(migs []migration) int {
	if len(migs) == 0 {
		return 0
	}
	return migs[len(migs)-1].Version
}
