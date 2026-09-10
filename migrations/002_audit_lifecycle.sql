-- Additive hardening for immutable configuration lineage and audit read models.
-- The first migration is intentionally kept compatible with the original local
-- fixture. New writes use these tables and columns; legacy rows remain readable.

CREATE TABLE IF NOT EXISTS config_versions (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    parent_id BIGINT REFERENCES config_versions(id),
    state TEXT NOT NULL CHECK (state IN ('DRAFT', 'FROZEN')),
    content_sha256 TEXT UNIQUE,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    frozen_at TIMESTAMPTZ,
    CHECK ((state = 'DRAFT' AND frozen_at IS NULL) OR (state = 'FROZEN' AND frozen_at IS NOT NULL)),
    CHECK (state = 'DRAFT' OR content_sha256 IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS config_version_files (
    config_version_id BIGINT NOT NULL REFERENCES config_versions(id) ON DELETE CASCADE,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('payment_config', 'settlement_config')),
    source_file_id BIGINT NOT NULL REFERENCES source_files(id),
    PRIMARY KEY (config_version_id, source_kind),
    UNIQUE (config_version_id, source_file_id)
);

ALTER TABLE source_files DROP CONSTRAINT IF EXISTS source_files_source_kind_check;
ALTER TABLE source_files ADD CONSTRAINT source_files_source_kind_check
    CHECK (source_kind IN ('payment', 'settlement', 'payment_config', 'settlement_config'));

ALTER TABLE mapping_rules ADD COLUMN IF NOT EXISTS config_version_id BIGINT REFERENCES config_versions(id);
ALTER TABLE mapping_rules DROP CONSTRAINT IF EXISTS mapping_rules_source_origin_file_origin_line_key;
CREATE UNIQUE INDEX IF NOT EXISTS mapping_rules_legacy_origin_uq
    ON mapping_rules(source, origin_file, origin_line) WHERE config_version_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS mapping_rules_version_origin_uq
    ON mapping_rules(config_version_id, source, origin_file, origin_line) WHERE config_version_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS mapping_rules_version_selector_idx
    ON mapping_rules(config_version_id, source, transaction_type, amount_field, amount_type, amount_description);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS config_version_id BIGINT REFERENCES config_versions(id);
ALTER TABLE runs ADD COLUMN IF NOT EXISTS normalization_version TEXT NOT NULL DEFAULT 'normalization-v1';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS layout_version TEXT NOT NULL DEFAULT 'layout-v1';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS engine_version TEXT NOT NULL DEFAULT 'reconciliation-v1';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS currency TEXT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS error_code TEXT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS error_detail TEXT;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt > 0);
CREATE UNIQUE INDEX IF NOT EXISTS runs_name_uq ON runs(name);
CREATE INDEX IF NOT EXISTS runs_config_version_idx ON runs(config_version_id);

CREATE TABLE IF NOT EXISTS normalization_versions (
    name TEXT PRIMARY KEY,
    policy JSONB NOT NULL,
    content_sha256 TEXT NOT NULL UNIQUE,
    frozen BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS layout_versions (
    name TEXT PRIMARY KEY,
    layout JSONB NOT NULL,
    content_sha256 TEXT NOT NULL UNIQUE,
    frozen BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO normalization_versions(name, policy, content_sha256)
VALUES ('normalization-v1', '{"description_aliases":[{"source":"payment","transaction_type":"TRANSFER","prefix":"TO_ACCOUNT_ENDING_WITH:","canonical":"TO_ACCOUNT_ENDING"}],"date_policy":"release_utc_day_for_reconciliation","currency_policy":"inherit_settlement_metadata"}'::jsonb, 'normalization-v1-shipped')
ON CONFLICT (name) DO NOTHING;

INSERT INTO layout_versions(name, layout, content_sha256)
VALUES ('layout-v1', '{"summary":"amazon-payments-settlement-summary","sheets":["Summary","Consolidated Data","Source Rows","Contributions","Mapping Issues","Run Info"]}'::jsonb, 'layout-v1-shipped')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS run_files (
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('payment', 'settlement', 'payment_config', 'settlement_config')),
    source_file_id BIGINT NOT NULL REFERENCES source_files(id),
    PRIMARY KEY (run_id, role)
);

CREATE TABLE IF NOT EXISTS settlement_controls (
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    settlement_id TEXT NOT NULL,
    metadata_row_id BIGINT NOT NULL REFERENCES source_rows(id),
    currency TEXT NOT NULL,
    header_total NUMERIC NOT NULL,
    PRIMARY KEY (run_id, settlement_id),
    UNIQUE (run_id, metadata_row_id)
);

CREATE TABLE IF NOT EXISTS summary_fields (
    field TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('DIRECT', 'INTERMEDIATE', 'CONTROL')),
    supported BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO summary_fields(field, kind) VALUES
 ('sales_product_charges','DIRECT'), ('sales_tax','DIRECT'), ('sales_shipping','DIRECT'),
 ('sales_amazon_fees','DIRECT'), ('sales_inventory_reimbursements','DIRECT'), ('sales_other','DIRECT'),
 ('refunded_expenses','DIRECT'), ('refunded_sales','DIRECT'), ('expenses_promotional_rebates','DIRECT'),
 ('expenses_fba_fees','DIRECT'), ('expenses_cost_of_advertising','DIRECT'), ('expenses_amazon_fees','DIRECT'),
 ('expenses_reversed_reimbursements','DIRECT'), ('expenses_other','DIRECT'), ('paid_to_amazon','DIRECT')
ON CONFLICT (field) DO NOTHING;

CREATE TABLE IF NOT EXISTS summary_contributions (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    source_row_id BIGINT NOT NULL REFERENCES source_rows(id) ON DELETE CASCADE,
    row_mapping_id BIGINT REFERENCES row_mappings(id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (source IN ('payment', 'settlement')),
    scope_reason TEXT NOT NULL,
    settlement_id TEXT,
    currency TEXT,
    summary_field TEXT NOT NULL REFERENCES summary_fields(field),
    amount NUMERIC NOT NULL,
    UNIQUE (run_id, row_mapping_id),
    CHECK (amount <> 0)
);

CREATE INDEX IF NOT EXISTS summary_contributions_run_field_idx
    ON summary_contributions(run_id, source, summary_field, source_row_id);

CREATE TABLE IF NOT EXISTS recon_members (
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    recon_group_id BIGINT NOT NULL REFERENCES recon_groups(id) ON DELETE CASCADE,
    source_row_id BIGINT NOT NULL REFERENCES source_rows(id) ON DELETE CASCADE,
    PRIMARY KEY (run_id, source_row_id),
    UNIQUE (run_id, recon_group_id, source_row_id)
);

CREATE TABLE IF NOT EXISTS run_issues (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WARN', 'ERROR')),
    code TEXT NOT NULL,
    source_row_id BIGINT REFERENCES source_rows(id),
    mapping_rule_id BIGINT REFERENCES mapping_rules(id),
    detail JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS config_fix_history (
    config_version_id BIGINT NOT NULL REFERENCES config_versions(id) ON DELETE CASCADE,
    defect_id TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    old_snapshot JSONB NOT NULL,
    new_snapshot JSONB NOT NULL,
    sql_sha256 TEXT NOT NULL,
    PRIMARY KEY (config_version_id, defect_id)
);

CREATE TABLE IF NOT EXISTS report_artifacts (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    report_kind TEXT NOT NULL,
    path TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
    report_status TEXT NOT NULL CHECK (report_status IN ('WRITTEN', 'VERIFIED', 'FAILED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, report_kind)
);
