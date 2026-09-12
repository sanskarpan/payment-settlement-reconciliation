# Submission acceptance

GitHub source repository: `https://github.com/sanskarpan/payment-settlement-reconciliation` (private; grant the reviewer access when submitting).

The GitHub repository contains the Go pipeline, immutable PostgreSQL replay path, artifact hash manifest, evidence, and implementation contracts. The assignment inputs, canonical before/after workbooks and restore-tested dump are delivered as a separate package because of size and financial-data sensitivity. The local acceptance evidence below is recorded in `PROGRESS.md`; external Neon raw-row replay and peak-RSS optimization remain operational limitations rather than hidden claims.

Run `make package-submission` to create the verified delivery directory at `output/submission/` and its single-file archive at `output/portone-sde2-submission-artifacts.zip`. Send that archive with the public repository URL. It deliberately excludes credentials and the supplied financial input files.

| Artifact | Required evidence |
| --- | --- |
| Go source and GitHub link | Clean build, meaningful tests, separation of ingestion/reconciliation/reporting; large provided data remains in the separate artifact package |
| README.md | Actual tested fresh-machine setup, end-to-end commands, schema rationale, assumptions and exact scope |
| MAPPING_FIXES.sql | Executable guarded SQL; commented F01/F02/F03 blocks, no monetary plugs, tested replay |
| `output/before_fix.xlsx` | Original config version, disclosed baseline ambiguity, source-derived values and audit sheets |
| `output/after_fix.xlsx` | Frozen fixed version, zero leaf/subtotal differences, full lineage and preserved unmatched/excluded rows |
| `output/reconciliation.dump` | Restore tested; contains both full runs and supports trace from source to config to report |
| PROGRESS.md | Actual chronology, failed hypotheses, row-level evidence, tests and measured performance |
| Run manifests/evidence exports | Input/config/version hashes, selected scope, counts, controls, workbook checksums |

## Final reviewer checks

- Both sources are present in one source_rows table; all 78,006 original records retained per run.
- Input hashes remain unchanged. Original config version remains available beside corrected version.
- Primary accounting scope is labeled; excluded populations and bank-transfer movement are auditable.
- Presence-based statuses and amount-difference flags are separate.
- Go mapping/report code contains no fixture totals, source-line-specific corrections or source cross-copying.
- SQL explains and reproduces the exact original-to-fixed mapping delta.
- Both workbooks pass ZIP/structure, cell-level, and visual rendering checks; Summary, audit-page samples, and Run Info are readable, and leaf/subtotal checks agree with SQL.
- One command reproduces the workflow on a fresh DB; repeated ingestion cannot duplicate money.
- Final narrative distinguishes settlement header control from unavailable standalone Amazon Statement Summary line validation.

A report that appears balanced while dropping unexplained rows or compensating errors does not satisfy acceptance. Keep the relevant error evidence and continue investigation.
