# Mapping investigation and SQL fix contract

Status: verified with the independent input probe and the fresh PostgreSQL replay. Config line numbers below are physical CSV lines, header = 1. They identify evidence, not magic runtime selectors.

## Baseline semantics

Same original files, selected settlement `12395580393`, Released payment scope, fixed UTC release-date normalization, explicit transfer-description alias. Original configs are not edited. Diagnostic baseline retains both duplicate tax routes and labels their ambiguity. This choice is explicit because the assignment does not define precedence for identical selectors. A first/last-wins implementation produces different before totals and conceals part of the supplied defect.

| Summary field | Before payment | Before settlement | Payment - settlement |
| --- | ---: | ---: | ---: |
| sales_product_charges | 348,815.93 | 348,908.77 | -92.84 |
| sales_shipping | 21,773.13 | 8,200.96 | 13,572.17 |
| sales_other | 11.61 | 11.97 | -0.36 |
| refunded_expenses | 258.87 | 216.28 | 42.59 |

All other active leaves already agree. Before payment leaves sum to 225,640.51; settlement leaves sum to 212,118.95. The difference 13,521.56 is routing duplication plus omitted refund tax, not a difference in the source activity total.

## F01: duplicated payment tax allocations

Delete payment config line 5: `(ORDER, any, low_value_goods)` with both targets `sales_shipping`. Retain line 6 routing that field to `sales_product_charges`.

Delete payment config line 72: `(ORDER, any, sales_tax_collected)` with both targets `sales_shipping`. Retain line 71 routing that field to `sales_product_charges`.

Evidence: line 5 contributes -196.62 from 50 nonzero rows; line 72 contributes 13,675.59 from 5,235 nonzero rows. Both amounts are already included by their identical-selector product-charge routes. Removing the shipping duplicates changes shipping by **-13,478.97**. Preserve all source rows and payment totals.

After F01 alone payment Shipping is 8,294.16, still 93.20 above settlement. The remaining difference is evidence of a second bucketing defect, not a reason to keep duplication.

## F02: settlement tax components do not follow the payment field's grain

Payment `sales_tax_collected` is already a combined net tax value. Source-level controls prove:

```text
Settlement Order ItemPrice Tax                 13,765.71
+ ItemPrice ShippingTax                           581.68
+ ItemPrice GiftWrapTax                             0.36
+ Promotion TaxDiscount                          -672.16
= Payment Order sales_tax_collected            13,675.59

Settlement Order ItemWithheldTax Principal       -193.90
+ ItemWithheldTax Shipping                         -2.72
= Payment Order low_value_goods                  -196.62
```

These equalities also hold per matched shared key after the proposed routing changes; no allocation from settlement into payments is needed.

Update BOTH sign targets to `sales_product_charges` on settlement config lines:

| Line | Selector (transaction / amount_type / description) | Old target | Nonzero count | Signed amount |
| --- | --- | --- | ---: | ---: |
| 61 | ORDER / ITEMPRICE / GIFTWRAPTAX | sales_other | 1 | 0.36 |
| 63 | ORDER / ITEMPRICE / SHIPPINGTAX | sales_shipping | 2,013 | 581.68 |
| 95 | ORDER / PROMOTION / TAXDISCOUNT | sales_shipping | 2,506 | -672.16 |
| 118 | ORDER / ITEMWITHHELDTAX / LOWVALUEGOODSTAX-SHIPPING | sales_shipping | 3 | -2.72 |

Net settlement Product Charges movement is -92.84. Shipping increases by 93.20; Other decreases by 0.36. Total activity is unchanged. This chooses the common information granularity available independently in both sources. It does not assert Amazon's absent statement allocates taxes identically: disclose that limitation.

## F03: refund tax is intentionally parsed but incorrectly excluded

Update BOTH targets on payment config line 14 `(REFUND, any, sales_tax_collected)` from empty to `refunded_expenses`.

The field is nonzero on 16 selected Released refund rows, total **-42.59**. Settlement refund tax components sum to the same amount: ItemPrice Tax -41.09, ShippingTax -3.19, Promotion TaxDiscount +1.69. Payment refund-expense summary changes from 258.87 to **216.28**.

Payment physical lines: 14661, 17010, 17024, 17464, 18036, 19050, 19051, 19186, 19619, 20142, 20483, 21034, 21087, 21495, 22745, 22849. The evidence JSON includes source lines for every active rule; use `explain` to export the actual component values and matching settlement rows.

Do not modify REFUND low_value_goods merely by analogy: it has no nonzero selected rows in this fixture. A future nonzero activation needs its own investigation and conservation test.

## Required MAPPING_FIXES.sql implementation

The root-level `MAPPING_FIXES.sql` is the executable psql wrapper, and migration `003_config_replay.sql` contains its guarded functions. The wrapper must be run with a caller-selected frozen baseline ID and new child name; it never mutates the original version.

- One commented block per defect F01/F02/F03: original rule, new rule, evidence, financial interpretation and expected delta.
- Target only a caller-selected DRAFT child config version cloned from the frozen original. The SQL executor uses one transaction.
- Locate rules by source kind, original config hash, original source line AND original selector/target values. Include expected old-state guards; do not rely on global numeric database IDs.
- Assert exactly 2 deleted rows, 4 settlement updates and 1 payment update. A PL/pgSQL block can check ROW_COUNT and raise if unexpected.
- Require correct base-version content hash and no prior application. Record fix identity and final version hash, then freeze. Re-running against a frozen/already-fixed version must give a clear already-applied result or fail without modifying anything; it must never apply against the original.
- Keep normalization aliases unchanged between before and after. No amounts, row-total adjustments, source-row updates, report patches or broad unscoped UPDATEs.
- Re-ingest both untouched input files using the new version. Store a separate after run. Compare the whole summary and per-key bucket deltas, not just the four changed leaves.

## Verified after controls

| Active field | Both sources |
| --- | ---: |
| sales_product_charges | 348,815.93 |
| sales_shipping | 8,294.16 |
| sales_other | 11.61 |
| sales_inventory_reimbursements | 679.69 |
| refunded_expenses | 216.28 |
| refunded_sales | -2,031.37 |
| expenses_amazon_fees | -132,593.41 |
| expenses_fba_fees | -0.41 |
| expenses_promotional_rebates | -11,273.53 |

All other template leaf amounts are zero for the supplied scope. Total operating activity is 212,118.95 on each side. Probe: 13,289 matched keys; no per-key total differences; no per-key bucket differences after changes. There are 26 selected payment-only keys (25 zero total, one -133,756.51 transfer); no selected settlement-only keys. These are regression controls stored in tests only, never report-code constants.
