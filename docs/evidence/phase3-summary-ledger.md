# Phase 3 summary-ledger evidence

The mapping engine compiles field and literal templates into collision-safe tuple keys, evaluates exact and wildcard selectors, preserves duplicate candidates in diagnostic mode, and rejects ambiguity in strict mode. Each mapped row retains its selected rule identity, source, scope, record reference, and signed-cent amount.

The in-memory ledger is source-isolated: payment and settlement contributions are accumulated independently before reconciliation. The PostgreSQL adapter persists the same decisions and nonzero contributions, while `verify-db` recomputes summary totals from persisted contributions rather than trusting the report workbook.

Reference controls from the supplied fixture:

- Baseline: two duplicate-selector diagnostics and 5,973 per-key bucket variance pairs.
- Fixed: zero mapping issues and zero per-key bucket variance pairs.
- Payment component conservation: every source row total equals the checked sum of its ten monetary components.
- Fixed operating activity: payment and settlement ledgers each equal AUD 212,118.95.

The fixture totals are acceptance evidence only. The engine reads the input rows and versioned mapping data; no source-line-specific amount or expected total is embedded in Go code.
