-- Versioned mapping replay contract.
--
-- Migration 003_config_replay.sql defines two guarded functions:
--   clone_config_version(parent_id, new_name, note)
--   apply_mapping_fixes(draft_version_id)
--
-- Clone the immutable baseline, apply the three evidence-backed changes to
-- the draft, and freeze it atomically:
--
--   SELECT clone_config_version(
--       (SELECT id FROM config_versions WHERE name = 'config-<baseline-hash>'),
--       'amazon-au-fixed-f01-f03',
--       'F01/F02/F03 guarded replay');
--   SELECT apply_mapping_fixes(
--       (SELECT id FROM config_versions WHERE name = 'amazon-au-fixed-f01-f03'));
--
-- F01 deletes only payment config lines 5 and 72 when their complete original
-- selectors and shipping targets still match. F02 moves four settlement tax
-- components to sales_product_charges. F03 routes payment refund tax to
-- refunded_expenses. Each function records old/new JSON snapshots in
-- config_fix_history and refuses to run against a frozen version or a version
-- that already has a fix history. A guard mismatch aborts the whole call.
--
-- Original source bytes remain attached to both versions; corrected mapping
-- data is a new immutable derived version, so re-ingestion uses the same files
-- and leaves the diagnostic baseline auditable.

-- Executable psql form. Supply a frozen baseline id and a new child name:
--   psql -v ON_ERROR_STOP=1 -v baseline_id=1 \
--     -v fixed_name=amazon-au-fixed-f01-f03 -f MAPPING_FIXES.sql "$DATABASE_URL"
\set ON_ERROR_STOP on
\if :{?baseline_id}
\if :{?fixed_name}
BEGIN;
SELECT clone_config_version(:'baseline_id'::bigint, :'fixed_name', 'F01/F02/F03 guarded replay') AS fixed_id \gset
SELECT apply_mapping_fixes(:fixed_id);
COMMIT;
\else
\echo 'fixed_name is required'
\quit 2
\endif
\else
\echo 'baseline_id is required'
\quit 2
\endif
