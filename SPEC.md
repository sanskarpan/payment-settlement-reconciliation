# Specification

Version: 1.0, 2026-09-10. Status: ready for implementation with explicit policies in docs/ASSUMPTIONS.md. Monetary controls are observed fixture results, not executable business constants.

## Outcome

An evaluator can run one documented flow from original source files and original configs to a diagnostic before-fix workbook; apply commented SQL fixes to a cloned config version; re-ingest the same bytes and generate an after-fix workbook. Both report columns derive only from their own source. The evaluator can follow a summary amount to its rules and exact raw rows.

## Requirements and traceability

| ID | Requirement | PDF | Design / verification |
| --- | --- | --- | --- |
| R01 | Go implementation; PostgreSQL; Excel output | p5 | Architecture; fresh-machine end-to-end test |
| R02 | Read CSV/TXT as supplied; normalization inside code | p2 | Data contract; byte hashes unchanged |
| R03 | Both source files in ONE source-row table | p2 | Database; row-count SQL |
| R04 | Configs stored as table data | p2, p5 | Mapping engine; SQL-only fix replay |
| R05 | Every input row and report contribution traceable | p2, p4 | Lineage tables; source audit tests |
| R06 | Compile `record_ref` templates; wildcard descriptions; fallback transaction type | p3 | Mapping engine fixtures |
| R07 | Route by amount sign; blank target excludes only summary contribution | p3 | Routing conservation tests |
| R08 | Shared-key presence defines three reconciliation statuses | p3 | Reconciliation; zero/different amounts tests |
| R09 | Aggregate differing granularities before matching | p3 | Independent group sums; no fan-out |
| R10 | Summary follows supplied layout; independent source columns | p4 | Design; workbook cell assertions |
| R11 | Consolidated records ordered matched, payment-only, settlement-only | p4 | Stable output sorting test |
| R12 | Exact mismatch evidence; config changes; re-ingest until fixed | p4–5 | Mapping fixes and before/after artifacts |
| R13 | Source/README, SQL, two workbooks, dump/sample, progress log | p5 | Submission checklist |
| R14 | Prefer summary computed during ingestion | p3 bonus | Transaction-local contribution aggregation |
| R15 | Error handling, idempotency, tests, larger-file performance | p6 bonus | Test plan and runbook |

## Population and meaning of success

Ingest all 23,026 payment rows and all 54,980 settlement rows, including the settlement metadata row. Do not discard deferred rows, other settlement IDs, transfers, zeros, or blank-summary mappings during ingestion.

For the primary Summary select payment rows whose settlement ID is the selected settlement ID and status is Released. Select settlement amount rows for that settlement ID. The provided run selects `12395580393`, inferred from the sole settlement metadata record, and AUD. With multiple settlement headers the CLI must require an explicit selection or generate separate runs; never combine currencies or settlements implicitly.

Transfers are included in reconciliation and audit but their supplied exact mapping has empty summary targets. Operating summary excludes that transfer by mapping, not by deleting the row or subtracting a fixed amount. Other settlement IDs and Deferred payments remain visible with scope reasons. They are not mapping defects.

Required fixed-run success:

- All original rows retained; no in-scope unresolved mappings or ambiguous nonzero routes.
- Every primary Summary leaf and subtotal has Payment minus Settlement = exactly 0.00.
- Every matched in-scope group has zero amount delta and zero leaf-bucket deltas for the supplied data; these are stronger fixture controls, not the definition of `reconciled`.
- Independent source activity sums and the settlement header control agree at 212,118.95 for this fixture.
- Out-of-scope rows and the in-scope unmatched transfer/zero rows are disclosed; no requirement to eliminate legitimate unmatched rows.
- Before/fixed workbooks and SQL replay demonstrate why each variance changed.

A zero Summary alone is insufficient: offsets can conceal row-level routing errors or duplicated contributions. Both population controls and per-key bucket checks are mandatory.

## Authority limits

The workbook is a structural reference with deliberately zero Summary cells. No standalone Amazon Statement Summary statement is supplied. The settlement header `total-amount` provides a payout control, not independent proof of every Amazon statement line. Label this limitation in the report metadata and README. Do not claim the absent statement has been verified.

## Deliverable boundaries

Build a local batch CLI, not an HTTP service. No seller API integration, OAuth, frontend, Kubernetes, queue, user management, ML/fuzzy matching, or FX conversion. PostgreSQL persistence, reproducible processing, exact money, and finance auditability are the substantive complexity. Generalize file paths and settlement selection, but do not pretend to support untested marketplaces or settlement types.

## Acceptance hierarchy

1. Source bytes, parser coverage, money and scope controls.
2. Mapping correctness and immutable rule lineage.
3. Key aggregation, group coverage and conservation.
4. Independent summary calculation and report layout.
5. Evidence-backed SQL fixes, replay and final submission.

Do not weaken an earlier layer to make a later workbook look correct.
