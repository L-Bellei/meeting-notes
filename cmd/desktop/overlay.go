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
	wsExTopmost    = 0x00000008
	wsExLayered    = 0x00080000
	wsExToolwindow = 0x00000080
	wsPopup        = 0x80000000

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

	transparentBk  = 1  // TRANSPARENT for SetBkMode
	psSolid        = 0  // pen style PS_SOLID
	nullBrushObj   = 5  // GetStockObject(NULL_BRUSH)
	nullPenObj     = 8  // GetStockObject(NULL_PEN)
	defaultGuiFont = 17 // GetStockObject(DEFAULT_GUI_FONT)

	// Pill geometry
	overlayWidth        = 220
	overlayHeight       = 44
	overlayWidthConfirm = 260
	overlayCorner       = 24 // RoundRect corner ellipse w/h

	// Stop button: 28px diameter, 8px margin from right edge
	stopBtnD  = 28
	stopBtnMR = 8

	swpNoActivate = 0x0010 // SetWindowPos flag: don't steal focus
)

// GDI COLORREF format: 0x00BBGGRR
const (
	colorBg     = 0x00111111 // dark background
	colorRed    = 0x000000FF // #FF0000 red in BGR
	colorWhite  = 0x00FFFFFF
	colorBorder = 0x001F1F1F // subtle border
)

// ---------------------------------------------------------------------------
// GDI32 procs and additional user32 procs
// ---------------------------------------------------------------------------

var (
	gdi32 = syscall.NewLazyDLL("gdi32.dll")

	// user32 procs not in tray.go
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
	procSetWindowRgn               = user32.NewProc("SetWindowRgn")

	// gdi32 procs
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

	elapsed  int64 // atomic; seconds elapsed
	stopping int32 // atomic; 1 while confirmStop is in-flight
}

var globalOverlay *OverlayWindow
var overlayWndProcCb = syscall.NewCallback(overlayWndProcImpl)

func NewOverlayWindow() *OverlayWindow {
	o := &OverlayWindow{}
	globalOverlay = o

	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

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
			close(ready)
			return
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
			close(ready)
			return
		}
		o.hwnd = hwnd
		procSetLayeredWindowAttributes.Call(hwnd, 0, 220, lwaAlpha)
		close(ready)

		var msg winMsg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), o.hwnd, 0, 0)
			if ret == 0 || ret == ^uintptr(0) {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()

	<-ready
	return o
}

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
		ret, _, _ := procSetWindowRgn.Call(o.hwnd, rgn, 1)
		if ret == 0 {
			procDeleteObject.Call(rgn)
		}
	}

	// Position: 20px from top-right of primary monitor
	screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
	x := int32(screenW) - overlayWidth - 20
	procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidth, overlayHeight, swpNoActivate)
	procShowWindow.Call(o.hwnd, swShow)

	go o.timerLoop(stopCh)
}

// stopTimer halts the active timer goroutine. Pure (no Win32), so it is safe
// even before the window exists.
func (o *OverlayWindow) stopTimer() {
	o.mu.Lock()
	if o.stopCh != nil {
		close(o.stopCh)
		o.stopCh = nil
	}
	o.mu.Unlock()
}

func (o *OverlayWindow) Hide() {
	o.stopTimer()
	if o.hwnd == 0 {
		return
	}
	procShowWindow.Call(o.hwnd, swHide)
}

// HideIfMeeting hides the overlay only when meetingID matches the recording it is
// currently showing. A pipeline of a previous meeting can emit a terminal status
// (transcribing/processing/completed/failed) after a newer recording has already
// taken over the overlay; without this guard that stale event would hide the new
// recording's overlay.
func (o *OverlayWindow) HideIfMeeting(meetingID string) {
	o.mu.Lock()
	match := o.meetingID == meetingID
	o.mu.Unlock()
	if match {
		o.Hide()
	}
}

func (o *OverlayWindow) Destroy() {
	if o.hwnd == 0 {
		return
	}
	o.Hide()
	procDestroyWindow.Call(o.hwnd)
}

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

func (o *OverlayWindow) paintRecording(hdc uintptr, rc winRect, elapsed int64, dotOn bool) {
	const dotSize = int32(8)
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
	const dtSingleline = uintptr(0x0020)
	const dtVcenter    = uintptr(0x0004)
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
	const dtRight = uintptr(0x0002)
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

	// Stop icon: white square inside the button circle
	const squareSz = int32(10)
	sqLeft := btnLeft + (stopBtnD-squareSz)/2
	sqTop  := btnTop  + (stopBtnD-squareSz)/2
	squareRC := winRect{Left: sqLeft, Top: sqTop, Right: sqLeft + squareSz, Bottom: sqTop + squareSz}
	whiteBrush, _, _ := procCreateSolidBrush.Call(colorWhite)
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&squareRC)), whiteBrush)
	procDeleteObject.Call(whiteBrush)
}

func (o *OverlayWindow) paintConfirmation(hdc uintptr, rc winRect) {
	procSetTextColor.Call(hdc, colorWhite)
	guiFont, _, _ := procGetStockObject.Call(defaultGuiFont)
	procSelectObject.Call(hdc, guiFont)

	// "Parar gravação?" label
	question, _ := syscall.UTF16PtrFromString("Parar gravação?")
	questionRC := winRect{Left: 12, Top: rc.Top, Right: 158, Bottom: rc.Bottom}
	const dtSingleline = uintptr(0x0020)
	const dtVcenter    = uintptr(0x0004)
	const dtCenter     = uintptr(0x0001)
	procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(question)), ^uintptr(0),
		uintptr(unsafe.Pointer(&questionRC)), dtSingleline|dtVcenter)

	btnH   := int32(24)
	btnW   := int32(40)
	btnTop := (rc.Bottom - btnH) / 2

	// "Sim" button — red rounded rect fill
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

	// "Não" button — border only (no fill)
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

func (o *OverlayWindow) onNcHitTest(hwnd, lparam uintptr) uintptr {
	o.mu.Lock()
	confirming := o.confirming
	o.mu.Unlock()

	// In confirmation state all clicks go to WM_LBUTTONDOWN
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

	// Stop button area (right edge) → HTCLIENT so WM_LBUTTONDOWN fires
	btnLeft := rc.Right - stopBtnD - stopBtnMR
	if pt.x >= btnLeft {
		return htClient
	}

	// Pill body → HTCAPTION enables native Win32 drag
	return htCaption
}

func (o *OverlayWindow) onLButtonDown(clientX, clientY int32) {
	o.mu.Lock()
	confirming := o.confirming
	o.mu.Unlock()

	if !confirming {
		// Stop button clicked → enter confirmation state, widen pill
		o.mu.Lock()
		o.confirming = true
		o.mu.Unlock()

		screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
		x := int32(screenW) - overlayWidthConfirm - 20
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidthConfirm, overlayHeight, overlayCorner, overlayCorner)
		if rgn != 0 {
			ret, _, _ := procSetWindowRgn.Call(o.hwnd, rgn, 1)
			if ret == 0 {
				procDeleteObject.Call(rgn)
			}
		}
		procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidthConfirm, overlayHeight, swpNoActivate)
		procInvalidateRect.Call(o.hwnd, 0, 1)
		return
	}

	// In confirmation state: hit-test Sim and Não buttons
	// Layout matches paintConfirmation: simLeft=162, btnW=40, naoLeft=simLeft+btnW+6=208
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
		if atomic.CompareAndSwapInt32(&o.stopping, 0, 1) {
			go o.confirmStop()
		}
	case inNao:
		// Cancel → return to normal recording state, shrink pill
		o.mu.Lock()
		o.confirming = false
		o.mu.Unlock()
		screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
		x := int32(screenW) - overlayWidth - 20
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidth, overlayHeight, overlayCorner, overlayCorner)
		if rgn != 0 {
			ret, _, _ := procSetWindowRgn.Call(o.hwnd, rgn, 1)
			if ret == 0 {
				procDeleteObject.Call(rgn)
			}
		}
		procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidth, overlayHeight, swpNoActivate)
		procInvalidateRect.Call(o.hwnd, 0, 1)
	}
}

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
func (o *OverlayWindow) confirmStop() {
	defer atomic.StoreInt32(&o.stopping, 0)

	o.mu.Lock()
	meetingID := o.meetingID
	port := o.port
	ctx := o.ctx
	o.mu.Unlock()

	url := fmt.Sprintf("http://localhost:%d/api/meetings/%s/stop", port, meetingID)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		log.Printf("overlay: stop failed: %v", err)
		o.mu.Lock()
		o.confirming = false
		o.mu.Unlock()
		screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
		x := int32(screenW) - overlayWidth - 20
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidth, overlayHeight, overlayCorner, overlayCorner)
		if rgn != 0 {
			ret, _, _ := procSetWindowRgn.Call(o.hwnd, rgn, 1)
			if ret == 0 {
				procDeleteObject.Call(rgn)
			}
		}
		procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidth, overlayHeight, swpNoActivate)
		procInvalidateRect.Call(o.hwnd, 0, 1)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("overlay: stop failed: status=%d", resp.StatusCode)
		o.mu.Lock()
		o.confirming = false
		o.mu.Unlock()
		screenW, _, _ := procGetSystemMetrics.Call(smCxscreen)
		x := int32(screenW) - overlayWidth - 20
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, overlayWidth, overlayHeight, overlayCorner, overlayCorner)
		if rgn != 0 {
			ret, _, _ := procSetWindowRgn.Call(o.hwnd, rgn, 1)
			if ret == 0 {
				procDeleteObject.Call(rgn)
			}
		}
		procSetWindowPos.Call(o.hwnd, 0, uintptr(x), 20, overlayWidth, overlayHeight, swpNoActivate)
		procInvalidateRect.Call(o.hwnd, 0, 1)
		return
	}

	// Success: bring main window to front.
	// The overlay hides when the orchestrator emits "transcribing" status (via app.go notify).
	if ctx != nil && isWailsContext(ctx) {
		wailsruntime.Show(ctx)
		wailsruntime.WindowUnminimise(ctx)
	}
}
