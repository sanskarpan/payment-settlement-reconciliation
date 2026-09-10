# Reconciliation and accounting invariants

## Group identity

Use `(run_id, settlement_id, currency, scope_partition, expanded record_ref tuple)`. IN_SCOPE payments and settlement transactions share the eligible partition. Deferred and other-settlement payment records use separate noneligible partitions and cannot match eligible settlement amounts. Preserve their exact scope reasons in output.

A payment row can match several component rules, but all selected nonempty templates must expand to the same tuple. Count its `total` exactly once. A settlement amount row contributes its `amount` exactly once. Conflicting keys on one row are a blocking mapping issue, not permission to split the payment total arbitrarily.

Aggregate each source separately to one row per identity. Sum exact signed amounts; count original rows; retain membership. Full outer join the two aggregate sets. Avoid raw many-to-many joins even when amounts look plausible.

| Presence | Status | Difference |
| --- | --- | --- |
| Both sources have at least one row | reconciled | payment sum - settlement sum |
| Payment only | unreconciled_payment | payment sum - 0 |
| Settlement only | unreconciled_settlement | 0 - settlement sum |

Presence uses row count, not nonzero amount. A matched 10.00 / 9.00 group is reconciled with amount_equal=false. A zero payment-only group is unreconciled_payment. Missing side amounts remain null in storage/export where useful; only difference arithmetic coalesces to zero.

## Independent summary

For each source, sum its own eligible summary contributions over ALL eligible rows, including unmatched rows. Do not filter by recon status. Do not join the two sources to determine a bucket, sign, amount, or allocation.

Persist source-isolated totals during ingestion from the same contribution decisions. Verify them with an independent SQL GROUP BY over the contribution ledger after ingestion. This provides the assignment's ingestion-time bonus without making a second calculation an inconsistent authority.

The baseline may violate routing conservation because of known duplicate/omitted targets; expose exact violations as diagnostics. The fixed run may not. Totals must never be 'repaired' by adjusting the summary table directly.

## Conservation controls

1. Parsed data-row count = committed source-row count for each source; settlement metadata is counted separately.
2. Every transaction source row has exactly one recon_members row. Every metadata row has zero recon_members rows and a control link.
3. For each source/scope, sum of recon group amounts = sum of source reconciliation amounts. A group join cannot change money.
4. Each nonzero contribution has exactly one decision and source row; its amount equals its mapped source amount. Baseline duplicate routes are explicitly listed.
5. Ingestion summary_totals equals recomputed contributions for all dimensions.
6. Every nonzero supported summary field maps to exactly one leaf or an explicit nonoperating control. No field disappears because it is not in the template.
7. Sum of Summary leaves equals reported subtotals without double-counting subtotals.
8. Fixed source activity total plus intentionally unsummarized source totals equals the raw selected reconciliation total. For this fixture: 212,118.95 + (-133,756.51) = 78,362.44 on payments; 212,118.95 + 0 = 212,118.95 on settlements.
9. Settlement amount sum equals metadata header total for this fixture. If a future file has reserve movements, prove its distinct bridge; do not assume this identity universally.
10. Fixed matched-key bucket deltas are zero on this fixture; a zero global delta cannot hide offsetting misclassifications.

## Audit read models

`explain --run ID --field FIELD --source payment|settlement` returns summary total, contributing rule identities, exact source amounts, source file/hash/physical lines and expanded keys. Additional selectors: `--key`, `--source-line`, `--rule-origin-line`. An explanation never recomputes with the current config; use the run's frozen mappings.

Per group export counts and distinct metadata lists. When several dates/descriptions/types/SKUs occur, show a truthful `multiple (N)` or a sorted set; do not show an arbitrary first row as if it described the whole aggregate. Offer drill-down through group ID into source-row/contribution sheets. Do not concatenate thousands of source lines into a cell or truncate without a navigable audit path.

Settlement quantities repeat across amount components. Preserve them on raw rows. Do not sum quantity across component rows and call it units sold; no quantity rollup is required to reconcile money.

## Reference-data controls

13,289 eligible shared keys, 26 eligible payment-only keys, zero eligible settlement-only keys. All shared total differences are zero even before mapping fixes. The difference between before and after is summary routing, not source-total matching. These counts are scoped controls; do not compare them to total Consolidated rows after adding noneligible payment groups.

The 26 payment-only keys consist of 25 zero Order keys and one transfer. Do not drop the zeros to make coverage appear perfect. Include noneligible groups later in the payment-only section, with scope reason and status visible.
