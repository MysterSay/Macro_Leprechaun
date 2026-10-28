//go:build windows

package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

//go:embed automation/win.ps1
var winScript string

//go:embed automation/audio.cs
var audioSource string

func winBridge(action string, payload any) (json.RawMessage, error) {
	return winBridgeContext(context.Background(), action, payload)
}
func winBridgeContext(parent context.Context, action string, payload any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{"action": action, "data": payload})
	file, e := os.CreateTemp("", "macroleprechaun-win-*.ps1")
	if e != nil {
		return nil, e
	}
	defer os.Remove(file.Name())
	_, e = file.WriteString("\xef\xbb\xbf" + strings.ReplaceAll(winScript, "__AUDIO_SOURCE__", audioSource))
	closeErr := file.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		return nil, closeErr
	}
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-File", file.Name())
	hideConsole(cmd)
	configureCommandCancellation(cmd)
	cmd.Stdin = bytes.NewReader(body)
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	err := cmd.Run()
	var result struct {
		OK    bool            `json:"ok"`
		Value json.RawMessage `json:"value"`
		Error string          `json:"error"`
	}
	if e := json.Unmarshal(bytes.TrimPrefix(bytes.TrimSpace(out.Bytes()), []byte{0xef, 0xbb, 0xbf}), &result); e != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Windows: перевищено час очікування")
		}
		return nil, fmt.Errorf("Windows adapter: %v (%s)", err, strings.TrimSpace(errout.String()))
	}
	if !result.OK {
		return nil, fmt.Errorf("Windows: %s", result.Error)
	}
	return result.Value, nil
}
func windowsInventory() (any, error) {
	v, e := winBridge("inventory", nil)
	if e == nil {
		var diagnostic struct {
			AudioError string `json:"audioError"`
		}
		if json.Unmarshal(v, &diagnostic) == nil && diagnostic.AudioError != "" {
			log.Print("Core Audio inventory: " + diagnostic.AudioError)
		}
	}
	return v, e
}
func windowsPick(kind string) (string, error) {
	v, e := winBridge("pick", map[string]string{"kind": kind})
	if e != nil {
		return "", e
	}
	var path string
	e = json.Unmarshal(v, &path)
	return path, e
}
func windowsCursor() (any, error) {
	var p point
	if call("GetCursorPos", uintptr(unsafe.Pointer(&p))) == 0 {
		return nil, fmt.Errorf("GetCursorPos failed")
	}
	return map[string]any{"x": p.X, "y": p.Y, "space": call("GetAsyncKeyState", 0x20)&0x8000 != 0}, nil
}
func executeWindowsAction(kind string, q WinAction) error {
	return executeWindowsActionContext(context.Background(), kind, q)
}
func executeWindowsActionContext(ctx context.Context, kind string, q WinAction) error {
	switch kind {
	case "win.hotkeys":
		return sendWindowsKeys(q.Keys)
	case "win.clicker":
		if q.X < -100000 || q.X > 100000 || q.Y < -100000 || q.Y > 100000 {
			return fmt.Errorf("invalid cursor coordinates")
		}
		if call("SetCursorPos", uintptr(q.X), uintptr(q.Y)) == 0 {
			return fmt.Errorf("SetCursorPos failed")
		}
		packets := [][40]byte{}
		for _, name := range q.Buttons {
			pair, e := mousePackets(name)
			if e != nil {
				return e
			}
			packets = append(packets, pair...)
		}
		return sendNativePackets(packets)
	case "win.browser":
		if q.Action == "open" {
			u, e := url.Parse(q.URL)
			if e != nil || !oneOf(strings.ToLower(u.Scheme), "http", "https") || u.Host == "" {
				return fmt.Errorf("потрібне http/https посилання")
			}
			_, e = winBridgeContext(ctx, "url", q)
			return e
		}
		k := map[string]string{"refresh": "BROWSER_REFRESH", "home": "BROWSER_HOME", "back": "BROWSER_BACK", "forward": "BROWSER_FORWARD"}[q.Action]
		if k == "" {
			return fmt.Errorf("browser action")
		}
		return sendWindowsKeys([]string{k})
	case "win.sound":
		k := map[string]string{"next": "MEDIA_NEXT", "previous": "MEDIA_PREV", "playpause": "MEDIA_PLAY_PAUSE", "stop": "MEDIA_STOP"}[q.Action]
		if k != "" {
			return sendWindowsKeys([]string{k})
		}
		if !oneOf(q.Action, "mute", "down", "up", "set", "mic_mute", "mic_down", "mic_up", "mic_set") {
			return fmt.Errorf("sound action")
		}
		if (strings.HasSuffix(q.Action, "up") || strings.HasSuffix(q.Action, "down")) && (!(q.Steps > 0) || q.Steps > 100) {
			return fmt.Errorf("крок гучності: 1–100 процентних пунктів")
		}
		_, e := winBridgeContext(ctx, "audio", q)
		return e
	case "win.command", "win.file", "win.folder", "win.program":
		_, e := winBridgeContext(ctx, strings.TrimPrefix(kind, "win."), q)
		return e
	}
	return fmt.Errorf("unsupported Windows action")
}
func sendWindowsKeys(names []string) error {
	if len(names) == 0 {
		return fmt.Errorf("оберіть клавішу")
	}
	codes := []uint16{}
	mouse := ""
	for _, name := range names {
		if strings.HasPrefix(name, "MOUSE_") {
			if mouse != "" {
				return fmt.Errorf("only one mouse action")
			}
			mouse = name
			continue
		}
		v, e := extendedKeyCode(name)
		if e != nil {
			return e
		}
		codes = append(codes, v)
	}
	packets := [][40]byte{}
	makeKey := func(v uint16, up bool) [40]byte {
		var event [40]byte
		binary.LittleEndian.PutUint32(event[:4], 1)
		binary.LittleEndian.PutUint16(event[8:10], v)
		binary.LittleEndian.PutUint16(event[10:12], uint16(call("MapVirtualKeyW", uintptr(v), 0)))
		flags := uint32(0)
		switch v {
		case 0xa3, 0xa5, 0x5b, 0x5c, 0x5d, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2d, 0x2e, 0x2c, 0x6f:
			flags = 1
		}
		if up {
			flags |= 2
		}
		binary.LittleEndian.PutUint32(event[12:16], flags)
		return event
	}
	for _, v := range codes {
		packets = append(packets, makeKey(v, false))
	}
	if mouse != "" {
		name := map[string]string{"MOUSE_LEFT": "left", "MOUSE_RIGHT": "right", "MOUSE_MIDDLE": "middle", "MOUSE_X1": "x1", "MOUSE_X2": "x2"}[mouse]
		pair, e := mousePackets(name)
		if e != nil {
			return e
		}
		packets = append(packets, pair...)
	}
	for i := len(codes) - 1; i >= 0; i-- {
		packets = append(packets, makeKey(codes[i], true))
	}
	e := sendNativePackets(packets)
	if e != nil {
		for _, v := range codes {
			key(v, true)
		}
	}
	return e
}
func sendNativePackets(packets [][40]byte) error {
	if len(packets) == 0 {
		return nil
	}
	n, _, err := user.NewProc("SendInput").Call(uintptr(len(packets)), uintptr(unsafe.Pointer(&packets[0])), 40)
	runtime.KeepAlive(packets)
	if n != uintptr(len(packets)) {
		return fmt.Errorf("SendInput: %d/%d events; %v", n, len(packets), err)
	}
	return nil
}

func extendedKeyCode(name string) (uint16, error) {
	m := map[string]uint16{"WIN-L": 0x5b, "WIN-R": 0x5c, "CTRL-L": 0xa2, "CTRL-R": 0xa3, "SHIFT-L": 0xa0, "SHIFT-R": 0xa1, "ALT-L": 0xa4, "ALT-R": 0xa5, "LEFT": 0x25, "UP": 0x26, "RIGHT": 0x27, "DOWN": 0x28, "HOME": 0x24, "END": 0x23, "PAGEUP": 0x21, "PAGEDOWN": 0x22, "INSERT": 0x2d, "CAPSLOCK": 0x14, "NUMLOCK": 0x90, "SCROLLLOCK": 0x91, "PRINTSCREEN": 0x2c, "PAUSE": 0x13, "APPS": 0x5d, "MULTIPLY": 0x6a, "ADD": 0x6b, "SUBTRACT": 0x6d, "DECIMAL": 0x6e, "DIVIDE": 0x6f, "BROWSER_BACK": 0xa6, "BROWSER_FORWARD": 0xa7, "BROWSER_REFRESH": 0xa8, "BROWSER_STOP": 0xa9, "BROWSER_SEARCH": 0xaa, "BROWSER_FAVORITES": 0xab, "BROWSER_HOME": 0xac, "MEDIA_STOP": 0xb2, "OEM_COLON": 0xba, "OEM_PLUS": 0xbb, "OEM_COMMA": 0xbc, "OEM_MINUS": 0xbd, "OEM_PERIOD": 0xbe, "OEM_QUESTION": 0xbf, "OEM_TILDE": 0xc0, "OEM_OPENBRACKET": 0xdb, "OEM_PIPE": 0xdc, "OEM_CLOSEBRACKET": 0xdd, "OEM_QUOTE": 0xde, "OEM_102": 0xe2}
	for i := 0; i <= 9; i++ {
		m[fmt.Sprintf("NUM%d", i)] = uint16(0x60 + i)
	}
	if v, ok := m[name]; ok {
		return v, nil
	}
	return keyCode(name)
}

func platformNetworkName() string {
	v, e := winBridge("network", nil)
	if e != nil {
		return ""
	}
	var name string
	_ = json.Unmarshal(v, &name)
	return name
}
