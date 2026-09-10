# Phase 4 reconciliation evidence

Reconciliation aggregates each source once per scoped tuple key and then performs a full outer join. Presence is represented separately from amount difference, so a payment-only zero, a settlement-only row, and a matched unequal amount cannot be confused.

The fixed fixture produces 13,289 shared keys, 26 payment-only keys, and zero settlement-only keys. All 78,005 transaction rows resolve to one persisted reconciliation group membership; metadata remains a source-row control and is not treated as a transaction.

The `explain` command traces a persisted contribution by field, rule, record reference, or physical source line. `verify-db` independently checks source counts, group membership, amount mismatches, bucket differences, summary contributions, settlement header control, and strict-run status. These checks operate against PostgreSQL lineage tables and do not read report totals as an authority.
