# Pipeline Consistency — Design

## Context

The Stage 1 audit concerns the meeting recording and AI-processing lifecycle. The relevant
implementation is in `internal/services/orchestrator.go`, `internal/services/meeting_service.go`,
`internal/repository/meeting_repository.go`, the meeting detail UI, and `cmd/desktop/app.go`.

Two deterministic characterization tests confirmed active defects:

- A second `Reprocess` request is accepted while the first AI pipeline is blocked in summary
  generation, resulting in two concurrent pipelines for one meeting.
- `RunAIPipeline` loads a complete `Meeting` before generation and writes that stale structure
  after generation. A title and transcript edit made while AI is blocked are overwritten by the
  pipeline's final full-row update.

The shutdown path closes the database without calling `WaitPipelines`; spawned jobs use
`context.Background()` and therefore are not canceled by application shutdown. The current UI also
allows manual summary/key-point/task generation and edits while a meeting is processing.

## Decisions

- Concurrent pipeline requests for one meeting are rejected with a conflict; they are not queued.
- Transcript and notes edits are rejected while transcription or AI processing is active.
- Application shutdown cancels active workers and waits for them to exit before closing the
  database. In-progress meeting data must remain retryable; an active recording must be finalized
  to an audio file when possible.
- Only Stage 1 is in scope. No work on later roadmap stages is included.

## Invariants

1. A meeting has at most one active transcription or AI-generation operation.
2. The operation reservation and persisted active status are established before asynchronous work
   begins, so a second request cannot pass a check-then-spawn window.
3. Pipeline persistence changes only fields owned by the pipeline; it never writes a stale full
   meeting snapshot over user edits.
4. Transcript and notes, which are generation inputs, cannot change while an operation using them
   is active.
5. Shutdown cancels and joins workers before database close. An interrupted operation leaves
   transcript and audio data available for retry where those data have already been produced.
6. A retry clears any previous error state.

## Out of scope

- Roadmap stages after Stage 1.
- Database schema changes unless implementation proves one necessary.
- Redesign of partial AI-result persistence or task lifecycle.
