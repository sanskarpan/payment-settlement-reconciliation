# Runbook

These commands describe the implemented CLI and the reproducible persistence/replay flow. `make before` and `make after` run the full in-memory parsing, mapping, reconciliation, and Excel flow against the supplied fixture. `make end-to-end` exercises migrations, frozen config import, guarded SQL replay, two persisted runs, report verification, and database verification against a fresh database. PostgreSQL persistence is implemented behind `--postgres-url`/`DATABASE_URL`; a live database is required for that portion.

## Environment and command contract

Use a local PostgreSQL instance via compose, DATABASE_URL from an ignored environment file, Go toolchain pinned in go.mod/Dockerfile, and `bin/recon`. A CLI `--run` accepts the unique stored run name or ID. Add `name` to runs with a unique constraint; it is a user label, not the idempotency key. A completed fingerprint reuses the existing run regardless of a newly requested label and reports that reuse clearly.

The current CLI returns 0 for success, 1 for validation, mapping, verification, database, or I/O failure, and 2 for missing/unknown command usage. A diagnostic baseline generation returns 0 when its diagnostics/report were successfully produced, with verification=DIAGNOSTIC. Errors preserve wrapped causes without printing secrets; specialized infrastructure exit codes remain an operational enhancement.

```sh
docker compose up -d db
make build
bin/recon patch-config --input workingData/amazon_payment_configs_au_old.csv --source payment --output output/fixed/payment_configs.csv
bin/recon patch-config --input workingData/amazon_settlement_configs_au.csv --source settlement --output output/fixed/settlement_configs.csv
bin/recon run --payments workingData/amazon_payments_data.csv --settlements workingData/amazon_settlements_data.txt --payment-config workingData/amazon_payment_configs_au_old.csv --settlement-config workingData/amazon_settlement_configs_au.csv --settlement-id 12395580393 --output output/before_fix.xlsx --mode diagnostic-baseline
bin/recon run --payments workingData/amazon_payments_data.csv --settlements workingData/amazon_settlements_data.txt --payment-config output/fixed/payment_configs.csv --settlement-config output/fixed/settlement_configs.csv --settlement-id 12395580393 --output output/after_fix.xlsx --mode strict
```

The `patch-config` command is a generic, guarded data transformation driven by `fixes/mapping_fixes.csv`; it does not contain defect-specific report logic. The PostgreSQL adapter persists frozen config provenance, source rows, mapping decisions, source-isolated contributions, settlement controls, reconciliation groups/membership, issues, and report artifacts. Neither report is computed from the sample workbook.

`tools/end_to_end.sh` is the isolated run harness. For dump validation, set `SOURCE_DATABASE_URL` and `RESTORE_DATABASE_URL` and run `tools/dump_restore_smoke.sh`; it uses custom-format `pg_dump`/`pg_restore` and checks restored row counts. Do not implement replay by copying precomputed workbooks or loading reference JSON totals.

## Retry and recovery

- Input parse failure: inspect physical file/line error. Fix parser or explicitly obtain corrected input; do not manually edit the supplied fixture. Retry creates a controlled attempt after failure.
- Mapping ambiguity in strict mode: inspect config provenance, clone and correct rules. Do not switch final reporting to baseline mode.
- Key mismatches: inspect scope, release timestamps, tuple fields and source membership before changing record_ref.
- Summary mismatches with zero key amount differences: investigate component/rule routes; use MAPPING_FIXES.md.
- Report write failure: rerun report for the same immutable run. Never reapply money on retry.
- A persisted data failure leaves a `FAILED` run claim; rerun the same fingerprint with `--retry` only after correcting the cause. Completed fingerprints are reused and concurrent `CLAIMED` fingerprints are rejected.
- Unknown nonzero summary intermediate/control: emit unsupported-policy evidence; do not hide it or force zero.
- Crash during active claim: inspect run ownership/attempt and database transaction state; explicit retry must lock and record the interrupted attempt. Do not start concurrent recovery.

## Reproducibility and dump

Store input hashes, original/fixed config hashes, normalization/layout versions, Go/build version, selected settlement/currency, command flags, row counts, timestamps and workbook checksum. Report timestamps need not be deterministic; financial content must be.

Use pg_dump custom format with a tested restore procedure, or the assignment-allowed representative sample. Include schemas, original and corrected rule versions, run metadata, and enough joined source/mapping/contribution/group records to reproduce a specific defect and correction. A sample must disclose that it cannot reproduce full-dataset totals alone. Retain full raw financial data locally; publish only with authorization.

Restore into an empty DB and run verify/report from the restored accepted run. Do not leave passwords in shell history, SQL dumps, README or logs. Health checks verify database readiness, not just container start.

## Operational logs

Use slog structured records with run/stage/duration/count/error code. No raw descriptions, addresses, bank-account text or DATABASE_URL in default logs. `explain` is an explicit local audit command and may expose requested source evidence. Use parameterized database calls; fixed SQL input is a trusted local migration artifact, not arbitrary externally uploaded code.
