# Mapping engine contract

## Separate concepts

A source row has one reconciliation amount: payment `total` or settlement `amount`. It can have many mapping decisions and summary contributions. A payment `total` rule often exists only to construct a key and has blank summary targets. Do not add its total to component contributions. Blank summary targets do not mean the source row is unreconcilable.

The common rule table stores source, original line, transaction selector, description selector, payment amount_field or settlement amount_type, record_ref template, positive target and negative target. Rules are immutable after their config version is frozen.

## Normalize selectors, not identities

Apply Unicode-aware surrounding whitespace trim, collapse internal whitespace to `_`, and uppercase for label comparisons on both source labels and config selectors. Preserve hyphens, colons, parentheses and existing underscores; do not remove all separators. Configs contain meaningful literals such as `OTHER-TRANSACTION`, `NON-SUBSCRIPTION_FEE_ADJUSTMENT` and `GENERAL ADJUSTMENT`. Cross-source transaction labels need not equal each other: their templates establish shared identity.

Maintain a small versioned alias registry for evidence-supported source-label variants. The active fixture requires payment transaction TRANSFER plus description starting `TO_ACCOUNT_ENDING_WITH:` to match selector `TO_ACCOUNT_ENDING`. Store this exact transaction-scoped prefix alias as data; keep the full normalized raw description available for template expansion. Prefix aliases are explicit normalization metadata, not an implicit general substring matcher. An ordinary product description must never accidentally match a fee. No SKU case folding, punctuation stripping or fuzzy order IDs.

## Rule selection

1. Find rules for the normalized transaction type. Test other selectors. `description=any` / `amount_description=any` is the reserved wildcard token, not an empty string. amount_type/amount_field remain exact selectors.
2. If at least one transaction-specific rule matches the row, suppress all blank-transaction fallback rules for that row. Do not apply fallback for every absent payment component: that would inject the catch-all total rule into Orders.
3. Within selected transaction scope and each payment amount_field (or the sole settlement amount), prefer exact description over wildcard. On settlement also require exact amount_type.
4. Retain all equally specific candidate rules for validation. Never let file order or a surrogate ID choose the winner.
5. Fixed/normal mode: reject duplicate/ambiguous selectors, even if they currently read zero, and reject multiple conflicting templates for a source row. Require a nonempty key for every transaction row unless an explicit ignored-event classification exists.
6. Baseline diagnostic mode only: preserve all highest-specificity rules and route each once. Emit a `DUPLICATE_SELECTOR` issue containing both config row identities and affected source rows. This makes the original duplicate tax routes measurable. The report is diagnostic and cannot be labeled accepted. No silent first/last-wins semantics.

Original payment lines 5/6 and 71/72 have identical selectors with different bucket targets. Strict compilation must diagnose them. The before workbook uses the explicit baseline mode; corrected config removes the redundant routes. All matching decisions, including zero and blank-target outcomes, remain queryable.

An uncovered nonzero component is a blocking issue for fixed runs. A component can be deliberately represented through a mapped `total` (advertising/coupon examples) instead of individually; classify its coverage as `COVERED_BY_TOTAL`. Never treat both total and component routes as additive unless an explicit tested decomposition proves disjointness. For active rows enforce summary contribution sum equals reconciliation amount, except documented summary-excluded transfers and baseline diagnosed duplicates/omissions.

## Template grammar

Templates consist of nonempty tokens separated by `+`. Recognized field tokens: `txn_ref`, `sku`, `date`, `settlement_id`, `shipment_id`, `merchant_order_id`, `description`. Every other token in the supplied configs is a literal, including `GENERAL ADJUSTMENT`; keep its bytes exactly. Import a catalog of original literal tokens and reject unknown lowercase field-like tokens in future edits to catch typos. A config editor must explicitly register a new literal when needed. Empty template means no key, never the shared empty-string key.

Expand field values after canonicalization. For description use the full normalized source description, not the shortened selector alias. For date use DATA_CONTRACT.md. Required field values cannot be missing silently. A known shared blank identifier can only be supported by an explicit versioned policy with collision tests; no such exception should be added merely to increase match rate.

Represent the expanded tuple as canonical JSON array of strings for collision-safe identity, optionally store its SHA-256 for indexing. Keep human-readable `record_ref` display and template text in audit output. Compare full tuple on hash matches. Partition matching by run, selected settlement, currency and scope eligibility in addition to record_ref; templates such as txn_ref+sku+date omit settlement ID.

Example: `txn_ref+sku+date` -> `["503-8856864-4518217","BIO-S000004059_AU","2026-07-17"]` after release-time conversion for payment line 11. Never use a delimiter-based encoding without escaping.

## Routing

For each selected rule obtain the exact source amount. Negative uses the negative target; zero/positive uses the positive target. NULL/empty target creates a decision with `NOT_SUMMARIZED` and no nonzero contribution. Named targets must exist in the summary-field registry; reject typos. Store source row ID, amount field/component, rule ID, amount, chosen target, config version and key.

Do not derive payment contributions from settlement components, even to split taxes. Do not drop unmatched rows from summaries. Do not take absolute values based on a label such as Expenses: retain signed money.

The template and selector compiler validates all imported rules structurally. Semantic support for nonzero intermediate fields is constrained by DESIGN.md. A dormant rule can be imported without inventing how an unseen business event should appear; it must fail clearly if activated without a supported accounting policy.
