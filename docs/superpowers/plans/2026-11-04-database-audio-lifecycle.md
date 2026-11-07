# Database and Audio Lifecycle — Implementation Plan

## Goal

Implement only Stage 2 of the engineering roadmap: atomic SQLite migrations and safe recovery,
finalization, and cleanup of meeting recordings.

## Tasks

1. [x] Add deterministic failure tests that interrupt a multi-statement migration and prove the current
   runner leaves partial schema behind; add audio-service tests for shutdown finalization and
   recovery of a previously written recording by identity.
2. [x] Make migration execution transactional, including insertion into `schema_migrations`, and
   verify a failed migration can be retried.
3. [x] Add an additive recording-identity migration and wire the identity through the meeting model,
   repository, audio client, and orchestrator without changing existing retained-audio behavior.
4. [x] Add a recording recovery path that finalizes a matching active recording or validates a
   completed file after audio-service restart. Preserve files that are unknown or invalid.
5. [x] Make recorder shutdown/stop failures explicit and retryable; make cleanup ordering safe so DB
   state never points to audio that has already been removed.
6. [x] Review existing-data compatibility, both app entry points, error handling, and all changed
   migration/audio call sites. Do not advance to Stage 3.

**Validation completed:** `go test ./...`, `go vet ./...`, and `pytest tests -q` from
`audio-service` (99 passed).

## Validation criteria

- A forced error after an earlier migration statement leaves the schema and migration ledger
  unchanged; retry then applies the migration successfully.
- Existing databases migrate without losing meeting, transcript, or retained-audio data.
- A recording identity survives app restart and only the corresponding valid file can be attached
  to its meeting.
- Audio-service shutdown attempts to finalize a live recording; failure is observable and does not
  cause automatic deletion or lose recovery metadata.
- Unknown, invalid, or unrelated audio files are never deleted or attached automatically.
- Existing Go and Python suites pass, with focused failure/recovery tests covering the new behavior.
