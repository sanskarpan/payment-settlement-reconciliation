# Phase 6 report acceptance evidence

The report writer produces six deterministic sheets: Summary, Consolidated Data, Source Rows, Contributions, Mapping Issues, and Run Info. Workbook publication is atomic through a temporary file and rename; the structure verifier reopens the temporary archive before it is published.

The fixed workbook was checked with ZIP integrity, Excelize structural and zero-delta verification, and LibreOffice rendering. Its Summary fits on one landscape page with the Payments, Settlements, and Payments - Settlements columns visible. The baseline workbook renders with the same layout and keeps its nonzero diagnostic deltas visible. Run Info exposes source/config filenames and hashes, selected settlement, mode, row counts, and mapping issue count.

The rendered fixed workbook contains 26,619 pages and the baseline contains 29,352 pages because the audit sheets retain every source and lineage row. Visual checks covered each Summary, a middle audit page, and each final Run Info page; no horizontal Summary split or truncated provenance column remains.
