# Ordered implementation checklist

Only check items supported by actual code/artifacts and verification. The checked gates below reflect the implemented local acceptance path; remaining unchecked items are explicitly future enhancements or environment-dependent work. Do not skip investigation because expected values are available.

## Phase 0 — established design inputs

- [x] Read all six assignment pages; inspect both raw formats and mapping configs.
- [x] Inspect Summary labels and Consolidated headers; treat values as non-authoritative.
- [x] Profile source hashes, counts, settlement populations and metadata controls.
- [x] Compare posted-date and release-date key policies against source totals.
- [x] Probe candidate config fixes with exact decimal arithmetic and per-key bucket checks.
- [x] Record requirements, architecture, schema, report layout, assumptions and handoff contracts.

## Phase 1 — bootstrap and exact primitives

- [x] Create go.mod/go.sum, cmd/recon and package boundaries in ARCHITECTURE.md.
- [x] Pin Go/PostgreSQL/library versions; create compose.yaml and Makefile.
- [x] Add .gitignore for secrets, raw input data, temporary/report/dump artifacts; retain original local files.
- [x] Implement money parse/format/checked arithmetic and synthetic tests.
- [x] Implement header and source-location readers, raw payload preservation and input limits.
- [x] Implement UTC date parsing and explicit source normalization profile.
- [x] Gate: parser and money tests pass; real source row count/hash/control profile reproduced without mapping.

## Phase 2 — database and immutable ingestion

- [x] Implement migrations, constraints and indexes in DATABASE.md.
- [x] Implement original config import with raw origin identity and immutable freeze.
- [x] Implement run claim/fingerprint, bounded COPY and atomic data transaction.
- [x] Ingest both sources into source_rows, including all excluded and metadata rows.
- [x] Implement scope classification and settlement controls.
- [x] Implement failure/retry lifecycle and concurrency/idempotency checks.
- [x] Gate: 78,006 raw rows retained per full run; source/metadata totals verified; no partial accepted state.

## Phase 3 — mapping and summary ledger

- [x] Compile field/literal templates and collision-safe expanded keys.
- [x] Implement exact/wildcard/fallback precedence, scoped transfer alias and ambiguity diagnostics.
- [x] Implement baseline diagnostic and strict modes explicitly.
- [x] Persist mapping decisions, nonzero contributions and zero/unsummarized states in the PostgreSQL adapter.
- [x] Enforce payment component/total conservation and no accidental double counting in the persisted contribution/group gates.
- [x] Accumulate source-isolated summary totals during ingestion in the in-memory pipeline.
- [x] Gate: baseline duplicates remain visible; independent SQL recomputation equals stored totals.

## Phase 4 — reconciliation and investigation

- [x] Aggregate each source once per scoped key, then full outer join.
- [x] Retain payment-only zeros/transfers and out-of-scope groups.
- [x] Implement separate presence status and amount difference.
- [x] Implement a database-backed explain command by field, rule, key and physical source line.
- [x] Gate: scoped key counts and zero total deltas match the reference controls in the full fixture run.

## Phase 5 — before report and config fixes

- [x] Generate before_fix.xlsx using untouched original rules and diagnostic mode.
- [x] Export rule/source-row evidence for each of F01–F03.
- [x] Write actual MAPPING_FIXES.sql with guarded expected-state changes and comments.
- [x] Test old-state mismatch rollback and repeat application safety.
- [x] Apply to a draft child version, freeze, re-ingest the original bytes and reconcile a new run.
- [x] Gate: only 2 deletes and 5 updates to mappings; source inputs and engine normalization unchanged.

## Phase 6 — final reporting and validation

- [x] Implement Summary rows, subtotals and source-independent values.
- [x] Implement Consolidated/Source Rows/Contributions/Mapping Issues/Run Info sheets.
- [x] Verify noneligible population visibility and original row lineage.
- [x] Implement atomic XLSX output, retry, string/precision/size-limit safeguards.
- [x] Generate after_fix.xlsx, verify it independently, and render the complete workbook for visual QA; Summary, first/middle/last audit pages, and Run Info are readable.
- [x] Gate: every populated Summary leaf/subtotal delta zero, activity equals header control, and independent per-key bucket SQL differences are zero in the fixed fixture.

## Phase 7 — submission and reproducibility

- [x] Run unit, race, vet, integration and reference acceptance checks; record actual results.
- [x] Measure performance and document hardware, duration and memory.
- [x] Produce database dump or representative auditable sample and restore-test it.
- [x] Run end-to-end from a fresh database without manual file changes.
- [x] Replace planned README commands with tested commands; document schema and assumptions.
- [x] Complete PROGRESS.md with actual investigation, rejected hypotheses and final evidence.
- [x] Verify docs/SUBMISSION.md inventory and original hashes.
- [x] Final review: no amount plugs, cross-source copying, suppressed rows, hidden ambiguity or unsupported claims.

## Phase 8 — adversarial audit hardening

- [x] Audit exact-money boundaries and every aggregation/subtraction path; reject empty and overflowing amounts.
- [x] Audit parser schemas, statuses, dates, metadata ownership, UTF-8, byte lineage and bounded inputs.
- [x] Audit config/rule/template validation and make guarded patch publication atomic.
- [x] Audit strict per-key bucket controls, report cells and independent SQL recomputation.
- [x] Enforce cross-run/config/source lineage, finite exact SQL money, frozen-version immutability and migration checksums.
- [x] Separate RECONCILED data state from atomic report registration; strengthen dump/restore comparisons.
- [x] Upgrade vulnerable dependencies; pass unit, randomized, race, vet, Staticcheck, Shellcheck and reachable-vulnerability scans.
- [x] Gate: fresh baseline/fixed PostgreSQL replay and fixed workbook pass with all 78,006 rows retained and zero fixed discrepancies.

## Phase 9 — Neon upgrade verification

- [x] Verify official direct and pooled endpoint formats and test both transports without exposing credentials.
- [x] Apply missing migrations through Neon's supported SQL-over-HTTP transport when raw TCP is unavailable.
- [x] Preserve legacy frozen-config hashes and backfill collision-safe canonical hashes through a checksummed migration.
- [x] Verify remote migration checksums, canonical config hashes, exact-money constraints, cross-run lineage and frozen immutability.
- [x] Serialize concurrent Go migrators and pass fresh replay, strict reconciliation, dump/restore, race, static analysis and vulnerability gates.

## Phase 10 — rubric and submission audit

- [x] Put guarded F01/F02/F03 DELETE, UPDATE and history INSERT statements directly in `MAPPING_FIXES.sql` and execute the artifact on PostgreSQL.
- [x] Generate canonical before/after workbooks and a current restore-tested PostgreSQL dump; commit their hashes and package them separately from GitHub.
- [x] Make README setup, schema rationale, assumptions and submission inventory self-contained.
- [ ] Add clean-clone CI and reproduce the documented flow from the pushed GitHub repository.
