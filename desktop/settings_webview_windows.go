//go:build windows

package main

import (
	"fmt"
	webview2 "github.com/jchv/go-webview2"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var settingsUIState struct {
	sync.Mutex
	cmd     *exec.Cmd
	pending string
	relay   bool
}
var settingsEnumLock sync.Mutex
var settingsEnumResult uintptr
var settingsEnumCallback = syscall.NewCallback(func(h, l uintptr) uintptr {
	var pid uint32
	call("GetWindowThreadProcessId", h, uintptr(unsafe.Pointer(&pid)))
	if pid == uint32(l) && call("IsWindowVisible", h) != 0 && call("GetPropW", h, uintptr(unsafe.Pointer(ws("MacroLeprechaun.SettingsReady")))) != 0 {
		settingsEnumResult = h
		return 0
	}
	return 1
})

func settingsWindowForPID(pid int) uintptr {
	settingsEnumLock.Lock()
	defer settingsEnumLock.Unlock()
	settingsEnumResult = 0
	call("EnumWindows", settingsEnumCallback, uintptr(pid))
	return settingsEnumResult
}

type settingsCopyData struct {
	Tag  uintptr
	Size uint32
	Data uintptr
}

const settingsMessageTag = 0x4c455052

var childSettingsView webview2.WebView
var childSettingsProc uintptr
var childSettingsOrigin string
var childSettingsCallback = syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
	if m == 0x4a && l != 0 {
		data := (*settingsCopyData)(unsafe.Pointer(l))
		if data.Tag != settingsMessageTag || data.Size < 2 || data.Size > 16384 || data.Size%2 != 0 {
			return 0
		}
		units := unsafe.Slice((*uint16)(unsafe.Pointer(data.Data)), int(data.Size/2))
		target := strings.TrimRight(string(utf16.Decode(units)), "\x00")
		parsed, err := url.Parse(target)
		if err == nil && parsed.Scheme+"://"+parsed.Host == childSettingsOrigin {
			childSettingsView.Navigate(target)
			call("ShowWindow", h, 9)
			call("SetForegroundWindow", h)
			return 1
		}
		return 0
	}
	return call("CallWindowProcW", childSettingsProc, h, uintptr(m), w, l)
})

func openSettings(target string) {
	settingsUIState.Lock()
	defer settingsUIState.Unlock()
	if settingsUIState.cmd == nil {
		exe, err := os.Executable()
		if err != nil {
			log.Print(err)
			return
		}
		directory := ""
		if app != nil {
			directory = app.root
		}
		cmd := exec.Command(exe, "-settings-ui", target, "-data", directory, "-ui-parent", strconv.Itoa(os.Getpid()))
		// GUI process: SW_HIDE suppresses the initial editor window as well.
		cmd.Stderr = log.Writer()
		if err = cmd.Start(); err != nil {
			log.Print("WebView2 UI: ", err)
			settingsUIError("Не вдалося запустити редактор: " + err.Error())
			return
		}
		settingsUIState.cmd = cmd
		go func() {
			err := cmd.Wait()
			settingsUIState.Lock()
			if settingsUIState.cmd == cmd {
				settingsUIState.cmd = nil
				settingsUIState.pending = ""
			}
			settingsUIState.Unlock()
			if err != nil {
				log.Print("WebView2 UI closed: ", err)
				settingsUIError("Редактор WebView2 завершився з помилкою: " + err.Error() + "\nДеталі: data/application.log")
			}
		}()
	} else {
		settingsUIState.pending = target
	}
	if settingsUIState.relay {
		return
	}
	settingsUIState.relay = true
	go relaySettingsWindow()
}
func relaySettingsWindow() {
	defer func() { settingsUIState.Lock(); settingsUIState.relay = false; settingsUIState.Unlock() }()
	for i := 0; i < 600; i++ {
		settingsUIState.Lock()
		cmd := settingsUIState.cmd
		settingsUIState.Unlock()
		if cmd == nil {
			return
		}
		h := settingsWindowForPID(cmd.Process.Pid)
		if h == 0 {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		settingsUIState.Lock()
		target := settingsUIState.pending
		settingsUIState.pending = ""
		settingsUIState.Unlock()
		if target != "" {
			text := utf16.Encode([]rune(target + "\x00"))
			data := settingsCopyData{Tag: settingsMessageTag, Size: uint32(len(text) * 2), Data: uintptr(unsafe.Pointer(&text[0]))}
			call("SendMessageTimeoutW", h, 0x4a, 0, uintptr(unsafe.Pointer(&data)), 2, 750, 0)
			runtime.KeepAlive(text)
			runtime.KeepAlive(data)
		}
		call("ShowWindow", h, 9)
		call("SetForegroundWindow", h)
		return
	}
	settingsUIState.Lock()
	cmd := settingsUIState.cmd
	settingsUIState.Unlock()
	if cmd != nil && settingsWindowForPID(cmd.Process.Pid) == 0 {
		log.Print("WebView2 editor: startup exceeded 30 seconds")
		settingsUIError("Вікно WebView2 не відкрилося протягом 30 секунд. Перевірте Microsoft Edge WebView2 Runtime.\nДеталі: data/application.log")
	}
}
func settingsUIError(message string) {
	call("MessageBoxW", 0, uintptr(unsafe.Pointer(ws(message))), uintptr(unsafe.Pointer(ws("MacroLeprechaun · WebView2"))), 0x10|0x10000)
}
func runSettingsWindow(target, directory string, parent int) error {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" {
		return fmt.Errorf("invalid settings URL")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	profile := filepath.Join(directory, "setting", "webview2")
	if err = os.MkdirAll(profile, 0700); err != nil {
		return err
	}
	// This process owns only the editor and uses no personal Edge profile.
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disable-extensions --disable-sync --disable-background-networking --no-first-run")
	view := webview2.NewWithOptions(webview2.WebViewOptions{Debug: false, AutoFocus: true, DataPath: profile, WindowOptions: webview2.WindowOptions{Title: "MacroLeprechaun", Width: 1280, Height: 840, Center: true}})
	if view == nil {
		message := "Для інтерфейсу потрібен Microsoft Edge WebView2 Runtime. Встановіть Evergreen Runtime із сайту Microsoft. Макроси та віджети продовжують працювати."
		call("MessageBoxW", 0, uintptr(unsafe.Pointer(ws(message))), uintptr(unsafe.Pointer(ws("MacroLeprechaun · WebView2"))), 0x10)
		return fmt.Errorf("WebView2 Runtime unavailable")
	}
	childSettingsView = view
	childSettingsOrigin = parsed.Scheme + "://" + parsed.Host
	h := uintptr(view.Window())
	childSettingsProc = call("SetWindowLongPtrW", h, ^uintptr(3), childSettingsCallback)
	view.SetSize(780, 540, webview2.HintMin)
	view.Navigate(target)
	call("ShowWindow", h, 9)
	call("SetForegroundWindow", h)
	call("SetPropW", h, uintptr(unsafe.Pointer(ws("MacroLeprechaun.SettingsReady"))), 1)
	log.Print("Settings UI: dedicated WebView2 process, private profile")
	if parent > 0 {
		handle, _, _ := kernel.NewProc("OpenProcess").Call(0x100000, 0, uintptr(parent))
		if handle == 0 {
			view.Destroy()
			return fmt.Errorf("editor parent exited")
		}
		go func() {
			defer kernel.NewProc("CloseHandle").Call(handle)
			result, _, _ := kernel.NewProc("WaitForSingleObject").Call(handle, 0xffffffff)
			if result == 0 {
				call("PostMessageW", h, 0x10, 0, 0)
			}
		}()
	}
	view.Run()
	view.Destroy()
	return nil
}
