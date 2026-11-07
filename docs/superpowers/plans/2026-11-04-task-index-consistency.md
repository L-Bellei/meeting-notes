# Task Lifecycle and Search Index Consistency — Implementation Plan

**Status:** completed on 2026-11-04.

## Goal

Implement only Stage 3: make task replacement safe and keep the FTS search index transactionally
consistent with all indexed source data.

## Tasks

1. [x] Add a transactional repository operation for replacing generated tasks; test that a forced insert
   failure rolls back deletions and inserts.
2. [x] Add an additive migration that installs source-table triggers for meetings, summaries,
   key-points, and tasks, and rebuilds existing FTS content from canonical rows.
3. [x] Verify with real SQLite tests that create/update/delete operations change search results and
   that a failed FTS write rolls the source mutation back.
4. [x] Remove or simplify best-effort FTS writes in `MeetingService` and `Orchestrator` that become
   redundant once database triggers own synchronization.
5. [x] Review generation, manual CRUD, meeting edits/deletes, migration compatibility, and both app
   entry points; preserve the search API and existing UI behavior.
6. [x] Run targeted tests, `go test ./...`, and `go vet ./...`. Do not advance to Stage 4.

**Validation completed:** focused SQLite/task/search regressions, `go test ./...`, and
`go vet ./...` passed.

## Validation criteria

- Existing tasks remain unchanged after a failed generated replacement.
- Task create/update/delete immediately updates FTS results.
- Meeting/source data and FTS changes commit or roll back together on index errors.
- Rebuilding FTS uses current canonical source rows and removes stale index rows.
- Existing meeting, summary, key-point, and task data survive the migration.
