# Data contract

## Immutable source inventory

Use docs/evidence/input-profile.json for exact byte counts and SHA-256 values. Paths are configurable; these names identify the supplied fixture.

| File | Shape | Parsed records |
| --- | --- | ---: |
| `amazon_payments_data.csv` | 9 preamble CSV records, header at physical line 10, 25 columns | 23,026 data rows |
| `amazon_settlements_data.txt` | Tab delimiter, header at line 1, 24 columns | 54,980 including 1 metadata row |
| `amazon_payment_configs_au_old.csv` | 6 columns, CSV header | 149 rules |
| `amazon_settlement_configs_au.csv` | 6 columns, CSV header | 137 rules |
| `amazon_sample_output_report.xlsx` | Summary and Consolidated Data | Layout reference only |

The current fixture has no need for manually cleaned copies. Preserve the payment preamble in the source-file metadata and original bytes. The two report tables must not be loaded into separate ingestion tables.

## Parser behavior

Use encoding/csv for CSV and TSV (`Comma='\t'`). Do not split strings by delimiter. Support quoted delimiters, escaped quotes, CRLF/LF and multiline cells. Detect the payment header by required header names inside a bounded preamble, not by blindly skipping nine lines; assert the observed line in the reference-data test. Strip a UTF-8 BOM only at the beginning of a file. Reject invalid UTF-8, duplicate canonical headers, malformed quotes, missing required headers and wrong data-row widths. Retain unknown extra columns in raw payload; emit a schema-change diagnostic.

Capture record ordinal, physical starting line, ending line, byte offsets, original file hash and ordered raw values. `Reader.FieldPos(0)` provides the field's physical start; `InputOffset` helps locate byte boundaries. Document blank physical lines skipped by the standard CSV reader; distinguish physical line from record ordinal. Do not claim JSONB preserves original quoting or byte order: retain the immutable source file and byte span for that purpose.

Row errors abort an accepted ingestion run. A diagnostic error manifest may be written, but no partial data may appear in final reconciliation. EOF is not an error. The implementation rejects files above 100 MiB before decoding and rejects a logical CSV record above 1 MiB immediately after `encoding/csv` returns it. The file cap is the hard preallocation bound; the record check is a stricter validation bound, not a claim that `encoding/csv` itself allocates incrementally. A future streaming decoder must account for quotes and multiline records.

## Payment canonical fields

| Raw header | Canonical field | Policy |
| --- | --- | --- |
| date/time | posted_at | Parse actual GMT offset; retain source text |
| settlement ID | settlement_id | Text; no numeric conversion |
| type | transaction_type | Label normalization only |
| order ID | txn_ref | Trim surrounding whitespace; preserve case/punctuation |
| sku | sku | Opaque identifier; preserve case and hyphens |
| description | description_raw / description_key | Preserve original; normalize selector separately |
| quantity | quantity | Integer, nullable if empty; never sum repeatedly via component mappings |
| marketplace | marketplace | Preserve and validate source locale |
| fulfilment | fulfillment | Preserve |
| order city/state/postal | raw payload | Preserve; omit from routine logs |
| product sales | product_sales | AUD amount |
| shipping credits | shipping_credits | AUD amount |
| gift wrap credits | gift_wrap_credits | AUD amount |
| promotional rebates | promotional_rebates | AUD amount |
| sales tax collected | sales_tax_collected | Combined net tax field; see fixes |
| low value goods | low_value_goods | Withheld tax; signed amount |
| selling fees | selling_fees | AUD amount |
| fulfilment by amazon fees | fba_fees | Explicit header alias |
| other transaction fees | other_transaction_fees | AUD amount |
| other | other | AUD amount |
| total | total | Reconciliation amount; not an additional component |
| Transaction status | transaction_status | Released / Deferred; unknown values block scope classification |
| Transaction Release Date | release_at | Present for all Released fixture rows; empty for Deferred |

The old configs mention `marketplace_withheld_tax`, absent from this payment header. Record its availability as absent, not indistinguishable from a genuine numeric zero. Existing rules for this absent field have empty targets. Permit only an explicitly registered optional-absent field with no nonblank routing on this schema; any nonempty route to a missing amount column fails validation. Do not alias it to low_value_goods and accidentally count tax twice.

## Settlement canonical fields

Header names replace hyphens with underscores for field lookup, not identifier contents. `order-id` provides txn_ref when present, otherwise `adjustment-id`. Retain both and the choice used. Do not use shipment-id or merchant-order-id as hidden fallbacks; templates name these explicitly. `amount-type` and `amount-description` are selectors. `amount` is the sole component amount.

The metadata row has settlement ID, start/end/deposit dates, total-amount and currency, but no transaction-type or transaction amount. Ingest it with `row_kind=settlement_metadata`; store the parsed header control in `source_rows.recon_amount` with `event_class=CONTROL`, but never turn it into a mapping or summary contribution. Extract a settlement control referencing that source row. Amount rows inherit currency only from metadata with their own settlement ID. Missing or contradictory metadata is an error. Do not forward-fill transaction fields.

## Exact AUD amounts

Accept a signed decimal with zero, one, or two fractional digits and optional valid three-digit grouping commas; examples: `0`, `-0.41`, `1,234.50`. Reject malformed grouping, currency symbols, exponent notation, NaN/Inf and more than two fractional digits. Empty required amount is an error, except explicitly typed metadata/optional fields. Do not indiscriminately strip arbitrary commas and accept `1,2` as 12. The evidence probe is narrower research tooling; the Go parser must implement this stricter grammar.

Parse into checked signed int64 cents without float conversion. Check multiplication/addition/subtraction overflow. All supplied values fit; large future input must fail explicitly. PostgreSQL amounts use exact numeric as specified in DATABASE.md; validate precision before insert, because a declared SQL scale can round inputs. Zero is retained, normalized without negative-zero formatting. Route zero using the positive branch consistently for decision lineage, but it contributes no money. Exact comparison tolerance is zero cents.

Per payment row, `total` must equal the sum of the 10 monetary components excluding `total`: product_sales, shipping_credits, gift_wrap_credits, promotional_rebates, sales_tax_collected, low_value_goods, selling_fees, fba_fees, other_transaction_fees, other. If a new header adds an amount, revise this explicit conservation contract and version it; do not silently ignore it.

## Dates and key day

Example input: `29 June 2026 5:39:32 pm GMT+9`. Parse the stated fixed offset, not the machine timezone or an assumed Australian timezone. Normalize `GMT+9` to a parseable `+09:00` offset and lowercase am/pm to the parser's expected form. Settlement timestamps are `17.07.2026 07:26:32 UTC`; posted-date is `17.07.2026`.

For Released payments, `date` in record_ref is release_at converted to UTC and truncated to its calendar day. If a non-deferred input schema genuinely lacks release dates, require a separately versioned policy before fallback; the provided Released rows have dates and should fail if one is removed. Deferred rows use posted_at UTC day only for audit grouping and remain outside accounting scope. Settlement keys use posted-date, validated against the UTC day of posted-date-time where both exist.

Evidence: using original payment posted dates gives only 34 shared Order/Refund keys, with 1 amount difference. Release-date UTC produces 13,251 shared keys, zero amount differences and 25 payment-only zero keys. This is an observed dataset policy, not a universal claim about every Amazon locale.

## Accounting scope and controls

| Settlement ID | Status | Rows | Sum of payment total |
| --- | --- | ---: | ---: |
| 12370691583 | Released | 14 | -83.55 |
| 12382593803 | Released | 5,642 | 655.70 |
| 12382593803 | Deferred | 7 | 101.07 |
| 12395580393 | Released | 13,472 | 78,362.44 |
| 12395580393 | Deferred | 2,398 | 43,540.07 |
| 12407469483 | Released | 1,370 | 25,461.48 |
| 12407469483 | Deferred | 123 | 2,171.94 |

Assign scope reasons with precedence: different settlement ID -> `OTHER_SETTLEMENT`; selected ID but Deferred -> `DEFERRED`; selected Released -> `IN_SCOPE`. Retain transaction status as a separate field even when OTHER_SETTLEMENT applies. A transfer is IN_SCOPE with a separate `BANK_TRANSFER` classification and intentionally empty summary routes.

Selected Released payment types: 13,353 Orders (213,271.73), 73 Refunds (-1,815.09), 43 Adjustments (679.69), 1 Transfer (-133,756.51), 1 FBA transaction fee (-0.41), 1 Service fee (-16.97). Nontransfer sum is 212,118.95. Full payment-file sum is 150,209.15 and is not a settlement control.

Settlement metadata: start `2026-07-17T07:06:34Z`, end `2026-07-31T07:06:35Z`, deposit `2026-08-02T07:06:35Z`, AUD 212,118.95. The sum of 54,979 amount records is independently 212,118.95. The lone selected payment transfer is dated July 18 and does not equal this payout; do not force it to the header amount.
