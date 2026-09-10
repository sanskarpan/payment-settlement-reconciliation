# Decision record

| Decision | Reason | Alternative rejected / consequence |
| --- | --- | --- |
| Modular CLI | Assignment is a reproducible local batch flow | HTTP stack adds deployment and authentication unrelated to evaluation |
| One shared source-row table | Explicit PDF requirement; uniform lineage | Two ingestion tables violate the requirement even if later unioned |
| Raw rows plus derived rule/contribution tables | Preserve source grain and many mappings without losing ownership | Exploding payments directly into raw rows obscures row counts and double-counts totals |
| Signed cents in Go; exact PostgreSQL numeric | AUD source precision is cents; arithmetic is deterministic | Float epsilon hides errors; integer arithmetic must check overflow |
| Source-isolated summary accumulation | Each column must stand alone | Copying one column to the other or reporting only matched rows invalidates the result |
| Settlement ID + Released scope | Full-file totals include unrelated settlements and deferred amounts | Date-only filtering loses old transactions released during the period |
| Release instant converted to UTC for payment keys | Empirical key coverage improves from 34 to 13,251 Order/Refund matches | Original posted date yields thousands of false mismatches |
| Tuple-encoded template values | Preserve literal/field values and avoid delimiter collisions | Bare concatenation can conflate distinct tuples |
| Presence status separate from amount equality | Required by PDF | Amount-based status would mislabel matched keys |
| Aggregate first, join second | Payment/settlement granularities differ | Raw joins multiply money |
| Diagnostic baseline allows disclosed overlapping routes | Preserve and quantify original config defects without inventing precedence | First/last rule wins hides a defect; strict rejection alone cannot produce a useful before report |
| Strict fixed-version selector uniqueness | Corrected rules must have one route per source component | Permissive ambiguity cannot qualify as an accepted run |
| Common tax bucket represented in config | Payments only expose combined tax; exact allocation cannot be reconstructed independently | Allocating from settlement would violate source independence; guessed tax rates are unjustified |
| Explicit transfer-description alias | Supplied config describes a dynamic text prefix | General substring matching can classify unrelated product descriptions |
| Fresh config version and fresh ingestion after SQL | Reproducible before/after evidence | Updating mappings under an existing report destroys audit history |
| Excelize export from calculated read models | Go implementation; stream large sheet, numeric output | Treating sample report values as an oracle is forbidden |
| Explicit unsupported nonzero intermediate fields | Current fixture does not establish their accounting semantics | Guessing from names can produce plausible but false totals |

External technical evidence is in RESEARCH.md. Business observations and counterexamples are in DATA_CONTRACT.md and MAPPING_FIXES.md. A decision change requires updating the owning contract, associated fixtures, and progress log together.
