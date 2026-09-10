# Progress log

## 2026-09-11 — checkpoint commits and visual report QA

Initialized the repository and committed the completed checklist as eight ordered commits (`phase-0` through `phase-7`), with matching lightweight tags. The working tree is clean; `.env`, raw `workingData`, generated `output`, binaries, and caches remain intentionally ignored.

Re-ran `go test ./...`, `go test -race ./...`, `go vet ./...`, shell syntax checks, Markdown-link checks, `make before`, `make after`, ZIP validation, `recon verify`, and the independent input/mapping profile. The fixed workbook rendered through LibreOffice to 26,619 pages and the baseline to 29,352 pages, both in landscape mode. Both Summaries fit on one page with their delta columns visible; visual inspection of the fixed Summary, a middle audit page, both final Run Info pages, and the baseline Summary passed. The report print layout was tightened for all sheets, with explicit widths for the Summary and Run Info columns.

## 2026-09-10 — database lifecycle and replay implementation

Closed the first persistence gap with additive migrations `002_audit_lifecycle.sql` through `005_immutability_triggers.sql`. The database now records frozen config versions and parent/child relationships, config source-file provenance, run files, settlement metadata controls, source-isolated summary contributions, reconciliation group membership, run issues, fix history, and report artifacts. Mapping rules are keyed by config version and frozen versions are protected by database triggers; they are never updated in place by a run.

Added `import-config`, `verify-db`, and `explain` CLI commands. `run` can load a frozen config version directly, automatically imports file-based configs when a database is supplied, persists all derived lineage with COPY/batched writes, and registers atomic report artifacts. `MAPPING_FIXES.sql` is now a versioned replay contract backed by guarded `clone_config_version` and `apply_mapping_fixes` functions; F01/F02/F03 snapshots are stored in `config_fix_history`.

Evidence from a fresh local PostgreSQL schema: `tools/end_to_end.sh` completed with 78,006 source rows in both runs, baseline `DIAGNOSTIC` with two issues and 5,973 per-key bucket differences, fixed `PASS` with zero issues and zero per-key bucket differences, 78,005 transaction memberships, zero unmatched transactions, zero amount mismatches, and zero independently recomputed summary mismatches. `tools/dump_restore_smoke.sh` restored a custom-format dump into an empty database with 2 runs, 156,012 source rows, 45,474 groups, and 2 config versions. Guard tests confirmed repeat application rejects a frozen child and a F01 guard mismatch rolls back without creating fix history. `verify-db` and `explain` both returned persisted evidence. The Neon schema was migrated through `005_immutability_triggers.sql`, and the original/fixed config versions were imported and replayed with three fix-history records; the full remote row replay remains intentionally unrun because the provider's COPY transfer rate is materially slower than the local fixture.

Migration `005_immutability_triggers.sql` was exercised after the fresh replay: a direct update against the frozen baseline was rejected and its rule count remained unchanged. The executable root `MAPPING_FIXES.sql` wrapper was also run in a clean config-only database and froze a child with the expected three fix-history records.

Failure lifecycle evidence: a clean scratch database with one deliberately removed summary-field registry row failed after the run claim, recorded `FAILED|FAIL|PERSIST_FAILED`, and retained no source rows. Restoring the registry row and rerunning the identical fingerprint with `--retry` reclaimed attempt 2, completed `PASS`, and `verify-db` passed with the full 78,006-row lineage.

The final stream-backed workbook run on the local Apple Silicon host completed in roughly 55 seconds for the strict in-memory path. The measured peak resident set was about 1.2 GiB, driven by the complete lineage model and Excel workbook generation; the architecture's 512 MiB engineering target is therefore recorded as an open performance optimization rather than claimed as met. Parser maps are released between sources and large workbook sheets use Excelize streaming to keep the next optimization boundary explicit.

## 2026-09-10 — specification and evidence preparation

Completed the requested planning/documentation phase. At that point no Go service, PostgreSQL run or final workbook was claimed complete; later entries record the implementation and acceptance runs.

- Read all six PDF pages and rendered them to check layout and requirement wording. Confirmed the explicit single-table ingestion requirement and presence-based reconciliation definition.
- Inspected the five files in workingData, including CSV preamble, all config selectors, settlement metadata, Summary row labels and Consolidated headers. The sample workbook was used only for structure.
- Found 23,026 payment rows across four settlement IDs and 54,979 settlement amount rows plus metadata for one settlement. A full-file payment sum was therefore ruled out as the primary statement comparison.
- Parsed formatted amounts with exact decimals for research. The first naive parse encountered comma-grouped monetary values; the production contract now specifies validated grouping rather than arbitrary comma deletion.
- Established selected Released activity: payment total 78,362.44 includes -133,756.51 bank transfer; operating activity is 212,118.95, exactly settlement amount sum and metadata total.
- Compared Order/Refund key dates. Original posted UTC day gives 34 shared keys and an amount difference; release UTC day gives 13,251 shared keys with zero amount differences. The remaining 25 payment-only Order keys have zero totals.
- A naive exact-only description match sent the dynamic transfer label through fallback and falsely added -133,756.51 to Amazon fees. Added an explicit scoped source-label alias to the specification; this is normalization, kept the same before and after config fixes.
- Identified duplicate payment selectors at config lines 5/6 and 71/72. Documented diagnostic all-candidate baseline behavior rather than silently inventing first-rule precedence.
- Verified tax decomposition at shared-key level. Proposed F01 duplicate deletions, F02 four settlement tax-route updates, F03 one payment refund-tax route update. Did not change inactive refund low_value_goods by analogy, since it has no nonzero evidence in scope.
- The independent probe reports 13,289 shared keys, 26 selected payment-only keys, no settlement-only keys and no shared amount differences. Before: 5,973 shared key/bucket variance pairs. After candidate edits: zero, with all active summary fields equal and both activity totals 212,118.95.
- Verified all payment row totals equal their 10 monetary components. Kept source hashes for all five artifacts.
- During full-pipeline validation, found and fixed a partitioning bug where settlement transaction rows did not inherit the metadata row's AUD currency. Before the fix all groups falsely appeared one-sided; after inheritance the Go report has 13,289 reconciled groups and 9,448 payment-only groups, matching the scoped controls. This is why full report validation is required beyond summary totals.
- Consulted official Amazon, Go, PostgreSQL, pgx and Excelize documentation for format, exact arithmetic, persistence and export choices; recorded sources in docs/RESEARCH.md.
- Wrote the implementation pack and a standard-library-only reproducible evidence tool. The checklist now records completed local application and persistence gates; the measured memory target and visual render QA remain explicitly open.

Final documentation checks: 20 Markdown files; all relative document links resolve; input SHA-256 hashes unchanged; all payment row component controls pass; the probe regenerates two original duplicate-selector diagnostics and none after fixing, with zero corrected bucket differences. These are documentation/data checks, not Go or database test results.

Implemented Phase 1 primitives and a working vertical pipeline: Go module, exact money parsing, CSV/TSV ingestion, UTC date normalization, config loading, indexed mapping engine, grouped reconciliation, Excel report generation, data-driven config patching, migrations, an optional pgx persistence adapter, and a structural/financial report verifier. `go test ./...`, `go vet ./...`, and `go test -race ./...` pass. `make before` and `make after` completed on the supplied data; `recon verify --report output/after_fix.xlsx` passes; XLSX archives pass `unzip -t` and independent workbook inspection confirms the expected sheets and Summary values.

The PostgreSQL adapter now uses pgx batch/COPY paths for source rows, mappings, and groups. A local PostgreSQL run completed in strict mode with `run_id=1`, `78006` source rows, `307468` row mappings, `22737` reconciliation groups, and `verification_status=PASS`; `recon verify` passed for that workbook. The supplied Neon credentials were written to the ignored `.env`, Neon connectivity was verified, and `recon migrate` applied successfully. The database-backed config-version/SQL replay path, immutable triggers, dump restore, and failure/retry lifecycle were subsequently exercised and are covered by the top evidence entry. A full Neon raw-row replay remains intentionally unrun because the provider's COPY transfer rate is materially slower than the local fixture. Spreadsheet conversion of the 18 MB audit workbook was also stopped after LibreOffice spent over a minute processing the large sheet; workbook integrity and cell-level inspection passed.

Remaining engineering work is limited to the documented operational follow-ups: visual rendering of the largest workbook sheet and reducing peak RSS below the 512 MiB target. Neither is allowed to weaken the exact-money, source-retention, SQL verification, or immutable-config gates.

## Builder log format

For each subsequent completed phase record: what changed; commands and results actually observed; source/rule/run evidence; rejected explanations; remaining limitations; next checklist item. Do not fabricate commits, successful checks or performance measurements.
