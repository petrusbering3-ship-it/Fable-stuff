//go:build windows

// Fable Buddy - Win32 shell: a transparent, always-on-top layered window
// drawn with per-pixel alpha. Pure Go, no cgo, no dependencies.
package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassEx      = user32.NewProc("RegisterClassExW")
	pCreateWindowEx       = user32.NewProc("CreateWindowExW")
	pDefWindowProc        = user32.NewProc("DefWindowProcW")
	pGetMessage           = user32.NewProc("GetMessageW")
	pTranslateMessage     = user32.NewProc("TranslateMessage")
	pDispatchMessage      = user32.NewProc("DispatchMessageW")
	pPostMessage          = user32.NewProc("PostMessageW")
	pPostQuitMessage      = user32.NewProc("PostQuitMessage")
	pSetTimer             = user32.NewProc("SetTimer")
	pUpdateLayeredWindow  = user32.NewProc("UpdateLayeredWindow")
	pGetDC                = user32.NewProc("GetDC")
	pGetCursorPos         = user32.NewProc("GetCursorPos")
	pGetLastInputInfo     = user32.NewProc("GetLastInputInfo")
	pSetCapture           = user32.NewProc("SetCapture")
	pReleaseCapture       = user32.NewProc("ReleaseCapture")
	pCreatePopupMenu      = user32.NewProc("CreatePopupMenu")
	pAppendMenu           = user32.NewProc("AppendMenuW")
	pTrackPopupMenuEx     = user32.NewProc("TrackPopupMenuEx")
	pDestroyMenu          = user32.NewProc("DestroyMenu")
	pSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	pLoadCursor           = user32.NewProc("LoadCursorW")
	pShowWindow           = user32.NewProc("ShowWindow")
	pSetWindowPos         = user32.NewProc("SetWindowPos")
	pSetProcessDPIAware   = user32.NewProc("SetProcessDPIAware")
	pSetDpiAwarenessCtx   = user32.NewProc("SetProcessDpiAwarenessContext")
	pDestroyWindow        = user32.NewProc("DestroyWindow")
	pGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	pGetShellWindow       = user32.NewProc("GetShellWindow")
	pGetWindowRect        = user32.NewProc("GetWindowRect")
	pGetClassName         = user32.NewProc("GetClassNameW")
	pEnumDisplayMonitors  = user32.NewProc("EnumDisplayMonitors")
	pGetMonitorInfo       = user32.NewProc("GetMonitorInfoW")
	pMonitorFromWindow    = user32.NewProc("MonitorFromWindow")
	pSystemParametersInfo = user32.NewProc("SystemParametersInfoW")

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

type monitorInfo struct {
	Size          uint32
	Monitor, Work rect
	Flags         uint32
}

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

	idQuip, idJump, idSide, idStartup, idQuit, idScreen = 1, 2, 3, 4, 5, 100
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

// ---- screens ----

var (
	found  []Area
	enumCB = syscall.NewCallback(func(hmon, hdc, lprc, lp uintptr) uintptr {
		mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if r, _, _ := pGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi))); r != 0 {
			w := mi.Work
			found = append(found, Area{float64(w.L), float64(w.T), float64(w.R), float64(w.B), mi.Flags&1 != 0})
		}
		return 1
	})
)

// screens returns the work area of every connected screen.
func screens() []Area {
	found = nil
	pEnumDisplayMonitors.Call(0, 0, enumCB, 0)
	if len(found) == 0 { // fall back to the main screen
		var r rect
		pSystemParametersInfo.Call(0x30, 0, uintptr(unsafe.Pointer(&r)), 0)
		found = []Area{{float64(r.L), float64(r.T), float64(r.R), float64(r.B), true}}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].L < found[j].L })
	return found
}

// ---- app ----

var (
	hwnd, screenDC, memDC uintptr
	buddy                 *Buddy
	canvas                *Canvas
	last                  time.Time
	frame                 int
	hidden, menuOpen      bool
)

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

// fullscreenApp reports whether a game/video covers the whole screen the
// buddy is on, so it can get out of the way. Other screens are unaffected.
func fullscreenApp() bool {
	fg, _, _ := pGetForegroundWindow.Call()
	shell, _, _ := pGetShellWindow.Call()
	if fg == 0 || fg == hwnd || fg == shell {
		return false
	}
	var cls [64]uint16
	pGetClassName.Call(fg, uintptr(unsafe.Pointer(&cls[0])), 64)
	if n := syscall.UTF16ToString(cls[:]); n == "Progman" || n == "WorkerW" {
		return false
	}
	var r rect
	pGetWindowRect.Call(fg, uintptr(unsafe.Pointer(&r)))
	mon, _, _ := pMonitorFromWindow.Call(fg, 2)
	mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := pGetMonitorInfo.Call(mon, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return false
	}
	m := mi.Monitor
	if r.L > m.L || r.T > m.T || r.R < m.R || r.B < m.B {
		return false
	}
	x, y := int32(buddy.X), int32(buddy.Y-10)
	return x >= m.L && x < m.R && y >= m.T && y < m.B
}

func tick() {
	now := time.Now()
	dt := now.Sub(last).Seconds()
	last = now
	if frame++; frame%60 == 0 && !menuOpen {
		buddy.SetAreas(screens())
		if fs := fullscreenApp(); fs != hidden && buddy.State != Drag {
			hidden = fs
			show := uintptr(4) // SW_SHOWNOACTIVATE
			if fs {
				show = 0
			}
			pShowWindow.Call(hwnd, show)
		}
		// stay above other windows without stealing focus (not while the
		// menu is open, or we would cover it)
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
	menuOpen, buddy.Paused = true, true
	defer func() { menuOpen, buddy.Paused = false, false }()
	buddy.Hush() // the speech bubble would sit where the menu opens

	m, _, _ := pCreatePopupMenu.Call()
	add := func(menu, id uintptr, text string, flags uintptr) {
		pAppendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(u16(text))))
	}
	const checked, grayed, popup, sep = 0x8, 0x1 | 0x2, 0x10, 0x800
	add(m, idQuip, "Say something", 0)
	add(m, idJump, "Jump!", 0)
	add(m, 0, "", sep)
	if buddy.HomeRight {
		add(m, idSide, "Move to left corner", 0)
	} else {
		add(m, idSide, "Move to right corner", 0)
	}
	n := len(buddy.Areas)
	if n > 1 {
		sub, _, _ := pCreatePopupMenu.Call()
		for i := range buddy.Areas {
			f := uintptr(0)
			if i == buddy.HomeIdx() {
				f = checked
			}
			add(sub, uintptr(idScreen+i), buddy.ScreenName(i), f)
		}
		add(m, sub, "Live on screen", popup)
	}
	label := "1 screen found"
	if n > 1 {
		label = fmt.Sprintf("%d screens found", n)
	}
	add(m, 0, label, grayed)
	add(m, 0, "", sep)
	sf := uintptr(0)
	if startupEnabled() {
		sf = checked
	}
	add(m, idStartup, "Start with Windows", sf)
	add(m, 0, "", sep)
	add(m, idQuit, "Quit", 0)

	// Open the menu above the buddy's head and tell Windows never to put it
	// over the buddy (it flips below/aside only if there is no room).
	var p point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	x0, y0, x1, y1 := buddy.BodyRect()
	tp := struct {
		Size    uint32
		Exclude rect
	}{20, rect{int32(x0), int32(y0), int32(x1), int32(y1)}}
	const tpmCenter, tpmBottom, tpmVertical, tpmRightButton, tpmReturn = 0x4, 0x20, 0x40, 0x2, 0x100
	pSetForegroundWindow.Call(hwnd)
	cmd, _, _ := pTrackPopupMenuEx.Call(m, tpmCenter|tpmBottom|tpmVertical|tpmRightButton|tpmReturn,
		uintptr(p.X), uintptr(y0), hwnd, uintptr(unsafe.Pointer(&tp)))
	pPostMessage.Call(hwnd, 0, 0, 0)
	pDestroyMenu.Call(m)
	switch {
	case cmd == idQuip:
		buddy.Quip()
	case cmd == idJump:
		buddy.Hop(650)
	case cmd == idSide:
		buddy.SetHome(!buddy.HomeRight)
		if buddy.HomeRight {
			regDel(appKey, "Left")
		} else {
			regSet(appKey, "Left", "1")
		}
	case cmd >= idScreen && cmd < uintptr(idScreen+n):
		i := int(cmd - idScreen)
		if i != buddy.HomeIdx() {
			buddy.SetHomeScreen(i)
			regSet(appKey, "Screen", strconv.Itoa(i))
		}
	case cmd == idStartup:
		setStartup(!startupEnabled())
	case cmd == idQuit:
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
	home := -1
	if v, ok := regGet(appKey, "Screen"); ok {
		if i, err := strconv.Atoi(v); err == nil {
			home = i
		}
	}

	// real pixel coordinates on every screen, even with mixed scaling
	if pSetDpiAwarenessCtx.Find() == nil {
		pSetDpiAwarenessCtx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2
	} else {
		pSetProcessDPIAware.Call()
	}
	screenDC, _, _ = pGetDC.Call(0)
	dpi, _, _ := pGetDeviceCaps.Call(screenDC, 88)
	scale := int((dpi + 48) / 96)
	scale = max(1, min(4, scale))

	buddy = NewBuddy(scale, screens(), home, !left)
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
