#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"
if [[ -z "${DATABASE_URL:-}" && -f .env ]]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi
: "${DATABASE_URL:?Set DATABASE_URL or source .env before running end_to_end.sh}"

go build -o bin/recon ./cmd/recon
./bin/recon migrate --postgres-url "$DATABASE_URL"

existing_runs=$(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc 'select count(*) from runs')
if [[ "$existing_runs" != "0" ]]; then
  echo "end-to-end requires an empty database; found ${existing_runs} runs" >&2
  exit 1
fi

config_info=$(./bin/recon import-config \
  --payment-config workingData/amazon_payment_configs_au_old.csv \
  --settlement-config workingData/amazon_settlement_configs_au.csv \
  --name amazon-au-baseline --note 'original assignment mapping files' \
  --postgres-url "$DATABASE_URL")
printf '%s\n' "$config_info"

baseline_id=$(printf '%s\n' "$config_info" | sed -n 's/^config_version=\([0-9][0-9]*\) .*/\1/p')
baseline_name=$(printf '%s\n' "$config_info" | sed -n 's/^config_version=[0-9][0-9]* name=\([^ ]*\) .*/\1/p')
if [[ -z "$baseline_id" || -z "$baseline_name" ]]; then
  echo 'baseline config version was not created' >&2
  exit 1
fi

fixed_id=$(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select id from config_versions where name='amazon-au-fixed-f01-f03'")
if [[ -z "$fixed_id" ]]; then
  fixed_id=$(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select clone_config_version(${baseline_id},'amazon-au-fixed-f01-f03','F01/F02/F03 guarded replay')")
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select apply_mapping_fixes(${fixed_id})" >/dev/null
fi

hash_mismatches=$(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select count(*) from config_versions where state='FROZEN' and content_sha256<>compute_config_sha256(id)")
if [[ "$hash_mismatches" != "0" ]]; then
  echo "found ${hash_mismatches} frozen configuration hash mismatches" >&2
  exit 1
fi

./bin/recon run \
  --payments workingData/amazon_payments_data.csv \
  --settlements workingData/amazon_settlements_data.txt \
  --config-version "$baseline_name" \
  --settlement-id 12395580393 --output output/e2e_before_fix.xlsx \
  --mode diagnostic-baseline --run-name e2e-before-fix \
  --postgres-url "$DATABASE_URL"

./bin/recon run \
  --payments workingData/amazon_payments_data.csv \
  --settlements workingData/amazon_settlements_data.txt \
  --config-version amazon-au-fixed-f01-f03 \
  --settlement-id 12395580393 --output output/e2e_after_fix.xlsx \
  --mode strict --run-name e2e-after-fix \
  --postgres-url "$DATABASE_URL"

./bin/recon verify --report output/e2e_after_fix.xlsx
before_id=$(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select id from runs where name='e2e-before-fix'")
after_id=$(psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc "select id from runs where name='e2e-after-fix'")
./bin/recon verify-db --run-id "$before_id" --expected-rows 78006 --postgres-url "$DATABASE_URL"
./bin/recon verify-db --run-id "$after_id" --expected-rows 78006 --postgres-url "$DATABASE_URL"
echo "end-to-end completed: baseline_run=${before_id} fixed_run=${after_id}"
