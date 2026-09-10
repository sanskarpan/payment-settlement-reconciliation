#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
: "${SOURCE_DATABASE_URL:?Set SOURCE_DATABASE_URL to the populated PostgreSQL database}"
: "${RESTORE_DATABASE_URL:?Set RESTORE_DATABASE_URL to an empty PostgreSQL database}"

dump_path=${DUMP_PATH:-output/reconciliation-smoke.dump}
mkdir -p "$(dirname -- "$dump_path")"
pg_dump --format=custom --no-owner --file "$dump_path" "$SOURCE_DATABASE_URL"
pg_restore --clean --if-exists --no-owner --dbname "$RESTORE_DATABASE_URL" "$dump_path"
psql "$RESTORE_DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select count(*) from runs; select count(*) from source_rows; select count(*) from recon_groups;"
echo "restored $dump_path"
