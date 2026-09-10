CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION clone_config_version(p_parent_id BIGINT, p_name TEXT, p_note TEXT DEFAULT '')
RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_parent_state TEXT;
    v_id BIGINT;
BEGIN
    SELECT state INTO v_parent_state FROM config_versions WHERE id = p_parent_id FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'config version % does not exist', p_parent_id;
    END IF;
    IF v_parent_state <> 'FROZEN' THEN
        RAISE EXCEPTION 'config version % must be FROZEN before cloning', p_parent_id;
    END IF;
    INSERT INTO config_versions(name, parent_id, state, note)
    VALUES (p_name, p_parent_id, 'DRAFT', p_note)
    RETURNING id INTO v_id;
    INSERT INTO config_version_files(config_version_id, source_kind, source_file_id)
    SELECT v_id, source_kind, source_file_id
      FROM config_version_files
     WHERE config_version_id = p_parent_id;
    INSERT INTO mapping_rules(config_version_id, source, origin_file, origin_line,
                              transaction_type, description, amount_field, amount_type,
                              amount_description, record_ref, positive_target, negative_target)
    SELECT v_id, source, origin_file, origin_line, transaction_type, description,
           amount_field, amount_type, amount_description, record_ref, positive_target,
           negative_target
      FROM mapping_rules
     WHERE config_version_id = p_parent_id;
    RETURN v_id;
END;
$$;

CREATE OR REPLACE FUNCTION apply_mapping_fixes(p_config_version_id BIGINT)
RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    v_state TEXT;
    v_old JSONB;
    v_new JSONB;
    v_count INTEGER;
    v_hash TEXT;
BEGIN
    SELECT state INTO v_state FROM config_versions WHERE id = p_config_version_id FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'config version % does not exist', p_config_version_id;
    END IF;
    IF v_state <> 'DRAFT' THEN
        RAISE EXCEPTION 'config version % is %, expected DRAFT; frozen versions are immutable', p_config_version_id, v_state;
    END IF;

    IF EXISTS (SELECT 1 FROM config_fix_history WHERE config_version_id = p_config_version_id) THEN
        RAISE EXCEPTION 'mapping fixes already recorded for config version %', p_config_version_id;
    END IF;

    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_old
      FROM mapping_rules m
     WHERE config_version_id = p_config_version_id
       AND source = 'payment'
       AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND origin_line IN (5, 72)
       AND ((origin_line = 5 AND transaction_type = 'ORDER' AND description = 'any' AND amount_field = 'low_value_goods' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 72 AND transaction_type = 'ORDER' AND description = 'any' AND amount_field = 'sales_tax_collected' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));
    DELETE FROM mapping_rules
     WHERE config_version_id = p_config_version_id
       AND source = 'payment'
       AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND origin_line IN (5, 72)
       AND ((origin_line = 5 AND transaction_type = 'ORDER' AND description = 'any' AND amount_field = 'low_value_goods' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 72 AND transaction_type = 'ORDER' AND description = 'any' AND amount_field = 'sales_tax_collected' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count <> 2 THEN
        RAISE EXCEPTION 'F01 expected 2 guarded deletes, got %', v_count;
    END IF;
    INSERT INTO config_fix_history(config_version_id, defect_id, old_snapshot, new_snapshot, sql_sha256)
    VALUES (p_config_version_id, 'F01', v_old, '[]'::jsonb, encode(digest('F01-v1','sha256'),'hex'));

    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_old
      FROM mapping_rules m
     WHERE config_version_id = p_config_version_id
       AND source = 'settlement'
       AND origin_file LIKE '%amazon_settlement_configs_au.csv'
       AND origin_line IN (61, 63, 95, 118)
       AND ((origin_line = 61 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE' AND amount_description = 'GIFTWRAPTAX' AND positive_target = 'sales_other' AND negative_target = 'sales_other')
         OR (origin_line = 63 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE' AND amount_description = 'SHIPPINGTAX' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 95 AND transaction_type = 'ORDER' AND amount_type = 'PROMOTION' AND amount_description = 'TAXDISCOUNT' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 118 AND transaction_type = 'ORDER' AND amount_type = 'ITEMWITHHELDTAX' AND amount_description = 'LOWVALUEGOODSTAX-SHIPPING' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));
    UPDATE mapping_rules
       SET positive_target = 'sales_product_charges', negative_target = 'sales_product_charges'
     WHERE config_version_id = p_config_version_id
       AND source = 'settlement'
       AND origin_file LIKE '%amazon_settlement_configs_au.csv'
       AND origin_line IN (61, 63, 95, 118)
       AND ((origin_line = 61 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE' AND amount_description = 'GIFTWRAPTAX' AND positive_target = 'sales_other' AND negative_target = 'sales_other')
         OR (origin_line = 63 AND transaction_type = 'ORDER' AND amount_type = 'ITEMPRICE' AND amount_description = 'SHIPPINGTAX' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 95 AND transaction_type = 'ORDER' AND amount_type = 'PROMOTION' AND amount_description = 'TAXDISCOUNT' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping')
         OR (origin_line = 118 AND transaction_type = 'ORDER' AND amount_type = 'ITEMWITHHELDTAX' AND amount_description = 'LOWVALUEGOODSTAX-SHIPPING' AND positive_target = 'sales_shipping' AND negative_target = 'sales_shipping'));
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count <> 4 THEN
        RAISE EXCEPTION 'F02 expected 4 guarded updates, got %', v_count;
    END IF;
    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_new FROM mapping_rules m
     WHERE config_version_id = p_config_version_id AND source = 'settlement' AND origin_line IN (61,63,95,118);
    INSERT INTO config_fix_history(config_version_id, defect_id, old_snapshot, new_snapshot, sql_sha256)
    VALUES (p_config_version_id, 'F02', v_old, v_new, encode(digest('F02-v1','sha256'),'hex'));

    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_old FROM mapping_rules m
     WHERE config_version_id = p_config_version_id
       AND source = 'payment' AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND origin_line = 14 AND transaction_type = 'REFUND' AND description = 'any'
       AND amount_field = 'sales_tax_collected' AND COALESCE(positive_target,'') = '' AND COALESCE(negative_target,'') = '';
    UPDATE mapping_rules
       SET positive_target = 'refunded_expenses', negative_target = 'refunded_expenses'
     WHERE config_version_id = p_config_version_id
       AND source = 'payment' AND origin_file LIKE '%amazon_payment_configs_au_old.csv'
       AND origin_line = 14 AND transaction_type = 'REFUND' AND description = 'any'
       AND amount_field = 'sales_tax_collected' AND COALESCE(positive_target,'') = '' AND COALESCE(negative_target,'') = '';
    GET DIAGNOSTICS v_count = ROW_COUNT;
    IF v_count <> 1 THEN
        RAISE EXCEPTION 'F03 expected 1 guarded update, got %', v_count;
    END IF;
    SELECT COALESCE(jsonb_agg(to_jsonb(m) ORDER BY origin_line), '[]'::jsonb)
      INTO v_new FROM mapping_rules m
     WHERE config_version_id = p_config_version_id AND source = 'payment' AND origin_line = 14;
    INSERT INTO config_fix_history(config_version_id, defect_id, old_snapshot, new_snapshot, sql_sha256)
    VALUES (p_config_version_id, 'F03', v_old, v_new, encode(digest('F03-v1','sha256'),'hex'));

    SELECT encode(digest(COALESCE(string_agg(
        concat_ws('|', source, origin_file, origin_line, transaction_type,
                  COALESCE(description,''), COALESCE(amount_field,''),
                  COALESCE(amount_type,''), COALESCE(amount_description,''),
                  record_ref, COALESCE(positive_target,''), COALESCE(negative_target,''))
        , E'\n' ORDER BY source, origin_file, origin_line), ''), 'sha256'), 'hex')
      INTO v_hash
      FROM mapping_rules WHERE config_version_id = p_config_version_id;
    UPDATE config_versions
       SET state = 'FROZEN', frozen_at = now(), content_sha256 = v_hash
     WHERE id = p_config_version_id;
    RETURN p_config_version_id;
END;
$$;
