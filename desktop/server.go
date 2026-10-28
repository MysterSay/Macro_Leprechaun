package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func clientSession(r *http.Request) string {
	if h := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); h != "" {
		return h
	}
	if c, e := r.Cookie("ml_admin"); e == nil {
		return c.Value
	}
	return "admin"
}
func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		http.Error(w, e.Error(), 400)
		return false
	}
	if e := d.Decode(new(any)); e != io.EOF {
		http.Error(w, "Expected one JSON object", 400)
		return false
	}
	return true
}
func (a *App) auth(r *http.Request, admin bool) bool {
	t := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if c, e := r.Cookie("ml_admin"); e == nil && t == "" {
		t = c.Value
	}
	if subtle.ConstantTimeCompare([]byte(t), []byte(a.admin)) == 1 {
		return true
	}
	if admin {
		return false
	}
	a.mu.RLock()
	_, ok := a.devices[t]
	a.mu.RUnlock()
	return ok && t != ""
}
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if ip, _, _ := net.SplitHostPort(r.RemoteAddr); a.runtimeConfig().Settings.DisabledSmart && !net.ParseIP(ip).IsLoopback() {
			http.Error(w, "Smart panel disabled", 503)
			return
		}
		path := r.URL.Path
		if path == "/api/discover" {
			reply(w, map[string]any{"name": "MacroLeprechaun", "version": appVersion, "port": a.runtimeConfig().Settings.Port})
			return
		}
		if path == "/api/pair" && r.Method == "POST" {
			var q struct {
				Code string `json:"code"`
				Name string `json:"name"`
			}
			if !decode(w, r, &q) {
				return
			}
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			a.mu.Lock()
			if a.attempts[ip] >= 5 || time.Now().Unix() > a.pairUntil || a.pairing == "" || subtle.ConstantTimeCompare([]byte(q.Code), []byte(a.pairing)) != 1 {
				a.attempts[ip]++
				a.mu.Unlock()
				http.Error(w, "Invalid or expired pairing code", 403)
				return
			}
			t := token()
			a.devices[t] = q.Name
			a.pairing = ""
			a.pairUntil = 0
			b, _ := json.Marshal(a.devices)
			e := os.WriteFile(filepath.Join(a.root, "setting", "devices.json"), b, 0600)
			a.mu.Unlock()
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			reply(w, map[string]string{"token": t})
			return
		}
		admin := path != "/api/panels" && path != "/api/control"
		if !a.auth(r, admin) {
			http.Error(w, "Unauthorized", 401)
			return
		}
		if r.Method != "GET" && r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host {
			http.Error(w, "Invalid origin", 403)
			return
		}
		switch path {
		case "/api/floating/settings":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var q struct {
				Panel string `json:"panel"`
			}
			if !decode(w, r, &q) {
				return
			}
			openSettings(fmt.Sprintf("http://127.0.0.1:%d/?admin=%s&mode=floating&panel=%s", a.runtimeConfig().Settings.Port, a.admin, q.Panel))
			reply(w, map[string]bool{"ok": true})
		case "/api/floating/themes":
			reply(w, a.floatingThemes())
		case "/api/floating/state":
			host := r.URL.Query().Get("host")
			c := a.runtimeConfig()
			scope := RunScope{Session: "desktop", Mode: "floating", Host: host, Panel: host}
			current := a.activePanel(scope)
			for _, p := range c.Floating {
				if p.ID == current {
					reply(w, map[string]any{"panel": p, "panels": c.Floating, "variables": c.Variables})
					return
				}
			}
			http.Error(w, "Panel not found", 404)
		case "/api/floating/activate":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var q struct {
				Host    string `json:"host"`
				Panel   string `json:"panel"`
				Control string `json:"control"`
			}
			if !decode(w, r, &q) {
				return
			}
			c := a.runtimeConfig()
			if c.Settings.DisabledFloating {
				http.Error(w, "Floating disabled", 503)
				return
			}
			scope := RunScope{Session: "desktop", Mode: "floating", Host: q.Host, Panel: q.Panel}
			if a.activePanel(scope) != q.Panel {
				http.Error(w, "Inactive panel", 409)
				return
			}
			for _, p := range c.Floating {
				if p.ID == q.Panel {
					for _, b := range p.Controls {
						if b.ID == q.Control {
							if err := a.activatePanelControl(p, b, "click", scope); err != nil {
								http.Error(w, err.Error(), 400)
								return
							}
							reply(w, map[string]bool{"ok": true})
							return
						}
					}
				}
			}
			http.NotFound(w, r)

		case "/api/variables/watch":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var q struct {
				All bool `json:"all"`
			}
			if !decode(w, r, &q) {
				return
			}
			a.mu.Lock()
			a.variableWatchUntil = time.Time{}
			if q.All {
				a.variableWatchUntil = time.Now().Add(8 * time.Second)
			}
			a.mu.Unlock()
			reply(w, map[string]bool{"ok": true})
		case "/api/macro":
			if r.Method != "GET" {
				http.Error(w, "Method", 405)
				return
			}
			m, err := a.macroForRun(r.URL.Query().Get("id"))
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			reply(w, m)

		case "/api/stop":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var q struct {
				All bool     `json:"all"`
				IDs []string `json:"ids"`
			}
			if !decode(w, r, &q) {
				return
			}
			a.stopMacros(q.IDs, q.All)
			reply(w, map[string]bool{"ok": true})
		case "/api/plugin/ui":
			a.pluginUI(w, r)
		case "/api/locales":
			entries, _ := os.ReadDir(filepath.Join(a.root, "locales"))
			names := map[string]bool{"uk": true, "en": true}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".json") {
					names[strings.TrimSuffix(entry.Name(), ".json")] = true
				}
			}
			list := []string{}
			for name := range names {
				list = append(list, name)
			}
			sort.Strings(list)
			reply(w, list)
		case "/api/win/inventory", "/api/win/pick", "/api/win/cursor":
			a.winAPI(w, r)
		case "/api/icons/search", "/api/icons/preview", "/api/media/import":
			a.iconAPI(w, r)
		case "/api/config/validate":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var c Config
			if !decode(w, r, &c) {
				return
			}
			a.mu.RLock()
			catalogErr := a.canonicalizeCatalog(&c)
			a.mu.RUnlock()
			if catalogErr != nil {
				http.Error(w, catalogErr.Error(), 400)
				return
			}
			if e := validate(c); e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			reply(w, map[string]bool{"ok": true})
		case "/api/config":
			if r.Method == "GET" {
				reply(w, a.runtimeConfig())
			} else if r.Method == "PUT" {
				var c Config
				if !decode(w, r, &c) {
					return
				}
				if e := a.save(c); e != nil {
					http.Error(w, e.Error(), 400)
					return
				}
				platformRefresh()
				reply(w, map[string]bool{"ok": true})
			} else {
				http.Error(w, "Method", 405)
			}
		case "/api/status":
			a.mu.RLock()
			logs := append([]string{}, a.logs...)
			variables := map[string]string{}
			for k, v := range a.systemVars {
				variables[k] = v
			}
			for k, v := range a.moduleVars {
				variables[k] = v
			}
			for k, v := range a.cfg.Variables {
				variables[k] = v
			}
			running := len(a.running)
			devices := []map[string]string{}
			for id, name := range a.devices {
				devices = append(devices, map[string]string{"id": id, "name": name})
			}
			for k, v := range a.systemVars {
				variables[k] = v
			}
			for k, v := range a.moduleVars {
				variables[k] = v
			}
			if a.systemVars != nil {
				now := time.Now()
				variables["system__time"] = now.Format("15:04:05")
				variables["system__date"] = now.Format("2006-01-02")
			}
			code := a.pairing
			until := a.pairUntil
			widgetStates := map[string]ControlState{}
			for key, state := range a.controlStates {
				widgetStates[key] = state
			}
			modules := append([]Module{}, a.moduleList...)
			for i := range modules {
				modules[i].RuntimeError = a.providerErrors[modules[i].ID]
			}
			a.mu.RUnlock()
			variables = a.runtimeConfig().Variables
			reply(w, map[string]any{"variableStates": a.variableStates(), "widgetStates": widgetStates, "widgetMetrics": widgetMetrics, "version": appVersion, "variables": variables, "logs": logs, "running": running, "devices": devices, "pairCode": code, "pairUntil": until, "modules": modules, "addresses": addresses(a.runtimeConfig().Settings.Port), "media": mediaFiles(a.root)})
		case "/api/pairing":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			b := token()
			n, _ := strconv.ParseUint(b[:8], 16, 32)
			a.mu.Lock()
			a.pairing = fmt.Sprintf("%06d", n%1000000)
			a.pairUntil = time.Now().Add(5 * time.Minute).Unix()
			a.attempts = map[string]int{}
			a.mu.Unlock()
			reply(w, map[string]bool{"ok": true})
		case "/api/revoke":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var q struct {
				ID string `json:"id"`
			}
			if !decode(w, r, &q) {
				return
			}
			a.mu.Lock()
			delete(a.devices, q.ID)
			b, _ := json.Marshal(a.devices)
			e := os.WriteFile(filepath.Join(a.root, "setting", "devices.json"), b, 0600)
			a.mu.Unlock()
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			reply(w, map[string]bool{"ok": true})
		case "/api/panels":
			c := a.runtimeConfig()
			panels := []Panel{}
			for _, p := range c.Smart {
				if p.Enabled {
					panels = append(panels, p)
				}
			}
			inputs, states := a.inputMetadata(c)
			reply(w, map[string]any{"panels": panels, "settings": c.Settings, "variables": c.Variables, "inputs": inputs, "states": states, "activePanel": a.activePanel(RunScope{Session: clientSession(r), Mode: "smart"})})
		case "/api/control":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var q struct {
				Panel   string `json:"panel"`
				Control string `json:"control"`
				Value   string `json:"value"`
			}
			if !decode(w, r, &q) {
				return
			}
			c := a.runtimeConfig()
			for _, p := range c.Smart {
				if p.ID != q.Panel || !p.Enabled {
					continue
				}
				for _, b := range controlLeaves(p.Controls) {
					if b.ID != q.Control {
						continue
					}
					if len(q.Value) > 4096 {
						http.Error(w, "value too large", 400)
						return
					}
					if err := a.activatePanelControl(p, b, q.Value, RunScope{Session: clientSession(r), Mode: "smart", Panel: p.ID}); err != nil {
						status := 409
						if _, invalid := err.(*ControlInputError); invalid {
							status = 400
						}
						http.Error(w, err.Error(), status)
						return
					}

					reply(w, map[string]bool{"ok": true})
					return
				}
			}
			http.Error(w, "Control not found", 404)
		case "/api/run":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			var q struct {
				ID string `json:"id"`
			}
			if !decode(w, r, &q) {
				return
			}
			if e := a.runScoped(q.ID, nil, InputEvent{Kind: "button", Value: "click"}, RunScope{Session: clientSession(r), Mode: "smart"}); e != nil {
				http.Error(w, e.Error(), 409)
				return
			}
			reply(w, map[string]bool{"ok": true})
		case "/api/modules/reload":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			a.scanModules()
			reply(w, map[string]bool{"ok": true})
		case "/api/quit":
			if r.Method != "POST" {
				http.Error(w, "Method", 405)
				return
			}
			reply(w, map[string]bool{"ok": true})
			go func() { time.Sleep(200 * time.Millisecond); os.Exit(0) }()
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		if r.URL.Path == "/floating.html" {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; media-src 'self' blob:; frame-ancestors 'self'")
		}
		if r.URL.Path == "/" && r.URL.Query().Get("admin") != "" {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if !net.ParseIP(ip).IsLoopback() || r.URL.Query().Get("admin") != a.admin {
				http.Error(w, "Forbidden", 403)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "ml_admin", Value: a.admin, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/", 303)
			return
		}
		files.ServeHTTP(w, r)
	})
	return mux
}
func addresses(port int) []string {
	out := []string{}
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				out = append(out, fmt.Sprintf("http://%s:%d", n.IP.String(), port))
			}
		}
	}
	return out
}
