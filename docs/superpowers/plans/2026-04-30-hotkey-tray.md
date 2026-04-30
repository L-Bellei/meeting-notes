# Hotkey Global + System Tray — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a global `Ctrl+Shift+R` hotkey that toggles recording from any window, and minimize the app to the system tray instead of closing.

**Architecture:** A single Win32 hidden window (`CreateWindowExW`) handles both features: `RegisterHotKey` sends `WM_HOTKEY` to its message queue, and `Shell_NotifyIcon` sends tray click events as `WM_TRAYICON`. All Win32 calls are made via `syscall.NewLazyDLL` — no CGO, no new dependencies. The `TrayManager` runs a locked goroutine (`runtime.LockOSThread`) to keep all Win32 calls on one OS thread.

**Tech Stack:** Go 1.22, `syscall` (stdlib), `unsafe` (stdlib), `runtime` (stdlib), Wails v2 runtime events, React + TypeScript frontend.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/repository/meeting_repository.go` | Modify | Add `GetRecording()` method |
| `internal/repository/meeting_repository_test.go` | Modify | Test for `GetRecording()` |
| `cmd/desktop/tray.go` | **Create** | Complete `TrayManager` — Win32 window, hotkey, tray icon, context menu |
| `cmd/desktop/app.go` | Modify | Add `tray`, `allowQuit` fields; wire in `OnStartup`/`OnShutdown`; add `OnBeforeClose` |
| `cmd/desktop/main.go` | Modify | Register `OnBeforeClose` hook |
| `frontend/src/App.tsx` | Modify | Listen for `"hotkey:recording-started"` Wails event |

---

## Task 1: `MeetingRepository.GetRecording()`

**Files:**
- Modify: `internal/repository/meeting_repository.go` (append after `Delete`)
- Modify: `internal/repository/meeting_repository_test.go` (append new test)

- [ ] **Step 1: Write the failing test**

Append to `internal/repository/meeting_repository_test.go`:

```go
func TestMeetingRepository_GetRecording(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	// no meeting recording → returns nil, nil
	got, err := repo.GetRecording(ctx)
	if err != nil {
		t.Fatalf("GetRecording: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got.ID)
	}

	// create a pending meeting — should NOT be returned
	now := time.Now().UTC().Truncate(time.Second)
	pending := &models.Meeting{
		ID: "rec-pending", Title: "Pending", Status: models.StatusPending,
		StartedAt: &now, CreatedAt: now,
	}
	if err := repo.Create(ctx, pending); err != nil {
		t.Fatalf("Create pending: %v", err)
	}
	got, err = repo.GetRecording(ctx)
	if err != nil {
		t.Fatalf("GetRecording after pending: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil after pending, got %v", got.ID)
	}

	// create a recording meeting — should be returned
	rec := &models.Meeting{
		ID: "rec-001", Title: "Em gravação", Status: models.StatusRecording,
		StartedAt: &now, CreatedAt: now,
	}
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatalf("Create recording: %v", err)
	}
	got, err = repo.GetRecording(ctx)
	if err != nil {
		t.Fatalf("GetRecording: %v", err)
	}
	if got == nil {
		t.Fatal("expected recording meeting, got nil")
	}
	if got.ID != rec.ID {
		t.Fatalf("expected ID %q, got %q", rec.ID, got.ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd F:/dev/meeting-notes
go test ./internal/repository/... -run TestMeetingRepository_GetRecording -v
```

Expected: `FAIL` — `repo.GetRecording undefined`

- [ ] **Step 3: Implement `GetRecording`**

Append after the `Delete` method in `internal/repository/meeting_repository.go`:

```go
// GetRecording returns the meeting currently in 'recording' status, or nil if none.
func (r *MeetingRepository) GetRecording(ctx context.Context) (*models.Meeting, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, created_at
		 FROM meetings WHERE status = 'recording' LIMIT 1`,
	)
	m, err := scanMeeting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get recording meeting: %w", err)
	}
	return m, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/repository/... -run TestMeetingRepository_GetRecording -v
```

Expected: `PASS`

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all packages pass (except `cmd/desktop` which needs `frontend/dist`).

- [ ] **Step 6: Commit**

```bash
git add internal/repository/meeting_repository.go internal/repository/meeting_repository_test.go
git commit -m "feat: add MeetingRepository.GetRecording for hotkey toggle"
```

---

## Task 2: `cmd/desktop/tray.go` — complete TrayManager

**Files:**
- Create: `cmd/desktop/tray.go`

This file is Windows-only (`//go:build windows`). It cannot be unit tested (requires a running Windows message loop). Verification is via `go build ./cmd/desktop/...`.

- [ ] **Step 1: Create `cmd/desktop/tray.go` with full implementation**

```go
//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

// ---------------------------------------------------------------------------
// Win32 constants
// ---------------------------------------------------------------------------

const (
	wmTrayIcon  = 0x0401 // WM_USER + 1; sent by Shell_NotifyIcon
	wmHotkey    = 0x0312
	wmDestroy   = 0x0002
	wmLbuttonup = 0x0202
	wmRbuttonup = 0x0205

	hotkeyID = 1
	modCtrl  = 0x0002
	modShift = 0x0004
	vkR      = 0x52

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002
	nifMessage = 0x00000001
	nifIcon   = 0x00000002
	nifTip    = 0x00000004

	mfString    = 0x00000000
	mfSeparator = 0x00000800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	idiApplication = 32512

	menuShow   = 1001
	menuRecord = 1002
	menuQuit   = 1003
)

// ---------------------------------------------------------------------------
// Win32 structs
// ---------------------------------------------------------------------------

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uTimeoutOrVersion uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type winMsg struct {
	hWnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type winPoint struct{ x, y int32 }

// ---------------------------------------------------------------------------
// Win32 procs
// ---------------------------------------------------------------------------

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procRegisterHotKey      = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey    = user32.NewProc("UnregisterHotKey")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenuEx    = user32.NewProc("TrackPopupMenuEx")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
)

// ---------------------------------------------------------------------------
// Package-level WndProc callback (syscall.NewCallback must be called once)
// ---------------------------------------------------------------------------

var wndProcCallback = syscall.NewCallback(trayWndProcImpl)

// globalTray is the single TrayManager instance; set in Start(), read in wndProcCallback.
var globalTray *TrayManager

// ---------------------------------------------------------------------------
// TrayManager
// ---------------------------------------------------------------------------

type TrayManager struct {
	ctx         context.Context
	orch        *services.Orchestrator
	meetingRepo *repository.MeetingRepository
	meetingSvc  *services.MeetingService
	app         *App
	hwnd        uintptr
	hIcon       uintptr
	running     bool
	isRecording bool
}

func NewTrayManager(
	app *App,
	orch *services.Orchestrator,
	meetingRepo *repository.MeetingRepository,
	meetingSvc *services.MeetingService,
) *TrayManager {
	return &TrayManager{app: app, orch: orch, meetingRepo: meetingRepo, meetingSvc: meetingSvc}
}

// Start registers the hotkey, adds the tray icon, and launches the message loop.
func (t *TrayManager) Start(ctx context.Context) error {
	t.ctx = ctx
	globalTray = t

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	t.hIcon, _, _ = procLoadIconW.Call(0, idiApplication)

	className, _ := syscall.UTF16PtrFromString("MeetingNotesTrayClass")
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   wndProcCallback,
		hInstance:     hInstance,
		lpszClassName: className,
	}
	ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}

	windowName, _ := syscall.UTF16PtrFromString("MeetingNotesTray")
	hwnd, _, err := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)),
		0, 0, 0, 0, 0, 0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	t.hwnd = hwnd

	if ret, _, err = procRegisterHotKey.Call(hwnd, hotkeyID, modCtrl|modShift, vkR); ret == 0 {
		log.Printf("tray: RegisterHotKey Ctrl+Shift+R: %v", err)
	}

	if err := t.addTrayIcon(); err != nil {
		log.Printf("tray: Shell_NotifyIcon: %v", err)
	}

	t.running = true
	go t.messageLoop()
	return nil
}

// Stop unregisters the hotkey, removes the tray icon, and destroys the window.
func (t *TrayManager) Stop() {
	if !t.running {
		return
	}
	t.running = false
	procUnregisterHotKey.Call(t.hwnd, hotkeyID)
	t.removeTrayIcon()
	procDestroyWindow.Call(t.hwnd)
}

func (t *TrayManager) IsRunning() bool { return t.running }

// UpdateState updates the tray tooltip to reflect the current recording state.
func (t *TrayManager) UpdateState(isRecording bool) {
	t.isRecording = isRecording
	t.updateTooltip()
}

// ---------------------------------------------------------------------------
// Message loop — must run on a locked OS thread for Win32 correctness
// ---------------------------------------------------------------------------

func (t *TrayManager) messageLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) { // WM_QUIT or error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// trayWndProcImpl is the Win32 window procedure for the hidden tray window.
// It must be a plain function (not a method) to be usable with syscall.NewCallback.
func trayWndProcImpl(hwnd, msg, wparam, lparam uintptr) uintptr {
	t := globalTray
	if t == nil {
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
		return ret
	}

	switch uint32(msg) {
	case wmHotkey:
		if wparam == hotkeyID {
			go t.toggleRecording()
		}

	case wmTrayIcon:
		switch uint32(lparam) {
		case wmLbuttonup:
			if t.ctx != nil && isWailsContext(t.ctx) {
				wailsruntime.Show(t.ctx)
				wailsruntime.WindowUnminimise(t.ctx)
			}
		case wmRbuttonup:
			// called on the message-loop thread — correct for TrackPopupMenuEx
			t.showContextMenu()
		}

	case wmDestroy:
		procPostQuitMessage.Call(0)
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
}

// ---------------------------------------------------------------------------
// toggleRecording — called from hotkey or tray menu (in a goroutine)
// ---------------------------------------------------------------------------

func (t *TrayManager) toggleRecording() {
	ctx := context.Background()

	recording, err := t.meetingRepo.GetRecording(ctx)
	if err != nil {
		log.Printf("tray: GetRecording: %v", err)
		return
	}

	if recording != nil {
		if err := t.orch.StopRecording(ctx, recording.ID); err != nil {
			log.Printf("tray: StopRecording: %v", err)
			return
		}
		t.UpdateState(false)
		return
	}

	title := "Reunião - " + time.Now().Format("02/01/2006 15:04")
	m, err := t.meetingSvc.Create(ctx, title, "", string(models.StatusPending), nil)
	if err != nil {
		log.Printf("tray: Create meeting: %v", err)
		return
	}
	if err := t.orch.StartRecording(ctx, m.ID); err != nil {
		log.Printf("tray: StartRecording: %v", err)
		return
	}
	t.UpdateState(true)
	if t.ctx != nil && isWailsContext(t.ctx) {
		wailsruntime.EventsEmit(t.ctx, "hotkey:recording-started", map[string]string{"meetingId": m.ID})
	}
}

// ---------------------------------------------------------------------------
// Tray icon management
// ---------------------------------------------------------------------------

func (t *TrayManager) buildNID() notifyIconData {
	nid := notifyIconData{
		hWnd:             t.hwnd,
		uID:              1,
		hIcon:            t.hIcon,
		uCallbackMessage: wmTrayIcon,
	}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	tip, _ := syscall.UTF16FromString("Meeting Notes")
	copy(nid.szTip[:], tip)
	return nid
}

func (t *TrayManager) addTrayIcon() error {
	nid := t.buildNID()
	nid.uFlags = nifMessage | nifIcon | nifTip
	ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return fmt.Errorf("Shell_NotifyIcon NIM_ADD: %w", err)
	}
	return nil
}

func (t *TrayManager) removeTrayIcon() {
	nid := t.buildNID()
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

func (t *TrayManager) updateTooltip() {
	nid := t.buildNID()
	nid.uFlags = nifTip
	tipStr := "Meeting Notes"
	if t.isRecording {
		tipStr = "Meeting Notes — Gravando..."
	}
	tip, _ := syscall.UTF16FromString(tipStr)
	copy(nid.szTip[:], tip)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

// ---------------------------------------------------------------------------
// Context menu (called on message-loop thread from WndProc)
// ---------------------------------------------------------------------------

func (t *TrayManager) showContextMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	showLabel, _ := syscall.UTF16PtrFromString("Abrir Meeting Notes")
	procAppendMenuW.Call(hMenu, mfString, menuShow, uintptr(unsafe.Pointer(showLabel)))

	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)

	var recLabel string
	if t.isRecording {
		recLabel = "Parar gravação"
	} else {
		recLabel = "Iniciar gravação"
	}
	recLabelPtr, _ := syscall.UTF16PtrFromString(recLabel)
	procAppendMenuW.Call(hMenu, mfString, menuRecord, uintptr(unsafe.Pointer(recLabelPtr)))

	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)

	quitLabel, _ := syscall.UTF16PtrFromString("Sair")
	procAppendMenuW.Call(hMenu, mfString, menuQuit, uintptr(unsafe.Pointer(quitLabel)))

	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(t.hwnd)

	// TPM_RETURNCMD: returns the selected item ID directly (no WM_COMMAND posted)
	cmdID, _, _ := procTrackPopupMenuEx.Call(
		hMenu, tpmRightButton|tpmReturnCmd,
		uintptr(pt.x), uintptr(pt.y), t.hwnd, 0,
	)

	switch cmdID {
	case menuShow:
		if t.ctx != nil && isWailsContext(t.ctx) {
			wailsruntime.Show(t.ctx)
			wailsruntime.WindowUnminimise(t.ctx)
		}
	case menuRecord:
		go t.toggleRecording()
	case menuQuit:
		if t.app != nil {
			t.app.allowQuit = true
		}
		if t.ctx != nil && isWailsContext(t.ctx) {
			wailsruntime.Quit(t.ctx)
		}
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd F:/dev/meeting-notes
go build ./cmd/desktop/...
```

Expected: build succeeds. If `frontend/dist` is missing, you'll see:
```
cmd\desktop\assets.go:5:12: pattern all:frontend/dist: no matching files found
```
That's pre-existing and unrelated to this task. Any other error must be fixed.

- [ ] **Step 3: Commit**

```bash
git add cmd/desktop/tray.go
git commit -m "feat: add TrayManager with Win32 hotkey and system tray"
```

---

## Task 3: Wire `TrayManager` into `app.go` and `main.go`

**Files:**
- Modify: `cmd/desktop/app.go` (lines 29-37 for struct; line 39 for OnStartup; line 183 for OnShutdown)
- Modify: `cmd/desktop/main.go` (add `OnBeforeClose`)

- [ ] **Step 1: Add `tray` and `allowQuit` fields to `App` struct**

In `cmd/desktop/app.go`, replace the `App` struct (lines 29-35):

```go
type App struct {
	ctx       context.Context
	db        *sql.DB
	port      int
	server    *http.Server
	audioProc *exec.Cmd
	tray      *TrayManager
	allowQuit bool
}
```

- [ ] **Step 2: Add `OnBeforeClose` method to `App`**

Append to `cmd/desktop/app.go` (after `GetPort()`):

```go
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.allowQuit {
		return false // allow the app to close normally
	}
	if a.tray != nil && a.tray.IsRunning() {
		wailsruntime.Hide(ctx)
		return true // prevent close; app stays in tray
	}
	return false
}
```

- [ ] **Step 3: Initialize TrayManager in `OnStartup`**

In `cmd/desktop/app.go`, at the end of `OnStartup` (after line 180, before the closing `}`), add:

```go
	a.tray = NewTrayManager(a, orch, meetingRepo, meetingSvc)
	if err := a.tray.Start(ctx); err != nil {
		log.Printf("tray: start: %v", err)
	}
```

- [ ] **Step 4: Stop TrayManager in `OnShutdown`**

In `cmd/desktop/app.go`, in `OnShutdown` (after closing the audio proc), add:

```go
	if a.tray != nil {
		a.tray.Stop()
	}
```

The full `OnShutdown` becomes:

```go
func (a *App) OnShutdown(ctx context.Context) {
	if a.server != nil {
		a.server.Shutdown(context.Background())
	}
	if a.db != nil {
		a.db.Close()
	}
	if a.audioProc != nil && a.audioProc.Process != nil {
		a.audioProc.Process.Kill()
	}
	if a.tray != nil {
		a.tray.Stop()
	}
}
```

- [ ] **Step 5: Register `OnBeforeClose` in `main.go`**

In `cmd/desktop/main.go`, add `OnBeforeClose` to the `wails.Run` options:

```go
func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "Meeting Notes",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.OnStartup,
		OnShutdown:    app.OnShutdown,
		OnBeforeClose: app.OnBeforeClose,
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 6: Build to verify**

```bash
cd F:/dev/meeting-notes
go build ./cmd/desktop/...
```

Expected: same as before (only `frontend/dist` warning, no new errors).

- [ ] **Step 7: Commit**

```bash
git add cmd/desktop/app.go cmd/desktop/main.go
git commit -m "feat: wire TrayManager into Wails app lifecycle"
```

---

## Task 4: Frontend — `EventsOn("hotkey:recording-started")`

**Files:**
- Modify: `frontend/src/App.tsx`

When the hotkey creates a new meeting and starts recording, the backend emits `"hotkey:recording-started"` with `{ meetingId: string }`. The frontend must navigate to that meeting and switch to the meetings view.

- [ ] **Step 1: Add the event listener**

In `frontend/src/App.tsx`, add a new `useEffect` after the existing `Ctrl+K` `useEffect` (after line 47):

```tsx
  useEffect(() => {
    const unlisten = EventsOn("hotkey:recording-started", (data: { meetingId: string }) => {
      setSelectedMeetingId(data.meetingId)
      setHighlightQuery(undefined)
      setActiveView("meetings")
    })
    return () => { if (typeof unlisten === "function") unlisten() }
  }, [])
```

- [ ] **Step 2: Add the `EventsOn` import**

In `frontend/src/App.tsx`, add `EventsOn` to the imports at the top. The current import is:

```tsx
import { GetPort } from "./wailsjs/go/main/App"
```

Add a new import line after it:

```tsx
import { EventsOn } from "./wailsjs/runtime/runtime"
```

- [ ] **Step 3: TypeScript check**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Expected: zero errors.

- [ ] **Step 4: Commit**

```bash
cd F:/dev/meeting-notes
git add frontend/src/App.tsx
git commit -m "feat: navigate to meeting on hotkey:recording-started event"
```

---

## Self-Review Checklist

| Spec requirement | Task |
|---|---|
| Hotkey `Ctrl+Shift+R` toggles recording | Task 2 (`RegisterHotKey`, `toggleRecording`) |
| No meeting recording → create new + start | Task 2 (`toggleRecording` else branch) |
| Meeting recording → stop it | Task 2 (`toggleRecording` if branch) |
| Frontend navigates to new meeting | Task 4 (`EventsOn`) |
| Close window → minimize to tray | Task 3 (`OnBeforeClose`) |
| Tray menu: Abrir / Iniciar\|Parar / Sair | Task 2 (`showContextMenu`) |
| Tray tooltip shows recording state | Task 2 (`UpdateState`, `updateTooltip`) |
| "Sair" actually quits | Task 2 (`app.allowQuit = true` + `Quit`) |
| `GetRecording` repo method | Task 1 |
| No CGO / no new external deps | All (only `syscall`, stdlib) |
