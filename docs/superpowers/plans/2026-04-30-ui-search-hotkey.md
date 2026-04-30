# UI Search Button, Hotkey Badge, and Configurable Hotkey — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a centered search pill to the toolbar, a hotkey badge on the Gravar button, and a configurable global recording hotkey (captured via click-to-record in Settings, applied immediately without restart).

**Architecture:** `SettingsHandler` gains an `onUpdate` callback wired in `app.go` to `TrayManager.ApplySettings`, which re-registers the Win32 hotkey live. The toolbar reads `recording_hotkey` from the React Query settings cache. A new `HotkeyCapture` component (inline in `SettingsModal`) captures keydown events.

**Tech Stack:** Go (Win32 syscall, no CGO), React 19 + TypeScript, React Query v5, Tailwind CSS, lucide-react

---

## File Map

| File | Change |
|---|---|
| `frontend/src/lib/formatHotkey.ts` | **New** — `formatHotkey("ctrl+shift+r")` → `"Ctrl+Shift+R"` |
| `frontend/src/components/layout/Toolbar.tsx` | Add `onSearch` prop + pill; add `recordingHotkey` prop + badge on Gravar |
| `frontend/src/App.tsx` | Pass `onSearch` + `recordingHotkey` to Toolbar; call `useSettings()` |
| `frontend/src/hooks/useSettings.ts` | Add `recording_hotkey: string` to `Settings` type |
| `frontend/src/components/settings/SettingsModal.tsx` | Add "Atalhos" section with `HotkeyCapture` component |
| `internal/handlers/settings_handler.go` | Add `onUpdate func(map[string]string)` field + `SetOnUpdate` method; call callback in `Update` |
| `cmd/desktop/app.go` | Wire `settingsHandler.SetOnUpdate` → `tray.ApplySettings` |
| `cmd/desktop/tray.go` | Add `hotkeyMods/VK` fields, `settingsRepo`, `parseHotkey`, `ApplySettings`; read setting in `Start` |

---

## Task 1: `formatHotkey` utility + Toolbar pill and badge

**Files:**
- Create: `frontend/src/lib/formatHotkey.ts`
- Modify: `frontend/src/components/layout/Toolbar.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Create `formatHotkey.ts`**

```typescript
// frontend/src/lib/formatHotkey.ts
export function formatHotkey(raw: string): string {
  return raw
    .split("+")
    .map(p => p.charAt(0).toUpperCase() + p.slice(1))
    .join("+")
}
```

- [ ] **Step 2: Replace `Toolbar.tsx` with updated version**

Replace the entire file:

```typescript
import { Menu, Mic, Settings, Search } from "lucide-react"
import { Button } from "../ui/button"

interface ToolbarProps {
  onToggleSidebar: () => void
  onRecord: () => void
  onSettings: () => void
  onSearch: () => void
  recordingHotkey?: string
  activeView: "meetings" | "board"
  onChangeView: (view: "meetings" | "board") => void
}

export function Toolbar({ onToggleSidebar, onRecord, onSettings, onSearch, recordingHotkey, activeView, onChangeView }: ToolbarProps) {
  return (
    <div className="h-14 border-b border-border flex items-center px-4 gap-3 flex-shrink-0 bg-background">
      <Button variant="ghost" size="icon" onClick={onToggleSidebar}>
        <Menu size={18} />
      </Button>
      <span className="font-semibold text-sm text-foreground">Meeting Notes</span>
      <div className="flex gap-1">
        <Button
          size="sm"
          variant={activeView === "meetings" ? "outline" : "ghost"}
          onClick={() => onChangeView("meetings")}
        >
          Reuniões
        </Button>
        <Button
          size="sm"
          variant={activeView === "board" ? "outline" : "ghost"}
          onClick={() => onChangeView("board")}
        >
          Board
        </Button>
      </div>
      <button
        onClick={onSearch}
        className="flex-1 mx-2 flex items-center gap-2 px-3 py-1.5 rounded-full bg-muted/50 border border-border text-muted-foreground text-sm hover:bg-muted transition-colors"
      >
        <Search size={13} />
        <span className="flex-1 text-left text-xs">Pesquisar reuniões...</span>
        <kbd className="text-[10px] bg-background border border-border rounded px-1.5 py-0.5 font-mono leading-none">
          Ctrl K
        </kbd>
      </button>
      <Button size="sm" onClick={onRecord}>
        <Mic size={14} className="mr-1.5" />
        Gravar
        {recordingHotkey && (
          <span className="ml-1.5 text-[10px] bg-white/20 rounded px-1 py-0.5 font-mono leading-none">
            {recordingHotkey}
          </span>
        )}
      </Button>
      <Button variant="ghost" size="icon" onClick={onSettings}>
        <Settings size={18} />
      </Button>
    </div>
  )
}
```

- [ ] **Step 3: Update `App.tsx` — add `useSettings`, pass new props to Toolbar**

Add the import at the top (after existing imports):
```typescript
import { useSettings } from "./hooks/useSettings"
import { formatHotkey } from "./lib/formatHotkey"
```

Inside `AppInner`, after the existing `useState` declarations, add:
```typescript
const { data: settings } = useSettings()
const recordingHotkey = formatHotkey(settings?.recording_hotkey ?? "ctrl+shift+r")
```

Update the `<Toolbar ...>` JSX — add two new props:
```typescript
<Toolbar
  onToggleSidebar={() => setSidebarOpen(o => !o)}
  onRecord={() => setRecordingModalOpen(true)}
  onSettings={() => setSettingsOpen(true)}
  onSearch={() => setSearchOpen(true)}
  recordingHotkey={recordingHotkey}
  activeView={activeView}
  onChangeView={setActiveView}
/>
```

- [ ] **Step 4: Verify TypeScript compiles**

```bash
cd C:/Users/leo_b/.config/superpowers/worktrees/meeting-notes/feat-ui-search-hotkey/frontend
npm run type-check 2>&1 || npx tsc --noEmit 2>&1
```

Expected: no errors (or no type-check script → check manually that imports resolve)

- [ ] **Step 5: Commit**

```bash
cd C:/Users/leo_b/.config/superpowers/worktrees/meeting-notes/feat-ui-search-hotkey
git add frontend/src/lib/formatHotkey.ts frontend/src/components/layout/Toolbar.tsx frontend/src/App.tsx
git commit -m "feat: add search pill and hotkey badge to toolbar"
```

---

## Task 2: `useSettings` type + `HotkeyCapture` in SettingsModal

**Files:**
- Modify: `frontend/src/hooks/useSettings.ts`
- Modify: `frontend/src/components/settings/SettingsModal.tsx`

- [ ] **Step 1: Add `recording_hotkey` to `Settings` type in `useSettings.ts`**

Open `frontend/src/hooks/useSettings.ts`. The current `Settings` interface ends with `whisper_model: string`. Add `recording_hotkey` after it:

```typescript
export interface Settings {
  user_name: string
  ai_provider: "anthropic" | "openai"
  anthropic_api_key: string
  anthropic_model: string
  openai_api_key: string
  openai_model: string
  auto_generate: string
  whisper_language: string
  whisper_model: string
  recording_hotkey: string
}
```

- [ ] **Step 2: Add `HotkeyCapture` component and "Atalhos" section to `SettingsModal.tsx`**

First, add this import at the top of `SettingsModal.tsx` (after existing imports):
```typescript
import { useEffect, useRef } from "react"
```

Note: `useState` and `useEffect` are already imported; add `useRef` to the existing import if not present. The file starts with:
```typescript
import { useState, useEffect } from "react"
```
Change it to:
```typescript
import { useState, useEffect, useRef } from "react"
```

Then add the `HotkeyCapture` component definition **before** the `SettingsModal` function (after the `WHISPER_MODELS` constant):

```typescript
function HotkeyCapture({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [listening, setListening] = useState(false)
  const btnRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!listening) return
    function onKey(e: KeyboardEvent) {
      e.preventDefault()
      const parts: string[] = []
      if (e.ctrlKey)  parts.push("ctrl")
      if (e.shiftKey) parts.push("shift")
      if (e.altKey)   parts.push("alt")
      const key = e.key.toLowerCase()
      if (!["control", "shift", "alt", "meta"].includes(key) && key.length === 1) {
        parts.push(key)
        onChange(parts.join("+"))
        setListening(false)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [listening, onChange])

  const display = value
    .split("+")
    .map(p => p.charAt(0).toUpperCase() + p.slice(1))
    .join("+")

  return (
    <div className="flex items-center gap-2 mt-1">
      <button
        ref={btnRef}
        onClick={() => setListening(l => !l)}
        className={cn(
          "flex-1 text-sm rounded-xl px-3 py-2 text-left border transition-colors font-mono",
          listening
            ? "bg-primary/10 border-primary text-primary animate-pulse"
            : "bg-[#111111] border-border text-foreground hover:border-primary/50"
        )}
      >
        {listening ? "Pressione o atalho..." : display}
      </button>
      <Button
        variant="outline"
        size="sm"
        onClick={() => { onChange("ctrl+shift+r"); setListening(false) }}
      >
        Restaurar
      </Button>
    </div>
  )
}
```

Then add the "Atalhos" section **inside** the modal content, between the Transcrição section and the footer `div`. Find the footer div that starts with `{/* footer */}` and insert before it:

```typescript
{/* Atalhos */}
<div className="px-5 py-4 border-b border-border space-y-3">
  <p className="text-[10px] uppercase tracking-widest text-muted-foreground">Atalhos</p>
  <div>
    <label className="text-xs text-muted-foreground">Atalho de gravação rápida</label>
    <HotkeyCapture
      value={form.recording_hotkey ?? "ctrl+shift+r"}
      onChange={v => set("recording_hotkey", v)}
    />
    <p className="text-[10px] text-muted-foreground/60 mt-1">
      Padrão: Ctrl+Shift+R — funciona com o app em segundo plano
    </p>
  </div>
</div>
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd C:/Users/leo_b/.config/superpowers/worktrees/meeting-notes/feat-ui-search-hotkey/frontend
npx tsc --noEmit 2>&1 | head -30
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
cd C:/Users/leo_b/.config/superpowers/worktrees/meeting-notes/feat-ui-search-hotkey
git add frontend/src/hooks/useSettings.ts frontend/src/components/settings/SettingsModal.tsx
git commit -m "feat: add HotkeyCapture component and Atalhos section to SettingsModal"
```

---

## Task 3: `SettingsHandler` callback + `app.go` wiring

**Files:**
- Modify: `internal/handlers/settings_handler.go`
- Modify: `cmd/desktop/app.go`

- [ ] **Step 1: Add `onUpdate` callback to `SettingsHandler`**

Open `internal/handlers/settings_handler.go`. Replace the entire file with:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"meeting-notes/internal/services"
)

type SettingsHandler struct {
	svc      *services.SettingsService
	onUpdate func(map[string]string)
}

func NewSettingsHandler(svc *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

func (h *SettingsHandler) SetOnUpdate(fn func(map[string]string)) {
	h.onUpdate = fn
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Update(r.Context(), updates); err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}
	settings, err := h.svc.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read updated settings")
		return
	}
	if h.onUpdate != nil {
		h.onUpdate(settings)
	}
	writeJSON(w, http.StatusOK, settings)
}
```

- [ ] **Step 2: Write a test for `SetOnUpdate` in the handler tests**

Open `internal/handlers/settings_handler_test.go` (create it if it doesn't exist — check with `ls internal/handlers/`). Add this test:

```go
func TestSettingsHandler_UpdateCallsOnUpdate(t *testing.T) {
	db := openTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	settingsSvc := services.NewSettingsService(settingsRepo)
	h := handlers.NewSettingsHandler(settingsSvc)

	var called map[string]string
	h.SetOnUpdate(func(s map[string]string) { called = s })

	body := `{"recording_hotkey":"ctrl+alt+r"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if called == nil {
		t.Fatal("onUpdate was not called")
	}
	if called["recording_hotkey"] != "ctrl+alt+r" {
		t.Fatalf("expected ctrl+alt+r, got %q", called["recording_hotkey"])
	}
}
```

Check how `openTestDB` is defined in other handler test files — it's a helper in the `handlers_test` package. Look at `internal/handlers/meeting_handler_test.go` for the exact helper signature and adapt accordingly. The pattern is `database.Open(t.TempDir() + "/test.db")`.

- [ ] **Step 3: Run the test to verify it fails before the implementation**

```bash
cd C:/Users/leo_b/.config/superpowers/worktrees/meeting-notes/feat-ui-search-hotkey
go test ./internal/handlers/ -run TestSettingsHandler_UpdateCallsOnUpdate -v 2>&1
```

Expected: FAIL — `SetOnUpdate` doesn't exist yet (or test file doesn't compile).

Note: the implementation in Step 1 already adds `SetOnUpdate`, so the test may pass immediately. If so, verify the test logic is correct before proceeding.

- [ ] **Step 4: Run all handler tests to make sure nothing is broken**

```bash
go test ./internal/handlers/ -v 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 5: Wire `SetOnUpdate` in `cmd/desktop/app.go`**

In `app.go`, find this block (around line 99):
```go
settingsHandler := handlers.NewSettingsHandler(settingsSvc)
```

Add after it (before the router setup):
```go
settingsHandler.SetOnUpdate(func(s map[string]string) {
    if a.tray != nil {
        a.tray.ApplySettings(s)
    }
})
```

- [ ] **Step 6: Build internal packages to verify**

```bash
cd C:/Users/leo_b/.config/superpowers/worktrees/meeting-notes/feat-ui-search-hotkey
go build ./internal/... 2>&1
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/handlers/settings_handler.go internal/handlers/settings_handler_test.go cmd/desktop/app.go
git commit -m "feat: add onUpdate callback to SettingsHandler and wire to tray in app.go"
```

---

## Task 4: `TrayManager` — `parseHotkey`, `ApplySettings`, live re-registration

**Files:**
- Modify: `cmd/desktop/tray.go`

This task adds `parseHotkey` and `ApplySettings` to `TrayManager`, reads the `recording_hotkey` setting at startup, and removes the hardcoded `modCtrl|modShift|vkR` constants from `RegisterHotKey`.

- [ ] **Step 1: Add `settingsRepo`, `hotkeyMods`, `hotkeyVK` to `TrayManager` struct and constructor**

In `tray.go`, find the `TrayManager` struct:
```go
type TrayManager struct {
	ctx         context.Context
	orch        *services.Orchestrator
	meetingRepo *repository.MeetingRepository
	meetingSvc  *services.MeetingService
	app         *App
	hwnd        uintptr
	hIcon       uintptr
	running     atomic.Bool
	isRecording bool
}
```

Replace with:
```go
type TrayManager struct {
	ctx          context.Context
	orch         *services.Orchestrator
	meetingRepo  *repository.MeetingRepository
	meetingSvc   *services.MeetingService
	settingsRepo *repository.SettingsRepository
	app          *App
	hwnd         uintptr
	hIcon        uintptr
	running      atomic.Bool
	isRecording  bool
	hotkeyMods   uint32
	hotkeyVK     uint32
}
```

Update `NewTrayManager` to accept `settingsRepo`:
```go
func NewTrayManager(
	app         *App,
	orch        *services.Orchestrator,
	meetingRepo *repository.MeetingRepository,
	meetingSvc  *services.MeetingService,
	settingsRepo *repository.SettingsRepository,
) *TrayManager {
	return &TrayManager{
		app:          app,
		orch:         orch,
		meetingRepo:  meetingRepo,
		meetingSvc:   meetingSvc,
		settingsRepo: settingsRepo,
	}
}
```

- [ ] **Step 2: Add `defaultHotkey` constant and `parseHotkey` function**

After the existing `const` block (after `menuQuit = 1003`), add:

```go
const defaultHotkey = "ctrl+shift+r"
```

After the `loadEmbeddedIcon` function, add:

```go
// parseHotkey parses "ctrl+shift+r" into Win32 modifier flags and virtual key code.
// Supported modifiers: ctrl, shift, alt, win. Key must be a single letter a–z.
func parseHotkey(s string) (mods, vk uint32, err error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("hotkey requires modifier+key, got %q", s)
	}
	keyPart := parts[len(parts)-1]
	for _, mod := range parts[:len(parts)-1] {
		switch mod {
		case "ctrl":
			mods |= 0x0002
		case "shift":
			mods |= 0x0004
		case "alt":
			mods |= 0x0001
		case "win":
			mods |= 0x0008
		default:
			return 0, 0, fmt.Errorf("unknown modifier %q", mod)
		}
	}
	if len(keyPart) != 1 || keyPart[0] < 'a' || keyPart[0] > 'z' {
		return 0, 0, fmt.Errorf("key must be a single letter a–z, got %q", keyPart)
	}
	vk = uint32(keyPart[0]-'a') + 0x41
	return mods, vk, nil
}
```

Add `"strings"` to the import block at the top of the file.

- [ ] **Step 3: Update `Start()` to read `recording_hotkey` from settings**

Find this block in `Start()`:
```go
if ret, _, err = procRegisterHotKey.Call(hwnd, hotkeyID, modCtrl|modShift, vkR); ret == 0 {
    log.Printf("tray: RegisterHotKey Ctrl+Shift+R: %v", err)
}
```

Replace with:
```go
hotkeyStr := defaultHotkey
if t.settingsRepo != nil {
    if all, err2 := t.settingsRepo.GetAll(ctx); err2 == nil {
        if v := all["recording_hotkey"]; v != "" {
            hotkeyStr = v
        }
    }
}
mods, vk, err2 := parseHotkey(hotkeyStr)
if err2 != nil {
    log.Printf("tray: invalid hotkey %q, using default: %v", hotkeyStr, err2)
    mods, vk, _ = parseHotkey(defaultHotkey)
}
t.hotkeyMods = mods
t.hotkeyVK = vk
if ret, _, err = procRegisterHotKey.Call(hwnd, hotkeyID, uintptr(mods), uintptr(vk)); ret == 0 {
    log.Printf("tray: RegisterHotKey %q: %v", hotkeyStr, err)
}
```

- [ ] **Step 4: Add `ApplySettings` method**

After the `UpdateState` method, add:

```go
// ApplySettings re-registers the hotkey if recording_hotkey changed.
func (t *TrayManager) ApplySettings(settings map[string]string) {
	key := settings["recording_hotkey"]
	if key == "" {
		key = defaultHotkey
	}
	mods, vk, err := parseHotkey(key)
	if err != nil {
		log.Printf("tray: invalid hotkey %q: %v", key, err)
		return
	}
	if mods == t.hotkeyMods && vk == t.hotkeyVK {
		return
	}
	procUnregisterHotKey.Call(t.hwnd, hotkeyID)
	t.hotkeyMods = mods
	t.hotkeyVK = vk
	if ret, _, regErr := procRegisterHotKey.Call(t.hwnd, hotkeyID, uintptr(mods), uintptr(vk)); ret == 0 {
		log.Printf("tray: RegisterHotKey %q: %v", key, regErr)
	}
}
```

- [ ] **Step 5: Update `app.go` to pass `settingsRepo` to `NewTrayManager`**

In `cmd/desktop/app.go`, find:
```go
a.tray = NewTrayManager(a, orch, meetingRepo, meetingSvc)
```

Replace with:
```go
a.tray = NewTrayManager(a, orch, meetingRepo, meetingSvc, settingsRepo)
```

- [ ] **Step 6: Build to verify**

```bash
cd C:/Users/leo_b/.config/superpowers/worktrees/meeting-notes/feat-ui-search-hotkey
go build ./internal/... 2>&1
```

Expected: no errors. (cmd/desktop won't build due to missing frontend/dist — expected.)

- [ ] **Step 7: Run all Go tests**

```bash
go test ./internal/... 2>&1
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add cmd/desktop/tray.go cmd/desktop/app.go
git commit -m "feat: live hotkey re-registration via parseHotkey and ApplySettings"
```

---

## Self-Review Checklist

After writing the plan, verify against the spec:

**Spec coverage:**
- ✅ Pill search button centered in toolbar (Task 1)
- ✅ Badge on Gravar button with current hotkey (Task 1)
- ✅ `onSearch` prop wired from App.tsx to Toolbar (Task 1)
- ✅ `recording_hotkey` in Settings type (Task 2)
- ✅ HotkeyCapture component with keydown capture (Task 2)
- ✅ "Restaurar" button resets to `ctrl+shift+r` (Task 2)
- ✅ `SetOnUpdate` callback on SettingsHandler (Task 3)
- ✅ Callback wired to `tray.ApplySettings` in app.go (Task 3)
- ✅ `parseHotkey` parsing modifier+key string (Task 4)
- ✅ `ApplySettings` re-registers hotkey immediately (Task 4)
- ✅ `Start()` reads setting from DB (Task 4)
- ✅ Default `ctrl+shift+r` when key not in DB (Task 4)
