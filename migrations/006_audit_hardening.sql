-- Enforce provenance, exact-money, and immutable-lineage invariants in the database.

ALTER TABLE source_rows ADD COLUMN IF NOT EXISTS byte_start BIGINT NOT NULL DEFAULT 0;
ALTER TABLE source_rows ADD COLUMN IF NOT EXISTS byte_end BIGINT NOT NULL DEFAULT 0;
ALTER TABLE source_rows ADD COLUMN IF NOT EXISTS posted_at TIMESTAMPTZ;
ALTER TABLE source_rows ADD COLUMN IF NOT EXISTS release_at TIMESTAMPTZ;
ALTER TABLE source_rows ADD COLUMN IF NOT EXISTS event_class TEXT;

ALTER TABLE source_rows DROP CONSTRAINT IF EXISTS source_rows_location_check;
ALTER TABLE source_rows ADD CONSTRAINT source_rows_location_check
    CHECK (ordinal > 0 AND line_start > 0 AND line_end >= line_start AND byte_start >= 0 AND byte_end >= byte_start);

INSERT INTO summary_fields(field,kind) VALUES
 ('bank_account_transfer_round_off','INTERMEDIATE'), ('amazon_carried_forward','INTERMEDIATE'),
 ('beginning_balance','CONTROL'), ('current_reserve_amount','CONTROL'),
 ('total_adjustment_other_buyer_recharge_amt','CONTROL'), ('total_refund_expense_or_sales_amt','CONTROL')
ON CONFLICT (field) DO NOTHING;

ALTER TABLE source_rows ADD CONSTRAINT source_rows_id_run_uq UNIQUE (id,run_id);
ALTER TABLE row_mappings ADD CONSTRAINT row_mappings_id_run_uq UNIQUE (id,run_id);
ALTER TABLE recon_groups ADD CONSTRAINT recon_groups_id_run_uq UNIQUE (id,run_id);

ALTER TABLE row_mappings DROP CONSTRAINT IF EXISTS row_mappings_source_row_id_fkey;
ALTER TABLE row_mappings ADD CONSTRAINT row_mappings_source_row_run_fk
    FOREIGN KEY (source_row_id,run_id) REFERENCES source_rows(id,run_id) ON DELETE CASCADE;
ALTER TABLE summary_contributions DROP CONSTRAINT IF EXISTS summary_contributions_source_row_id_fkey;
ALTER TABLE summary_contributions ADD CONSTRAINT summary_contributions_source_row_run_fk
    FOREIGN KEY (source_row_id,run_id) REFERENCES source_rows(id,run_id) ON DELETE CASCADE;
ALTER TABLE summary_contributions DROP CONSTRAINT IF EXISTS summary_contributions_row_mapping_id_fkey;
ALTER TABLE summary_contributions ADD CONSTRAINT summary_contributions_mapping_run_fk
    FOREIGN KEY (row_mapping_id,run_id) REFERENCES row_mappings(id,run_id) ON DELETE CASCADE;
ALTER TABLE recon_members DROP CONSTRAINT IF EXISTS recon_members_recon_group_id_fkey;
ALTER TABLE recon_members ADD CONSTRAINT recon_members_group_run_fk
    FOREIGN KEY (recon_group_id,run_id) REFERENCES recon_groups(id,run_id) ON DELETE CASCADE;
ALTER TABLE recon_members DROP CONSTRAINT IF EXISTS recon_members_source_row_id_fkey;
ALTER TABLE recon_members ADD CONSTRAINT recon_members_source_row_run_fk
    FOREIGN KEY (source_row_id,run_id) REFERENCES source_rows(id,run_id) ON DELETE CASCADE;
ALTER TABLE settlement_controls DROP CONSTRAINT IF EXISTS settlement_controls_metadata_row_id_fkey;
ALTER TABLE settlement_controls ADD CONSTRAINT settlement_controls_metadata_row_run_fk
    FOREIGN KEY (metadata_row_id,run_id) REFERENCES source_rows(id,run_id);
ALTER TABLE run_issues DROP CONSTRAINT IF EXISTS run_issues_source_row_id_fkey;
ALTER TABLE run_issues ADD CONSTRAINT run_issues_source_row_run_fk
    FOREIGN KEY (source_row_id,run_id) REFERENCES source_rows(id,run_id);

ALTER TABLE runs ADD CONSTRAINT runs_normalization_version_fk FOREIGN KEY (normalization_version) REFERENCES normalization_versions(name);
ALTER TABLE runs ADD CONSTRAINT runs_layout_version_fk FOREIGN KEY (layout_version) REFERENCES layout_versions(name);

CREATE OR REPLACE FUNCTION valid_exact_cents(v NUMERIC) RETURNS BOOLEAN
LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN
    IF v IS NULL THEN RETURN TRUE; END IF;
    IF v::text IN ('NaN','Infinity','-Infinity') THEN RETURN FALSE; END IF;
    RETURN scale(v) <= 2 AND v >= -92233720368547758.08 AND v <= 92233720368547758.07;
END;
$$;

ALTER TABLE source_rows ADD CONSTRAINT source_rows_recon_amount_exact CHECK (valid_exact_cents(recon_amount));
ALTER TABLE row_mappings ADD CONSTRAINT row_mappings_amount_exact CHECK (valid_exact_cents(amount));
ALTER TABLE summary_totals ADD CONSTRAINT summary_totals_amount_exact CHECK (valid_exact_cents(amount));
ALTER TABLE summary_contributions ADD CONSTRAINT summary_contributions_amount_exact CHECK (valid_exact_cents(amount));
ALTER TABLE settlement_controls ADD CONSTRAINT settlement_controls_header_exact CHECK (valid_exact_cents(header_total));
ALTER TABLE recon_groups ADD CONSTRAINT recon_groups_payment_exact CHECK (valid_exact_cents(payment_amount));
ALTER TABLE recon_groups ADD CONSTRAINT recon_groups_settlement_exact CHECK (valid_exact_cents(settlement_amount));

CREATE OR REPLACE FUNCTION validate_mapping_lineage() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE v_run_config BIGINT; v_rule_config BIGINT; v_rule_source TEXT; v_row_source TEXT;
BEGIN
    SELECT config_version_id INTO v_run_config FROM runs WHERE id=NEW.run_id;
    SELECT config_version_id,source INTO v_rule_config,v_rule_source FROM mapping_rules WHERE id=NEW.rule_id;
    SELECT source INTO v_row_source FROM source_rows WHERE id=NEW.source_row_id AND run_id=NEW.run_id;
    IF v_row_source IS NULL OR v_rule_config IS DISTINCT FROM v_run_config OR v_rule_source IS DISTINCT FROM v_row_source THEN
        RAISE EXCEPTION 'row mapping violates run/config/source lineage';
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS row_mappings_lineage_guard ON row_mappings;
CREATE TRIGGER row_mappings_lineage_guard BEFORE INSERT OR UPDATE ON row_mappings
FOR EACH ROW EXECUTE FUNCTION validate_mapping_lineage();

CREATE OR REPLACE FUNCTION reject_frozen_config_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE v_old_state TEXT; v_new_state TEXT;
BEGIN
    IF TG_OP <> 'INSERT' AND OLD.config_version_id IS NOT NULL THEN
        SELECT state INTO v_old_state FROM config_versions WHERE id=OLD.config_version_id;
        IF v_old_state='FROZEN' THEN RAISE EXCEPTION 'config version % is frozen and cannot be changed',OLD.config_version_id; END IF;
    END IF;
    IF TG_OP <> 'DELETE' AND NEW.config_version_id IS NOT NULL THEN
        SELECT state INTO v_new_state FROM config_versions WHERE id=NEW.config_version_id;
        IF v_new_state='FROZEN' THEN RAISE EXCEPTION 'config version % is frozen and cannot be changed',NEW.config_version_id; END IF;
    END IF;
    IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$;

CREATE OR REPLACE FUNCTION reject_frozen_version_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state='FROZEN' THEN RAISE EXCEPTION 'config version % is frozen and cannot be changed',OLD.id; END IF;
    IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$;
DROP TRIGGER IF EXISTS config_versions_frozen_guard ON config_versions;
CREATE TRIGGER config_versions_frozen_guard BEFORE UPDATE OR DELETE ON config_versions
FOR EACH ROW EXECUTE FUNCTION reject_frozen_version_mutation();
