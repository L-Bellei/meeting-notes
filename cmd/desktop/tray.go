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
	wmTrayIcon  = 0x0401 // WM_USER + 1; sent by Shell_NotifyIcon to our window
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
	cbSize            uint32
	hWnd              uintptr
	uID               uint32
	uFlags            uint32
	uCallbackMessage  uint32
	hIcon             uintptr
	szTip             [128]uint16
	dwState           uint32
	dwStateMask       uint32
	szInfo            [256]uint16
	uTimeoutOrVersion uint32
	szInfoTitle       [64]uint16
	dwInfoFlags       uint32
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
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
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
// Package-level WndProc callback — syscall.NewCallback must be called once
// ---------------------------------------------------------------------------

var wndProcCallback = syscall.NewCallback(trayWndProcImpl)

// globalTray is set in Start() and read by wndProcCallback.
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
	meetingSvc  *services.MeetingService,
) *TrayManager {
	return &TrayManager{app: app, orch: orch, meetingRepo: meetingRepo, meetingSvc: meetingSvc}
}

// Start registers the hotkey, adds the tray icon, and launches the message loop goroutine.
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
// Message loop — locked to one OS thread for Win32 correctness
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
				ctx := t.ctx
				go func() {
					wailsruntime.Show(ctx)
					wailsruntime.WindowUnminimise(ctx)
				}()
			}
		case wmRbuttonup:
			t.showContextMenu()
		}

	case wmDestroy:
		procPostQuitMessage.Call(0)
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return ret
}

// ---------------------------------------------------------------------------
// toggleRecording — called from hotkey or tray menu (always in a goroutine)
// ---------------------------------------------------------------------------

func (t *TrayManager) toggleRecording() {
	ctx := t.ctx

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
// Context menu (called on message-loop thread from WndProc via wmRbuttonup)
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

	cmdID, _, _ := procTrackPopupMenuEx.Call(
		hMenu, tpmRightButton|tpmReturnCmd,
		uintptr(pt.x), uintptr(pt.y), t.hwnd, 0,
	)

	switch cmdID {
	case menuShow:
		if t.ctx != nil && isWailsContext(t.ctx) {
			ctx := t.ctx
			go func() {
				wailsruntime.Show(ctx)
				wailsruntime.WindowUnminimise(ctx)
			}()
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
