# Recording Overlay Widget

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show a minimal always-on-top floating pill on the screen whenever a recording is active, giving the user a visible timer, a stop button, and a confirmation step — without requiring the main app window to be open.

**Architecture:** A native Win32 layered window (`OverlayWindow`) created in Go, following the same pattern as `tray.go`. The overlay is painted with GDI32 and communicates with the rest of the app via the existing `TrayManager` and Wails runtime events.

**Tech Stack:** Go, Win32 API (user32, gdi32, kernel32), `//go:build windows`

---

## File Structure

- Create: `cmd/desktop/overlay.go` — `OverlayWindow` struct, Win32 window creation, GDI painting, input handling, timer goroutine
- Modify: `cmd/desktop/tray.go` — show/hide overlay on recording start/stop
- Modify: `cmd/desktop/app.go` — instantiate `OverlayWindow`, pass to `TrayManager`

---

## OverlayWindow — Visual Design

A horizontal pill in the top-right corner of the primary monitor, 20px from the top and right edges.

**Normal state (recording):**
```
╭─────────────────────────────╮
│  ● Gravando  04:32       ●  │
╰─────────────────────────────╯
```
- Dark semi-transparent background (`#111111`, alpha ~220/255)
- 1px border `rgba(255,255,255,0.12)`
- Red pulsing dot (8px, toggled by timer goroutine every 500ms)
- "Gravando" label in white, 12px
- Timer in monospace white, MM:SS format
- Circular red stop button on the right (28px diameter)
- Rounded corners: 24px radius (`RoundRect`)

**Confirmation state (after clicking stop):**
```
╭──────────────────────────────────╮
│  Parar gravação?   [Sim]  [Não]  │
╰──────────────────────────────────╯
```
- Pill widens to 260px (normal state: 220px) to fit two buttons
- "Sim" button: red background, white text
- "Não" button: transparent, border, white text
- Clicking anywhere outside the overlay while in confirmation state cancels (returns to normal state)

---

## OverlayWindow — Behavior

**Lifecycle:**
- Created once in `app.go` alongside `TrayManager`
- `Show(meetingID, meetingTitle string)` — positions the window top-right, starts the timer goroutine, makes the window visible
- `Hide()` — stops the timer goroutine, hides the window
- Destroyed in `OnShutdown`

**Dragging:**
- `WM_NCHITTEST` returns `HTCAPTION` when the cursor is over the pill body (not the stop button area), enabling native Win32 drag without any extra code

**Timer:**
- A goroutine started on `Show()`, stopped by a channel on `Hide()`
- Increments elapsed seconds every 1s
- Calls `InvalidateRect` to trigger a repaint for the timer text and dot pulse

**Stop button click:**
- `WM_LBUTTONDOWN`: if click coords fall within the circular button bounds → transition to confirmation state, repaint
- In confirmation state, `WM_LBUTTONDOWN`: hit-test Sim/Não buttons
  - Sim: call `POST /api/meetings/{id}/stop` via HTTP to localhost, then `wailsruntime.Show(ctx)` to bring main app to front, then `Hide()`
  - Não: return to normal recording state, repaint

**Always-on-top:**
- Window created with `WS_EX_TOPMOST | WS_EX_LAYERED | WS_EX_TOOLWINDOW`
- `WS_EX_TOOLWINDOW` prevents it from appearing in the taskbar or Alt+Tab list

**Transparency:**
- `SetLayeredWindowAttributes` with `LWA_ALPHA`, alpha=220
- Window background painted with a rounded dark rect; areas outside the rounded rect left transparent via region clipping (`CreateRoundRectRgn` + `SetWindowRgn`)

---

## Integration with TrayManager

`TrayManager` already receives recording events and manages the tray icon tooltip. Two new calls are added:

- When recording starts (currently updates tray tooltip): also call `overlay.Show(meetingID, meetingTitle)`
- When recording stops or pipeline completes: also call `overlay.Hide()`

The `OverlayWindow` holds a reference to the app context so it can call `wailsruntime.Show()` on confirm-stop.

---

## Win32 APIs Required

All already used or available via existing lazy DLL pattern in `tray.go`:

| API | Purpose |
|---|---|
| `CreateWindowExW` | Create the overlay HWND |
| `SetLayeredWindowAttributes` | Alpha transparency |
| `CreateRoundRectRgn` | Clip window to pill shape |
| `SetWindowRgn` | Apply the clip region |
| `BeginPaint` / `EndPaint` | GDI painting |
| `CreateSolidBrush` / `FillRect` | Background fill |
| `RoundRect` | Pill border shape |
| `DrawTextW` | Timer and label text |
| `CreateFontW` | Monospace font for timer |
| `Ellipse` | Stop button circle |
| `InvalidateRect` | Trigger repaint from timer goroutine |
| `SetWindowPos` | Position top-right on show |
| `ShowWindow` / `SW_SHOW` / `SW_HIDE` | Visibility |

---

## Error Handling

- If `POST /api/meetings/{id}/stop` fails from the overlay: show a brief error state in the pill ("Erro ao parar") for 2s, then return to normal recording state
- If the overlay window fails to create: log the error and continue — the app works without the overlay
