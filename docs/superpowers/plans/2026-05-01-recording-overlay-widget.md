# Recording Overlay Widget — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a floating always-on-top pill widget on screen during recording, with a pulsing timer, a stop button, and a confirmation step before stopping.

**Architecture:** New `cmd/desktop/overlay.go` creates `OverlayWindow` — a native Win32 layered window following the same pattern as `tray.go`. Painted with GDI32. A timer goroutine drives dot-pulse and elapsed counter. The orchestrator's existing `notifyFn` in `app.go` calls `overlay.Show()`/`overlay.Hide()` when recording status changes.

**Tech Stack:** Go, Win32 API (user32, gdi32, kernel32), `//go:build windows`, `sync/atomic`, `sync`

---

## File Structure

| File | Change |
|---|---|
| `cmd/desktop/overlay.go` | New: `OverlayWindow` struct, Win32/GDI32 procs, message loop, paint handler, timer goroutine, input handling, HTTP stop call |
| `cmd/desktop/tray.go` | Add `overlay *OverlayWindow` field to `TrayManager` |
| `cmd/desktop/app.go` | Instantiate `OverlayWindow`; extend `SetNotifyFn` to call overlay Show/Hide |

---

### Task 1 — OverlayWindow struct, window creation, Show/Hide

**Files:**
- Create: `cmd/desktop/overlay.go`

- [ ] **Step 1: Create the file with build tag, imports, constants, and GDI32 procs**

Create `cmd/desktop/overlay.go`:

```go
//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------------------------------------------------------------------------
// Overlay Win32 constants
// ---------------------------------------------------------------------------

const (
	wsExTopmost     = 0x00000008
	wsExLayered     = 0x00080000
	wsExToolwindow  = 0x00000080
	wsPopup         = 0x80000000

	wmPaint       = 0x000F
	wmLbuttondown = 0x0201
	wmNchittest   = 0x0084
	wmClose       = 0x0010

	htCaption = 2
	htClient  = 1

	swShow = 5
	swHide = 0

	lwaAlpha = 0x00000002

	smCxscreen = 0

	transparentBk = 1 // TRANSPARENT for SetBkMode
	psSolid       = 0 // pen style
	nullBrushObj  = 5 // GetStockObject(NULL_BRUSH)
	nullPenObj    = 8 // GetStockObject(NULL_PEN)
	defaultGuiFont = 17 // GetStockObject(DEFAULT_GUI_FONT)

	// Pill geometry
	overlayWidth        = 220
	overlayHeight       = 44
	overlayWidthConfirm = 260
	overlayCorner       = 24 // RoundRect ellipse w/h for 24px radius

	// Stop button: 28px diameter, 8px from right edge
	stopBtnD  = 28
	stopBtnMR = 8
)

// GDI COLORREF format: 0x00BBGGRR
const (
	colorBg     = 0x00111111 // #111111 dark background
	colorRed    = 0x000000FF // #FF0000 red (in BGR = 0x000000FF)
	colorWhite  = 0x00FFFFFF
	colorBorder = 0x001F1F1F // subtle border on dark bg
)

// ---------------------------------------------------------------------------
// GDI32 procs (not in tray.go)
// ---------------------------------------------------------------------------

var (
	gdi32 = syscall.NewLazyDLL("gdi32.dll")

	procGetClientRect              = user32.NewProc("GetClientRect")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procFillRect                   = user32.NewProc("FillRect")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procScreenToClient             = user32.NewProc("ScreenToClient")

	procCreateSolidBrush   = gdi32.NewProc("CreateSolidBrush")
	procCreatePen          = gdi32.NewProc("CreatePen")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procRoundRect          = gdi32.NewProc("RoundRect")
	procEllipse            = gdi32.NewProc("Ellipse")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
	procGetStockObject     = gdi32.NewProc("GetStockObject")
	procSaveDC             = gdi32.NewProc("SaveDC")
	procRestoreDC          = gdi32.NewProc("RestoreDC")
	procCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn       = user32.NewProc("SetWindowRgn")
)

// winRect maps to Win32 RECT.
type winRect struct{ Left, Top, Right, Bottom int32 }

// paintStruct maps to Win32 PAINTSTRUCT (64 bytes).
type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     winRect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

// ---------------------------------------------------------------------------
// OverlayWindow
// ---------------------------------------------------------------------------

type OverlayWindow struct {
	hwnd uintptr
	ctx  context.Context
	port int

	mu         sync.Mutex
	meetingID  string
	confirming bool
	dotOn      bool
	stopCh     chan struct{}

	elapsed int64 // atomic; seconds elapsed
}

var globalOverlay *OverlayWindow
var overlayWndProcCb = syscall.NewCallback(overlayWndProcImpl)
```

- [ ] **Step 2: Write NewOverlayWindow — window class registration and CreateWindowExW**

Append to `cmd/desktop/overlay.go`:

```go
func NewOverlayWindow() *OverlayWindow {
	o := &OverlayWindow{}
	globalOverlay = o

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("MeetingNotesOverlayClass")
	windowName, _ := syscall.UTF16PtrFromString("MeetingNotesOverlay")

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   overlayWndProcCb,
		hInstance:     hInstance,
		lpszClassName: className,
	}
	if ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		log.Printf("overlay: RegisterClassEx: %v", err)
		return o
	}

	hwnd, _, err := procCreateWindowExW.Call(
		wsExTopmost|wsExLayered|wsExToolwindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsPopup,
		0, 0, overlayWidth, overlayHeight,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		log.Printf("overlay: CreateWindowEx: %v", err)
		return o
	}
	o.hwnd = hwnd

	// 220/255 alpha — whole-window uniform transparency
	procSetLayeredWindowAttributes.Call(hwnd, 0, 220, lwaAlpha)

	go o.runMessageLoop()
	return o
}

func (o *OverlayWindow) runMessageLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), o.hwnd, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
```

- [ ] **Step 3: Write Show() and Hide()**

Append to `cmd/desktop/overlay.go`:

```go
func (o *OverlayWindow) Show(ctx context.Context, port int, meetingID string) {
	if o.hwnd == 0 {
		return
	}

	o.mu.Lock()
	o.ctx = ctx
	o.port = port
	o.meetingID = meetingID
	o.confirming = false
	o.dotOn = true
	atomic.StoreInt64(&o.elapsed, 0)
	if o.stopCh != nil {
		close(o.stopCh)
	}
	stopCh := make(chan struct{})
	o.stopCh = stopCh
	o.mu.Unlock()

	// Clip window to pill shape
	rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidth, overlayHeight, overlayCorner, overlayCorner)
	if rgn != 0 {
		procSetWindowRgn.Call(o.hwnd, rgn, 0)
	}

	// Position: 20px from top-right of primary monitor
	screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
	x := int32(screenW) - overlayWidth - 20
	const swpNoActivate = 0x0010
	procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidth, overlayHeight, swpNoActivate)
	procShowWindow.Call(o.hwnd, swShow)

	go o.timerLoop(stopCh)
}

func (o *OverlayWindow) Hide() {
	if o.hwnd == 0 {
		return
	}
	o.mu.Lock()
	if o.stopCh != nil {
		close(o.stopCh)
		o.stopCh = nil
	}
	o.mu.Unlock()
	procShowWindow.Call(o.hwnd, swHide)
}
```

- [ ] **Step 4: Write overlayWndProcImpl stub (full implementation in later tasks)**

Append to `cmd/desktop/overlay.go`:

```go
func overlayWndProcImpl(hwnd, msg, wparam, lparam uintptr) uintptr {
	o := globalOverlay
	if o == nil || o.hwnd == 0 {
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
		return ret
	}

	switch uint32(msg) {
	case wmPaint:
		o.onPaint()
		return 0
	case wmNchittest:
		return o.onNcHitTest(hwnd, lparam)
	case wmLbuttondown:
		clientX := int32(int16(lparam & 0xFFFF))
		clientY := int32(int16((lparam >> 16) & 0xFFFF))
		o.onLButtonDown(clientX, clientY)
		return 0
	case wmClose:
		return 0 // prevent close
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
}
```

- [ ] **Step 5: Add placeholder stubs so the file compiles**

Append to `cmd/desktop/overlay.go`:

```go
func (o *OverlayWindow) onPaint()                            {}
func (o *OverlayWindow) onNcHitTest(hwnd, lparam uintptr) uintptr { return htCaption }
func (o *OverlayWindow) onLButtonDown(x, y int32)            {}
func (o *OverlayWindow) timerLoop(stopCh <-chan struct{})     {}
```

- [ ] **Step 6: Build to verify no compile errors**

```bash
cd cmd/desktop && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add cmd/desktop/overlay.go
git commit -m "feat: add OverlayWindow skeleton with Win32 window creation, Show, and Hide"
```

---

### Task 2 — GDI painting: pill, timer, stop button, confirmation

**Files:**
- Modify: `cmd/desktop/overlay.go`

Replace the stub `onPaint` and add `paintRecording`/`paintConfirmation` helpers.

- [ ] **Step 1: Replace onPaint stub with real implementation**

Remove the `func (o *OverlayWindow) onPaint() {}` stub and replace with:

```go
func (o *OverlayWindow) onPaint() {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(o.hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(o.hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc winRect
	procGetClientRect.Call(o.hwnd, uintptr(unsafe.Pointer(&rc)))

	// Dark background fill
	bgBrush, _, _ := procCreateSolidBrush.Call(colorBg)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), bgBrush)
	procDeleteObject.Call(bgBrush)

	// Pill border (rounded rect, no fill)
	procSaveDC.Call(hdc)
	borderPen, _, _ := procCreatePen.Call(psSolid, 1, colorBorder)
	nullBrush, _, _ := procGetStockObject.Call(nullBrushObj)
	procSelectObject.Call(hdc, borderPen)
	procSelectObject.Call(hdc, nullBrush)
	procRoundRect.Call(hdc,
		uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom),
		overlayCorner, overlayCorner)
	procRestoreDC.Call(hdc, ^uintptr(0))
	procDeleteObject.Call(borderPen)

	procSetBkMode.Call(hdc, transparentBk)

	o.mu.Lock()
	confirming := o.confirming
	dotOn := o.dotOn
	o.mu.Unlock()
	elapsed := atomic.LoadInt64(&o.elapsed)

	if confirming {
		o.paintConfirmation(hdc, rc)
	} else {
		o.paintRecording(hdc, rc, elapsed, dotOn)
	}
}
```

- [ ] **Step 2: Write paintRecording**

Append to `cmd/desktop/overlay.go`:

```go
func (o *OverlayWindow) paintRecording(hdc uintptr, rc winRect, elapsed int64, dotOn bool) {
	const dotSize = 8
	const dotX    = int32(12)
	dotY := (rc.Bottom - dotSize) / 2

	// Pulsing red dot
	if dotOn {
		procSaveDC.Call(hdc)
		redBrush, _, _ := procCreateSolidBrush.Call(colorRed)
		nullPen, _, _ := procGetStockObject.Call(nullPenObj)
		procSelectObject.Call(hdc, redBrush)
		procSelectObject.Call(hdc, nullPen)
		procEllipse.Call(hdc,
			uintptr(dotX), uintptr(dotY),
			uintptr(dotX+dotSize), uintptr(dotY+dotSize))
		procRestoreDC.Call(hdc, ^uintptr(0))
		procDeleteObject.Call(redBrush)
	}

	// "Gravando" label
	procSetTextColor.Call(hdc, colorWhite)
	guiFont, _, _ := procGetStockObject.Call(defaultGuiFont)
	procSelectObject.Call(hdc, guiFont)
	label, _ := syscall.UTF16PtrFromString("Gravando")
	labelRC := winRect{Left: dotX + dotSize + 6, Top: rc.Top, Right: 130, Bottom: rc.Bottom}
	const dtSingleline = 0x0020
	const dtVcenter    = 0x0004
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(label)), ^uintptr(0),
		uintptr(unsafe.Pointer(&labelRC)), dtSingleline|dtVcenter)

	// Timer MM:SS — right-aligned before stop button
	timerStr, _ := syscall.UTF16PtrFromString(fmt.Sprintf("%02d:%02d", elapsed/60, elapsed%60))
	timerRC := winRect{
		Left:   130,
		Top:    rc.Top,
		Right:  rc.Right - stopBtnD - stopBtnMR - 4,
		Bottom: rc.Bottom,
	}
	const dtRight = 0x0002
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(timerStr)), ^uintptr(0),
		uintptr(unsafe.Pointer(&timerRC)), dtSingleline|dtVcenter|dtRight)

	// Stop button: red filled circle
	btnLeft := rc.Right - stopBtnD - stopBtnMR
	btnTop  := (rc.Bottom - stopBtnD) / 2
	procSaveDC.Call(hdc)
	redBrush2, _, _ := procCreateSolidBrush.Call(colorRed)
	nullPen2, _, _ := procGetStockObject.Call(nullPenObj)
	procSelectObject.Call(hdc, redBrush2)
	procSelectObject.Call(hdc, nullPen2)
	procEllipse.Call(hdc,
		uintptr(btnLeft), uintptr(btnTop),
		uintptr(btnLeft+stopBtnD), uintptr(btnTop+stopBtnD))
	procRestoreDC.Call(hdc, ^uintptr(0))
	procDeleteObject.Call(redBrush2)

	// Stop icon: white square inside the circle
	const squareSz = int32(10)
	sqLeft := btnLeft + (stopBtnD-squareSz)/2
	sqTop  := btnTop  + (stopBtnD-squareSz)/2
	squareRC := winRect{Left: sqLeft, Top: sqTop, Right: sqLeft + squareSz, Bottom: sqTop + squareSz}
	whiteBrush, _, _ := procCreateSolidBrush.Call(colorWhite)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&squareRC)), whiteBrush)
	procDeleteObject.Call(whiteBrush)
}
```

- [ ] **Step 3: Write paintConfirmation**

Append to `cmd/desktop/overlay.go`:

```go
func (o *OverlayWindow) paintConfirmation(hdc uintptr, rc winRect) {
	procSetTextColor.Call(hdc, colorWhite)
	guiFont, _, _ := procGetStockObject.Call(defaultGuiFont)
	procSelectObject.Call(hdc, guiFont)

	// "Parar gravação?" label
	question, _ := syscall.UTF16PtrFromString("Parar gravação?")
	questionRC := winRect{Left: 12, Top: rc.Top, Right: 158, Bottom: rc.Bottom}
	const dtSingleline = 0x0020
	const dtVcenter    = 0x0004
	const dtCenter     = 0x0001
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(question)), ^uintptr(0),
		uintptr(unsafe.Pointer(&questionRC)), dtSingleline|dtVcenter)

	btnH   := int32(24)
	btnW   := int32(40)
	btnTop := (rc.Bottom - btnH) / 2

	// "Sim" button — red rounded rect
	simLeft := int32(162)
	procSaveDC.Call(hdc)
	redBrush, _, _ := procCreateSolidBrush.Call(colorRed)
	nullPen, _, _ := procGetStockObject.Call(nullPenObj)
	procSelectObject.Call(hdc, redBrush)
	procSelectObject.Call(hdc, nullPen)
	procRoundRect.Call(hdc,
		uintptr(simLeft), uintptr(btnTop),
		uintptr(simLeft+btnW), uintptr(btnTop+btnH),
		6, 6)
	procRestoreDC.Call(hdc, ^uintptr(0))
	procDeleteObject.Call(redBrush)

	simText, _ := syscall.UTF16PtrFromString("Sim")
	simRC := winRect{Left: simLeft, Top: btnTop, Right: simLeft + btnW, Bottom: btnTop + btnH}
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(simText)), ^uintptr(0),
		uintptr(unsafe.Pointer(&simRC)), dtSingleline|dtVcenter|dtCenter)

	// "Não" button — border only
	naoLeft := simLeft + btnW + 6
	procSaveDC.Call(hdc)
	borderPen, _, _ := procCreatePen.Call(psSolid, 1, colorBorder)
	nullBrush, _, _ := procGetStockObject.Call(nullBrushObj)
	procSelectObject.Call(hdc, borderPen)
	procSelectObject.Call(hdc, nullBrush)
	procRoundRect.Call(hdc,
		uintptr(naoLeft), uintptr(btnTop),
		uintptr(naoLeft+btnW), uintptr(btnTop+btnH),
		6, 6)
	procRestoreDC.Call(hdc, ^uintptr(0))
	procDeleteObject.Call(borderPen)

	naoText, _ := syscall.UTF16PtrFromString("Não")
	naoRC := winRect{Left: naoLeft, Top: btnTop, Right: naoLeft + btnW, Bottom: btnTop + btnH}
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(naoText)), ^uintptr(0),
		uintptr(unsafe.Pointer(&naoRC)), dtSingleline|dtVcenter|dtCenter)
}
```

- [ ] **Step 4: Build to verify**

```bash
cd cmd/desktop && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/desktop/overlay.go
git commit -m "feat: add overlay GDI painting for recording and confirmation states"
```

---

### Task 3 — Timer goroutine

**Files:**
- Modify: `cmd/desktop/overlay.go`

Replace the stub `timerLoop` with a real implementation that ticks every 500ms, pulses the dot, increments the elapsed counter every second, and triggers a repaint.

- [ ] **Step 1: Replace timerLoop stub**

Remove `func (o *OverlayWindow) timerLoop(stopCh <-chan struct{}) {}` and replace with:

```go
func (o *OverlayWindow) timerLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	ticks := 0
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			ticks++
			o.mu.Lock()
			o.dotOn = !o.dotOn
			o.mu.Unlock()
			if ticks%2 == 0 { // every second
				atomic.AddInt64(&o.elapsed, 1)
			}
			if o.hwnd != 0 {
				procInvalidateRect.Call(o.hwnd, 0, 0)
			}
		}
	}
}
```

- [ ] **Step 2: Build and verify**

```bash
cd cmd/desktop && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/desktop/overlay.go
git commit -m "feat: add overlay timer goroutine with dot pulse and elapsed counter"
```

---

### Task 4 — Input handling: WM_NCHITTEST, stop button, confirmation buttons

**Files:**
- Modify: `cmd/desktop/overlay.go`

Replace the `onNcHitTest` and `onLButtonDown` stubs with real hit-testing logic.

- [ ] **Step 1: Replace onNcHitTest stub**

Remove `func (o *OverlayWindow) onNcHitTest(hwnd, lparam uintptr) uintptr { return htCaption }` and replace with:

```go
func (o *OverlayWindow) onNcHitTest(hwnd, lparam uintptr) uintptr {
	o.mu.Lock()
	confirming := o.confirming
	o.mu.Unlock()

	// In confirmation state: all clicks are handled via WM_LBUTTONDOWN
	if confirming {
		return htClient
	}

	// Convert screen coords (lParam) to client coords
	screenX := int32(int16(lparam & 0xFFFF))
	screenY := int32(int16((lparam >> 16) & 0xFFFF))
	pt := winPoint{x: screenX, y: screenY}
	procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))

	var rc winRect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))

	// Stop button area (right edge): return HTCLIENT so WM_LBUTTONDOWN fires
	btnLeft := rc.Right - stopBtnD - stopBtnMR
	if pt.x >= btnLeft {
		return htClient
	}

	// Pill body: HTCAPTION enables native Win32 drag without extra code
	return htCaption
}
```

- [ ] **Step 2: Replace onLButtonDown stub**

Remove `func (o *OverlayWindow) onLButtonDown(x, y int32) {}` and replace with:

```go
func (o *OverlayWindow) onLButtonDown(clientX, clientY int32) {
	o.mu.Lock()
	confirming := o.confirming
	o.mu.Unlock()

	if !confirming {
		// Clicked stop button → enter confirmation state and widen pill
		o.mu.Lock()
		o.confirming = true
		o.mu.Unlock()

		screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
		x := int32(screenW) - overlayWidthConfirm - 20
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidthConfirm, overlayHeight, overlayCorner, overlayCorner)
		if rgn != 0 {
			procSetWindowRgn.Call(o.hwnd, rgn, 0)
		}
		const swpNoActivate = 0x0010
		procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidthConfirm, overlayHeight, swpNoActivate)
		procInvalidateRect.Call(o.hwnd, 0, 1)
		return
	}

	// Confirmation state: hit-test Sim and Não buttons.
	// Sim: x in [162, 202], Não: x in [208, 248] (matches paintConfirmation layout).
	var rc winRect
	procGetClientRect.Call(o.hwnd, uintptr(unsafe.Pointer(&rc)))
	btnH   := int32(24)
	btnTop := (rc.Bottom - btnH) / 2
	const simLeft = int32(162)
	const btnW    = int32(40)
	const naoLeft = simLeft + btnW + 6

	inSim := clientX >= simLeft && clientX <= simLeft+btnW && clientY >= btnTop && clientY <= btnTop+btnH
	inNao := clientX >= naoLeft && clientX <= naoLeft+btnW && clientY >= btnTop && clientY <= btnTop+btnH

	switch {
	case inSim:
		go o.confirmStop()
	case inNao:
		// Cancel confirmation — shrink pill back to normal
		o.mu.Lock()
		o.confirming = false
		o.mu.Unlock()
		screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
		x := int32(screenW) - overlayWidth - 20
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidth, overlayHeight, overlayCorner, overlayCorner)
		if rgn != 0 {
			procSetWindowRgn.Call(o.hwnd, rgn, 0)
		}
		const swpNoActivate = 0x0010
		procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidth, overlayHeight, swpNoActivate)
		procInvalidateRect.Call(o.hwnd, 0, 1)
	}
}
```

- [ ] **Step 3: Build and verify**

```bash
cd cmd/desktop && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/desktop/overlay.go
git commit -m "feat: add overlay input handling for stop button and confirmation"
```

---

### Task 5 — Stop action: HTTP call, wailsruntime.Show, Hide

**Files:**
- Modify: `cmd/desktop/overlay.go`

- [ ] **Step 1: Write confirmStop**

Append to `cmd/desktop/overlay.go`:

```go
func (o *OverlayWindow) confirmStop() {
	o.mu.Lock()
	meetingID := o.meetingID
	port := o.port
	ctx := o.ctx
	o.mu.Unlock()

	url := fmt.Sprintf("http://localhost:%d/api/meetings/%s/stop", port, meetingID)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", nil)
	if err != nil || (resp != nil && resp.StatusCode >= 300) {
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("overlay: stop failed: %v", err)
		// Return to recording state on error
		o.mu.Lock()
		o.confirming = false
		o.mu.Unlock()
		screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
		x := int32(screenW) - overlayWidth - 20
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidth, overlayHeight, overlayCorner, overlayCorner)
		if rgn != 0 {
			procSetWindowRgn.Call(o.hwnd, rgn, 0)
		}
		const swpNoActivate = 0x0010
		procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidth, overlayHeight, swpNoActivate)
		procInvalidateRect.Call(o.hwnd, 0, 1)
		return
	}
	resp.Body.Close()

	// Success: bring main window to front; Hide is called when orchestrator
	// emits "transcribing" status (which fires o.Hide() via app.go notify).
	if ctx != nil && isWailsContext(ctx) {
		wailsruntime.Show(ctx)
		wailsruntime.WindowUnminimise(ctx)
	}
}
```

- [ ] **Step 2: Build and verify**

```bash
cd cmd/desktop && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/desktop/overlay.go
git commit -m "feat: add overlay confirmStop with HTTP stop call and window bring-to-front"
```

---

### Task 6 — Wire into TrayManager and app.go

**Files:**
- Modify: `cmd/desktop/tray.go`
- Modify: `cmd/desktop/app.go`

- [ ] **Step 1: Add overlay field to TrayManager in tray.go**

In `cmd/desktop/tray.go`, add `overlay *OverlayWindow` to the `TrayManager` struct (after `hotkeyUpdateCh`):

```go
type TrayManager struct {
	ctx            context.Context
	orch           *services.Orchestrator
	meetingRepo    *repository.MeetingRepository
	meetingSvc     *services.MeetingService
	settingsRepo   *repository.SettingsRepository
	app            *App
	hwnd           uintptr
	hIcon          uintptr
	running        atomic.Bool
	isRecording    bool
	hotkeyMods     uint32
	hotkeyVK       uint32
	hotkeyUpdateCh chan string
	overlay        *OverlayWindow
}
```

- [ ] **Step 2: Check status strings emitted by the orchestrator**

Run this to confirm the exact status values used in `SetNotifyFn`:

```bash
grep -n "StatusRecording\|StatusTranscribing\|StatusCompleted\|StatusFailed\|string(status)" internal/services/orchestrator.go | head -20
```

The models define: `"recording"`, `"transcribing"`, `"processing"`, `"completed"`, `"failed"`. The overlay shows during `"recording"` and hides on anything else after that.

- [ ] **Step 3: Update OnStartup in app.go to create overlay and extend notify function**

In `cmd/desktop/app.go`, in `OnStartup`, before the `NewTrayManager` call, create the overlay:

```go
overlay := NewOverlayWindow()
```

Then replace the existing `orch.SetNotifyFn(...)` block with:

```go
orch.SetNotifyFn(func(meetingID, status string) {
    if isWailsContext(ctx) {
        type payload struct {
            MeetingID string `json:"meeting_id"`
            Status    string `json:"status"`
        }
        wailsruntime.EventsEmit(ctx, "pipeline:status", payload{MeetingID: meetingID, Status: status})
    }
    switch status {
    case "recording":
        if overlay != nil {
            overlay.Show(ctx, a.port, meetingID)
        }
        if a.tray != nil {
            a.tray.UpdateState(true)
        }
    case "transcribing", "processing", "completed", "failed":
        if overlay != nil {
            overlay.Hide()
        }
        if a.tray != nil {
            a.tray.UpdateState(false)
        }
    }
})
```

After `NewTrayManager`, assign the overlay:

```go
a.tray = NewTrayManager(a, orch, meetingRepo, meetingSvc, settingsRepo)
a.tray.overlay = overlay
```

- [ ] **Step 4: Build and verify**

```bash
cd cmd/desktop && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 5: Run backend tests**

```bash
go test ./internal/... 2>&1
```

Expected: all pass (no backend code was changed).

- [ ] **Step 6: Manual smoke test**

Run `wails dev` from `cmd/desktop`. Start a recording — the overlay pill should appear in the top-right corner. Verify:
- Pill appears with pulsing red dot and "Gravando" label
- Timer increments every second
- Pill can be dragged by clicking and holding the body (not the stop button)
- Clicking the stop button shows the confirmation state with widened pill
- Clicking "Não" returns to recording state
- Clicking "Sim" calls stop, brings main window to front, and hides the overlay

- [ ] **Step 7: Commit**

```bash
git add cmd/desktop/tray.go cmd/desktop/app.go
git commit -m "feat: wire OverlayWindow into TrayManager and orchestrator notify"
```

---

## Verification

```bash
# Go build
cd cmd/desktop && go build ./...

# Backend tests (unchanged but verify)
go test ./internal/...
```

Manual verification checklist:
- Overlay appears automatically when recording starts (via hotkey or frontend modal)
- Overlay hides when recording stops (via overlay Sim button, tray menu, or frontend stop)
- Dragging the pill body repositions the window
- Confirmation state widens the pill and shows Sim/Não buttons
- Stop button from overlay brings main window to front
- App works normally without overlay (if window creation fails, log is printed and app continues)
