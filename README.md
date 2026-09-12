# Amazon Payments / Settlement Reconciliation

This repository contains the implementation specification and a working Go batch pipeline. The implementation parses the supplied reports, applies immutable config-driven mappings, aggregates and reconciles keys, persists auditable lineage to PostgreSQL, replays guarded mapping fixes, and writes verified Excel workbooks.

Build a Go CLI backed by PostgreSQL that ingests both source files into one shared table, applies versioned mapping data, reconciles shared keys, and produces independently calculated accounting summaries and auditable Excel reports. This is the PortOne SDE II assignment; the PDF is the requirements authority.

## Submission artifacts

| Requirement | Artifact |
| --- | --- |
| Go source and run instructions | This repository and the commands below |
| Executable, commented config corrections | [MAPPING_FIXES.sql](MAPPING_FIXES.sql) |
| Original-config report | `output/before_fix.xlsx`, in the external artifact package |
| Corrected-config report | `output/after_fix.xlsx`, in the external artifact package |
| Restore-tested PostgreSQL dump | `output/reconciliation.dump`, in the external artifact package |
| Investigation history | [PROGRESS.md](PROGRESS.md) and [mapping evidence](docs/MAPPING_FIXES.md) |
| Artifact integrity manifest | [SUBMISSION_SHA256SUMS](SUBMISSION_SHA256SUMS) |

The assignment inputs, generated workbooks and database dump are delivered outside GitHub because they are large and contain provided financial data. Their expected hashes are committed in `SUBMISSION_SHA256SUMS`. `.env`, binaries, scratch reports and local database volumes are excluded.

## Prerequisites and quickest verification

Install Go 1.27. From a clean clone, source-only verification requires no assignment data:

```sh
make submission-smoke
```

To assemble the exact files that should accompany the public GitHub link, run:

```bash
make package-submission
```

This creates `output/submission/` and `output/portone-sde2-submission-artifacts.zip`, including the mapping SQL, both canonical reports, the restore-tested database dump, progress log, checksums, and delivery instructions. The generated package remains ignored by Git and is intended for the separate Drive or email delivery described below.

To reproduce the reference result, copy the separately delivered `workingData/` and `output/` directories into the repository, then install Docker with Compose, PostgreSQL client tools (`psql`, `pg_dump`, and `pg_restore`), and `unzip`:

```sh
make submission-smoke
make reference-test
make before
make after
```

When the external package is present, `submission-smoke` verifies every artifact hash, verifies the strict workbook, checks both XLSX archives, and reads the dump catalog. It always compiles the CLI and runs unit tests and vet. For the complete persisted flow, start an empty PostgreSQL database and run:

```sh
docker compose down -v
docker compose up -d db
export DATABASE_URL='postgres://recon:recon@localhost:54329/reconciliation?sslmode=disable'
make end-to-end
```

The end-to-end command migrates the empty database, imports and freezes the original mappings, creates and fixes an immutable child mapping version, persists separate before/after runs, generates both reports, and independently verifies database and workbook controls.

## Schema rationale and assumptions

Both reports enter `source_rows`, as required, with their source file, physical line/byte span, raw payload and normalized audit fields. Mapping decisions, summary contributions and reconciliation membership use separate tables because they have different grains; composite foreign keys prevent cross-run lineage. Money is parsed as checked signed cents in Go and stored as constrained exact `numeric` in PostgreSQL. Configurations and completed runs are versioned and immutable, so the original and corrected results remain independently reproducible. See [docs/DATABASE.md](docs/DATABASE.md) for the full schema.

The accounting scope is settlement `12395580393`, its Settlement rows, and Released Payment rows for the same settlement. Payment key dates use the release instant converted to UTC. Reconciliation status is determined by key presence after each source is aggregated, while amount and bucket equality are separate controls. The sample workbook defines presentation structure only. Amazon's standalone Statement Summary was not supplied, so the implementation proves equality between both independent sources and the Settlement header control without claiming validation against an absent artifact. See [docs/ASSUMPTIONS.md](docs/ASSUMPTIONS.md) for the complete policy register.

## Start here

1. Read [SPEC.md](SPEC.md) and [docs/DATA_CONTRACT.md](docs/DATA_CONTRACT.md).
2. Read [ARCHITECTURE.md](ARCHITECTURE.md), [docs/DATABASE.md](docs/DATABASE.md), and [docs/MAPPING_ENGINE.md](docs/MAPPING_ENGINE.md).
3. Read [docs/MAPPING_FIXES.md](docs/MAPPING_FIXES.md), [docs/RECONCILIATION.md](docs/RECONCILIATION.md), and [DESIGN.md](DESIGN.md).
4. Review [CHECKLIST.md](CHECKLIST.md), [docs/BUILDER_HANDOFF.md](docs/BUILDER_HANDOFF.md), and the actual evidence in [PROGRESS.md](PROGRESS.md).

| Document | Owns |
| --- | --- |
| [SPEC.md](SPEC.md) | Scope, requirements, acceptance, non-goals |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Modules, data flow, transaction boundaries |
| [DESIGN.md](DESIGN.md) | Finance-facing workbook and CLI design |
| [CHECKLIST.md](CHECKLIST.md) | Ordered build tasks and evidence gates |
| [Data contract](docs/DATA_CONTRACT.md) | Headers, amounts, dates, population selection |
| [Database](docs/DATABASE.md) | Tables, constraints, indexes, lifecycle |
| [Mapping engine](docs/MAPPING_ENGINE.md) | Matching, templates, sign routing, ambiguity |
| [Mapping fixes](docs/MAPPING_FIXES.md) | Observed defects and SQL change contract |
| [Reconciliation](docs/RECONCILIATION.md) | Aggregation, matching, invariants, audit |
| [Testing](docs/TEST_PLAN.md) | Unit, integration, property and dataset checks |
| [Runbook](docs/RUNBOOK.md) | Commands, replay, dump, recovery |
| [Decisions](docs/DECISIONS.md) | Alternatives and reasons |
| [Research](docs/RESEARCH.md) | External primary sources and limits |
| [Evidence](docs/evidence/README.md) | Reproducible observations and controls |
| [Risks and assumptions](docs/ASSUMPTIONS.md) | Explicit policies and unsupported claims |
| [Submission](docs/SUBMISSION.md) | Final deliverable inventory |

## Verified data findings

The files have 23,026 payment records and 54,979 settlement amount records plus one settlement header record. The payment file includes four settlement IDs; the settlement file covers `12395580393` only. Its header control is AUD 212,118.95. Released payments for that settlement, excluding the bank transfer from operating activity, independently total exactly the same amount.

The supplied configs contain overlapping tax routes and inconsistent tax bucketing. A read-only Python probe applying the documented candidate data changes obtains exact summary equality and zero per-bucket differences for all 13,289 matched keys. This is evidence for the Go implementation, not a license to hardcode these values. The production pipeline must reproduce the findings from the untouched inputs and SQL changes.

## Run the implemented pipeline

The supplied data can be processed without external services:

```sh
make before
make after
```

This writes `output/before_fix.xlsx` using the original mapping data in diagnostic mode and `output/after_fix.xlsx` using the data-driven patch file in strict mode. The original files under `workingData/` are not modified.

For PostgreSQL persistence, start the included local database and pass its URL:

```sh
docker compose up -d db
DATABASE_URL='postgres://recon:recon@localhost:54329/reconciliation?sslmode=disable' \
  ./bin/recon run --payments workingData/amazon_payments_data.csv \
  --settlements workingData/amazon_settlements_data.txt \
  --payment-config workingData/amazon_payment_configs_au_old.csv \
  --settlement-config workingData/amazon_settlement_configs_au.csv \
  --settlement-id 12395580393 --output output/before_fix.xlsx \
  --mode diagnostic-baseline
```

`MAPPING_FIXES.sql` documents the guarded, versioned SQL replay. Import the original rules, clone the frozen version, apply the fixes to the draft, and then run against the frozen child:

```sh
./bin/recon import-config \
  --payment-config workingData/amazon_payment_configs_au_old.csv \
  --settlement-config workingData/amazon_settlement_configs_au.csv \
  --name amazon-au-baseline --postgres-url "$DATABASE_URL"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c \
  "select clone_config_version((select id from config_versions where name='amazon-au-baseline'),'amazon-au-fixed-f01-f03','F01/F02/F03 guarded replay');"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c \
  "select apply_mapping_fixes((select id from config_versions where name='amazon-au-fixed-f01-f03'));"
```

Run the full fresh-database replay with `make end-to-end` after setting `DATABASE_URL`. It retains one shared `source_rows` population, writes the before and after runs, and calls both `verify` and `verify-db`. Use `bin/recon explain --run-id ID` to trace a persisted contribution by field, record reference, or physical source line. `bin/recon verify-db --run-id ID --expected-rows 78006` independently checks source counts, group membership, summary-contribution totals, and strict-run status.

The current `run` command persists source rows, mapping decisions, summary totals, contributions, settlement controls, reconciliation groups, group membership, issues, and report artifact metadata when `DATABASE_URL`/`--postgres-url` is supplied.

For the configured Neon database, `.env` contains the supplied connection settings and is ignored by Git. Load it into the current shell before running database commands:

```sh
set -a; source .env; set +a
./bin/recon migrate --postgres-url "$DATABASE_URL"
```

Do not commit `.env` or copy its credentials into source files, reports, or logs.

## Evidence reproduction available now

```sh
python3 tools/profile_inputs.py
```

Requires Python 3.10+ standard library only. It reads `workingData/` and writes the three JSON evidence files under `docs/evidence/`. It does not modify input files, create a database, or produce submission reports. Commands requiring an external PostgreSQL service are explicitly identified in the runbook.

Preserve the original files. The provided financial data and generated artifacts are distributed separately with their committed hashes. Never publish them or copy credentials from `.env` without explicit authorization.
