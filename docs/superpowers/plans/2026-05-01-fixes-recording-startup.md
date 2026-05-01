# Fixes: Recording Failure Cleanup + Startup Safe Check — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two bugs: delete orphaned meetings when recording fails to start, and poll `/health` before marking the app ready to eliminate first-launch empty state.

**Architecture:** Two isolated frontend-only changes. No backend code is touched. `RecordingModal.tsx` gets a cleanup path after a failed `/start` call. `App.tsx` replaces the immediate `setReady(true)` with a polling loop.

**Tech Stack:** React 19, TypeScript, Wails v2 runtime (`GetPort`), `fetch` API

---

## File Structure

| File | Change |
|---|---|
| `frontend/src/components/recording/RecordingModal.tsx` | Delete meeting on `/start` failure |
| `frontend/src/App.tsx` | Poll `/health` before `setReady(true)` |

---

### Task 1 — Delete orphaned meeting when /start fails

**Files:**
- Modify: `frontend/src/components/recording/RecordingModal.tsx`

The current `handleStart` catch block only sets an error message. We need to add a `DELETE /api/meetings/{id}` call when the meeting was successfully created but `/start` failed. The meeting `id` comes from `createMeeting.mutateAsync()` return value `m.id`.

- [ ] **Step 1: Open the file and locate `handleStart`**

Open `frontend/src/components/recording/RecordingModal.tsx`. The relevant function is `handleStart` starting at line 32. The try block currently looks like:

```ts
const m = await createMeeting.mutateAsync({ title: title.trim(), theme_id: themeId || undefined })
await api<void>(`/api/meetings/${m.id}/start`, { method: "POST" })
qc.invalidateQueries({ queryKey: ["meetings"] })
qc.invalidateQueries({ queryKey: ["meeting", m.id] })
onMeetingCreated(m.id)
setTitle("")
setThemeId("")
onClose()
```

The catch block starts at line 45. The fix is: track whether `m` was created (so we can delete it on failure), and delete inside catch before showing the error.

- [ ] **Step 2: Rewrite `handleStart` with cleanup logic**

Replace the entire `handleStart` function with:

```ts
async function handleStart() {
  if (!title.trim()) { setError("Título obrigatório"); return }
  setError("")
  setLoading(true)
  let createdId: string | null = null
  try {
    const m = await createMeeting.mutateAsync({ title: title.trim(), theme_id: themeId || undefined })
    createdId = m.id
    await api<void>(`/api/meetings/${m.id}/start`, { method: "POST" })
    qc.invalidateQueries({ queryKey: ["meetings"] })
    qc.invalidateQueries({ queryKey: ["meeting", m.id] })
    onMeetingCreated(m.id)
    setTitle("")
    setThemeId("")
    onClose()
  } catch (e: any) {
    if (createdId) {
      try {
        await api<void>(`/api/meetings/${createdId}`, { method: "DELETE" })
        qc.invalidateQueries({ queryKey: ["meetings"] })
      } catch {
        // ignore cleanup error — meeting stays pending but doesn't block the user
      }
    }
    const msg: string = e.message ?? ""
    if (msg.includes("503") || msg.toLowerCase().includes("unavailable")) {
      setError("Serviço de áudio indisponível")
    } else if (msg.includes("409")) {
      setError("Já existe uma gravação em andamento")
    } else {
      setError(msg)
    }
  } finally {
    setLoading(false)
  }
}
```

- [ ] **Step 3: Verify no TypeScript errors**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Manually test the failure path**

Stop the audio service process if running. Open the app → click "Nova Gravação" → fill in a title → click "Iniciar Gravação". Expected: error message shown, meeting list does NOT show a new pending entry.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/recording/RecordingModal.tsx
git commit -m "fix: delete orphaned meeting when /start fails"
```

---

### Task 2 — Poll /health before marking app ready

**Files:**
- Modify: `frontend/src/App.tsx`

Currently `App.tsx` line 40-44:
```ts
useEffect(() => {
  GetPort().then(port => {
    initApi(port)
    setReady(true)
  })
}, [])
```

Replace the immediate `setReady(true)` with a polling loop that checks `/health` every 500ms, gives up after 15 seconds, and sets an error state on timeout.

- [ ] **Step 1: Add `error` state and update the startup effect**

In `frontend/src/App.tsx`, add `error` state and rewrite the startup `useEffect`.

Add to the state declarations (after line 25 `const [ready, setReady] = useState(false)`):
```ts
const [startupError, setStartupError] = useState(false)
```

Replace the startup `useEffect` (lines 40–45):
```ts
useEffect(() => {
  GetPort().then(async port => {
    initApi(port)
    const deadline = Date.now() + 15_000
    while (Date.now() < deadline) {
      try {
        const res = await fetch(`http://localhost:${port}/health`)
        if (res.ok) { setReady(true); return }
      } catch {
        // server not up yet
      }
      await new Promise(r => setTimeout(r, 500))
    }
    setStartupError(true)
  })
}, [])
```

- [ ] **Step 2: Update the loading render to show the error state**

The current loading check at line 90:
```tsx
if (!ready) {
  return (
    <div className="flex h-screen items-center justify-center flex-col gap-3 text-muted-foreground text-sm animate-fade-in">
      <Spinner size={24} className="text-primary" />
      Iniciando...
    </div>
  )
}
```

Replace with:
```tsx
if (!ready) {
  return (
    <div className="flex h-screen items-center justify-center flex-col gap-3 text-muted-foreground text-sm animate-fade-in">
      {startupError ? (
        <p className="text-destructive text-center px-8">
          Não foi possível conectar ao servidor. Tente reiniciar o app.
        </p>
      ) : (
        <>
          <Spinner size={24} className="text-primary" />
          Iniciando...
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 3: Verify no TypeScript errors**

```bash
cd frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Test normal startup**

Run `wails dev` from `cmd/desktop`. App should load normally — the polling resolves quickly once the server is up. The "Iniciando..." spinner should be visible for at most ~1 second on a fresh launch.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "fix: poll /health before marking app ready to eliminate first-launch empty state"
```

---

## Verification

```bash
# TypeScript
cd frontend && npx tsc --noEmit

# Go (backend unchanged, but verify it still builds)
cd cmd/desktop && go build ./...
```
