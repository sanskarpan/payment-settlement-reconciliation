CREATE TABLE IF NOT EXISTS runs (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    selected_settlement_id TEXT NOT NULL,
    mode TEXT NOT NULL CHECK (mode IN ('diagnostic-baseline','strict')),
    stage TEXT NOT NULL,
    verification_status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS source_files (
    id BIGSERIAL PRIMARY KEY,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('payment','settlement')),
    path TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size >= 0),
    UNIQUE (source_kind, sha256)
);

-- Both report types intentionally share this table.
CREATE TABLE IF NOT EXISTS source_rows (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    source_file_id BIGINT NOT NULL REFERENCES source_files(id),
    source TEXT NOT NULL CHECK (source IN ('payment','settlement')),
    row_kind TEXT NOT NULL CHECK (row_kind IN ('transaction','settlement_metadata')),
    ordinal INTEGER NOT NULL,
    line_start INTEGER NOT NULL,
    line_end INTEGER NOT NULL,
    settlement_id TEXT,
    currency TEXT,
    transaction_type TEXT,
    description TEXT,
    amount_type TEXT,
    amount_description TEXT,
    sku TEXT,
    txn_ref TEXT,
    key_date DATE,
    status TEXT,
    recon_amount NUMERIC,
    scope_reason TEXT NOT NULL,
    raw_payload JSONB NOT NULL,
    canonical_payload JSONB NOT NULL,
    UNIQUE(run_id, source_file_id, ordinal)
);

CREATE TABLE IF NOT EXISTS mapping_rules (
    id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL CHECK (source IN ('payment','settlement')),
    origin_file TEXT NOT NULL,
    origin_line INTEGER NOT NULL,
    transaction_type TEXT NOT NULL,
    description TEXT,
    amount_field TEXT,
    amount_type TEXT,
    amount_description TEXT,
    record_ref TEXT NOT NULL,
    positive_target TEXT,
    negative_target TEXT,
    UNIQUE(source, origin_file, origin_line)
);

CREATE TABLE IF NOT EXISTS row_mappings (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    source_row_id BIGINT NOT NULL REFERENCES source_rows(id) ON DELETE CASCADE,
    rule_id BIGINT NOT NULL REFERENCES mapping_rules(id),
    amount_field TEXT NOT NULL,
    amount NUMERIC NOT NULL,
    target TEXT,
    decision TEXT NOT NULL,
    record_ref TEXT NOT NULL,
    UNIQUE(run_id, source_row_id, rule_id)
);

CREATE TABLE IF NOT EXISTS summary_totals (
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    field TEXT NOT NULL,
    amount NUMERIC NOT NULL,
    contribution_count BIGINT NOT NULL,
    PRIMARY KEY(run_id, source, field)
);

CREATE TABLE IF NOT EXISTS recon_groups (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    record_ref TEXT NOT NULL,
    settlement_id TEXT,
    currency TEXT,
    scope_reason TEXT NOT NULL,
    payment_rows BIGINT NOT NULL,
    payment_amount NUMERIC NOT NULL,
    settlement_rows BIGINT NOT NULL,
    settlement_amount NUMERIC NOT NULL,
    status TEXT NOT NULL,
    UNIQUE(run_id, record_ref, settlement_id, currency, scope_reason)
);

CREATE INDEX IF NOT EXISTS source_rows_run_source_idx ON source_rows(run_id, source, settlement_id, scope_reason);
CREATE INDEX IF NOT EXISTS row_mappings_run_idx ON row_mappings(run_id, record_ref);
CREATE INDEX IF NOT EXISTS recon_groups_run_status_idx ON recon_groups(run_id, status);
