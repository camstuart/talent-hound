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
	{
		Version: 9,
		Name:    "embedding_spaces_and_embeddings",
		// A vector is a number only relative to the model that produced it, and
		// two incompatible geometries are indistinguishable as bytes. So the
		// identity of the space is a row, the unique index makes it the only
		// one, and every vector names it.
		SQL: []string{
			"CREATE TABLE `embedding_spaces` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`endpoint` text NOT NULL," +
				"`model` text NOT NULL," +
				"`digest` text NOT NULL DEFAULT ''," +
				// The embed assignment revision that produced this space. Phase 8
				// made this append-only precisely so it could be pointed at.
				"`revision` integer NOT NULL," +
				"`dimensions` integer NOT NULL CHECK (`dimensions` > 0)," +
				"`metric` text NOT NULL CHECK (`metric` IN ('cosine'))," +
				"`normalized` numeric NOT NULL DEFAULT 0," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			// Every column of the identity, so the same configuration can never
			// end up as two spaces whose vectors then never meet.
			"CREATE UNIQUE INDEX `idx_embedding_spaces_identity` ON `embedding_spaces`" +
				"(`endpoint`,`model`,`digest`,`revision`,`dimensions`,`metric`,`normalized`)",
			"CREATE TABLE `embeddings` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`space_id` integer NOT NULL REFERENCES `embedding_spaces`(`id`)," +
				"`owner_kind` text NOT NULL CHECK (`owner_kind` IN ('chunk','aspect'))," +
				"`owner_id` integer NOT NULL," +
				"`dimensions` integer NOT NULL CHECK (`dimensions` > 0)," +
				// Little-endian float32, exactly 4×dimensions bytes.
				"`vector` blob NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			// One vector per retrieval unit per space: this is what makes a retry
			// an upsert into a slot rather than a way to grow duplicates.
			"CREATE UNIQUE INDEX `idx_embeddings_unit` ON `embeddings`" +
				"(`space_id`,`owner_kind`,`owner_id`)",
			// The blob length is the whole integrity story, and it is exact:
			// there is no corruption that keeps the length and changes the
			// meaning, so a length check catches everything a header would.
			"CREATE TRIGGER `embeddings_length_insert` BEFORE INSERT ON `embeddings` " +
				"FOR EACH ROW WHEN " + badVectorLength("NEW") +
				" BEGIN SELECT RAISE(ABORT, 'vector length does not match its dimensions'); END",
			"CREATE TRIGGER `embeddings_length_update` BEFORE UPDATE ON `embeddings` " +
				"FOR EACH ROW WHEN " + badVectorLength("NEW") +
				" BEGIN SELECT RAISE(ABORT, 'vector length does not match its dimensions'); END",
			"CREATE INDEX `idx_embeddings_owner` ON `embeddings`(`owner_kind`,`owner_id`)",
		},
	},
	{
		Version: 10,
		Name:    "profiles_and_aspects",
		// The taxonomy is checked in Go and again here. Two checks of one rule
		// is not redundancy: the Go one is what the classifier is held to, and
		// this one is what any future writer is held to, including one written
		// by someone who has never read the validator.
		SQL: []string{
			"CREATE TABLE `profiles` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`subject_kind` text NOT NULL CHECK (`subject_kind` IN ('candidate','role'))," +
				"`subject_id` integer NOT NULL," +
				"`version` integer NOT NULL," +
				"`state` text NOT NULL DEFAULT 'proposed' " +
				"CHECK (`state` IN ('proposed','approved','failed'))," +
				"`schema_version` text NOT NULL," +
				"`prompt_version` text NOT NULL," +
				"`model_revision` integer NOT NULL," +
				"`model_name` text NOT NULL DEFAULT ''," +
				"`source_hash` text NOT NULL," +
				// The hash of everything that could change what this means.
				"`identity` text NOT NULL," +
				"`failure_reason` text NOT NULL DEFAULT ''," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE UNIQUE INDEX `idx_profiles_subject_version` ON " +
				"`profiles`(`subject_kind`,`subject_id`,`version`)",
			"CREATE INDEX `idx_profiles_identity` ON `profiles`(`identity`)",
			"CREATE TABLE `profile_aspects` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`profile_id` integer NOT NULL REFERENCES `profiles`(`id`)," +
				"`ordinal` integer NOT NULL," +
				"`type` text NOT NULL CHECK (`type` IN (" +
				"'skill','responsibility','experience','qualification','seniority'," +
				"'location','work_arrangement','work_rights','employment_type'," +
				"'compensation','other'))," +
				"`wording` text NOT NULL," +
				"`structured` text NOT NULL DEFAULT '{}'," +
				"`priority` text NOT NULL DEFAULT 'unspecified' " +
				"CHECK (`priority` IN ('must_have','nice_to_have','unspecified'))," +
				"`origin` text NOT NULL DEFAULT 'extracted' " +
				"CHECK (`origin` IN ('extracted','recruiter_supplied'))," +
				// Never empty: an aspect with no evidence is the thing the whole
				// contract exists to refuse.
				"`citations` text NOT NULL DEFAULT '[]' CHECK (`citations` <> '[]')," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE UNIQUE INDEX `idx_profile_aspects_ordinal` ON " +
				"`profile_aspects`(`profile_id`,`ordinal`)",
			"CREATE INDEX `idx_profile_aspects_type` ON `profile_aspects`(`type`)",
			// A profile that failed carries a code, on the same terms as a job.
			"CREATE TRIGGER `profiles_reason_insert` BEFORE INSERT ON `profiles` " +
				"FOR EACH ROW WHEN " + badReason("NEW.`failure_reason`") +
				" BEGIN SELECT RAISE(ABORT, 'failure reason must be a short code'); END",
			"CREATE TRIGGER `profiles_reason_update` BEFORE UPDATE ON `profiles` " +
				"FOR EACH ROW WHEN " + badReason("NEW.`failure_reason`") +
				" BEGIN SELECT RAISE(ABORT, 'failure reason must be a short code'); END",
		},
	},
	{
		Version: 11,
		Name:    "profile_approval",
		// Approval is a fact about a version, and staleness is deliberately not
		// a column: it is the comparison between the source hash an approval
		// was about and the sources in force now. A stored flag would need
		// something to notice a source changed, and that something is exactly
		// what will be missing the next time a new way to attach evidence is
		// added.
		SQL: []string{
			"ALTER TABLE `profiles` ADD COLUMN `approved_at` datetime",
			// The evidence the approval was about. Equal to source_hash at the
			// moment of approval, and kept separately so a later edit-derived
			// version cannot quietly move what was approved.
			"ALTER TABLE `profiles` ADD COLUMN `approved_source_hash` text NOT NULL DEFAULT ''",
			"CREATE INDEX `idx_profiles_approved` ON " +
				"`profiles`(`subject_kind`,`subject_id`,`approved_at`)",
		},
	},
	{
		Version: 12,
		Name:    "search_criteria",
		// Criteria belong to an initiative, not a candidate: they are the
		// recruiter's intent, and a resume saying someone worked somewhere is
		// not a statement that they want to again.
		SQL: []string{
			"CREATE TABLE `search_criteria` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`initiative_id` integer NOT NULL REFERENCES `initiatives`(`id`)," +
				// Position is presentation. The PRD is explicit that ordering is
				// not weighting, so it deliberately does not affect the version.
				"`position` integer NOT NULL," +
				"`text` text NOT NULL," +
				// No unspecified: a criterion is the recruiter's choice, and an
				// unweighted one would be a preference nobody expressed.
				"`priority` text NOT NULL CHECK (`priority` IN ('must_have','nice_to_have'))," +
				// Recorded once, on write, so it cannot change under the reader
				// when the classify model changes.
				"`warning` text NOT NULL DEFAULT ''," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX `idx_search_criteria_initiative` ON " +
				"`search_criteria`(`initiative_id`,`position`)",
			// One row per initiative, bumped when content changes. An assessment
			// records the version it was made under, so Phase 16 can tell a
			// stale match from a current one.
			"CREATE TABLE `criteria_versions` (" +
				"`initiative_id` integer PRIMARY KEY REFERENCES `initiatives`(`id`)," +
				"`version` integer NOT NULL DEFAULT 1," +
				"`updated_at` datetime)",
		},
	},
	{
		Version: 13,
		Name:    "discovery_and_disclosure",
		// Two records with two lifetimes, deliberately. The search keeps the
		// visible query because reproducing a search needs it and the
		// initiative is where the recruiter already has that information. The
		// disclosure event keeps none of it, because the audit log is the thing
		// that might be exported, reviewed, or retained longest.
		SQL: []string{
			"CREATE TABLE `searches` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`initiative_id` integer NOT NULL REFERENCES `initiatives`(`id`)," +
				"`provider` text NOT NULL," +
				// Exactly what was sent, byte for byte.
				"`query` text NOT NULL," +
				"`result_count` integer NOT NULL DEFAULT 0," +
				"`skipped_count` integer NOT NULL DEFAULT 0," +
				"`partial` numeric NOT NULL DEFAULT 0," +
				"`failure_reason` text NOT NULL DEFAULT ''," +
				"`sent_at` datetime NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX `idx_searches_initiative` ON `searches`(`initiative_id`,`sent_at`)",
			// No query column, no content column, and none may be added: the
			// point of this table is what it does not hold.
			"CREATE TABLE `disclosure_events` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`occurred_at` datetime NOT NULL," +
				"`provider` text NOT NULL," +
				"`task` text NOT NULL," +
				// A comma-separated list of category names — "professional
				// requirements", never the requirements themselves.
				"`categories` text NOT NULL DEFAULT ''," +
				"`initiative_id` integer REFERENCES `initiatives`(`id`)," +
				"`candidate_id` integer REFERENCES `candidates`(`id`)," +
				"`role_id` integer REFERENCES `roles`(`id`)," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX `idx_disclosure_events_time` ON `disclosure_events`(`occurred_at`)",
			// A role's source links carry whether they are the current source
			// or a historical one. Historical artifacts stay visible for
			// provenance and leave current retrieval.
			"ALTER TABLE `artifact_links` ADD COLUMN `historical` numeric NOT NULL DEFAULT 0",
			"ALTER TABLE `roles` ADD COLUMN `content_hash` text NOT NULL DEFAULT ''",
			"ALTER TABLE `roles` ADD COLUMN `retrieved_at` datetime",
		},
	},
	{
		Version: 14,
		Name:    "matches_and_results",
		// A match is a conclusion, and a conclusion is only valid while the
		// inputs that produced it are. The hash is indexed because it is the
		// lookup: reuse asks "is there a stored match with this hash", and
		// nothing else decides.
		SQL: []string{
			"CREATE TABLE `matches` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`initiative_id` integer NOT NULL REFERENCES `initiatives`(`id`)," +
				"`candidate_id` integer NOT NULL REFERENCES `candidates`(`id`)," +
				"`role_id` integer NOT NULL REFERENCES `roles`(`id`)," +
				"`input_hash` text NOT NULL," +
				// The counts ranking needs, summed across both directions, so
				// ordering does not re-read every result row.
				"`unmet_must_haves` integer NOT NULL DEFAULT 0," +
				"`unknown_must_haves` integer NOT NULL DEFAULT 0," +
				"`met_nice_to_haves` integer NOT NULL DEFAULT 0," +
				"`retrieval_position` integer NOT NULL DEFAULT 0," +
				"`failure_reason` text NOT NULL DEFAULT ''," +
				"`assessed_at` datetime NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE UNIQUE INDEX `idx_matches_identity` ON " +
				"`matches`(`initiative_id`,`candidate_id`,`role_id`)",
			"CREATE INDEX `idx_matches_hash` ON `matches`(`input_hash`)",
			"CREATE TABLE `match_results` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`match_id` integer NOT NULL REFERENCES `matches`(`id`)," +
				"`direction` text NOT NULL " +
				"CHECK (`direction` IN ('role_fits_candidate','candidate_fits_role'))," +
				"`ordinal` integer NOT NULL," +
				"`requirement` text NOT NULL," +
				"`priority` text NOT NULL DEFAULT 'unspecified' " +
				"CHECK (`priority` IN ('must_have','nice_to_have','unspecified'))," +
				// Three states and nothing else. A model that invents a fourth
				// is refused in Go, and refused again here.
				"`result` text NOT NULL CHECK (`result` IN ('met','not_met','unknown'))," +
				"`reason` text NOT NULL DEFAULT ''," +
				// Evidence as JSON. A met result with none of it is refused
				// before it reaches this table.
				"`citations` text NOT NULL DEFAULT '[]'," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE UNIQUE INDEX `idx_match_results_ordinal` ON " +
				"`match_results`(`match_id`,`direction`,`ordinal`)",
			"CREATE TRIGGER `matches_reason_insert` BEFORE INSERT ON `matches` " +
				"FOR EACH ROW WHEN " + badReason("NEW.`failure_reason`") +
				" BEGIN SELECT RAISE(ABORT, 'failure reason must be a short code'); END",
		},
	},
	{
		Version: 15,
		Name:    "drafts_and_answers",
		SQL: []string{
			"CREATE TABLE `drafts` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`initiative_id` integer NOT NULL REFERENCES `initiatives`(`id`)," +
				"`candidate_id` integer REFERENCES `candidates`(`id`)," +
				"`role_id` integer REFERENCES `roles`(`id`)," +
				"`kind` text NOT NULL CHECK (`kind` IN ('pitch','outreach'))," +
				// Two states, and copying is neither: a copy is an event, which
				// is what makes "copied twice" expressible and what keeps
				// discarding from ever looking like a send.
				"`state` text NOT NULL DEFAULT 'active' " +
				"CHECK (`state` IN ('active','discarded'))," +
				"`subject` text NOT NULL DEFAULT ''," +
				"`body` text NOT NULL," +
				// The claim-to-evidence map as it was at generation, as JSON.
				"`claims` text NOT NULL DEFAULT '[]'," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX `idx_drafts_initiative` ON `drafts`(`initiative_id`,`state`)",
			"CREATE TABLE `answers` (" +
				"`id` integer PRIMARY KEY AUTOINCREMENT," +
				"`initiative_id` integer NOT NULL REFERENCES `initiatives`(`id`)," +
				"`question` text NOT NULL," +
				"`answer` text NOT NULL DEFAULT ''," +
				// Supported says the evidence backs it. An unsupported answer
				// carries no factual assertion at all.
				"`supported` numeric NOT NULL DEFAULT 0," +
				"`citations` text NOT NULL DEFAULT '[]'," +
				"`asked_at` datetime NOT NULL," +
				"`created_at` datetime," +
				"`updated_at` datetime)",
			"CREATE INDEX `idx_answers_initiative` ON `answers`(`initiative_id`,`asked_at`)",
			// A copy is an audit event beside the disclosure ones, on the same
			// terms: it records that, never what. The table already has no
			// content column, which is the property being reused.
			"ALTER TABLE `disclosure_events` ADD COLUMN `draft_id` integer REFERENCES `drafts`(`id`)",
		},
	},
}

// badVectorLength is the condition an embedding is refused on: a blob that is
// not exactly four bytes per dimension, or one whose dimensions disagree with
// the space it names.
func badVectorLength(row string) string {
	return "(length(" + row + ".`vector`) <> " + row + ".`dimensions` * 4" +
		" OR " + row + ".`dimensions` <> (SELECT `dimensions` FROM `embedding_spaces`" +
		" WHERE `id` = " + row + ".`space_id`))"
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
