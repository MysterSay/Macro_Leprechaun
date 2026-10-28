package main

import (
	"encoding/binary"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWinActionPayload(t *testing.T) {
	q, e := parseWinAction(Puzzle{Type: "win.command", Value: `{"terminal":"wsl:Ubuntu","command":"printf 'a b'","directory":"C:\\work","output":"C:\\result.txt"}`})
	if e != nil || q.Terminal != "wsl:Ubuntu" || q.Command != "printf 'a b'" || q.Output != `C:\result.txt` {
		t.Fatal(q, e)
	}
	q, e = parseWinAction(Puzzle{Type: "win.program", Value: "notepad.exe"})
	if e != nil || q.Path != "notepad.exe" {
		t.Fatal(q, e)
	}
	q, e = parseWinAction(Puzzle{Type: "win.hotkeys", Value: "CTRL+F5"})
	if e != nil || len(q.Keys) != 2 {
		t.Fatal(q, e)
	}
	if _, e = parseWinAction(Puzzle{Type: "win.command", Value: "{invalid"}); e == nil {
		t.Fatal("invalid JSON accepted")
	}
}
func TestWinAPILocalAndAuth(t *testing.T) {
	a := &App{admin: "test-secret", cfg: defaults(), root: t.TempDir()}
	handler := a.routes()
	for _, tc := range []struct {
		method, path, remote, auth, body string
		want                             int
	}{
		{"GET", "/api/win/inventory", "127.0.0.1:1", "", "", 401},
		{"GET", "/api/win/inventory", "192.168.0.3:1", "test-secret", "", 403},
		{"POST", "/api/win/pick", "127.0.0.1:1", "test-secret", `{"kind":"invalid"}`, 400},
		{"GET", "/api/win/pick", "127.0.0.1:1", "test-secret", "", 405},
	} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		r.RemoteAddr = tc.remote
		if tc.auth != "" {
			r.Header.Set("Authorization", "Bearer "+tc.auth)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s got %d want %d %s", tc.method, tc.path, w.Code, tc.want, w.Body.String())
		}
	}
}
func TestMouseInputABI(t *testing.T) {
	for _, c := range []struct {
		name           string
		data, down, up uint32
	}{{"left", 0, 2, 4}, {"right", 0, 8, 16}, {"middle", 0, 32, 64}, {"x1", 1, 128, 256}, {"x2", 2, 128, 256}} {
		p, e := mousePackets(c.name)
		if e != nil || len(p) != 2 {
			t.Fatal(c.name, e)
		}
		for i, flags := range []uint32{c.down, c.up} {
			if binary.LittleEndian.Uint32(p[i][:4]) != 0 || binary.LittleEndian.Uint32(p[i][16:20]) != c.data || binary.LittleEndian.Uint32(p[i][20:24]) != flags {
				t.Fatal(c.name, i)
			}
		}
	}
	if _, e := mousePackets("invalid"); e == nil {
		t.Fatal("invalid mouse accepted")
	}
}
