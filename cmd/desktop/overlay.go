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

	elapsed int64 // atomic; seconds elapsed
}

var globalOverlay *OverlayWindow
var overlayWndProcCb = syscall.NewCallback(overlayWndProcImpl)

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

// Stubs — implemented in later tasks
func (o *OverlayWindow) onPaint()                                  {}
func (o *OverlayWindow) onNcHitTest(hwnd, lparam uintptr) uintptr { return htCaption }
func (o *OverlayWindow) onLButtonDown(x, y int32)                 {}
func (o *OverlayWindow) timerLoop(stopCh <-chan struct{})          {}
func (o *OverlayWindow) confirmStop()                              {}

// ---------------------------------------------------------------------------
// Silence unused-import errors for imports needed by later tasks
// ---------------------------------------------------------------------------

var (
	_ = fmt.Sprintf
	_ = http.DefaultClient
	_ = time.Second
	_ = wailsruntime.Show
)
