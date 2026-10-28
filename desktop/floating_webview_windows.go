//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	edge "github.com/jchv/go-webview2/pkg/edge"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var floatingWeb struct {
	sync.Mutex
	cmd         *exec.Cmd
	host, panel string
	cursor      point
	foreground  uintptr
}

func toggleFloatingWeb(p Panel) {
	floatingWeb.Lock()
	if floatingWeb.cmd != nil {
		cmd := floatingWeb.cmd
		floatingWeb.cmd = nil
		floatingWeb.host = ""
		floatingWeb.Unlock()
		cmd.Process.Kill()
		return
	}
	var cursor point
	call("GetCursorPos", uintptr(unsafe.Pointer(&cursor)))
	foreground := call("GetForegroundWindow")
	floatingWeb.Unlock()
	launchFloatingWeb(p.ID, p.ID, cursor, foreground)
}
func launchFloatingWeb(hostID, panelID string, cursor point, foreground uintptr) {
	floatingWeb.Lock()
	defer floatingWeb.Unlock()
	if floatingWeb.cmd != nil {
		floatingWeb.cmd.Process.Kill()
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	u, _ := url.Parse(settingsURL)
	u.Path = "/floating.html"
	q := u.Query()
	area := areaAt(cursor.X, cursor.Y, -1)
	q.Set("host", hostID)
	q.Set("cursorX", strconv.Itoa(int(cursor.X-area.L)))
	q.Set("cursorY", strconv.Itoa(int(cursor.Y-area.T)))
	q.Set("left", strconv.Itoa(int(area.L)))
	q.Set("top", strconv.Itoa(int(area.T)))
	q.Set("width", strconv.Itoa(int(area.R-area.L)))
	q.Set("height", strconv.Itoa(int(area.B-area.T)))
	q.Set("foreground", strconv.FormatUint(uint64(foreground), 10))
	u.RawQuery = q.Encode()
	cmd := exec.Command(exe, "-floating-ui", u.String(), "-data", nativeApp.root, "-ui-parent", strconv.Itoa(os.Getpid()))
	cmd.Stderr = log.Writer()
	if err = cmd.Start(); err != nil {
		nativeApp.log("Floating WebView2: " + err.Error())
		return
	}
	floatingWeb.cmd = cmd
	log.Printf("Floating launch: pid=%d; panel=%s; bounds=%dx%d", cmd.Process.Pid, panelID, area.R-area.L, area.B-area.T)
	time.AfterFunc(30*time.Second, func() {
		floatingWeb.Lock()
		current := floatingWeb.cmd == cmd
		floatingWeb.Unlock()
		if current && settingsWindowForPID(cmd.Process.Pid) == 0 {
			floatingWeb.Lock()
			if floatingWeb.cmd == cmd {
				floatingWeb.cmd = nil
			}
			floatingWeb.Unlock()
			cmd.Process.Kill()
			log.Print("Floating startup timeout")
			settingsUIError("Плаваюча панель WebView2 не відкрилася. Деталі: data/application.log")
		}
	})
	floatingWeb.host = hostID
	floatingWeb.panel = panelID
	floatingWeb.cursor = cursor
	floatingWeb.foreground = foreground
	go func() {
		err := cmd.Wait()
		floatingWeb.Lock()
		current := floatingWeb.cmd == cmd
		if current {
			floatingWeb.cmd = nil
		}
		floatingWeb.Unlock()
		if err != nil && current {
			nativeApp.log("Floating WebView2: " + err.Error())
			settingsUIError("Не вдалося відкрити плаваючу панель WebView2. Деталі: data/application.log")
		}
	}()
}
func refreshFloatingWeb(open bool) {
	floatingWeb.Lock()
	host, panel, cursor, foreground := floatingWeb.host, floatingWeb.panel, floatingWeb.cursor, floatingWeb.foreground
	cmd := floatingWeb.cmd
	floatingWeb.Unlock()
	if nativeApp.runtimeConfig().Settings.DisabledFloating {
		if cmd != nil {
			floatingWeb.Lock()
			if floatingWeb.cmd == cmd {
				floatingWeb.cmd = nil
				floatingWeb.host = ""
			}
			floatingWeb.Unlock()
			cmd.Process.Kill()
		}
		return
	}
	if !open || host == "" {
		return
	}
	current := nativeApp.activePanel(RunScope{Session: "desktop", Mode: "floating", Host: host, Panel: host})
	if current != "" && current != panel {
		launchFloatingWeb(host, current, cursor, foreground)
	}
}

var floatingBrowser *edge.Chromium
var floatingWindow uintptr
var floatingReady, floatingKeepFocus, floatingHideOnBlur bool
var floatingFade int
var floatingCallback = syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
	switch m {
	case 0x5:
		if floatingBrowser != nil && floatingBrowser.GetController() != nil {
			floatingBrowser.Resize()
		}
		return 0
	case 0x8006:
		if !floatingReady {
			log.Print("Floating renderer startup timeout")
			call("PostMessageW", h, 0x10, 0, 0)
		}
		return 0
	case 0x14:
		var r rect
		call("GetClientRect", h, uintptr(unsafe.Pointer(&r)))
		fill(w, r, 0)
		return 1
	case 0x10:
		call("DestroyWindow", h)
		return 0
	case 0x2:
		call("PostQuitMessage", 0)
		return 0
	case 0x6:
		if floatingReady && !floatingKeepFocus && floatingHideOnBlur && w&0xffff == 0 {
			call("PostMessageW", h, 0x10, 0, 0)
		}
	}
	return call("DefWindowProcW", h, uintptr(m), w, l)
})

func runFloatingWindow(target, directory string, parent int) error {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" {
		return fmt.Errorf("invalid floating URL")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	call("SetProcessDPIAware")
	q := u.Query()
	number := func(k string) int { v, _ := strconv.Atoi(q.Get(k)); return v }
	left, top, width, height := number("left"), number("top"), number("width"), number("height")
	if width < 100 || width > 16384 || height < 100 || height > 16384 {
		return fmt.Errorf("invalid floating window bounds")
	}
	log.Printf("Floating UI initializing: pid=%d; bounds=%dx%d", os.Getpid(), width, height)
	original := point{int32(left + number("cursorX")), int32(top + number("cursorY"))}
	foreground, _ := strconv.ParseUint(q.Get("foreground"), 10, 64)
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	name := ws("MacroLeprechaun.Floating.WebView2")
	class := wc{Size: uint32(unsafe.Sizeof(wc{})), Proc: floatingCallback, Name: name, Instance: instance, Cursor: call("LoadCursorW", 0, 32512)}
	call("RegisterClassExW", uintptr(unsafe.Pointer(&class)))
	floatingWindow = call("CreateWindowExW", 0x00200088, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(ws("MacroLeprechaun · Floating"))), 0x80000000, uintptr(left), uintptr(top), uintptr(width), uintptr(height), 0, 0, instance, 0)
	if floatingWindow == 0 {
		return fmt.Errorf("cannot create floating window")
	}
	margins := [4]int32{-1, -1, -1, -1}
	syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmExtendFrameIntoClientArea").Call(floatingWindow, uintptr(unsafe.Pointer(&margins)))
	// DWM transparent host + controller background alpha=0, no chroma key.
	region := gc("CreateRectRgn", 0, 0, ^uintptr(0), ^uintptr(0))
	blur := struct {
		Flags      uint32
		Enable     int32
		Region     uintptr
		Transition int32
	}{3, 1, region, 0}
	syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmEnableBlurBehindWindow").Call(floatingWindow, uintptr(unsafe.Pointer(&blur)))
	gc("DeleteObject", region)
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disable-extensions --disable-sync --disable-background-networking --no-first-run")
	os.Setenv("WEBVIEW2_DEFAULT_BACKGROUND_COLOR", "00000000")
	browser := edge.NewChromium()
	floatingBrowser = browser
	browser.DataPath = filepath.Join(directory, "setting", "floating-webview2")
	if err := os.MkdirAll(browser.DataPath, 0700); err != nil {
		return fmt.Errorf("floating profile: %w", err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	closeWindow := func(delay int, returnPointer bool) {
		floatingReady = false
		call("SetForegroundWindow", uintptr(foreground))
		if returnPointer {
			call("SetCursorPos", uintptr(original.X), uintptr(original.Y))
		}
		time.AfterFunc(time.Duration(max(0, min(delay, 2000)))*time.Millisecond, func() { call("PostMessageW", floatingWindow, 0x10, 0, 0) })
	}
	browser.MessageCallback = func(raw string) {
		var message struct {
			Type           string `json:"type"`
			Panel          string `json:"panel"`
			Control        string `json:"control"`
			Message        string `json:"message"`
			X, Y           float64
			ViewportWidth  float64 `json:"viewportWidth"`
			ViewportHeight float64 `json:"viewportHeight"`
			KeepInputFocus bool    `json:"keepInputFocus"`
			HideOnFocusOut bool    `json:"hideOnFocusOut"`
			Fade           int     `json:"fade"`
			ReturnPointer  bool    `json:"returnPointer"`
		}
		if len(raw) > 8192 || json.Unmarshal([]byte(raw), &message) != nil {
			return
		}
		switch message.Type {
		case "ready":
			log.Printf("Floating renderer ready: viewport=%.0fx%.0f", message.ViewportWidth, message.ViewportHeight)
			call("SetPropW", floatingWindow, uintptr(unsafe.Pointer(ws("MacroLeprechaun.SettingsReady"))), 1)
			floatingKeepFocus = message.KeepInputFocus
			floatingHideOnBlur = message.HideOnFocusOut
			floatingFade = message.Fade
			floatingBrowser.Resize()
			if err := floatingBrowser.Show(); err != nil {
				log.Print("Floating controller show: ", err)
			}
			call("ShowWindow", floatingWindow, 4)
			floatingBrowser.Resize()
			call("UpdateWindow", floatingWindow)
			log.Printf("Floating window shown: visible=%t", call("IsWindowVisible", floatingWindow) != 0)
			if !floatingKeepFocus {
				call("SetForegroundWindow", floatingWindow)
				browser.Focus()
			}
			floatingReady = true
		case "warp":
			if mathAbs(message.X) < float64(width)*2 && mathAbs(message.Y) < float64(height)*2 {
				var cursor point
				call("GetCursorPos", uintptr(unsafe.Pointer(&cursor)))
				call("SetCursorPos", uintptr(int32(float64(cursor.X)+message.X)), uintptr(int32(float64(cursor.Y)+message.Y)))
			}
		case "settings":
			closeWindow(0, false)
			body, _ := json.Marshal(map[string]string{"panel": message.Panel})
			req, _ := http.NewRequest("POST", u.Scheme+"://"+u.Host+"/api/floating/settings", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+q.Get("admin"))
			req.Header.Set("Content-Type", "application/json")
			response, e := client.Do(req)
			if e == nil {
				response.Body.Close()
			}
		case "activate":
			closeWindow(message.Fade, message.ReturnPointer)
			body, _ := json.Marshal(map[string]string{"host": q.Get("host"), "panel": message.Panel, "control": message.Control})
			req, _ := http.NewRequest("POST", u.Scheme+"://"+u.Host+"/api/floating/activate", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+q.Get("admin"))
			req.Header.Set("Content-Type", "application/json")
			response, err := client.Do(req)
			if err != nil {
				log.Print("Floating activation: ", err)
			} else {
				if response.StatusCode != 200 {
					log.Print("Floating activation HTTP ", response.StatusCode)
				}
				response.Body.Close()
			}
		case "close":
			closeWindow(message.Fade, message.ReturnPointer)
		case "error":
			log.Print("Floating renderer: ", message.Message)
			settingsUIError(message.Message)
			closeWindow(0, false)
		}
	}
	if !browser.Embed(floatingWindow) {
		call("DestroyWindow", floatingWindow)
		return fmt.Errorf("WebView2 Runtime unavailable")
	}
	// Embed creates the controller but leaves its bounds at the default 0x0.
	// The high-level webview wrapper calls Resize here; this custom host must too.
	browser.Resize()
	var clientRect rect
	call("GetClientRect", floatingWindow, uintptr(unsafe.Pointer(&clientRect)))
	log.Printf("Floating controller initialized: client=%dx%d", clientRect.R-clientRect.L, clientRect.B-clientRect.T)
	if clientRect.R <= clientRect.L || clientRect.B <= clientRect.T {
		return fmt.Errorf("floating client area is empty")
	}
	if err = browser.Show(); err != nil {
		return fmt.Errorf("floating controller visibility: %w", err)
	}
	controller := browser.GetController().GetICoreWebView2Controller2()
	if controller == nil {
		return fmt.Errorf("WebView2 transparent controller unavailable")
	}
	if err = controller.PutDefaultBackgroundColor(edge.COREWEBVIEW2_COLOR{}); err != nil {
		return err
	}
	if settings, e := browser.GetSettings(); e == nil {
		settings.PutAreDefaultContextMenusEnabled(false)
		settings.PutAreDevToolsEnabled(false)
	}
	log.Print("Floating navigation starting")
	browser.Navigate(target)
	if parent > 0 {
		handle, _, _ := kernel.NewProc("OpenProcess").Call(0x100000, 0, uintptr(parent))
		if handle == 0 {
			return fmt.Errorf("parent exited")
		}
		go func() {
			defer kernel.NewProc("CloseHandle").Call(handle)
			result, _, _ := kernel.NewProc("WaitForSingleObject").Call(handle, 0xffffffff)
			if result == 0 {
				call("PostMessageW", floatingWindow, 0x10, 0, 0)
			}
		}()
	}
	startup := time.AfterFunc(30*time.Second, func() { call("PostMessageW", floatingWindow, 0x8006, 0, 0) })
	defer startup.Stop()
	var message msg
	for call("GetMessageW", uintptr(unsafe.Pointer(&message)), 0, 0, 0) > 0 {
		call("TranslateMessage", uintptr(unsafe.Pointer(&message)))
		call("DispatchMessageW", uintptr(unsafe.Pointer(&message)))
	}
	return nil
}
func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func reportFloatingFailure(err error) {
	settingsUIError("Не вдалося відкрити плаваючу панель: " + err.Error())
}
