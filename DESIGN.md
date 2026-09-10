# Report and interaction design

The primary product surface is an auditable workbook, supported by a CLI that makes runs reproducible. No browser dashboard is needed. The layout must help a finance reviewer distinguish matched identity, amount variance, scope exclusion and mapping defects without interpreting developer logs.

## Workbook sheets

Create a new workbook; use the sample's Summary structure and labels. Never copy its populated Consolidated Data rows into an output. Recommended ordered sheets:

1. `Summary` — template accounting layout and independent values.
2. `Consolidated Data` — all transaction groups, with matched sides together.
3. `Source Rows` — one row per original parsed source row, including metadata and excluded populations.
4. `Contributions` — nonzero rule contributions with group/source-row references.
5. `Mapping Issues` — duplicate selectors, unsupported mappings, row conservation errors; present even when empty.
6. `Run Info` — immutable provenance, scope census, control bridge, verification and source limits.

These extra sheets preserve audit access without turning the main consolidated view into a raw Cartesian join. Every original row must be reachable from Source Rows; every transaction row must also belong to one Consolidated group. Metadata is a control record, not an unreconciled money transaction.

## Summary cells and field registry

Preserve B labels and C/D/E value roles: C1 Payments, D1 Settlements, E1 Payments - Settlements. Keep the sample row positions below. All values are signed AUD currency amounts, two decimals; missing contribution to a known supported leaf displays numeric zero.

| Row | Label in B | Direct field or subtotal |
| --- | --- | --- |
| 4 | Sales | SUM(rows 5:13) |
| 5 | Product Charges | sales_product_charges |
| 6 | Tax | sales_tax |
| 7 | Shipping | sales_shipping |
| 8 | Amazon fees | sales_amazon_fees |
| 9 | Inventory Reimbursements | sales_inventory_reimbursements |
| 10 | Cross-account Debt Adjustment | No active supplied direct target; zero |
| 11 | Other | sales_other |
| 12 | FBA Fees | No active supplied direct target; zero |
| 13 | Micro Deposit (Failed) | Positive bank_account_transfer_round_off, if supported |
| 15 | Refunds | SUM(rows 16:17) |
| 16 | Refund expenses | refunded_expenses |
| 17 | Refunded sales | refunded_sales |
| 19 | Expenses | SUM(rows 20:28) |
| 20 | Promo rebates | expenses_promotional_rebates |
| 21 | FBA fees | expenses_fba_fees |
| 22 | Cost of Advertising | expenses_cost_of_advertising |
| 23 | Shipping Charges | No active supplied direct target; zero |
| 24 | Amazon fees | expenses_amazon_fees |
| 25 | Reversed Reimbursements | expenses_reversed_reimbursements |
| 26 | Cross-account Debt Adjustment | No active supplied direct target; zero |
| 27 | Other | expenses_other |
| 28 | Micro Deposit | Negative bank_account_transfer_round_off, if supported |
| 32 | Paid To Amazon | paid_to_amazon |

Row 13/28 sign interpretation is a proposed extension, not exercised by the input fixture. Until a synthetic business fixture and documented semantics are approved as part of implementation, a nonzero bank_account_transfer_round_off must raise UNSUPPORTED_SUMMARY_POLICY rather than silently pick a line.

Registry must include all target names from original configs. `beginning_balance`, `current_reserve_amount`, `amazon_carried_forward` are nonoperating control fields, absent from the visible template. Display any such nonzero totals separately in Run Info and require a defined payout bridge before accepting that new input. `total_adjustment_other_buyer_recharge_amt` and `total_refund_expense_or_sales_amt` are intermediate targets whose final sign/netting behavior is not established by this fixture. Preserve mappings; if activated nonzero, fail accepted reporting with a specific unsupported-policy error. Do not guess meaning from a prefix, silently omit money, or sweep it into Other. All these fields are zero/inactive in the selected fixture, so this is not a blocker for the assignment's supplied dataset.

Keep blank spacer rows from the sample. Add a clearly separated block below row 40 for `Net operating activity` (Sales + Refunds + Expenses), `Settlement header control`, and `Activity minus header control`; do not overload Paid To Amazon with a net-total or bank-transfer figure. Label the header as a settlement-source control, not a second independently supplied statement. Paid To Amazon is a separate signed leaf, zero here; no undocumented universal payout equation.

E values are always C minus D, including subtotal rows. Expenses remain negative where the source is negative. After controls: Sales 357,801.39; Refunds -1,815.09; Expenses -143,867.35; Net operating activity 212,118.95. Verify these arithmetic values independently when implementing.

## Consolidated Data columns

Use a single filterable header row with a short metadata band above. Freeze key columns and header. In order:

- group_id, reconciliation_status, scope_reason, currency, settlement_id, record_ref, amount_equal, summary_equal.
- payment_row_count, payment_amount, settlement_row_count, settlement_amount, difference.
- payment transaction type(s), description(s), SKU(s), posted/release date(s), matched summary fields.
- settlement transaction type(s), amount type(s), amount description(s), SKU(s), posted date(s), matched summary fields.
- Per-leaf payment, settlement and difference columns in Summary leaf order (may be grouped/collapsible).
- Audit link into Source Rows and/or group filter key; representative metadata must indicate multiplicity.

Sort status rank: reconciled, unreconciled_payment, unreconciled_settlement. Within each rank sort scope reason, settlement_id, currency, canonical key using a stable lexical ordering. For the payment-only section place IN_SCOPE before DEFERRED/OTHER_SETTLEMENT. Expose all transaction groups, including noneligible ones; primary Summary covers only the declared eligible scope. The workbook must not make excluded money look like a reconciliation defect.

Source Rows includes source row ID, group ID where applicable, file/hash/lines, source, row kind, scope, identifiers/dates/status, original amounts, and normalized fields. Contributions includes contribution ID, group ID, source row ID, config version/rule/origin line, source component, signed amount, chosen field. For full raw payloads exceeding cell limits, store columns rather than one giant JSON cell; provide a lossless companion audit export if a cell cannot fit and disclose its path/hash. Never silently truncate evidence.

## Formatting and XLSX correctness

Use a restrained dark header, white background, bold subtotals, thin separators and readable 11-point type. Format money with two decimals and negative parentheses, e.g. `#,##0.00;[Red](#,##0.00);0.00`. Use a subtle variance highlight only when a value is nonzero. Status is text plus color so grayscale and color-blind readers can still interpret it. Avoid celebratory badges or claims of accounting perfection.

Write IDs and raw text explicitly as strings (preserve leading zeros, avoid scientific notation and formula injection). Only trusted generated formulas may use formula APIs. For Summary prefer precomputed numeric values with independent checks, or formulas with verified cached values; never depend on a viewer recalculating empty formula caches. Go/SQL exact values remain authoritative; bound Excel numeric precision and verify cents round-trip. Refuse numbers whose cents cannot survive Excel precision, or export exact text audit values with a clear limitation; do not silently round them.

Excelize streaming requires ascending rows and Flush; do not mix streaming and ordinary cell writes on the same sheet. Set widths/styles/panes through supported stream APIs before writing. Use regular APIs on small Summary and stream large audit sheets. Use deterministic logical row ordering; XLSX ZIP metadata need not be byte-identical across runs. Enforce Excel limits (1,048,576 rows, 16,384 columns, 32,767 characters per cell); split audit sheets with an index if needed. Never silently drop rows.

Reopen output with an independent XLSX reader, check sheet names/types/values/row counts, and render Summary plus representative first/middle/last consolidated and audit sections in a spreadsheet viewer. Verify no clipped amounts, stale formulas, broken links or missing groups. Validate the entire ledger logically rather than relying on screenshots.

## CLI behavior

Each command reports run ID, stage, config version, counts, scope and artifact path. Errors identify file/line/rule when applicable and explain remediation. Do not print full raw rows or credentials by default. Emit structured JSON with `--json`; keep decimal amounts as strings. `verify` must return nonzero for an unaccepted fixed run even if an XLSX was produced. Before reports explicitly say `Diagnostic baseline: original mapping defects`.
