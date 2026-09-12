# Research notes and primary sources

Reviewed 2026-09-10. External documentation supports technical choices. The assignment and supplied rows/configs determine business requirements; external sources do not establish the intended mapping fixes.

| Source | What it supports | Decision |
| --- | --- | --- |
| [Amazon Settlement Reports](https://developer-docs.amazon/sp-api/lang-US/docs/report-type-values-settlement) | Flat File V2 is tab-delimited and represents money through amount-type, amount-description and amount; local numeric formats vary | Parse TSV components with an explicit AUD locale, retain metadata; do not infer universal decimal syntax |
| [Go encoding/csv](https://pkg.go.dev/encoding/csv) | Standard CSV reader, configurable delimiter, field positions and input offsets | Use proper CSV/TSV parsing and physical-source lineage |
| [PostgreSQL numeric types](https://www.postgresql.org/docs/current/datatype-numeric.html) | Numeric is exact; declared scale may round inputs | Exact numeric domain with explicit scale/range validation, no float accounting |
| [pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5) | COPY support and PostgreSQL-native type handling | Bounded bulk insert and explicit numeric codecs |
| [PostgreSQL transaction isolation](https://www.postgresql.org/docs/18/transaction-iso.html) | Transaction behavior and concurrency guarantees | Atomic ingestion and immutable completed-run reads |
| [PostgreSQL locking](https://www.postgresql.org/docs/17/explicit-locking.html) | Row locks and transaction/session advisory-lock distinctions | Use unique run claim and row-locked retry; avoid unnecessary distributed lock service |
| [PostgreSQL versioning policy](https://www.postgresql.org/support/versioning/) | PostgreSQL 16.15 is the current supported 16.x minor release; current minors contain bug and security fixes | Pin local and CI containers to 16.15 rather than the obsolete 16.4 image |
| [Neon connection errors](https://neon.com/docs/connect/connection-errors) | Direct and pooled connections require TLS/SNI; pooled connections have session-feature limitations | Use the direct endpoint for migrations and verify transport failures before changing credentials |
| [Neon pooled connections](https://neon.com/docs/changelog/2023-02-06) | The pooled hostname adds `-pooler` to the endpoint ID | Derive the pooler hostname without changing database credentials |
| [Neon serverless driver](https://neon.com/docs/serverless/serverless-driver) | SQL-over-HTTP supports non-interactive transactions | Permit an audited operator fallback when HTTPS works but raw PostgreSQL is unavailable |
| [Excelize stream writer](https://xuri.me/excelize/en/stream.html) | Streaming, ordered writes, flush and worksheet API constraints | Stream large audit sheets; isolate regular Summary writes |
| [Go release history](https://go.dev/doc/devel/release) | Go 1.27.1 released September 1, 2026 | Initial toolchain pin; recheck security updates at implementation |

PostgreSQL's current numeric documentation header reported 18.6 as released on August 13, 2026. Use a pinned supported patch, not an unqualified `latest` container tag. Library exact patch pins remain an implementation dependency-resolution task; this pack deliberately does not invent an unverified pgx or Excelize release number.

## What was inferred from the supplied files

- Accounting population: use selected Released payments; payment transfers are retained but not operating summary contributions.
- Key day: release timestamp converted to UTC empirically matches settlement posted-day identity.
- Payment tax grain: combined fields correspond to sums of several settlement components, verified at shared-key level.
- Mapping changes: two duplicate-route deletions, four settlement target updates and one refund-tax target update make every matched key's buckets agree.

The evidence probe and JSON files make these observations reproducible. Their results must be independently reproduced by Go plus PostgreSQL during implementation. A documentation claim that a library supports an operation does not establish that our eventual use is correct; integration and export checks are still required.
