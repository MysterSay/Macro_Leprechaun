//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")

//go:uintptrescapes
func kc(name string, args ...uintptr) uintptr {
	v, _, _ := kernel.NewProc(name).Call(args...)
	return v
}

//go:uintptrescapes
func message(h, msg, w, l uintptr) (uintptr, error) {
	var result uintptr
	if call("SendMessageTimeoutW", h, msg, w, l, 2, 1000, uintptr(unsafe.Pointer(&result))) == 0 {
		return 0, fmt.Errorf("поле не відповідає або доступ заборонено")
	}
	return result, nil
}

type guiThreadInfo struct {
	Size, Flags                                        uint32
	Active, Focus, Capture, MenuOwner, MoveSize, Caret uintptr
	CaretRect                                          rect
}

func focusedEdit() (uintptr, error) {
	var g guiThreadInfo
	g.Size = uint32(unsafe.Sizeof(g))
	if call("GetGUIThreadInfo", 0, uintptr(unsafe.Pointer(&g))) == 0 || g.Focus == 0 {
		return 0, fmt.Errorf("немає активного поля")
	}
	var class [256]uint16
	call("GetClassNameW", g.Focus, uintptr(unsafe.Pointer(&class[0])), 256)
	// Custom browser/Electron/Office controls require a dedicated UI Automation adapter.
	if !strings.EqualFold(syscall.UTF16ToString(class[:]), "Edit") {
		return 0, fmt.Errorf("це поле не підтримується: потрібне стандартне Windows Edit; буфер обміну не використано")
	}
	if call("GetWindowLongPtrW", g.Focus, ^uintptr(15))&0x20 != 0 {
		return 0, fmt.Errorf("поле пароля не підтримується")
	}
	return g.Focus, nil
}
func readEdit(h uintptr) ([]uint16, int, int, error) {
	n, e := message(h, 0x0e, 0, 0)
	if e != nil {
		return nil, 0, 0, e
	}
	if n > 60000 {
		return nil, 0, 0, fmt.Errorf("поле перевищує підтримувані 60000 UTF-16 символів")
	}
	buf := make([]uint16, n+1)
	n, e = message(h, 0x0d, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	runtime.KeepAlive(buf)
	if e != nil {
		return nil, 0, 0, e
	}
	if n >= uintptr(len(buf)) {
		return nil, 0, 0, fmt.Errorf("текст поля змінився під час читання")
	}
	sel, e := message(h, 0xb0, 0, 0)
	return buf[:n], int(sel & 0xffff), int((sel >> 16) & 0xffff), e
}
func selectEdit(h uintptr, lo, hi int) error {
	current, e := focusedEdit()
	if e != nil {
		return e
	}
	if current != h {
		return fmt.Errorf("активне поле змінилося")
	}
	_, e = message(h, 0xb1, uintptr(lo), uintptr(hi))
	if e == nil {
		_, e = message(h, 0xb7, 0, 0)
	}
	return e
}
func setClipboardText(value string) error {
	chars := append(utf16.Encode([]rune(value)), 0)
	mem := kc("GlobalAlloc", 0x42, uintptr(len(chars)*2))
	if mem == 0 {
		return fmt.Errorf("не вдалося виділити буфер")
	}
	owned := true
	defer func() {
		if owned {
			kc("GlobalFree", mem)
		}
	}()
	ptr := kc("GlobalLock", mem)
	if ptr == 0 {
		return fmt.Errorf("не вдалося відкрити пам'ять буфера")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(chars)), chars)
	kc("GlobalUnlock", mem)
	// A real owner window is required when opening and emptying the clipboard.
	owner := call("CreateWindowExW", 0, uintptr(unsafe.Pointer(ws("STATIC"))), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, 0, 0)
	if owner == 0 {
		return fmt.Errorf("не вдалося створити власника буфера")
	}
	defer call("DestroyWindow", owner)
	if call("OpenClipboard", owner) == 0 {
		return fmt.Errorf("буфер обміну зайнятий")
	}
	defer call("CloseClipboard")
	if call("EmptyClipboard") == 0 {
		return fmt.Errorf("не вдалося очистити буфер")
	}
	if call("SetClipboardData", 13, mem) == 0 {
		return fmt.Errorf("не вдалося записати буфер")
	}
	owned = false
	return nil
}
func executeTextPuzzle(p Puzzle) error { return executeTextPuzzleContext(context.Background(), p) }
func executeTextPuzzleContext(ctx context.Context, p Puzzle) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if strings.ContainsRune(p.Value, 0) {
		return fmt.Errorf("текст містить непідтримуваний нульовий символ")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var info guiThreadInfo
	info.Size = uint32(unsafe.Sizeof(info))
	call("GetGUIThreadInfo", 0, uintptr(unsafe.Pointer(&info)))
	var targetClass [256]uint16
	call("GetClassNameW", info.Focus, uintptr(unsafe.Pointer(&targetClass[0])), 256)
	log.Printf("text action=%s; foreground=%x; focus=%x; class=%s", p.Type, call("GetForegroundWindow"), info.Focus, syscall.UTF16ToString(targetClass[:]))
	switch p.Type {
	case "text.insert":
		return typeTextContext(ctx, p.Value)
	case "text.clipboard":
		return setClipboardText(p.Value)
	}
	h, e := focusedEdit()
	if e != nil {
		switch p.Type {
		case "text.select":
			_, e = uiTextContext(ctx, "select", p.Value)
			return e
		case "text.find":
			_, e = uiTextContext(ctx, "find", p.Value)
			return e
		case "text.transform":
			foreground := call("GetForegroundWindow")
			var selected string
			selected, e = uiTextContext(ctx, "read", "")
			if e != nil {
				return e
			}
			result, e := transformSelection(selected, p.Value)
			if e != nil {
				return e
			}
			if call("GetForegroundWindow") != foreground {
				return fmt.Errorf("активне вікно змінилося")
			}
			payload, _ := json.Marshal(map[string]string{"expected": selected, "replacement": result})
			mode, e := uiTextContext(ctx, "replace", string(payload))
			if e != nil {
				return e
			}
			if mode == "done" {
				return nil
			}
			if call("GetForegroundWindow") != foreground {
				return fmt.Errorf("активне вікно змінилося")
			}
			if result == "" {
				return sendKeys("BACKSPACE")
			}
			return typeTextContext(ctx, result)
		}
		return e
	}
	text, lo, hi, e := readEdit(h)
	if e != nil {
		return e
	}
	if lo > len(text) || hi > len(text) {
		return fmt.Errorf("виділення недоступне")
	}
	switch p.Type {
	case "text.transform":
		if lo == hi {
			return fmt.Errorf("спочатку виділіть текст")
		}
		result, e := transformSelection(string(utf16.Decode(text[lo:hi])), p.Value)
		if e != nil {
			return e
		}
		if e = selectEdit(h, lo, hi); e != nil {
			return e
		}
		chars := append(utf16.Encode([]rune(strings.ReplaceAll(strings.ReplaceAll(result, "\r\n", "\n"), "\n", "\r\n"))), 0)
		_, e = message(h, 0xc2, 1, uintptr(unsafe.Pointer(&chars[0])))
		runtime.KeepAlive(chars)
		return e
	case "text.select":
		lo, hi, e = selectionBounds(text, hi, p.Value)
	case "text.find":
		lo, hi, e = findSelection(text, p.Value, hi)
	default:
		return fmt.Errorf("невідома текстова дія")
	}
	if e != nil {
		return e
	}
	return selectEdit(h, lo, hi)
}

func readSelectedText() (string, error) { return readSelectedTextContext(context.Background()) }
func readSelectedTextContext(ctx context.Context) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if h, e := focusedEdit(); e == nil {
		text, lo, hi, e := readEdit(h)
		if e != nil {
			return "", e
		}
		if lo > len(text) || hi > len(text) {
			return "", fmt.Errorf("виділення недоступне")
		}
		return string(utf16.Decode(text[lo:hi])), nil
	}
	return uiTextContext(ctx, "read", "")
}
func readClipboardText() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if call("OpenClipboard", 0) == 0 {
		return "", fmt.Errorf("буфер зайнятий")
	}
	defer call("CloseClipboard")
	h := call("GetClipboardData", 13)
	if h == 0 {
		return "", fmt.Errorf("у буфері немає Unicode-тексту")
	}
	p := kc("GlobalLock", h)
	if p == 0 {
		return "", fmt.Errorf("не вдалося прочитати буфер")
	}
	defer kc("GlobalUnlock", h)
	size := kc("GlobalSize", h) / 2
	if size > 1<<19 {
		return "", fmt.Errorf("текст буфера перевищує 1 MiB")
	}
	buf := unsafe.Slice((*uint16)(unsafe.Pointer(p)), int(size))
	end := 0
	for end < len(buf) && buf[end] != 0 {
		end++
	}
	return string(utf16.Decode(buf[:end])), nil
}
