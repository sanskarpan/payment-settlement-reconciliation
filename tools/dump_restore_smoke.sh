#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
: "${SOURCE_DATABASE_URL:?Set SOURCE_DATABASE_URL to the populated PostgreSQL database}"
: "${RESTORE_DATABASE_URL:?Set RESTORE_DATABASE_URL to an empty PostgreSQL database}"

dump_path=${DUMP_PATH:-output/reconciliation-smoke.dump}
mkdir -p "$(dirname -- "$dump_path")"
pg_dump --format=custom --no-owner --file "$dump_path" "$SOURCE_DATABASE_URL"
pg_restore --clean --if-exists --no-owner --dbname "$RESTORE_DATABASE_URL" "$dump_path"
for table in schema_migrations config_versions config_hash_history runs source_rows row_mappings summary_contributions recon_groups recon_members report_artifacts; do
  source_count=$(psql "$SOURCE_DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select count(*) from ${table}")
  restore_count=$(psql "$RESTORE_DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select count(*) from ${table}")
  if [[ "$source_count" != "$restore_count" ]]; then
    echo "restore count mismatch for ${table}: source=${source_count} restore=${restore_count}" >&2
    exit 1
  fi
  printf '%s=%s\n' "$table" "$restore_count"
done
null_migration_hashes=$(psql "$RESTORE_DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select count(*) from schema_migrations where sha256 is null")
if [[ "$null_migration_hashes" != "0" ]]; then
  echo "restored database has ${null_migration_hashes} migration rows without checksums" >&2
  exit 1
fi
echo "restored $dump_path"
