# Builder handoff

This pack is intended to let a builder complete a bounded phase without reconstructing finance semantics from conversation history. The next task is application implementation, not rewriting these documents or running an unrelated scaffold generator.

## How to use a short context window

1. Read AGENTS.md, README.md, SPEC.md and the next incomplete CHECKLIST phase.
2. Read only that phase's contracts plus MAPPING_FIXES.md before any accounting work.
3. Implement a complete vertical unit with its tests and error paths. Do not replace specific contracts with TODOs.
4. Run the phase gate and record actual outcomes in PROGRESS.md.
5. Leave exact next action, current run/config identities and any failure evidence. Continue the next phase when authorized by the implementation task.

## Phase inputs

| Phase | Read before editing |
| --- | --- |
| Primitives | DATA_CONTRACT, TEST_PLAN |
| Persistence | ARCHITECTURE, DATABASE, RUNBOOK |
| Mapping | MAPPING_ENGINE, MAPPING_FIXES, DATA_CONTRACT |
| Reconciliation | RECONCILIATION, DATABASE |
| Reports | DESIGN, RECONCILIATION, TEST_PLAN |
| Fix replay | MAPPING_FIXES, RUNBOOK, SUBMISSION |

## Ready-to-use implementation request

> Implement the next incomplete phase in CHECKLIST.md in Go. Follow AGENTS.md and the owning contracts. Keep the supplied data unchanged. Complete the phase gate, record actual commands/results in PROGRESS.md, and identify remaining work. Do not hardcode evidence values into production logic or claim unrun checks passed.

For an end-to-end build, request all remaining phases instead of only the next one. The ordered checklist still applies.

## Forbidden shortcuts

- Comparing entire payment file to the single settlement and deleting rows until totals match.
- Matching on original posted date or local-machine date without the documented UTC release policy.
- Joining raw records and then SUMming duplicated totals.
- Counting payment `total` together with its monetary components.
- Treating nonzero amount equality as the definition of reconciled.
- Guessing duplicate-rule precedence, hiding unknown mappings, dropping zero-value keys.
- Copying tax amounts from settlement into the payment summary.
- Changing Go/report logic for F01–F03 instead of applying SQL to mapping data.
- Claiming the sample workbook or absent Amazon statement supplies expected numeric answers.
- Using a Python tool as the production implementation while labeling the project Go.

## Session exit note

Record implemented files, tests run and results, database/migration state, run/config IDs and artifact paths, open issues with evidence, and the exact next checklist item. If evidence contradicts a contract, preserve the evidence, update the owning contract and tests with the reason, and explain the change. Do not silently accept the contradiction.
