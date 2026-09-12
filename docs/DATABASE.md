# PostgreSQL schema contract

This contract is implemented by the versioned SQL migrations under `migrations/` and exercised by the fresh-database replay in `tools/end_to_end.sh`. The table list remains the authoritative audit model; additive compatibility columns retained from the first migration are documented in the migration comments. Use explicit foreign keys and checks; do not delegate integrity solely to Go structs.

## Types and conventions

Use bigint generated identity for internal IDs, text for externally supplied identifiers, timestamptz for instants, date for key days, JSONB for parsed raw payload and structured diagnostic detail. Persist original source files separately with hashes and offsets; JSONB is not byte-exact source storage.

Money columns use unconstrained `numeric` plus `valid_exact_cents` checks for finite values, at most two decimal places, and the signed-int64-cent range. This avoids silent `NUMERIC(p,2)` input rounding. Go parses signed cents first and sends formatted decimals without floating point. SQL sums remain numeric. Counts use bigint.

Use database-generated IDs only as locators. Stable identity is content hash + source ordinal, not auto-increment ordering. Every derived table has run_id and prevents cross-run FK references through composite keys.

Migration execution holds a session advisory lock on one dedicated pgx connection. This serializes checksum inspection, DDL, and ledger writes across concurrent application starts; cancellation uses a bounded independent context to release the lock before the connection returns to the pool.

## Tables

| Table | Required columns and constraints |
| --- | --- |
| `source_files` | id PK, source_kind (payment/settlement/payment_config/settlement_config), path, sha256, byte_size. UNIQUE(source_kind,sha256). Conflict reuse never mutates the first recorded path. |
| `config_versions` | id PK, name unique, parent_id FK nullable, state DRAFT/FROZEN, content_sha256 unique when frozen, created_at, note. Frozen rows cannot change. |
| `config_hash_history` | config_version_id FK, hash_algorithm, historical content_sha256, recorded_at. Preserves the legacy delimiter-based hash when an existing frozen version is upgraded to the canonical hash algorithm; update and delete are trigger-rejected. |
| `mapping_rules` | id PK, config_version_id FK, source, origin_file, origin_line, raw selector fields, record_ref and positive/negative targets. UNIQUE(config_version_id,source,origin_file,origin_line). Selector uniqueness is intentionally absent so original ambiguity remains representable. Frozen-parent moves are trigger-blocked. |
| `normalization_versions` | name PK, policy JSONB, content_sha256 unique, frozen flag. |
| `layout_versions` | name PK, layout JSONB, content_sha256 unique, frozen flag. |
| `runs` | id PK, name unique, fingerprint unique, config/normalization/layout lineage FKs, engine_version, selected_settlement_id, currency, mode, stage, verification_status, attempt, timestamps and error detail. Fingerprints and config hashes use length-prefixed canonical fields. |
| `run_files` | run_id FK, source_file_id FK, role, PK(run_id,role); exactly one payment and one settlement for this implementation. Config file origins referenced through rules. |
| `source_rows` | id PK, run/source-file FKs, source, row_kind, ordinal, line and byte spans, raw/canonical JSONB, settlement/currency/selectors/identifiers, posted/release/key dates, status, exact recon_amount, scope_reason and event_class. UNIQUE(run_id,source_file_id,ordinal), UNIQUE(id,run_id). Both sources and metadata live here. Metadata recon_amount is its header control. |
| `settlement_controls` | run_id, settlement_id, metadata_row_id FK scoped to run, currency, exact header_total. Only the selected settlement is registered. |
| `row_mappings` | id PK, run/source-row/rule FKs, amount_field, exact amount, target nullable, decision and encoded record_ref. UNIQUE(run_id,source_row_id,rule_id). A trigger enforces run/config/source lineage. |
| `summary_contributions` | id PK, run_id, source_row_id FK, row_mapping_id FK, source, scope_reason, settlement_id, currency, summary_field FK registry, amount money NOT NULL and nonzero. UNIQUE(row_mapping_id). Source and scope must agree with parent row; no mixing sources. |
| `summary_fields` | field text PK, kind DIRECT/INTERMEDIATE/CONTROL, supported_policy nullable. Seed all original config targets; route validation detects unsupported nonzero fields. |
| `summary_totals` | run_id, source, field, exact amount, contribution_count. PK(run_id,source,field). Zero states are retained for every encountered field and both sources. |
| `recon_groups` | id/run identity, encoded record_ref, settlement_id, currency, scope_reason, row counts, exact source amounts and presence status. UNIQUE(run_id,record_ref,settlement_id,currency,scope_reason). |
| `recon_members` | run_id, group_id FK, source_row_id FK, PK(run_id,source_row_id); each transaction belongs to exactly one group. Non-eligible payment rows form payment-only groups in their own scope partitions. |
| `run_issues` | id PK, run_id, severity, code, source_row_id nullable, mapping_rule_id nullable, detail JSONB. Bounded display, full persisted evidence. |
| `config_fix_history` | config_version_id, defect_id, applied_at, old/new snapshots JSONB, SQL hash; PK(config_version_id,defect_id). |
| `report_artifacts` | id PK, run_id, path, sha256, byte_size, report_status, created_at; UNIQUE(run_id,report_kind). |

`row_mappings` represents decisions, `summary_contributions` represents nonzero additive money, and `source_rows.recon_amount` represents source totals. They are intentionally different grains. Do not sum recon_amount through joins to mappings. Add FK consistency triggers or use explicit composite FKs where a check spans tables. Null summary target is legal; a nonempty target must reference the field registry.

## Useful indexes

- source_rows(run_id, source, settlement_id, scope_reason).
- source_rows(run_id, source_file_id, line_start) for explain-by-line.
- row_mappings(run_id, record_ref_hash) with full tuple comparison.
- summary_contributions(run_id, source, summary_field, source_row_id).
- recon_groups(run_id, status, record_ref_hash).
- mapping_rules(config_version_id, source, canonical_transaction_type, canonical_amount_field/type).

Create only indexes justified by these queries. Partitioning and generalized multi-tenant row security are not required for this batch assignment.

## Run lifecycle

Stages are `CLAIMED -> RECONCILED -> REPORTED`; persistence failure changes a claimed run to `FAILED`. The data transaction moves to RECONCILED only after all lineage and controls commit. Atomic report registration moves it to REPORTED and sets DIAGNOSTIC or PASS. Verification states are NOT_CHECKED, DIAGNOSTIC, PASS and FAIL. A baseline run can be REPORTED + DIAGNOSTIC; a completed strict run requires REPORTED + PASS.

Use transactions described in ARCHITECTURE.md. Retry a failed report without re-ingestion. For config fixes create a child version and a new run fingerprint; old rows and report artifacts stay immutable. Support deletion only for explicit local cleanup, not as normal replay semantics.

## Required independent queries

Implement named query files for raw counts by source, scope census, payment component-vs-total controls, metadata-vs-component total, unmapped/ambiguous rules, duplicate source membership, contribution-vs-summary totals, per-key amount delta, per-key bucket delta, and explain-by-summary-field/rule/source-line. Queries must be parameterized by run; no default 'latest' run for audit work.

Example aggregation pattern (conceptual): GROUP source_rows and their one resolved key by run/source/settlement/currency/scope/key; FULL OUTER JOIN the resulting payment/settlement aggregates. Never sum a raw-to-mapping join without first deduplicating to source-row identity.
