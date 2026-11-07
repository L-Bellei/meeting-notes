# Database and Audio Lifecycle — Design

## Context

Stage 2 of `docs/engineering-improvements-roadmap.md` covers SQLite migration recovery and
recording-file lifecycle. The current implementation has two independently confirmed risks:

- `internal/database/database.go` executes each SQL migration and records its name in separate
  `db.Exec` calls, without a transaction. A multi-statement migration can therefore leave schema
  changes behind without its `schema_migrations` row if a later statement fails.
- `Recorder.start()` creates a file named from a random recording ID. The Go orchestrator ignores
  the ID returned by `/recording/start`; `audio_path` is only persisted after `/recording/stop`.
  After the audio-service process restarts, its in-memory recording state is gone and the backend
  cannot associate an existing file with the meeting.

The audio-service lifespan currently has no shutdown finalizer. `Recorder.stop()` also resets its
in-memory state in a `finally` block even if finalization raises, leaving no retry path through the
current stop endpoint. The existing Go shutdown recovery covers graceful app shutdown when the
audio service is still responsive, but not recovery by durable recording identity after service
restart.

## Decisions

- Each migration and its `schema_migrations` record must commit atomically. A failed migration
  rolls back all its statements and can be retried on the next launch.
- Add an additive SQLite field for the active recording identity. Keep existing databases and
  meeting/audio records intact.
- Recovery must match a file to a meeting by the persisted recording identity and validate the
  audio before attaching it. Never delete unrecognized or invalid recording files automatically.
- Audio-service shutdown should attempt to finalize an active recording. A failure must be
  observable and must not silently discard the identity/path needed for a later recovery attempt.
- Preserve existing retention semantics: recordings marked to keep remain; recordings not kept
  are removed only after the meeting has a safe persisted state.
- Scope is Stage 2 only. Do not start Stage 3 or later.

## Invariants

1. A migration is either fully applied and recorded or has no persistent effects.
2. A recording identity is durable before the app relies on it for later recovery.
3. Only a validated recording with a matching persisted identity can be attached to a meeting.
4. Unknown files are retained; cleanup must not delete user audio based on filename heuristics.
5. A failed finalization remains diagnosable and retryable when the audio data is still available.
6. Audio deletion must not leave the database pointing at a file that has already been removed.

## Out of scope

- Reworking task lifecycle or search indexing (Stage 3).
- Broad redesign of audio formats, recording UI, or transcription engines.
- Automatic deletion of stale or unrecognized recordings.
- Changes to later roadmap stages.
