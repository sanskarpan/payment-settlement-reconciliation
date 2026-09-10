# Assumptions and evidence limits

| ID | Policy / limitation | Basis | Consequence |
| --- | --- | --- | --- |
| A01 | Primary scope is selected settlement + Released payments | Four payment settlements vs one settlement file; released nontransfer sum ties exactly | Retain all data and disclose excluded populations |
| A02 | Payment key date uses release instant in UTC | 13,251 Order/Refund shared keys with zero amount deltas, compared with 34 using original date | Version this policy; never use host timezone |
| A03 | Normalize a Transfer description prefix explicitly for payment TRANSFER rows | Actual description `To account ending with: 334`, config selector `TO_ACCOUNT_ENDING` | Same alias before/after; no arbitrary substring matching; the transaction type guard prevents product descriptions from using it |
| A04 | Diagnostic baseline routes all equal-specificity candidates and flags ambiguity | Two duplicate-selector pairs; PDF does not define tie precedence | Before values depend on disclosed diagnostic semantics; final config must be unambiguous |
| A05 | Tax belongs to a common source-independent bucket | Payment exposes combined net tax, settlement exposes components; per-key identities verified | Config routes tax components into Product Charges; cannot claim absent statement line validation |
| A06 | AUD only, cents only | Preamble/currency metadata and observed amounts | Reject unsupported locale/currency/precision rather than guess |
| A07 | Missing optional marketplace_withheld_tax is distinct from zero | Old config references it; raw header lacks it; targets blank | No alias to another amount or nonzero synthetic value |
| A08 | Settlement metadata is not an amount component | One header row has total-amount and no transaction amount | Retain in shared source table and expose as control |
| A09 | Sample workbook determines shape only | PDF explicitly says zero Summary is intentional | Do not compare produced values with sample zeros or populated consolidation |
| A10 | Unexercised intermediate/control targets are not fully specified | No contributing rows in selected fixture and no formulas explaining them | Fail clearly on future nonzero unsupported routes; current dataset remains implementable |
| A11 | No standalone Amazon Statement Summary supplied | Inventory contains exactly the five listed data/config/sample artifacts | Prove header total and cross-source equality; label detailed statement verification unavailable |
| A12 | Textually repeated source records are not automatically duplicates | Payments/settlements can legitimately repeat transaction lines | Idempotency applies to file/run identity, not raw-payload deduplication |

These policies make the supplied assignment implementable without silently inventing data. They do not authorize general claims about all Amazon marketplaces or future report schemas. Changes to scope/date/normalization policies require new versions and runs, not edits to old results.

## Known remaining work

The complete local database replay, guarded SQL child-version replay, report verification, explain path, custom-format dump restore, and full workbook rendering have been exercised. A full remote Neon row replay is intentionally not part of the recorded acceptance run because the provider's COPY transfer rate is materially slower than the local fixture. The measured Go RSS is above the architecture's 512 MiB engineering target; this remains an operational optimization and is not represented as a financial correctness pass.

No assumption is permission to plug a total or suppress an unresolved row. An unseen business event that requires new accounting policy should produce explainable evidence rather than a fabricated classification.
