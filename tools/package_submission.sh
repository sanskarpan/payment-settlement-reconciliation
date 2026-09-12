#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

required=(
  MAPPING_FIXES.sql
  PROGRESS.md
  SUBMISSION_SHA256SUMS
  output/before_fix.xlsx
  output/after_fix.xlsx
  output/reconciliation.dump
)
for path in "${required[@]}"; do
  if [[ ! -s "$path" ]]; then
    echo "required submission artifact missing or empty: $path" >&2
    exit 1
  fi
done

python3 - "$repo_dir" <<'PY'
from __future__ import annotations

import hashlib
import shutil
import sys
import tempfile
import zipfile
from pathlib import Path

repo = Path(sys.argv[1]).resolve()
output = repo / "output"
target = output / "submission"
archive = output / "portone-sde2-submission-artifacts.zip"
archive_checksum = output / "portone-sde2-submission-artifacts.zip.sha256"

files = {
    "MAPPING_FIXES.sql": repo / "MAPPING_FIXES.sql",
    "PROGRESS.md": repo / "PROGRESS.md",
    "SUBMISSION_SHA256SUMS": repo / "SUBMISSION_SHA256SUMS",
    "before_fix.xlsx": output / "before_fix.xlsx",
    "after_fix.xlsx": output / "after_fix.xlsx",
    "reconciliation.dump": output / "reconciliation.dump",
}

readme = """# PortOne SDE II submission artifacts

Public source repository:
https://github.com/sanskarpan/payment-settlement-reconciliation

Contents:
- `MAPPING_FIXES.sql`: executable, guarded mapping corrections.
- `before_fix.xlsx`: reconciliation report produced with the original mappings.
- `after_fix.xlsx`: strict reconciliation report produced after the fixes.
- `reconciliation.dump`: PostgreSQL custom-format dump with the reproducible baseline and fixed runs.
- `PROGRESS.md`: implementation and verification log.
- `SUBMISSION_SHA256SUMS`: hashes for the canonical repository inputs and outputs.
- `SHA256SUMS`: hashes for the files in this delivery directory.

Verification from this directory:
```sh
shasum -a 256 -c SHA256SUMS
unzip -tq before_fix.xlsx
unzip -tq after_fix.xlsx
pg_restore --list reconciliation.dump >/dev/null
```

The source repository README contains build, schema, assumptions, reproduction,
and end-to-end test instructions. Secrets and supplied financial input data are
not included in this delivery archive.
"""

with tempfile.TemporaryDirectory(prefix="recon-submission-") as temp_name:
    stage = Path(temp_name) / "submission"
    stage.mkdir()
    for name, source in files.items():
        shutil.copy2(source, stage / name)
    (stage / "README.md").write_text(readme, encoding="utf-8")

    hash_names = [
        "MAPPING_FIXES.sql",
        "PROGRESS.md",
        "SUBMISSION_SHA256SUMS",
        "before_fix.xlsx",
        "after_fix.xlsx",
        "reconciliation.dump",
        "README.md",
    ]
    lines = []
    for name in hash_names:
        digest = hashlib.sha256((stage / name).read_bytes()).hexdigest()
        lines.append(f"{digest}  {name}\n")
    (stage / "SHA256SUMS").write_text("".join(lines), encoding="ascii")

    if target.exists():
        shutil.rmtree(target)
    shutil.copytree(stage, target)

    temp_archive = output / ".portone-sde2-submission-artifacts.zip.tmp"
    if temp_archive.exists():
        temp_archive.unlink()
    with zipfile.ZipFile(temp_archive, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=6) as zf:
        for path in sorted(stage.iterdir(), key=lambda item: item.name):
            info = zipfile.ZipInfo(f"submission/{path.name}")
            info.date_time = (2026, 9, 12, 0, 0, 0)
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            zf.writestr(info, path.read_bytes(), compresslevel=6)
    temp_archive.replace(archive)
    archive_digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    archive_checksum.write_text(
        f"{archive_digest}  {archive.name}\n", encoding="ascii"
    )

print(target)
print(archive)
print(archive_checksum)
PY

if command -v sha256sum >/dev/null 2>&1; then
  (cd output/submission && sha256sum --check SHA256SUMS)
else
  (cd output/submission && shasum -a 256 -c SHA256SUMS)
fi
unzip -tq output/submission/before_fix.xlsx
unzip -tq output/submission/after_fix.xlsx
pg_restore --list output/submission/reconciliation.dump >/dev/null
unzip -tq output/portone-sde2-submission-artifacts.zip
(cd output && shasum -a 256 -c portone-sde2-submission-artifacts.zip.sha256)

echo 'submission package created and verified'
