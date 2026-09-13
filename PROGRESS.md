# Progress log

This log records the investigation path and checks actually executed. Monetary values use exact decimal arithmetic; the sample workbook supplied the output structure only.

## 2026-09-10 - source profiling and hypotheses

- Read and visually checked all six assignment pages and inspected the five supplied files without editing them.
- Counted 23,026 payment rows, 54,979 settlement amount rows, and one settlement metadata row. The payment file spans four settlement IDs while the settlement file covers `12395580393`, so a whole-file comparison was rejected.
- Established the accounting scope as Released payments for `12395580393` plus the matching settlement rows. Payment total 78,362.44 includes a -133,756.51 bank transfer; operating activity is 212,118.95, equal to the settlement amount sum and metadata header.
- Compared posted and release dates for key construction. Posted UTC date produced only 34 shared Order/Refund keys and an amount difference. Release UTC date produced 13,251 shared Order/Refund keys with no amount differences, so the versioned key-date policy uses release UTC date.
- Rejected fuzzy description matching. One payment Transfer label requires a transaction-scoped normalization from `To account ending with: 334` to the configured `TO_ACCOUNT_ENDING` selector.
- Found equal-specificity payment selector pairs at config lines 5/6 and 71/72. The baseline retains and reports this ambiguity rather than inventing first-rule or last-rule precedence.

## 2026-09-10 - mismatch isolation

- Verified every payment row total against its ten monetary components.
- Aggregated payment and settlement values independently before joining. The scoped result has 13,289 shared keys, 26 payment-only keys, no settlement-only keys, and no shared-key amount differences.
- The original mappings produced 5,973 `(shared key, summary bucket)` differences. Four active Summary leaves differed: Product charges -92.84, Shipping +13,572.17, Other sales -0.36, and Refunded expenses +42.59, expressed as Payment minus Settlement.
- F01: traced 13,478.97 of excess Payment Shipping to duplicated payment rules at lines 5 and 72. The same fields already route to Product charges through equal-specificity rules. The correction deletes the two duplicate routes.
- F02: proved per shared key that Settlement tax components combine to the Payment tax-field grain. The correction routes settlement lines 61, 63, 95, and 118 to Product charges for both signs. It moves -92.84 net into Product charges, +93.20 into Shipping, and -0.36 out of Other without changing activity.
- F03: traced -42.59 across 16 Released refund rows to payment config line 14, whose summary targets were empty. Independent settlement refund-tax components total the same amount. The correction routes both signs to Refunded expenses.
- Did not change inactive refund `low_value_goods` by analogy because no nonzero selected row supported such a change.
- After F01-F03, all active leaves agree, both source totals equal 212,118.95, and all 13,289 shared keys have zero amount and bucket differences.

## 2026-09-10 to 2026-09-11 - implementation and defect correction

- Implemented exact checked cents, bounded CSV/TSV ingestion, UTC normalization, config matching, per-source summary accumulation, aggregate-before-join reconciliation, PostgreSQL persistence, and deterministic six-sheet XLSX output.
- Stored both source kinds in `source_rows` with raw/canonical payloads, file hashes, physical line and byte spans, scope, and source identifiers. Separate mapping, contribution, and membership tables preserve their distinct grains.
- Added immutable config versions, canonical config hashes, fix history, run fingerprints, retry lifecycle, mapping decisions, settlement controls, report registration, and database-backed `verify-db` and `explain` commands.
- A full-pipeline run exposed a currency-partition bug: settlement transaction rows did not inherit AUD from their metadata row, making every group appear one-sided. Fixed metadata inheritance and added coverage.
- Adversarial review added overflow-safe subtraction and aggregation, UTF-8 and schema rejection, missing-date/status checks, collision-safe identities, cross-run/config/source constraints, migration checksums, frozen-config triggers, strict bucket verification, and atomic workbook publication.
- The first dependency scan found reachable issues in older Excelize and pgx versions. Upgraded to Excelize 2.11.0, pgx 5.9.2, `x/text` 0.41.0, and `x/crypto` 0.56.0. The final symbol scan reported no reachable vulnerability.

## 2026-09-12 - database, report, and clean-clone acceptance

- `go test ./...`, `go test -race ./...`, `go vet ./...`, Staticcheck, Shellcheck, randomized money/reconciliation tests, and shell syntax checks passed.
- A fresh PostgreSQL 16 replay retained 78,006 rows per run. Baseline remained `DIAGNOSTIC` with two mapping issues and 5,973 bucket differences. Fixed strict mode reached `PASS` with zero issues, amount differences, bucket differences, Summary differences, or header differences.
- `MAPPING_FIXES.sql` created a child with three history records, 147 payment rules, and 137 settlement rules. Its canonical hash matched independent recomputation. Duplicate replay and an altered-old-state replay both failed without changing committed data.
- Failure lifecycle testing removed a required summary field, observed a recorded failed run with no partial source rows, restored the field, retried the same fingerprint, and completed on attempt two without duplicate money.
- Dump/restore testing reproduced 10 migration records, 2 config versions, 2 runs, 156,012 source rows, 660,502 mapping decisions, 214,897 contributions, and 2 report artifacts in an empty database.
- Both workbooks passed ZIP integrity and structural/financial verification. LibreOffice rendering confirmed the Summary and Run Info layouts plus representative first, middle, and final audit pages. All source and lineage rows remain in the workbooks, creating 26,619 fixed-report pages and 29,352 baseline-report pages.
- The strict in-memory reference run completed in roughly 55 seconds on the recorded Apple Silicon host. Peak RSS was about 1.2 GiB during complete lineage and workbook generation. Financial correctness passed; reducing that memory remains a scale optimization.
- A clean public GitHub clone passed source-only checks. After installing the supplied inputs and canonical artifacts, it passed reference, report, dump-catalog, and full fresh-database replay checks.

## 2026-09-13 - submission reduction and final audit

- Reduced the public repository to the artifacts required by the assignment: executable source and tests, `README.md`, `MAPPING_FIXES.sql`, and this progress log. The assignment PDF, builder specifications, checklists, exploratory evidence, and packaging helpers remain ignored local references.
- Current source checks passed: `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, build, gofmt, Staticcheck, Shellcheck, and shell syntax. `govulncheck` found no reachable vulnerability and no vulnerability in imported packages.
- Current fresh PostgreSQL 16 replay reproduced 78,006 rows per run, baseline with 5,973 diagnostic bucket differences, and strict fixed mode with zero issues, unmatched eligible rows, amount differences, bucket differences, Summary differences, or header differences.
- Current standalone `MAPPING_FIXES.sql` replay produced three history records, 147 payment rules, 137 settlement rules, and zero canonical-hash mismatches; a duplicate replay was rejected and left two config versions and three history records.
- Current canonical dump restore reproduced 10 migrations, 2 configs, 2 runs, 156,012 source rows, 660,502 mappings, 214,897 contributions, and 2 reports. `verify-db` passed the restored strict run.
- Cloned the minimized public tree anonymously into a new directory containing 47 tracked files. Source-only and external-artifact smoke checks passed. A new PostgreSQL 16 volume then reproduced the 78,006-row diagnostic baseline and strict fixed run; the latter finished with zero issues, unmatched eligible rows, amount differences, bucket differences, Summary differences, or header differences.
