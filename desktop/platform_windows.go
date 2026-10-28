//go:build windows

package main

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var user = syscall.NewLazyDLL("user32.dll")
var gdi = syscall.NewLazyDLL("gdi32.dll")

//go:uintptrescapes
func call(n string, args ...uintptr) uintptr { v, _, _ := user.NewProc(n).Call(args...); return v }

//go:uintptrescapes
func gc(n string, args ...uintptr) uintptr { v, _, _ := gdi.NewProc(n).Call(args...); return v }
func ws(s string) *uint16                  { p, _ := syscall.UTF16PtrFromString(s); return p }
func hideConsole(c *exec.Cmd)              { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func shellCommand(c context.Context, s string) *exec.Cmd {
	return exec.CommandContext(c, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", s)
}

func key(v uint16, up bool) {
	flags := uintptr(0)
	switch v {
	case 0xa3, 0xa5, 0x5b, 0x5c, 0x5d, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2d, 0x2e, 0x2c, 0x6f:
		flags |= 1
	}
	if up {
		flags |= 2
	}
	scan := call("MapVirtualKeyW", uintptr(v), 0)
	call("keybd_event", uintptr(v), scan, flags, 0)
}

func keyCode(s string) (uint16, error) {
	s = strings.ToUpper(s)
	switch s {
	case "CTRL":
		return 0x11, nil
	case "ALT":
		return 0x12, nil
	case "SHIFT":
		return 0x10, nil
	case "WIN":
		return 0x5b, nil
	case "ENTER":
		return 13, nil
	case "TAB":
		return 9, nil
	case "ESC":
		return 27, nil
	case "SPACE":
		return 32, nil
	case "DELETE":
		return 46, nil
	case "BACKSPACE":
		return 8, nil
	case "MEDIA_PLAY_PAUSE":
		return 0xb3, nil
	case "MEDIA_NEXT":
		return 0xb0, nil
	case "MEDIA_PREV":
		return 0xb1, nil
	case "VOLUME_UP":
		return 0xaf, nil
	case "VOLUME_DOWN":
		return 0xae, nil
	case "VOLUME_MUTE":
		return 0xad, nil
	}
	if len(s) == 1 && ((s[0] >= 'A' && s[0] <= 'Z') || (s[0] >= '0' && s[0] <= '9')) {
		return uint16(s[0]), nil
	}
	if strings.HasPrefix(s, "F") {
		n, e := strconv.Atoi(s[1:])
		if e == nil && n >= 1 && n <= 24 {
			return uint16(n + 111), nil
		}
	}
	return 0, fmt.Errorf("unknown key %s", s)
}
func sendKeys(s string) error {
	var keys []uint16
	for _, k := range strings.Split(s, "+") {
		v, e := keyCode(k)
		if e != nil {
			return e
		}
		keys = append(keys, v)
	}
	for _, k := range keys {
		key(k, false)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		key(keys[i], true)
	}
	return nil
}

// INPUT union aligned for Windows amd64. Unicode injection does not depend on the active keyboard layout.
func typeText(s string) error { return typeTextContext(context.Background(), s) }
func typeTextContext(ctx context.Context, s string) error {
	// A single batch preserves the down/up ordering and prevents interleaving.
	if strings.ContainsRune(s, 0) {
		return fmt.Errorf("text contains NUL")
	}
	targetWindow := call("GetForegroundWindow")
	if targetWindow == 0 {
		return fmt.Errorf("немає активного вікна для введення")
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	// Hotkey modifiers must be released before sending text packets.
	deadline := time.Now().Add(time.Second)
	for {
		held := false
		for _, vk := range []uintptr{0x10, 0x11, 0x12, 0x5b, 0x5c} {
			if call("GetAsyncKeyState", vk)&0x8000 != 0 {
				held = true
			}
		}
		if !held {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("відпустіть Ctrl, Alt, Shift та Win перед введенням тексту")
		}
		time.Sleep(10 * time.Millisecond)
	}
	units := utf16.Encode([]rune(s))
	if len(units) == 0 {
		return nil
	}
	for start := 0; start < len(units); start += 128 {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if call("GetForegroundWindow") != targetWindow {
			return fmt.Errorf("активне вікно змінилося під час введення")
		}
		end := start + 128
		if end > len(units) {
			end = len(units)
		}
		inputs := keyboardPackets(units[start:end])
		sent, _, winErr := user.NewProc("SendInput").Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), 40)
		runtime.KeepAlive(inputs)
		if sent != uintptr(len(inputs)) {
			if sent > 0 {
				key(0x10, true)
			}
			return fmt.Errorf("SendInput: передано %d з %d подій; Windows error=%v; foreground=%x. Якщо код 0 — можливе блокування UIPI через різні права процесів", sent, len(inputs), winErr, call("GetForegroundWindow"))
		}
	}
	// Allow the target's message loop to consume VK_PACKET before the next puzzle reads its selection.
	time.Sleep(60 * time.Millisecond)
	return nil
}

type point struct{ X, Y int32 }
type rect struct{ L, T, R, B int32 }
type msg struct {
	H       uintptr
	M       uint32
	W, L    uintptr
	T       uint32
	P       point
	Private uint32
}
type wc struct {
	Size, Style                   uint32
	Proc                          uintptr
	Cls, Wnd                      int32
	Instance, Icon, Cursor, Brush uintptr
	Menu, Name                    *uint16
	Small                         uintptr
}
type paint struct {
	DC              uintptr
	Erase           int32
	Rect            rect
	Restore, Update int32
	Reserved        [32]byte
}
type overlay struct {
	renderControls   []Control
	paintedBoxes     []WidgetBox
	sliderMaximum    float64
	pressedActive    bool
	sliderPending    map[string]*widgetPending
	hoverProgress    []float64
	pressedID        string
	pressedUntil     time.Time
	sliderIndex      int
	sliderDragging   bool
	sliderQueued     bool
	sliderValue      float64
	sliderDue        time.Time
	iconImages       map[string]*overlay
	buttonImages     map[string]*overlay
	labelTick        int64
	materialFallback bool
	background       []byte
	imageW, imageH   int32
	hostID           string
	h                uintptr
	p                Panel
	mode             string
	expanded         bool
	items            []rect
	area             rect
	delay            int
	oldKey           uintptr
	hover            int
	holdKey          uint32
	held             bool
}

var overlays = map[uintptr]*overlay{}
var uiThread uint32
var nativeApp *App
var settingsURL string
var host uintptr
var appIcon uintptr
var className = ws("MacroLeprechaunOverlay")
var windowsCallback = syscall.NewCallback(windowProc)

func platformRefresh() {
	if uiThread != 0 {
		call("PostThreadMessageW", uintptr(uiThread), 0x8001, 0, 0)
	}
}
func color(s string) uintptr {
	n, e := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 32)
	if e != nil {
		n = 0x6dde91
	}
	return uintptr((n&255)<<16 | (n & 0xff00) | (n >> 16))
}

var monitorAreas []rect
var enumMonitorCallback = syscall.NewCallback(func(h, dc, r, lp uintptr) uintptr {
	var info struct {
		Size          uint32
		Monitor, Work rect
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	call("GetMonitorInfoW", h, uintptr(unsafe.Pointer(&info)))
	monitorAreas = append(monitorAreas, info.Work)
	return 1
})

func areaAt(x, y int32, monitor int) rect {
	monitorAreas = nil
	call("EnumDisplayMonitors", 0, 0, enumMonitorCallback, 0)
	if monitor >= 0 && monitor < len(monitorAreas) {
		return monitorAreas[monitor]
	}
	packed := uintptr(uint64(uint32(x)) | uint64(uint32(y))<<32)
	h := call("MonitorFromPoint", packed, 2)
	var info struct {
		Size          uint32
		Monitor, Work rect
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	if call("GetMonitorInfoW", h, uintptr(unsafe.Pointer(&info))) == 0 {
		return rect{0, 0, 1920, 1080}
	}
	return info.Work
}

func show(o *overlay, x, y, w, h int32) {
	call("SetWindowPos", o.h, ^uintptr(0), uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0x50)
	region := gc("CreateRoundRectRgn", 0, 0, uintptr(w+1), uintptr(h+1), 18, 18)
	call("SetWindowRgn", o.h, region, 1)
	call("InvalidateRect", o.h, 0, 1)
}
func position(o *overlay) {
	p := o.p
	a := o.area
	w, h := int32(p.Thickness), int32(90)
	x, y := a.R-w, a.T+(a.B-a.T-h)*int32(p.Position)/100
	if p.Edge == "left" {
		x = a.L
	}
	if p.Edge == "top" || p.Edge == "bottom" {
		w, h = 90, int32(p.Thickness)
		x = a.L + (a.R-a.L-w)*int32(p.Position)/100
		y = a.T
		if p.Edge == "bottom" {
			y = a.B - h
		}
	}
	if o.expanded {
		width, height, _ := widgetLayout(p, int(a.B-a.T)-16)
		w = int32(width)
		h = int32(height)
		if h > a.B-a.T-16 {
			h = a.B - a.T - 16
		}
		if p.Edge == "right" {
			x = a.R - w - 8
		}
		if p.Edge == "left" {
			x = a.L + 8
		}
		if p.Edge == "top" {
			y = a.T + 8
		}
		if p.Edge == "bottom" {
			y = a.B - h - 8
		}
		if x+w > a.R {
			x = a.R - w
		}
		if y+h > a.B {
			y = a.B - h
		}
		if x < a.L {
			x = a.L
		}
		if y < a.T {
			y = a.T
		}
	}
	show(o, x, y, w, h)
}
func rebuild() {
	for h := range overlays {
		call("DestroyWindow", h)
	}
	overlays = map[uintptr]*overlay{}
	for i := 1; i <= 32; i++ {
		call("UnregisterHotKey", host, uintptr(i))
	}
	c := nativeApp.runtimeConfig()
	for _, p := range c.Widgets {
		if p.Enabled && p.Parent == "" && !c.Settings.DisabledWidgets {
			createOverlay(p, "widget")
		}
	}
	for i, p := range c.Floating {
		if p.Enabled && p.Parent == "" && !c.Settings.DisabledFloating {
			o := createOverlay(p, "floating")
			mod, v, e := parseHotkey(p.Hotkey)
			if e == nil && call("RegisterHotKey", host, uintptr(i+1), uintptr(mod), uintptr(v)) == 0 {
				nativeApp.log("Hotkey unavailable: " + p.Hotkey)
			}
			_ = o
		}
	}
	applyNavigation(false)
}
func createOverlay(p Panel, mode string) *overlay {
	o := &overlay{hostID: p.ID, p: p, mode: mode, area: areaAt(0, 0, p.Monitor), hover: -1, sliderIndex: -1}
	style := uintptr(0x08000088)
	if mode == "floating" {
		style |= 0x80000
	}
	o.h = call("CreateWindowExW", style, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(ws(p.Name))), 0x80000000, 0, 0, 320, 240, 0, 0, 0, 0)
	if mode == "floating" {
		loadButtonImages(o)
	}
	overlays[o.h] = o
	if mode == "widget" {
		loadWidgetImage(o)
		loadButtonImages(o)
		applyWidgetMaterial(o)
		position(o)
	}
	return o
}
func floating(index int) {
	c := nativeApp.runtimeConfig()
	if c.Settings.DisabledFloating || index < 0 || index >= len(c.Floating) {
		return
	}
	toggleFloatingWeb(c.Floating[index])
}

func fill(dc uintptr, r rect, c uintptr) {
	b := gc("CreateSolidBrush", c)
	call("FillRect", dc, uintptr(unsafe.Pointer(&r)), b)
	gc("DeleteObject", b)
}
func text(dc uintptr, s string, r rect, c uintptr) {
	gc("SetTextColor", dc, c)
	gc("SetBkMode", dc, 1)
	call("DrawTextW", dc, uintptr(unsafe.Pointer(ws(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), 0x25)
}
func draw(o *overlay) {
	var ps paint
	dc := call("BeginPaint", o.h, uintptr(unsafe.Pointer(&ps)))
	defer call("EndPaint", o.h, uintptr(unsafe.Pointer(&ps)))
	var r rect
	call("GetClientRect", o.h, uintptr(unsafe.Pointer(&r)))
	if o.mode == "widget" && o.expanded && r.R > 0 && r.B > 0 {
		memory, finish := beginWidgetBuffer(dc, o, r)
		defer finish()
		dc = memory
	}
	if o.mode == "floating" && o.expanded && r.R > 0 && r.B > 0 {
		memory, finish := beginRadialBuffer(dc, o, r)
		defer finish()
		dc = memory
	}
	c := nativeApp.runtimeConfig()
	bg, fg, card := uintptr(0x161210), uintptr(0xeef3ed), uintptr(0x2e2822)
	if c.Settings.Theme == "light" {
		bg = 0xf4f6f3
		fg = 0x25291b
		card = 0xe1e8df
	}
	if o.mode == "widget" {
		bg = 0x221d19
		if c.Settings.Theme == "light" {
			bg = 0xffffff
		}
	}
	accent := color(c.Settings.Accent)
	if !o.expanded {
		fill(dc, r, accent)
		return
	}
	if o.mode == "floating" {
		fill(dc, r, 0xff00ff)
	} else {
		if oneOf(o.p.Material, "mica", "acrylic", "glass") && !o.materialFallback {
			bg = 0
		}
		if o.p.Material == "transparent" {
			bg = 0xff00ff
		}
		fill(dc, r, bg)
	}
	if o.mode == "widget" && o.p.Material == "custom" {
		drawWidgetImage(dc, o, r)
	}
	font := gc("CreateFontW", 16, 0, 0, 0, 500, 0, 0, 0, 1, 0, 0, 0, 0, uintptr(unsafe.Pointer(ws("Segoe UI"))))
	old := gc("SelectObject", dc, font)
	defer gc("DeleteObject", font)
	defer gc("SelectObject", dc, old)
	o.items = nil
	if o.mode == "widget" {
		drawWidgetControls(dc, o, r, c, fg, card, accent)

	} else {
		drawRadialControls(dc, o, r, c)
	}

}
func windowProc(h uintptr, m uint32, w, l uintptr) uintptr {
	if m == 0x312 {
		floating(int(w) - 1)
		return 0
	}
	if m == 0x8003 {
		if l == 0x202 {
			openSettings(settingsURL)
		}
		if l == 0x205 {
			menu := call("CreatePopupMenu")
			call("AppendMenuW", menu, 0, 1, uintptr(unsafe.Pointer(ws("MacroLeprechaun"))))
			call("AppendMenuW", menu, 0, 2, uintptr(unsafe.Pointer(ws("Exit"))))
			var p point
			call("GetCursorPos", uintptr(unsafe.Pointer(&p)))
			call("SetForegroundWindow", h)
			chosen := call("TrackPopupMenu", menu, 0x100, uintptr(p.X), uintptr(p.Y), 0, h, 0)
			call("DestroyMenu", menu)
			if chosen == 1 {
				openSettings(settingsURL)
			}
			if chosen == 2 {
				os.Exit(0)
			}
		}
		return 0
	}
	if m == 0x8002 {
		openSettings(settingsURL)
		return 0
	}
	o := overlays[h]
	if o != nil {
		switch m {
		case 0x20:
			if o.mode == "widget" {
				cursor := uintptr(32512)
				if index, _ := widgetHit(o); index >= 0 {
					cursor = 32649
				}
				call("SetCursor", call("LoadCursorW", 0, cursor))
				return 1
			}
		case 0x21:
			return 3
		case 0x14:
			return 1
		case 0xf:
			draw(o)
			return 0
		case 0x200:
			if o.mode == "widget" && o.sliderDragging {
				widgetDrag(o)
				return 0
			}
			if o.mode == "floating" {
				return 0
			}
			if !o.expanded {
				o.expanded = true
				position(o)
				call("SetTimer", h, 1, 60, 0)
			}
			o.delay = 0
			return 0
		case 0x113:
			if o.mode == "widget" {
				tickWidget(o)
			}
			if now := time.Now().Unix(); now != o.labelTick {
				o.labelTick = now
				call("InvalidateRect", h, 0, 1)
			}
			var p point
			call("GetCursorPos", uintptr(unsafe.Pointer(&p)))
			var r rect
			call("GetWindowRect", h, uintptr(unsafe.Pointer(&r)))
			if o.mode == "floating" {
				idx := radialPanelIndex(o.p, float64(p.X-(r.L+r.R)/2), float64(p.Y-(r.T+r.B)/2))
				if o.held && call("GetAsyncKeyState", uintptr(o.holdKey))&0x8000 == 0 {
					o.held = false
					if idx >= 0 {
						activateRadial(o)
						return 0
					}
				}
				if idx != o.hover {
					o.hover = idx
					call("InvalidateRect", h, 0, 1)
				}
			}
			outside := !o.sliderDragging && len(o.sliderPending) == 0 && (p.X < r.L-12 || p.X > r.R+12 || p.Y < r.T-12 || p.Y > r.B+12)
			esc := call("GetAsyncKeyState", 27)&0x8000 != 0
			if outside {
				o.delay++
			} else {
				o.delay = 0
			}
			if esc || (o.mode == "widget" && o.delay > 6) {
				o.expanded = false
				call("KillTimer", h, 1)
				if o.mode == "widget" {
					position(o)
				} else {
					call("ShowWindow", h, 0)
				}
			}
			return 0
		case 0x204:
			nativeControlMenu(o)
			return 0
		case 0x205:
			return 0
		case 0x31e:
			if o.mode == "widget" {
				applyWidgetMaterial(o)
			}
			return 0
		case 0x201:
			if o.mode == "widget" {
				widgetDown(o)
				return 0
			}
		case 0x215:
			if o.mode == "widget" && o.sliderDragging {
				o.sliderDragging = false
				o.sliderQueued = true
			}
			return 0
		case 0x202:
			if o.mode == "widget" {
				widgetUp(o)
				return 0
			}
			if o.mode == "floating" {
				activateRadial(o)
				return 0
			}
			x, y := int32(int16(l&0xffff)), int32(int16((l>>16)&0xffff))
			for i, r := range o.items {
				if x >= r.L && x <= r.R && y >= r.T && y <= r.B {
					b := o.p.Controls[i]
					if o.mode == "floating" {
						o.expanded = false
						call("ShowWindow", h, 0)
					}
					if e := nativeApp.runNative(b.Macro, o); e != nil {
						nativeApp.log(e.Error())
					}
					break
				}
			}
			return 0
		}
	}
	return call("DefWindowProcW", h, uintptr(m), w, l)
}
func platformLoop(a *App, u string) {
	runtime.LockOSThread()
	nativeApp = a
	settingsURL = u
	tid, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId").Call()
	uiThread = uint32(tid)
	call("SetProcessDPIAware")
	instance, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	appIcon = call("LoadImageW", instance, 1, 1, 0, 0, 0x40)
	c := wc{Cursor: call("LoadCursorW", 0, 32512), Icon: appIcon, Small: appIcon, Instance: instance, Size: uint32(unsafe.Sizeof(wc{})), Proc: windowsCallback, Name: className}
	call("RegisterClassExW", uintptr(unsafe.Pointer(&c)))
	host = call("CreateWindowExW", 0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(ws("MacroLeprechaun"))), 0, 0, 0, 0, 0, 0, 0, 0, 0)
	addTray(host)
	defer removeTray(host)
	mouseHook = call("SetWindowsHookExW", 14, mouseCallback, instance, 0)
	if mouseHook == 0 {
		a.log("Radial mouse hook unavailable; circle clicks remain available")
	}
	defer call("UnhookWindowsHookEx", mouseHook)
	rebuild()
	var message msg
	for call("GetMessageW", uintptr(unsafe.Pointer(&message)), 0, 0, 0) > 0 {
		if message.M == 0x8004 {
			applyNavigation(true)
			continue
		}
		if message.M == 0x8001 {
			rebuild()
			continue
		}
		call("TranslateMessage", uintptr(unsafe.Pointer(&message)))
		call("DispatchMessageW", uintptr(unsafe.Pointer(&message)))
	}
}

type notifyIcon struct {
	Size                uint32
	H                   uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Timeout             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                [16]byte
	Balloon             uintptr
}

func trayData(h uintptr) notifyIcon {
	n := notifyIcon{H: h, ID: 1, Flags: 7, Callback: 0x8003, Icon: appIcon}
	n.Size = uint32(unsafe.Sizeof(n))
	copy(n.Tip[:], syscall.StringToUTF16("MacroLeprechaun"))
	return n
}
func addTray(h uintptr) {
	n := trayData(h)
	syscall.NewLazyDLL("shell32.dll").NewProc("Shell_NotifyIconW").Call(0, uintptr(unsafe.Pointer(&n)))
}
func removeTray(h uintptr) {
	n := trayData(h)
	syscall.NewLazyDLL("shell32.dll").NewProc("Shell_NotifyIconW").Call(2, uintptr(unsafe.Pointer(&n)))
}

func circle(dc uintptr, x, y, r int32, fillColor, border uintptr) {
	brush := gc("CreateSolidBrush", fillColor)
	pen := gc("CreatePen", 0, 1, border)
	ob, op := gc("SelectObject", dc, brush), gc("SelectObject", dc, pen)
	gc("Ellipse", dc, uintptr(x-r), uintptr(y-r), uintptr(x+r), uintptr(y+r))
	gc("SelectObject", dc, ob)
	gc("SelectObject", dc, op)
	gc("DeleteObject", brush)
	gc("DeleteObject", pen)
}
func activateRadial(o *overlay) {
	var p point
	call("GetCursorPos", uintptr(unsafe.Pointer(&p)))
	var r rect
	call("GetWindowRect", o.h, uintptr(unsafe.Pointer(&r)))
	i := radialPanelIndex(o.p, float64(p.X-(r.L+r.R)/2), float64(p.Y-(r.T+r.B)/2))
	o.expanded = false
	call("ShowWindow", o.h, 0)
	call("KillTimer", o.h, 1)
	if i >= 0 {
		if e := nativeApp.runNative(o.p.Controls[i].Macro, o); e != nil {
			nativeApp.log(e.Error())
		}
	}
}

var mouseHook uintptr
var swallowRelease bool
var swallowRightRelease bool
var mouseCallback = syscall.NewCallback(func(code int32, w, l uintptr) uintptr {
	if code >= 0 {
		for _, o := range overlays {
			if o.mode == "floating" && o.expanded {
				if w == 0x201 {
					swallowRelease = true
					return 1
				}
				if w == 0x202 {
					activateRadial(o)
					swallowRelease = false
					return 1
				}
				if w == 0x204 || w == 0x205 {
					if w == 0x204 {
						swallowRightRelease = true
						call("PostMessageW", o.h, 0x204, 0, 0)
					}
					return 1
				}
			}
		}
		if w == 0x205 && swallowRightRelease {
			swallowRightRelease = false
			return 1
		}
		if w == 0x202 && swallowRelease {
			swallowRelease = false
			return 1
		}
	}
	return call("CallNextHookEx", mouseHook, uintptr(code), w, l)
})

func platformNavigationRefresh() {
	if uiThread != 0 {
		call("PostThreadMessageW", uintptr(uiThread), 0x8004, 0, 0)
	}
}
func (a *App) runNative(id string, o *overlay) error {
	mode := "widgets"
	if o.mode == "floating" {
		mode = "floating"
	}
	return a.runScoped(id, nil, InputEvent{Kind: "button", Value: "click"}, RunScope{Session: "desktop", Mode: mode, Host: o.hostID, Panel: o.p.ID})
}
func applyNavigation(open bool) {
	refreshFloatingWeb(open)
	c := nativeApp.runtimeConfig()
	for _, o := range overlays {
		if o.mode == "floating" {
			continue
		}
		mode := "widgets"
		if o.mode == "floating" {
			mode = "floating"
		}
		id := nativeApp.activePanel(RunScope{Session: "desktop", Mode: mode, Host: o.hostID, Panel: o.p.ID})
		for _, p := range modePanels(c, mode) {
			if p.ID == id {
				changed := o.p.ID != id
				if changed && o.mode == "widget" {
					o.sliderDragging = false
					o.sliderIndex = -1
					call("ReleaseCapture")
				}
				o.p.ID = p.ID
				o.p.Name = p.Name
				o.p.Controls = p.Controls
				o.renderControls = nil
				o.p.Columns = p.Columns
				o.p.Radial = p.Radial
				o.p.Radius = p.Radius
				o.p.Material = p.Material
				o.p.Background = p.Background
				o.background = nil
				loadWidgetImage(o)
				loadButtonImages(o)
				if o.mode == "widget" {
					applyWidgetMaterial(o)
				}
				call("InvalidateRect", o.h, 0, 1)
				if o.mode == "widget" {
					position(o)
				} else if changed && open {
					var r rect
					call("GetWindowRect", o.h, uintptr(unsafe.Pointer(&r)))
					o.held = false
					o.hover = -1
					o.expanded = true
					show(o, r.L, r.T, r.R-r.L, r.B-r.T)
					call("SetTimer", o.h, 1, 60, 0)
				}
				break
			}
		}
	}
}

func configureCommandCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		killer := exec.Command("taskkill.exe", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		hideConsole(killer)
		if e := killer.Run(); e != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = time.Second
}

func roundedFill(dc uintptr, r rect, c uintptr) {
	brush := gc("CreateSolidBrush", c)
	pen := gc("CreatePen", 0, 1, c)
	oldB := gc("SelectObject", dc, brush)
	oldP := gc("SelectObject", dc, pen)
	gc("RoundRect", dc, uintptr(r.L), uintptr(r.T), uintptr(r.R), uintptr(r.B), 18, 18)
	gc("SelectObject", dc, oldB)
	gc("SelectObject", dc, oldP)
	gc("DeleteObject", brush)
	gc("DeleteObject", pen)
}
func applyWidgetMaterial(o *overlay) {
	if o.mode != "widget" {
		return
	}
	material := o.p.Material
	o.materialFallback = false
	// Reset all mechanisms: a previous material must never leak into the next.
	style := call("GetWindowLongPtrW", o.h, ^uintptr(19))
	call("SetWindowLongPtrW", o.h, ^uintptr(19), style&^0x80000)
	type accentPolicy struct{ State, Flags, Gradient, Animation uint32 }
	type compositionData struct {
		Attribute uint32
		Data      uintptr
		Size      uintptr
	}
	proc := user.NewProc("SetWindowCompositionAttribute")
	accent := func(state, gradient uint32) bool {
		if proc.Find() != nil {
			return false
		}
		policy := accentPolicy{State: state, Flags: 2, Gradient: gradient}
		data := compositionData{Attribute: 19, Data: uintptr(unsafe.Pointer(&policy)), Size: unsafe.Sizeof(policy)}
		result, _, _ := proc.Call(o.h, uintptr(unsafe.Pointer(&data)))
		runtime.KeepAlive(policy)
		runtime.KeepAlive(data)
		return result != 0
	}
	dwm := syscall.NewLazyDLL("dwmapi.dll")
	set := dwm.NewProc("DwmSetWindowAttribute")
	extend := dwm.NewProc("DwmExtendFrameIntoClientArea")
	kind := uint32(1)
	if set.Find() == nil {
		set.Call(o.h, 38, uintptr(unsafe.Pointer(&kind)), 4)
	}
	margins := [4]int32{}
	if extend.Find() == nil {
		extend.Call(o.h, uintptr(unsafe.Pointer(&margins)))
	}
	accent(0, 0)
	if oneOf(material, "", "solid", "custom") {
		return
	}
	if material == "transparent" || material == "tinted" {
		call("SetWindowLongPtrW", o.h, ^uintptr(19), style|0x80000)
		if material == "transparent" {
			call("SetLayeredWindowAttributes", o.h, 0xff00ff, 255, 1)
		} else {
			call("SetLayeredWindowAttributes", o.h, 0, 220, 2)
		}
		return
	}
	margins = [4]int32{-1, -1, -1, -1}
	if extend.Find() == nil {
		extend.Call(o.h, uintptr(unsafe.Pointer(&margins)))
	}
	dark := uint32(1)
	if nativeApp.runtimeConfig().Settings.Theme == "light" {
		dark = 0
	}
	if set.Find() == nil {
		set.Call(o.h, 20, uintptr(unsafe.Pointer(&dark)), 4)
	}
	applied := false
	if material == "mica" || material == "acrylic" {
		kind = 2
		if material == "acrylic" {
			kind = 3
		}
		if set.Find() == nil {
			hr, _, _ := set.Call(o.h, 38, uintptr(unsafe.Pointer(&kind)), 4)
			applied = int32(hr) >= 0
		}
	}
	if !applied {
		state, gradient := uint32(3), uint32(0x55221d19)
		if material == "acrylic" {
			state = 4
			gradient = 0xaa221d19
		}
		if material == "mica" {
			gradient = 0xdd221d19
		}
		applied = accent(state, gradient)
	}
	if !applied {
		o.materialFallback = true
		margins = [4]int32{}
		if extend.Find() == nil {
			extend.Call(o.h, uintptr(unsafe.Pointer(&margins)))
		}
		call("SetWindowLongPtrW", o.h, ^uintptr(19), style|0x80000)
		call("SetLayeredWindowAttributes", o.h, 0, 225, 2)
		nativeApp.log("Матеріал " + material + ": Windows не надає backdrop; застосовано прозорість")
	}
}

func loadWidgetImage(o *overlay) {
	if o.p.Material != "custom" || o.p.Background == "" || filepath.Base(o.p.Background) != o.p.Background {
		return
	}
	file, err := os.Open(filepath.Join(nativeApp.root, "image", o.p.Background))
	if err != nil {
		return
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width > 8192 || config.Height > 8192 || config.Width*config.Height > 16000000 {
		return
	}
	file.Seek(0, 0)
	img, _, err := image.Decode(file)
	if err != nil {
		return
	}
	bounds := img.Bounds()
	o.imageW = int32(bounds.Dx())
	o.imageH = int32(bounds.Dy())
	o.background = make([]byte, int(o.imageW*o.imageH)*4)
	for y := 0; y < int(o.imageH); y++ {
		for x := 0; x < int(o.imageW); x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			i := (y*int(o.imageW) + x) * 4
			o.background[i] = byte(b >> 8)
			o.background[i+1] = byte(g >> 8)
			o.background[i+2] = byte(r >> 8)
		}
	}
}
func drawWidgetImage(dc uintptr, o *overlay, r rect) {
	if len(o.background) == 0 {
		return
	}
	type bitmapInfo struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		Used, Important        uint32
	}
	info := bitmapInfo{Size: 40, Width: o.imageW, Height: -o.imageH, Planes: 1, BitCount: 32}
	gc("SetStretchBltMode", dc, 4)
	gc("StretchDIBits", dc, uintptr(r.L), uintptr(r.T), uintptr(r.R-r.L), uintptr(r.B-r.T), 0, 0, uintptr(o.imageW), uintptr(o.imageH), uintptr(unsafe.Pointer(&o.background[0])), uintptr(unsafe.Pointer(&info)), 0, 0xcc0020)
}

func loadButtonImages(o *overlay) {
	o.buttonImages = map[string]*overlay{}
	o.iconImages = map[string]*overlay{}
	for _, b := range controlLeaves(o.p.Controls) {
		if strings.HasPrefix(b.Icon, "/media/icon/") {
			name := strings.TrimPrefix(b.Icon, "/media/icon/")
			if filepath.Base(name) == name {
				if image, err := rasterIcon(filepath.Join(nativeApp.root, "icon", name), 32); err == nil {
					pixels := make([]byte, len(image.Pix))
					for i := 0; i < len(pixels); i += 4 {
						pixels[i] = image.Pix[i+2]
						pixels[i+1] = image.Pix[i+1]
						pixels[i+2] = image.Pix[i]
						pixels[i+3] = image.Pix[i+3]
					}
					o.iconImages[b.ID] = &overlay{background: pixels, imageW: 32, imageH: 32}
				}
			}
		}
		if strings.HasPrefix(b.Appearance.Image, "/media/image/") {
			name := strings.TrimPrefix(b.Appearance.Image, "/media/image/")
			image := &overlay{p: Panel{Material: "custom", Background: name}}
			loadWidgetImage(image)
			if len(image.background) > 0 {
				o.buttonImages[b.ID] = image
			}
		}
	}
}
func strokeRound(dc uintptr, r rect, c uintptr) {
	pen := gc("CreatePen", 0, 1, c)
	oldP := gc("SelectObject", dc, pen)
	oldB := gc("SelectObject", dc, gc("GetStockObject", 5))
	gc("RoundRect", dc, uintptr(r.L), uintptr(r.T), uintptr(r.R), uintptr(r.B), 18, 18)
	gc("SelectObject", dc, oldB)
	gc("SelectObject", dc, oldP)
	gc("DeleteObject", pen)
}
func drawButtonImage(dc uintptr, image *overlay, r rect, fit string) {
	saved := gc("SaveDC", dc)
	defer gc("RestoreDC", dc, saved)
	region := gc("CreateRoundRectRgn", uintptr(r.L), uintptr(r.T), uintptr(r.R), uintptr(r.B), 18, 18)
	gc("SelectClipRgn", dc, region)
	gc("DeleteObject", region)
	if fit == "contain" || fit == "cover" {
		scale := math.Min(float64(r.R-r.L)/float64(image.imageW), float64(r.B-r.T)/float64(image.imageH))
		if fit == "cover" {
			scale = math.Max(float64(r.R-r.L)/float64(image.imageW), float64(r.B-r.T)/float64(image.imageH))
		}
		w, h := int32(float64(image.imageW)*scale), int32(float64(image.imageH)*scale)
		cx, cy := (r.L+r.R)/2, (r.T+r.B)/2
		r = rect{cx - w/2, cy - h/2, cx - w/2 + w, cy - h/2 + h}
	}
	drawWidgetImage(dc, image, r)
}
func nativeControlMenu(o *overlay) {
	controls := o.p.Controls
	if o.mode == "widget" {
		controls = widgetControls(o)
	}
	var pt point
	call("GetCursorPos", uintptr(unsafe.Pointer(&pt)))
	var window rect
	call("GetWindowRect", o.h, uintptr(unsafe.Pointer(&window)))
	index := -1
	if o.mode == "floating" {
		index = radialPanelIndex(o.p, float64(pt.X-(window.L+window.R)/2), float64(pt.Y-(window.T+window.B)/2))
	} else {
		x, y := pt.X-window.L, pt.Y-window.T
		for i, r := range o.items {
			if x >= r.L && x <= r.R && y >= r.T && y <= r.B {
				index = i
				break
			}
		}
	}
	if index < 0 || index >= len(controls) {
		return
	}
	menu := call("CreatePopupMenu")
	defer call("DestroyMenu", menu)
	call("AppendMenuW", menu, 0, 1, uintptr(unsafe.Pointer(ws("Вигляд кнопки"))))
	call("AppendMenuW", menu, 0, 2, uintptr(unsafe.Pointer(ws("Редагувати макрос"))))
	call("SetForegroundWindow", host)
	chosen := call("TrackPopupMenu", menu, 0x100, uintptr(pt.X), uintptr(pt.Y), 0, host, 0)
	if chosen == 0 {
		return
	}
	mode := "widgets"
	if o.mode == "floating" {
		mode = "floating"
	}
	url := settingsURL + "&mode=" + mode + "&panel=" + o.p.ID + "&control=" + controls[index].ID
	if chosen == 2 {
		url += "&edit=macro"
	}
	openSettings(url)
}

func platformKeyboardLayout() string {
	thread := call("GetWindowThreadProcessId", call("GetForegroundWindow"), 0)
	layout := call("GetKeyboardLayout", thread)
	var name [85]uint16
	if kc("LCIDToLocaleName", layout&0xffff, uintptr(unsafe.Pointer(&name[0])), 85, 0) == 0 {
		return "unavailable"
	}
	return syscall.UTF16ToString(name[:])
}
