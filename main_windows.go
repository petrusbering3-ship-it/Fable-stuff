//go:build windows

// Fable Buddy - Win32 shell: a transparent, always-on-top layered window
// drawn with per-pixel alpha. Pure Go, no cgo, no dependencies.
package main

import (
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassEx     = user32.NewProc("RegisterClassExW")
	pCreateWindowEx      = user32.NewProc("CreateWindowExW")
	pDefWindowProc       = user32.NewProc("DefWindowProcW")
	pGetMessage          = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessage     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pSetTimer            = user32.NewProc("SetTimer")
	pUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	pGetDC               = user32.NewProc("GetDC")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pSystemParametersInf = user32.NewProc("SystemParametersInfoW")
	pGetLastInputInfo    = user32.NewProc("GetLastInputInfo")
	pSetCapture          = user32.NewProc("SetCapture")
	pReleaseCapture      = user32.NewProc("ReleaseCapture")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenu          = user32.NewProc("AppendMenuW")
	pTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	pDestroyMenu         = user32.NewProc("DestroyMenu")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pLoadCursor          = user32.NewProc("LoadCursorW")
	pShowWindow          = user32.NewProc("ShowWindow")
	pSetWindowPos        = user32.NewProc("SetWindowPos")
	pSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	pDestroyWindow       = user32.NewProc("DestroyWindow")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pGetDeviceCaps      = gdi32.NewProc("GetDeviceCaps")

	pGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	pCreateMutex     = kernel32.NewProc("CreateMutexW")
	pGetTickCount    = kernel32.NewProc("GetTickCount")

	pRegCreateKeyEx  = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueEx   = advapi32.NewProc("RegSetValueExW")
	pRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
	pRegDeleteValue  = advapi32.NewProc("RegDeleteValueW")
	pRegCloseKey     = advapi32.NewProc("RegCloseKey")
)

type point struct{ X, Y int32 }
type rect struct{ L, T, R, B int32 }

type wndClassEx struct {
	Size                         uint32
	Style                        uint32
	WndProc                      uintptr
	ClsExtra, WndExtra           int32
	Instance, Icon, Cursor, Back uintptr
	MenuName, ClassName          *uint16
	IconSm                       uintptr
}

type msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
}

type bitmapInfo struct {
	Size          uint32
	Width, Height int32
	Planes, Bits  uint16
	Compression   uint32
	SizeImage     uint32
	XPPM, YPPM    int32
	ClrUsed       uint32
	ClrImportant  uint32
	Colors        [4]byte
}

const (
	wmDestroy     = 0x0002
	wmTimer       = 0x0113
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205

	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	appKey  = `Software\FableBuddy`
	appName = "FableBuddy"

	idQuip, idJump, idSide, idStartup, idQuit = 1, 2, 3, 4, 5
)

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

// ---- registry helpers (HKEY_CURRENT_USER) ----

func regOpen(path string) (h uintptr, ok bool) {
	r, _, _ := pRegCreateKeyEx.Call(0x80000001, uintptr(unsafe.Pointer(u16(path))), 0, 0, 0,
		0x2001F, 0, uintptr(unsafe.Pointer(&h)), 0)
	return h, r == 0
}

func regSet(path, name, val string) {
	if h, ok := regOpen(path); ok {
		d, _ := syscall.UTF16FromString(val)
		pRegSetValueEx.Call(h, uintptr(unsafe.Pointer(u16(name))), 0, 1,
			uintptr(unsafe.Pointer(&d[0])), uintptr(len(d)*2))
		pRegCloseKey.Call(h)
	}
}

func regGet(path, name string) (string, bool) {
	h, ok := regOpen(path)
	if !ok {
		return "", false
	}
	defer pRegCloseKey.Call(h)
	var buf [1024]uint16
	n := uint32(len(buf) * 2)
	r, _, _ := pRegQueryValueEx.Call(h, uintptr(unsafe.Pointer(u16(name))), 0, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf[:]), true
}

func regDel(path, name string) {
	if h, ok := regOpen(path); ok {
		pRegDeleteValue.Call(h, uintptr(unsafe.Pointer(u16(name))))
		pRegCloseKey.Call(h)
	}
}

func startupEnabled() bool { _, ok := regGet(runKey, appName); return ok }

func setStartup(on bool) {
	if on {
		if exe, err := os.Executable(); err == nil {
			regSet(runKey, appName, `"`+exe+`"`)
		}
		regDel(appKey, "NoStartup")
	} else {
		regDel(runKey, appName)
		regSet(appKey, "NoStartup", "1")
	}
}

// ---- app ----

var (
	hwnd, screenDC, memDC uintptr
	buddy                 *Buddy
	canvas                *Canvas
	last                  time.Time
	frame                 int
)

func workArea() (float64, float64, float64, float64) {
	var r rect
	pSystemParametersInf.Call(0x30, 0, uintptr(unsafe.Pointer(&r)), 0)
	return float64(r.L), float64(r.T), float64(r.R), float64(r.B)
}

func cursor() (float64, float64) {
	var p point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return float64(p.X), float64(p.Y)
}

func userIdle() float64 {
	li := struct{ Size, Time uint32 }{Size: 8}
	pGetLastInputInfo.Call(uintptr(unsafe.Pointer(&li)))
	now, _, _ := pGetTickCount.Call()
	return float64(uint32(now)-li.Time) / 1000
}

func tick() {
	now := time.Now()
	dt := now.Sub(last).Seconds()
	last = now
	if frame++; frame%60 == 0 {
		buddy.SetArea(workArea())
		// stay above other windows without stealing focus
		pSetWindowPos.Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002|0x0010)
	}
	cx, cy := cursor()
	buddy.Update(dt, cx, cy, userIdle())
	buddy.Draw(canvas)
	x, y := buddy.WinPos()
	dst := point{int32(x), int32(y)}
	size := point{int32(canvas.W), int32(canvas.H)}
	var src point
	blend := [4]byte{0, 0, 255, 1} // AC_SRC_OVER, 0, alpha 255, AC_SRC_ALPHA
	pUpdateLayeredWindow.Call(hwnd, screenDC, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)),
		memDC, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), 2)
}

func menu() {
	m, _, _ := pCreatePopupMenu.Call()
	add := func(id uintptr, text string, checked bool) {
		flags := uintptr(0)
		if checked {
			flags = 8
		}
		pAppendMenu.Call(m, flags, id, uintptr(unsafe.Pointer(u16(text))))
	}
	add(idQuip, "Say something", false)
	add(idJump, "Jump!", false)
	pAppendMenu.Call(m, 0x800, 0, 0)
	if buddy.HomeRight {
		add(idSide, "Move to left corner", false)
	} else {
		add(idSide, "Move to right corner", false)
	}
	add(idStartup, "Start with Windows", startupEnabled())
	pAppendMenu.Call(m, 0x800, 0, 0)
	add(idQuit, "Quit", false)
	var p point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	pSetForegroundWindow.Call(hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(m, 0x0100|0x0002, uintptr(p.X), uintptr(p.Y), 0, hwnd, 0)
	pDestroyMenu.Call(m)
	switch cmd {
	case idQuip:
		buddy.Quip()
	case idJump:
		buddy.Hop(650)
	case idSide:
		buddy.SetHome(!buddy.HomeRight)
		if buddy.HomeRight {
			regDel(appKey, "Left")
		} else {
			regSet(appKey, "Left", "1")
		}
	case idStartup:
		setStartup(!startupEnabled())
	case idQuit:
		pDestroyWindow.Call(hwnd)
	}
}

func wndProc(h uintptr, m uint32, wp, lp uintptr) uintptr {
	switch m {
	case wmTimer:
		tick()
		return 0
	case wmLButtonDown:
		pSetCapture.Call(h)
		buddy.Down(cursor())
		return 0
	case wmLButtonUp:
		pReleaseCapture.Call()
		buddy.Up()
		return 0
	case wmRButtonUp:
		menu()
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProc.Call(h, uintptr(m), wp, lp)
	return r
}

func main() {
	runtime.LockOSThread()
	// single instance
	if _, _, err := pCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(u16("FableBuddySingleInstance")))); err == syscall.Errno(183) {
		return
	}
	// start with Windows by default; keep the path fresh if the exe was moved
	if _, off := regGet(appKey, "NoStartup"); !off {
		setStartup(true)
	}
	_, left := regGet(appKey, "Left")

	pSetProcessDPIAware.Call()
	screenDC, _, _ = pGetDC.Call(0)
	dpi, _, _ := pGetDeviceCaps.Call(screenDC, 88)
	scale := int((dpi + 48) / 96)
	scale = max(1, min(4, scale))

	l, t, r, b := workArea()
	buddy = NewBuddy(scale, l, t, r, b, !left)
	canvas = &Canvas{W: canvasW * scale, H: canvasH * scale}

	memDC, _, _ = pCreateCompatibleDC.Call(screenDC)
	bi := bitmapInfo{Size: 40, Width: int32(canvas.W), Height: -int32(canvas.H), Planes: 1, Bits: 32}
	var bits unsafe.Pointer
	dib, _, _ := pCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 || bits == nil {
		return
	}
	pSelectObject.Call(memDC, dib)
	canvas.Pix = unsafe.Slice((*byte)(bits), canvas.W*canvas.H*4)

	inst, _, _ := pGetModuleHandle.Call(0)
	hand, _, _ := pLoadCursor.Call(0, 32649)
	class := u16("FableBuddyWindow")
	wc := wndClassEx{WndProc: syscall.NewCallback(wndProc), Instance: inst, Cursor: hand, ClassName: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	x, y := buddy.WinPos()
	// WS_EX_LAYERED | WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE, WS_POPUP
	hwnd, _, _ = pCreateWindowEx.Call(0x00080000|0x8|0x80|0x08000000, uintptr(unsafe.Pointer(class)),
		uintptr(unsafe.Pointer(u16("Fable Buddy"))), 0x80000000,
		uintptr(x), uintptr(y), uintptr(canvas.W), uintptr(canvas.H), 0, 0, inst, 0)
	if hwnd == 0 {
		return
	}
	last = time.Now()
	tick()
	pShowWindow.Call(hwnd, 4) // SW_SHOWNOACTIVATE
	pSetTimer.Call(hwnd, 1, 15, 0)

	var m msg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}
