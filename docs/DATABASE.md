# PostgreSQL schema contract

This contract is implemented by the versioned SQL migrations under `migrations/` and exercised by the fresh-database replay in `tools/end_to_end.sh`. The table list remains the authoritative audit model; additive compatibility columns retained from the first migration are documented in the migration comments. Use explicit foreign keys and checks; do not delegate integrity solely to Go structs.

## Types and conventions

Use bigint generated identity for internal IDs, text for externally supplied identifiers, timestamptz for instants, date for key days, JSONB for parsed raw payload and structured diagnostic detail. Persist original source files separately with hashes and offsets; JSONB is not byte-exact source storage.

Money: define an unconstrained `numeric` domain with checks for finite values, `scale(VALUE) <= 2`, and `abs(VALUE) <= 92233720368547758.07`. This avoids silent NUMERIC(p,2) input rounding. Go parses into signed cents first and converts to pgtype.Numeric without floating point. SQL SUM remains numeric; check range before converting an aggregate back to int64 cents. Counts use bigint. Business enum values can be text plus CHECK constraints to keep migrations simple.

Use database-generated IDs only as locators. Stable identity is content hash + source ordinal, not auto-increment ordering. Every derived table has run_id and prevents cross-run FK references through composite keys.

## Tables

| Table | Required columns and constraints |
| --- | --- |
| `source_files` | id PK, kind (payment/settlement/payment_config/settlement_config), original_name, sha256, byte_size, encoding, header_line, headers JSONB, preamble JSONB, immutable_storage_path. UNIQUE(kind,sha256). No path as sole identity. |
| `config_versions` | id PK, name unique, parent_id FK nullable, state DRAFT/FROZEN, content_sha256 unique when frozen, created_at, note. Frozen rows cannot change. |
| `mapping_rules` | id PK, config_version_id FK, source payment/settlement, origin_file_id FK, origin_line, transaction_type, description nullable for settlement, amount_field nullable for settlement, amount_type nullable for payment, amount_description nullable for payment, record_ref, positive_target nullable, negative_target nullable. Preserve original values plus canonical selector fields. UNIQUE(config_version_id,source,origin_file_id,origin_line). Do not impose selector uniqueness here: original defects must be representable. |
| `normalization_profiles` | id/version PK, canonical policy JSONB, hash unique, frozen flag. Includes scoped description alias entries and date/header policy. Treat policy as data with a fixed supported vocabulary, not executable SQL. |
| `summary_layouts` | version PK, immutable rows/labels/field mapping JSONB, hash. Must implement DESIGN.md. |
| `runs` | id PK, name unique, fingerprint unique, config_version_id FK, normalization_version FK, layout_version FK, engine_version, selected_settlement_id, currency, mode DIAGNOSTIC_BASELINE/STRICT, stage, verification_status, attempt, started_at, completed_at, error_code/detail nullable. |
| `run_files` | run_id FK, source_file_id FK, role, PK(run_id,role); exactly one payment and one settlement for this implementation. Config file origins referenced through rules. |
| `source_rows` | id PK, run_id FK, source_file_id FK, source payment/settlement, record_ordinal, line_start/end, byte_start/end, row_kind TRANSACTION/SETTLEMENT_METADATA, raw_payload JSONB, canonical_payload JSONB, settlement_id, currency, posted_at, release_at nullable, key_date nullable, txn_ref, sku, transaction_type, description_key, amount_type/description nullable, transaction_status nullable, recon_amount nullable money, scope_reason, event_class. UNIQUE(run_id,source_file_id,record_ordinal), UNIQUE(run_id,id). Both sources live here. Metadata has no recon_amount. |
| `settlement_controls` | run_id, settlement_id, metadata_row_id FK scoped to run, start_at, end_at, deposit_at, currency, header_total money. PK(run_id,settlement_id). |
| `row_mappings` | id PK, run_id, source_row_id FK, mapping_rule_id FK, amount_field, amount nullable money, field_availability PRESENT/OPTIONAL_ABSENT, decision ROUTED/NOT_SUMMARIZED/ZERO/COVERED_BY_TOTAL, chosen_target nullable, record_ref_parts JSONB nullable, record_ref_hash nullable. UNIQUE(run_id,source_row_id,mapping_rule_id). Enforce rule belongs to run's config version. |
| `summary_contributions` | id PK, run_id, source_row_id FK, row_mapping_id FK, source, scope_reason, settlement_id, currency, summary_field FK registry, amount money NOT NULL and nonzero. UNIQUE(row_mapping_id). Source and scope must agree with parent row; no mixing sources. |
| `summary_fields` | field text PK, kind DIRECT/INTERMEDIATE/CONTROL, supported_policy nullable. Seed all original config targets; route validation detects unsupported nonzero fields. |
| `summary_totals` | run_id, source, settlement_id, currency, scope_reason, summary_field, amount money, contribution_count; composite PK over all dimensions. Written from ingestion contributions, never manually patched. |
| `recon_groups` | id PK, run_id, settlement_id, currency, scope_partition, record_ref_hash, record_ref_parts JSONB, payment_count, settlement_count, payment_amount nullable money, settlement_amount nullable money, difference money, status. UNIQUE(run_id,settlement_id,currency,scope_partition,record_ref_parts). Hash index is an accelerator, full tuple is authoritative. |
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

Stages: CLAIMED -> INGESTED -> RECONCILED -> REPORTED. FAILED records the last successful stage and error. Verification is separate: NOT_CHECKED, DIAGNOSTIC, PASS, FAIL. A before-fix run can be REPORTED + DIAGNOSTIC, never PASS. A completed strict after-fix run requires REPORTED + PASS. Block stage transitions if predecessor data is not complete.

Use transactions described in ARCHITECTURE.md. Retry a failed report without re-ingestion. For config fixes create a child version and a new run fingerprint; old rows and report artifacts stay immutable. Support deletion only for explicit local cleanup, not as normal replay semantics.

## Required independent queries

Implement named query files for raw counts by source, scope census, payment component-vs-total controls, metadata-vs-component total, unmapped/ambiguous rules, duplicate source membership, contribution-vs-summary totals, per-key amount delta, per-key bucket delta, and explain-by-summary-field/rule/source-line. Queries must be parameterized by run; no default 'latest' run for audit work.

Example aggregation pattern (conceptual): GROUP source_rows and their one resolved key by run/source/settlement/currency/scope/key; FULL OUTER JOIN the resulting payment/settlement aggregates. Never sum a raw-to-mapping join without first deduplicating to source-row identity.
