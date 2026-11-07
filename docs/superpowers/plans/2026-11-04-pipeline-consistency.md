# Pipeline Consistency — Implementation Plan

## Goal

Implement the approved Stage 1 invariants from
`docs/superpowers/specs/2026-11-04-pipeline-consistency-design.md` without advancing to later
roadmap stages.

## Tasks

1. Replace the temporary characterization probes with deterministic regression tests for duplicate
   requests, stale pipeline writes, blocked generation-input edits, and shutdown cancellation.
2. Add per-meeting operation admission before state changes or goroutine creation. Return HTTP 409
   for an operation that conflicts with an active operation; ensure the synchronous database
   transition closes the current check-then-spawn race.
3. Make pipeline persistence field-specific and clear stale errors when a retry starts. Reject
   transcript/notes edits while transcription or AI processing is active, including at the API
   boundary.
4. Wire cancellation and worker joining into desktop shutdown. Preserve active recordings and
   retryable pipeline state before closing the database and audio-service process.
5. Reflect active state in the meeting UI, then review all changed call sites and both application
   entry points for consistent behavior.
6. Run focused Go and frontend checks, followed by the relevant broader suites if the focused
   checks reveal integration issues.

## Validation criteria

- A blocked AI call permits exactly one active pipeline; a concurrent request receives conflict.
- An edit to title or other non-input meeting fields survives pipeline completion.
- Transcript and notes edits are rejected while their values are being consumed by an active
  pipeline, and the same rejection is enforced by HTTP handlers.
- Shutdown cancels active work, waits for completion, and does not close the database while a
  worker can still access it.
- Cancellation preserves available audio/transcript data and leaves the meeting retryable.
- Existing sequential recording, transcription, and reprocessing behavior remains covered by the
  service tests.
