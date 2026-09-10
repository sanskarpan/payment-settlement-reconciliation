# Test plan

Tests establish reproducible correctness; they cannot guarantee no bugs. Cover failure modes that can change money, hide rows, or invalidate the audit trail. Use small synthetic fixtures for most tests and the supplied files for an opt-in acceptance suite.

## Unit and property tests

| Area | Cases and expected behavior |
| --- | --- |
| Money | 0, -0.00, -0.41, 1,234.50, one decimal, largest accepted magnitude; reject 1,2, 1.001, NaN, infinity, exponent, currency symbols and overflow. No float parse. |
| CSV/TSV | Preamble varies, BOM, quoted commas/tabs, escaped quotes, multiline description, CRLF, final newline absent, blank physical lines; exact physical source location. |
| Bad inputs | Duplicate header, missing required column, invalid UTF-8, malformed quotes, wrong width, too-large file/record; explicit error and no accepted partial run. |
| Dates | GMT+9 and GMT-3, midnight UTC rollover, mixed AM/PM, leap day, invalid date, missing Released date, empty Deferred date; UTC day independent of host TZ. |
| Aliases | FBA header alias, space-to-underscore label normalization, meaningful hyphen preservation, transfer prefix restricted to Transfer, no SKU/order-ID folding. |
| Matching | Exact beats any; transaction-specific suppresses blank-type fallback at row level; fallback used only when no specific match; no unordered map/file-order precedence. |
| Duplicate rules | Original duplicates diagnosed, baseline routes both and flags them, strict rejects, fixed removes duplicates. Input rule order permutation cannot change money. |
| Key grammar | Field/literal separation; GENERAL ADJUSTMENT literal; missing token value; empty template; `+` inside identifier; collision-safe tuple; same key across separate settlements/currencies cannot match. |
| Routing | Positive/negative/zero; empty target retains row; unknown target errors; optional-absent field distinct from zero; routed total does not also count covered components. |
| Aggregation | 1 payment to 3 settlement rows; 2 payments to 3 settlement rows; opposing signs; matched unequal sums stays reconciled; unmatched zero retained. |
| Scope | Selected Released included, selected Deferred excluded from Summary, other settlements disclosed; excluded rows cannot match eligible rows. |
| Audit | Every contribution resolves to one rule/source; metadata has no transaction amount; every transaction maps to one group. |
| Reporting | Source strings starting = + - @ stay strings; IDs/leading zeros preserved; missing side not misrepresented as a present zero; signed subtotals; zero bucket rows visible. |

Property/fuzz tests: parsing never panics; money Parse(Format(x))=x within range; group sums conserve source sums; input ordering does not change grouped results; duplicating a legitimate raw row doubles its contribution (do not auto-dedupe identical payloads); replaying the same complete file/run does not double the database; partitioning unrelated settlement IDs cannot change eligible totals. Seed fuzzers with multiline and malformed quoted data.

## PostgreSQL integration tests

Use a real isolated PostgreSQL instance of the pinned version, not SQLite. Migrate an empty DB. Test constraints and pgx numeric/COPY round-trips. Verify:

- Both source kinds occupy source_rows; no alternative raw ingestion table.
- Late parse failure rolls back all source/derived rows for that attempt and records FAILED.
- Two concurrent attempts with the same fingerprint yield one logical ingestion, no duplicates.
- Same bytes with changed config/normalization version produce a new immutable run.
- Frozen config mutation rejected; SQL fixes only change a draft child and expected rows.
- SQL old-state mismatch rolls back all fixes; repeat application has no side effects.
- Cross-run/source/config lineage violations are rejected.
- Streaming summary equals independent SQL aggregation of contributions.
- Report retry after file/DB registration failure preserves source/config/run data.
- Dump restores into a clean database and reproduces logical report totals.

## Reference-data acceptance

First check all input SHA-256 hashes. Only apply these fixed expected counts to matching original hashes. Assertions come from observed source data; never use them to drive the engine.

| Check | Expected |
| --- | ---: |
| Payment source rows | 23,026 |
| Settlement transaction rows | 54,979 |
| Settlement metadata rows | 1 |
| Combined source rows per full run | 78,006 |
| Original payment/settlement config rules | 149 / 137 |
| Fixed payment/settlement rules | 147 / 137 |
| Selected Released payment rows | 13,472 |
| Selected Deferred payment rows | 2,398 |
| Other-settlement payment rows | 7,156 |
| Eligible matched keys | 13,289 |
| Eligible payment-only keys | 26 |
| Eligible settlement-only keys | 0 |
| Eligible matched amount mismatches before and after | 0 |
| Before matched group/leaf variance pairs | 5,973 |
| After matched group/leaf variance pairs | 0 |
| Settlement amount/header total | 212,118.95 |
| After independent operating activity per source | 212,118.95 |

Before leaf differences: Product Charges -92.84, Shipping +13,572.17, Other -0.36, Refund expenses +42.59; others zero. Diagnostic mode and normalization are fixed in MAPPING_FIXES.md. After leaf amounts are listed there. Every Summary E amount must be zero after fixing, not merely rounded to zero for display.

Validate per-key tax identities as well as aggregate controls: payment Order sales_tax_collected equals settlement Order ItemPrice Tax + ShippingTax + GiftWrapTax + Promotion TaxDiscount; payment low_value_goods equals settlement Order withheld principal + shipping. Refund tax identity excludes unrelated transaction types sharing a group. Include the single GiftWrapTax source row 31122 as a tiny regression fixture.

The Python probe is independent exploratory evidence and uses a permissive amount parser and narrow input assumptions. Go acceptance must not call it to get runtime results. Freeze small expected synthetic fixtures by hand, and use SQL/raw decimal checks independently of the Go mapping implementation.

## Report verification

Reopen both files using a reader independent of Excelize where practical. Assert sheet order, numeric cells, labels, counts, summary values, delta sign, source row coverage, no formula error cells and no copied sample data. Check logical checksums of sorted records instead of binary XLSX equality. Render/view Summary and first/middle/last ranges of large sheets; verify readability and audit navigation. Test an export that exceeds a cell limit to prove no silent truncation.

## Commands required in the implemented repository

`go test ./...`, `go test -race ./...`, `go vet ./...`, an integration test target, an opt-in reference-data target, and a fresh-database end-to-end target. Record actual commands and results. Run the appropriate checks after each phase; avoid repeatedly rerunning the entire large suite when no relevant code changed.

Performance: report stage durations, rows/sec and peak Go RSS on reference and generated 10x files. Synthetic scale generation must preserve valid row/key uniqueness rules and expected totals. The two-minute/512-MB target is not a current benchmark result.

## Recorded acceptance evidence

The fresh-database `tools/end_to_end.sh` run exercised migrations, config import, SQL child-version replay, baseline and strict runs, workbook verification, and `verify-db`. It retained 78,006 source rows per run, produced 5,973 baseline per-key bucket differences and zero fixed differences, and found zero fixed amount, summary, membership, or unmatched-row violations. `go test ./...`, `go test -race ./...`, and `go vet ./...` pass. `tools/dump_restore_smoke.sh` restored the custom dump into an empty database and reproduced the run/source/group counts. The independent LibreOffice render produced 26,619 pages; visual inspection covered the Summary, a middle audit page, and the final Run Info page, while ZIP integrity and the Excelize structural verifier also pass.
