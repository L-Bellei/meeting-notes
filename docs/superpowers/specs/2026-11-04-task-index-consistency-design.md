# Task Lifecycle and Search Index Consistency — Design

## Context

Stage 3 of `docs/engineering-improvements-roadmap.md` covers generated and manual task lifecycle,
search indexing, and recovery from consistency failures.

The current implementation has three test-confirmed failures:

- `TaskService.Generate` deletes the meeting's existing tasks, then inserts suggestions one by one.
  A forced insert failure leaves the meeting with no tasks rather than preserving the previous set.
- Manual task create/update/delete does not update `meetings_fts`, so searches return stale results.
- `MeetingService.Update` ignores search-upsert errors. With the index unavailable, the title change
  still succeeds and the caller receives success while search remains stale.

Search content is derived from meetings, summaries, key points, and tasks. Those tables are the
source of truth; FTS must not be allowed to commit a different view of them.

## Decisions

- When a primary mutation and its FTS update cannot commit together, fail the operation and retain
  the previous primary and index state. This is the user's selected behavior.
- Replace generated tasks as a single transaction. AI generation remains outside the transaction;
  only the replacement persistence is transactional.
- Maintain FTS synchronously from SQLite triggers on the source tables. Trigger writes participate
  in the same transaction as the source mutation, so an indexing error rolls back both.
- Install triggers and rebuild the existing FTS table from canonical meeting, summary, key-point,
  and task data in an additive migration. Existing content remains authoritative.
- Remove redundant application-level best-effort FTS updates after trigger-based synchronization is
  in place. Keep the existing search API and result format unchanged.
- Scope is Stage 3 only. Do not start Stage 4 until this stage is reviewed and complete.

## Invariants

1. Every indexed meeting reflects its current title, transcript, summary, key points, and tasks.
2. A source mutation and its FTS representation commit or roll back together.
3. Failed task generation preserves the entire pre-existing task set.
4. Rebuilding the FTS index from source tables is deterministic and does not mutate source data.
5. Search behavior and API response shape remain unchanged.

## Out of scope

- Task UI redesign, notification lifecycle, export, and roadmap Stages 4–5.
- Changing AI prompt or task suggestion semantics.
- New asynchronous workers or an indexing outbox; synchronous SQLite transactions are sufficient
  for this local single-user application.
