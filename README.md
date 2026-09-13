# Amazon Payments / Settlement Reconciliation

A Go CLI that ingests Amazon Payments and Settlement reports into PostgreSQL, applies versioned mapping configurations, reconciles records across different source grains, and generates before-fix and after-fix Excel reports with source-row lineage.

## Prerequisites

- Go 1.27
- Docker with Compose
- PostgreSQL client tools: `psql`, `pg_dump`, and `pg_restore`
- `unzip`

Place the five supplied assignment files at these paths without modifying them:

```text
workingData/amazon_payments_data.csv
workingData/amazon_settlements_data.txt
workingData/amazon_payment_configs_au_old.csv
workingData/amazon_settlement_configs_au.csv
workingData/amazon_sample_output_report.xlsx
```

`workingData/`, `output/`, and `.env` are intentionally ignored by Git.

## Run the full flow

From a clean clone, start an empty PostgreSQL database and run the complete ingest, mapping-fix, reconciliation, report, and verification flow:

```sh
docker compose up -d db
export DATABASE_URL='postgres://recon:recon@localhost:54329/reconciliation?sslmode=disable'
make end-to-end
```

`make end-to-end` performs the following steps:

1. Builds the CLI and applies all checksummed migrations.
2. Imports and freezes the original payment and settlement mapping configs.
3. Creates a child config version and applies the mapping corrections.
4. Ingests both untouched source files into the shared `source_rows` table.
5. Persists separate baseline and fixed runs with mapping and source lineage.
6. Aggregates each source before reconciling on the expanded `record_ref`.
7. Generates before-fix and after-fix workbooks.
8. Verifies the workbook and independently recomputes database controls.

The generated workbooks are:

```text
output/e2e_before_fix.xlsx
output/e2e_after_fix.xlsx
```

The command requires an empty database. To reset the included local database before another complete replay:

```sh
docker compose down -v
docker compose up -d db
```

The reports can also be generated without PostgreSQL:

```sh
make before
make after
```

This writes `output/before_fix.xlsx` and `output/after_fix.xlsx`. The source and config files under `workingData/` are never modified.

## Verification

Source-only checks require no assignment data:

```sh
make submission-smoke
```

With the supplied files installed, run:

```sh
make reference-test
make submission-smoke
```

The full acceptance command is `make end-to-end`. It verifies both persisted runs and fails if ingestion counts, group membership, mapping issues, amount differences, summary differences, bucket differences, or settlement-header controls violate the selected run mode.

## Schema design and rationale

Both reports are stored in `source_rows`, as required. Derived facts are kept in separate tables at their natural grains so that joining a source row to multiple mapping decisions cannot multiply money.

| Area | Tables | Rationale |
| --- | --- | --- |
| Input identity and lineage | `source_files`, `run_files`, `source_rows` | Retains file hash, row ordinal, physical line and byte span, raw and canonical payloads, source identifiers, scope, and exact amount. |
| Mapping configuration | `config_versions`, `config_version_files`, `mapping_rules`, `config_fix_history` | Stores the configs as data, preserves the original version, records each correction, and prevents mutation after freeze. |
| Accounting summary | `summary_fields`, `summary_contributions`, `summary_totals` | Keeps independently derived payment and settlement contributions separate and prevents raw many-to-many joins from double counting. |
| Reconciliation | `recon_groups`, `recon_members` | Aggregates each source by settlement, currency, scope, and expanded key before the full outer join, while retaining every contributing row. |
| Controls and lifecycle | `runs`, `settlement_controls`, `run_issues`, `report_artifacts`, `schema_migrations` | Provides idempotent run identity, failure/retry state, header controls, diagnostics, report identity, and migration checksums. |

Money is parsed as checked signed cents in Go and stored as constrained exact `numeric` in PostgreSQL. Floating point is not used for matching, routing, totals, or acceptance checks. Composite foreign keys and triggers prevent cross-run, cross-config, and cross-source lineage.

## Assumptions

- The accounting scope is settlement `12395580393`, its Settlement rows, and Released Payment rows for that settlement. All other rows remain ingested and auditable.
- Payment reconciliation dates use the transaction release instant converted to a UTC calendar date.
- The supplied fixture is AUD and all accepted amounts have cent precision. Unsupported currency or precision is rejected rather than inferred.
- The settlement metadata row is retained in `source_rows` as a header control and is not treated as an amount component.
- The bank transfer is retained and reconciled separately from operating activity.
- The sample workbook defines report structure only; its zero Summary values are not expected results.
- The standalone Amazon Statement Summary was not supplied. The implementation compares independently calculated source summaries and the Settlement header without claiming validation against an absent artifact.
- `description=any` and `amount_description=any` are wildcards. An empty `transaction_type` is a fallback. Equal-specificity baseline matches are reported as ambiguity rather than resolved by arbitrary file order.
- Reconciliation status is based on key presence after per-source aggregation. Amount equality and summary-bucket equality are separate controls.
- A payment transfer description has one transaction-scoped normalization alias. The implementation does not use fuzzy matching.
- Repeated source text is not assumed to be a duplicate transaction. Idempotency is based on file, config, policy, and run identity.

## Mapping corrections

[MAPPING_FIXES.sql](MAPPING_FIXES.sql) contains the actual guarded `DELETE`, `UPDATE`, and history `INSERT` statements, with one commented block per defect. It applies changes to a new child of the frozen baseline, checks the expected old state and affected-row count, records before/after evidence, and freezes the corrected version. It does not update source rows or add balancing amounts.

The complete replay is automated by `make end-to-end`. To execute the standalone SQL after importing a baseline configuration:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
  -v baseline_id="$(psql "$DATABASE_URL" -Atqc \
    "select id from config_versions where name='amazon-au-baseline')" \
  -v fixed_name=amazon-au-fixed-f01-f03 \
  -f MAPPING_FIXES.sql
```

## Restore the submitted dump

Restore `output/reconciliation.dump` into an empty PostgreSQL database:

```sh
PGPASSWORD=recon createdb --host localhost --port 54329 --username recon \
  reconciliation_review
export RESTORE_DATABASE_URL='postgres://recon:recon@localhost:54329/reconciliation_review?sslmode=disable'
pg_restore --no-owner --no-privileges --dbname "$RESTORE_DATABASE_URL" \
  output/reconciliation.dump
psql "$RESTORE_DATABASE_URL" -c \
  'select name, stage, verification_status from runs order by id;'
```

The investigation history and executed checks are recorded in [PROGRESS.md](PROGRESS.md).
