package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type WinAction struct {
	Terminal  string   `json:"terminal,omitempty"`
	Command   string   `json:"command,omitempty"`
	Directory string   `json:"directory,omitempty"`
	Output    string   `json:"output,omitempty"`
	Path      string   `json:"path,omitempty"`
	App       string   `json:"app,omitempty"`
	Args      string   `json:"args,omitempty"`
	Action    string   `json:"action,omitempty"`
	Device    string   `json:"device,omitempty"`
	Steps     float64  `json:"steps,omitempty"`
	Variable  string   `json:"variable,omitempty"`
	Value     float64  `json:"value,omitempty"`
	URL       string   `json:"url,omitempty"`
	Keys      []string `json:"keys,omitempty"`
	X         int      `json:"x,omitempty"`
	Y         int      `json:"y,omitempty"`
	Buttons   []string `json:"buttons,omitempty"`
}

func parseWinAction(p Puzzle) (WinAction, error) {
	var q WinAction
	if strings.HasPrefix(strings.TrimSpace(p.Value), "{") {
		if e := json.Unmarshal([]byte(p.Value), &q); e != nil {
			return q, e
		}
	} else {
		switch p.Type {
		case "win.command":
			q.Command = p.Value
		case "win.program", "win.file", "win.folder":
			q.Path = p.Value
		case "win.hotkeys":
			q.Keys = strings.Split(p.Value, "+")
		}
	}
	if q.Action == "" {
		switch p.Type {
		case "win.sound":
			q.Action = "playpause"
		case "win.browser":
			q.Action = "open"
		}
	}
	if q.Terminal == "" {
		q.Terminal = "powershell"
	}
	return q, nil
}
func (a *App) executeWinPuzzle(p Puzzle) error {
	return a.executeWinPuzzleContext(context.Background(), p)
}
func (a *App) executeWinPuzzleContext(ctx context.Context, p Puzzle) error {
	q, e := parseWinAction(p)
	if e != nil {
		return e
	}
	if p.Type == "win.sound" && (q.Action == "set" || q.Action == "mic_set") {
		value, ok := a.config().Variables[q.Variable]
		if !ok {
			return fmt.Errorf("змінну %s не знайдено", q.Variable)
		}
		q.Value, e = strconv.ParseFloat(value, 64)
		if e != nil || !(q.Value >= 0 && q.Value <= 100) {
			return fmt.Errorf("змінна гучності має містити число 0–100")
		}
	}
	return executeWindowsActionContext(ctx, p.Type, q)
}
func (a *App) winAPI(w http.ResponseWriter, r *http.Request) {
	// Native pickers and cursor monitoring belong to the local desktop settings.
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		http.Error(w, "Local desktop only", 403)
		return
	}
	switch r.URL.Path {
	case "/api/win/inventory":
		if r.Method != "GET" {
			http.Error(w, "Method", 405)
			return
		}
		v, e := windowsInventory()
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		reply(w, v)
	case "/api/win/pick":
		if r.Method != "POST" {
			http.Error(w, "Method", 405)
			return
		}
		var q struct {
			Kind string `json:"kind"`
		}
		if !decode(w, r, &q) {
			return
		}
		if !oneOf(q.Kind, "file", "program", "folder", "output") {
			http.Error(w, "picker kind", 400)
			return
		}
		path, e := windowsPick(q.Kind)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		reply(w, map[string]string{"path": path})
	case "/api/win/cursor":
		if r.Method != "GET" {
			http.Error(w, "Method", 405)
			return
		}
		v, e := windowsCursor()
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		reply(w, v)
	default:
		http.NotFound(w, r)
	}
}
