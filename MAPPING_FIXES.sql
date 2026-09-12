-- Amazon AU mapping corrections, applied to a new child of the immutable
-- baseline. Run with psql so ON_ERROR_STOP and variables are enforced:
--
--   psql -v ON_ERROR_STOP=1 -v baseline_id=1 \
--     -v fixed_name=amazon-au-fixed-f01-f03 \
--     -f MAPPING_FIXES.sql "$DATABASE_URL"

\set ON_ERROR_STOP on
\if :{?baseline_id}
\if :{?fixed_name}

BEGIN;
SELECT clone_config_version(
    :'baseline_id'::bigint,
    :'fixed_name',
    'F01/F02/F03 guarded replay'
) AS fixed_id \gset
SELECT set_config('reconciliation.fixed_config_id', :'fixed_id', true);

-- F01: The payment config contains two duplicate tax allocations. Lines 5
-- and 72 send low_value_goods and sales_tax_collected to sales_shipping even
-- though equal-specificity rules already send those same fields to
-- sales_product_charges. The duplicated routes add 13,478.97 to Shipping.
-- Delete exactly those two old-state-matched rules; source rows remain intact.
DO $fix$
DECLARE
    v_id bigint := current_setting('reconciliation.fixed_config_id')::bigint;
    v_old jsonb;
    v_count integer;
BEGIN
    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_old
      FROM mapping_rules m
     WHERE config_version_id = v_id
       AND source = 'payment'
       AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND ((origin_line = 5 AND transaction_type = 'ORDER' AND description = 'any'
             AND amount_field = 'low_value_goods'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 72 AND transaction_type = 'ORDER' AND description = 'any'
             AND amount_field = 'sales_tax_collected'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));

    DELETE FROM mapping_rules
     WHERE config_version_id = v_id
       AND source = 'payment'
       AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND ((origin_line = 5 AND transaction_type = 'ORDER' AND description = 'any'
             AND amount_field = 'low_value_goods'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 72 AND transaction_type = 'ORDER' AND description = 'any'
             AND amount_field = 'sales_tax_collected'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count <> 2 THEN
        RAISE EXCEPTION 'F01 expected 2 guarded deletes, got %', v_count;
    END IF;

    INSERT INTO config_fix_history(
        config_version_id, defect_id, old_snapshot, new_snapshot, sql_sha256
    ) VALUES (
        v_id, 'F01', v_old, '[]'::jsonb, encode(digest('F01-v1','sha256'),'hex')
    );
END
$fix$;

-- F02: Settlement tax is split across components while Payments exposes the
-- combined net tax fields. Per-key component controls prove that GiftWrapTax,
-- ShippingTax, TaxDiscount and withheld shipping tax belong in the common
-- sales_product_charges grain. Update both sign routes on exactly four rules;
-- the net bucket movement is -92.84 and total activity is unchanged.
DO $fix$
DECLARE
    v_id bigint := current_setting('reconciliation.fixed_config_id')::bigint;
    v_old jsonb;
    v_new jsonb;
    v_count integer;
BEGIN
    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_old
      FROM mapping_rules m
     WHERE config_version_id = v_id
       AND source = 'settlement'
       AND origin_file LIKE '%amazon_settlement_configs_au.csv'
       AND ((origin_line = 61 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE'
             AND amount_description = 'GIFTWRAPTAX'
             AND positive_target = 'sales_other' AND negative_target = 'sales_other')
         OR (origin_line = 63 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE'
             AND amount_description = 'SHIPPINGTAX'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 95 AND transaction_type = 'ORDER' AND amount_type = 'PROMOTION'
             AND amount_description = 'TAXDISCOUNT'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 118 AND transaction_type = 'ORDER' AND amount_type = 'ITEMWITHHELDTAX'
             AND amount_description = 'LOWVALUEGOODSTAX-SHIPPING'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));

    UPDATE mapping_rules
       SET positive_target = 'sales_product_charges',
           negative_target = 'sales_product_charges'
     WHERE config_version_id = v_id
       AND source = 'settlement'
       AND origin_file LIKE '%amazon_settlement_configs_au.csv'
       AND ((origin_line = 61 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE'
             AND amount_description = 'GIFTWRAPTAX'
             AND positive_target = 'sales_other' AND negative_target = 'sales_other')
         OR (origin_line = 63 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE'
             AND amount_description = 'SHIPPINGTAX'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 95 AND transaction_type = 'ORDER' AND amount_type = 'PROMOTION'
             AND amount_description = 'TAXDISCOUNT'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 118 AND transaction_type = 'ORDER' AND amount_type = 'ITEMWITHHELDTAX'
             AND amount_description = 'LOWVALUEGOODSTAX-SHIPPING'
             AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count <> 4 THEN
        RAISE EXCEPTION 'F02 expected 4 guarded updates, got %', v_count;
    END IF;

    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_new
      FROM mapping_rules m
     WHERE config_version_id = v_id
       AND source = 'settlement'
       AND origin_line IN (61, 63, 95, 118);
    INSERT INTO config_fix_history(
        config_version_id, defect_id, old_snapshot, new_snapshot, sql_sha256
    ) VALUES (
        v_id, 'F02', v_old, v_new, encode(digest('F02-v1','sha256'),'hex')
    );
END
$fix$;

-- F03: Payment refund tax is parsed but both summary targets are empty. The
-- 16 selected rows total -42.59 and independently equal Settlement refund tax
-- components. Route both signs to refunded_expenses on exactly the original
-- line-14 rule; do not infer a change for unexercised fields.
DO $fix$
DECLARE
    v_id bigint := current_setting('reconciliation.fixed_config_id')::bigint;
    v_old jsonb;
    v_new jsonb;
    v_count integer;
BEGIN
    SELECT COALESCE(jsonb_agg(to_jsonb(m)), '[]'::jsonb)
      INTO v_old
      FROM mapping_rules m
     WHERE config_version_id = v_id
       AND source = 'payment'
       AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND origin_line = 14 AND transaction_type = 'REFUND' AND description = 'any'
       AND amount_field = 'sales_tax_collected'
       AND COALESCE(positive_target,'') = '' AND COALESCE(negative_target,'') = '';

    UPDATE mapping_rules
       SET positive_target = 'refunded_expenses',
           negative_target = 'refunded_expenses'
     WHERE config_version_id = v_id
       AND source = 'payment'
       AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND origin_line = 14 AND transaction_type = 'REFUND' AND description = 'any'
       AND amount_field = 'sales_tax_collected'
       AND COALESCE(positive_target,'') = '' AND COALESCE(negative_target,'') = '';
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count <> 1 THEN
        RAISE EXCEPTION 'F03 expected 1 guarded update, got %', v_count;
    END IF;

    SELECT COALESCE(jsonb_agg(to_jsonb(m)), '[]'::jsonb)
      INTO v_new
      FROM mapping_rules m
     WHERE config_version_id = v_id AND source = 'payment' AND origin_line = 14;
    INSERT INTO config_fix_history(
        config_version_id, defect_id, old_snapshot, new_snapshot, sql_sha256
    ) VALUES (
        v_id, 'F03', v_old, v_new, encode(digest('F03-v1','sha256'),'hex')
    );
END
$fix$;

-- Freeze only after every guarded block succeeds. Migration 007's trigger
-- computes the collision-safe canonical hash from the resulting rule set.
UPDATE config_versions
   SET state = 'FROZEN', frozen_at = now()
 WHERE id = :'fixed_id'::bigint AND state = 'DRAFT';

DO $fix$
DECLARE
    v_id bigint := current_setting('reconciliation.fixed_config_id')::bigint;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM config_versions
         WHERE id = v_id AND state = 'FROZEN'
           AND content_sha256 = compute_config_sha256(v_id)
    ) THEN
        RAISE EXCEPTION 'fixed config % was not canonically frozen', v_id;
    END IF;
END
$fix$;

COMMIT;
\echo 'created frozen config version' :fixed_id

\else
\echo 'fixed_name is required'
\quit 2
\endif
\else
\echo 'baseline_id is required'
\quit 2
\endif
