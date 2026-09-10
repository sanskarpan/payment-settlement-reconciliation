CREATE INDEX IF NOT EXISTS source_rows_run_kind_idx ON source_rows(run_id, row_kind);
CREATE INDEX IF NOT EXISTS row_mappings_run_source_row_idx ON row_mappings(run_id, source_row_id);
CREATE INDEX IF NOT EXISTS row_mappings_run_target_ref_idx ON row_mappings(run_id, target, record_ref);
CREATE INDEX IF NOT EXISTS summary_totals_run_field_idx ON summary_totals(run_id, source, field);
