# Builder instructions

Implement the Go project described in this repository. This file is a navigation and correctness contract, not a claim that implementation already exists.

- Read README.md, SPEC.md, CHECKLIST.md, and the documents for the phase you are implementing. Follow docs/BUILDER_HANDOFF.md.
- The assignment PDF has priority over these design documents. Report an actual contradiction and update the affected contract with the reason; do not silently change accounting semantics.
- Both payment and settlement source records go into `source_rows`. Do not introduce separate payment/settlement ingestion tables.
- Do not modify `workingData/`. Do not read the sample workbook values as expected answers. Do not use the Python evidence probe as production logic.
- Money is exact signed cents in Go, with checked arithmetic; SQL amounts are exact numeric. Never use floating point for matching, routing, totals, or acceptance checks.
- Retain all rows and their physical source locations. Selection into a settlement accounting scope happens after ingestion and is auditable.
- Reconciliation status depends on key presence, not amount equality. Aggregate each source before joining. Never join raw many-to-many rows to compute amounts.
- Bucket defects are fixed in versioned mapping data through `MAPPING_FIXES.sql`. Never embed the reference fixture totals, source line numbers, or defect-specific amounts in Go.
- Do not implement fuzzy matching, arbitrary rule-order precedence, tax allocation guessed from rates, or cross-source copying of summary amounts.
- Keep baseline and fixed runs immutable and independently reproducible. Diagnostic baseline mapping ambiguity must remain visible. Fixed runs must reject ambiguity.
- Use parameterized SQL, explicit errors, bounded input processing, real PostgreSQL integration tests, and deterministic report ordering.
- Complete phases sequentially. Mark a checklist item only when its verification exists. Add actual commands/results and unresolved problems to PROGRESS.md. Do not invent test results.
- Do not add a web UI, queue, microservices, or a generalized finance rules language. These are outside the assignment.
- When finishing a task, report changes, checks actually run, and remaining work. No claim of guaranteed absence of bugs.
