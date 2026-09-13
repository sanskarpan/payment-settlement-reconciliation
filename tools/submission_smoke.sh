#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

required=(
  MAPPING_FIXES.sql
  PROGRESS.md
)
for path in "${required[@]}"; do
  if [[ ! -s "$path" ]]; then
    echo "required submission artifact missing or empty: $path" >&2
    exit 1
  fi
done

go test ./...
go vet ./...
go build -o /tmp/recon-submission-smoke ./cmd/recon

external=(
  workingData/amazon_payments_data.csv
  workingData/amazon_settlements_data.txt
  workingData/amazon_payment_configs_au_old.csv
  workingData/amazon_settlement_configs_au.csv
  workingData/amazon_sample_output_report.xlsx
  output/before_fix.xlsx
  output/after_fix.xlsx
  output/reconciliation.dump
)
external_complete=true
for path in "${external[@]}"; do
  if [[ ! -s "$path" ]]; then
    external_complete=false
    break
  fi
done
if [[ "$external_complete" == true ]]; then
  /tmp/recon-submission-smoke verify --report output/after_fix.xlsx
  unzip -tq output/before_fix.xlsx
  unzip -tq output/after_fix.xlsx
  pg_restore --list output/reconciliation.dump >/dev/null
  echo 'external submission artifacts verified'
else
  echo 'external submission artifacts not installed; source checks completed'
fi

echo 'submission smoke passed'
