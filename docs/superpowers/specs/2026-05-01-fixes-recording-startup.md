# Fixes: Recording Failure Cleanup + Startup Safe Check

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix two bugs: (1) meetings left in `pending` state when recording fails to start, and (2) app shows empty state on first launch due to a race condition between the HTTP server and the frontend.

**Architecture:** Both fixes are isolated changes — one in the frontend (`RecordingModal.tsx`, `App.tsx`), one touches no backend code.

**Tech Stack:** React 19, TypeScript, React Query v5, Wails v2 runtime

---

## Fix 1 — Delete meeting if recording fails to start

### Problem

`RecordingModal.tsx` creates the meeting (`POST /api/meetings`) before calling `POST /api/meetings/{id}/start`. If `/start` returns an error (503 audio unavailable, 409 already recording, or any other error), the meeting remains in the database with status `pending` indefinitely.

### Solution

In the `RecordingModal.tsx` start handler, if the `/start` call fails, immediately delete the meeting that was just created via `DELETE /api/meetings/{id}`. The error toast is shown as before. The meeting list is invalidated after the delete so the UI stays consistent.

### Files

- Modify: `frontend/src/components/recording/RecordingModal.tsx`

### Behavior

- Success path: unchanged — meeting created, start succeeds, modal closes, meeting selected
- Failure path: meeting created → start fails → meeting deleted → error toast shown → modal stays open

---

## Fix 2 — Safe check before marking app ready

### Problem

`App.tsx` calls the Wails-exposed `GetPort()` to discover the dynamic HTTP port, then calls `initApi(port)` and immediately sets `ready = true`. React Query hooks fire as soon as `ready` is true. However, there is a race condition: the Go HTTP server may not be fully bound and accepting connections yet when `GetPort()` returns. On first launch, queries fail silently, returning empty results. On the second launch, the server is already running so there is no race.

### Solution

After `initApi(port)`, poll `GET /health` at 500ms intervals before setting `ready = true`. Only when the health endpoint returns HTTP 200 does the app mark itself ready and allow queries to run. If the health check does not succeed within 15 seconds, set an `error` state and show a message to the user.

### Files

- Modify: `frontend/src/App.tsx`

### Behavior

- Normal startup: short delay (typically < 1s) while polling, then app loads normally
- Slow startup (audio service loading): continues polling — health endpoint responds once the Go server is up, independently of the audio service
- Failure (server never starts): after 15s timeout, shows error message in place of the app

---

## Error state UI

A minimal centered message: `"Não foi possível conectar ao servidor. Tente reiniciar o app."` — no full error page needed, just a `<p>` in place of the loading indicator.
