package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PluginVariable struct {
	EnableSetting string `json:"enableSetting,omitempty"`
	ID            string `json:"id"`
	Name          string `json:"name"`
}

func (a *App) pluginCall(ctx context.Context, m Module, op, block string, value any) (map[string]any, error) {
	return a.pluginCallRequested(ctx, m, op, block, value, nil)
}
func (a *App) pluginCallRequested(ctx context.Context, m Module, op, block string, value any, requested []string) (map[string]any, error) {
	payload, _ := json.Marshal(map[string]any{"requestedVariables": requested, "api": 2, "op": op, "block": block, "value": value, "settings": a.runtimeConfig().ModuleSettings[m.ID], "variables": a.variablesSnapshot(), "host": map[string]any{"keyboardLayout": platformKeyboardLayout()}})
	cmd, e := moduleCommand(ctx, m)
	if e != nil {
		return nil, e
	}
	hideConsole(cmd)
	configureCommandCancellation(cmd)
	cmd.Stdin = bytes.NewReader(payload)
	var out, errOut limitedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if e := cmd.Run(); e != nil {
		return nil, fmt.Errorf("%s: %v %s", m.ID, e, errOut.String())
	}
	var reply map[string]any
	if e := json.Unmarshal(out.Bytes(), &reply); e != nil {
		return nil, e
	}
	if reply["ok"] != true {
		return nil, fmt.Errorf("%s: %v", m.ID, reply["error"])
	}
	return reply, nil
}
func (a *App) executePlugin(ctx context.Context, p Puzzle) error {
	parts := strings.Split(strings.TrimPrefix(p.Type, "plugin:"), "/")
	if len(parts) != 3 {
		return fmt.Errorf("plugin puzzle id")
	}
	a.mu.RLock()
	var module *Module
	for _, m := range a.moduleList {
		if m.ID == parts[0] {
			copy := m
			module = &copy
			break
		}
	}
	enabled := a.cfg.Modules[parts[0]]
	a.mu.RUnlock()
	if module == nil || !enabled || module.Error != "" {
		return fmt.Errorf("доповнення недоступне")
	}
	declared := false
	editor := ""
	for _, c := range module.PuzzleCategories {
		if c.ID == parts[1] {
			for _, b := range c.Blocks {
				if b.ID == parts[2] {
					declared = true
					editor = b.Editor
				}
			}
		}
	}
	if !declared {
		return fmt.Errorf("невідомий пазл доповнення")
	}
	if module.API == 1 {
		return a.module(ctx, module.ID+"/"+parts[2], a.variablesSnapshot())
	}
	var value any = p.Value
	if editor == "button" {
		if p.Value == "" {
			value = map[string]any{}
		} else if e := json.Unmarshal([]byte(p.Value), &value); e != nil {
			return e
		}
	}
	reply, e := a.pluginCall(ctx, *module, "execute", parts[2], value)
	if e == nil {
		a.acceptPluginVariables(*module, reply)
	}
	return e
}
func (a *App) startProviders() {
	a.mu.Lock()
	a.systemVars = map[string]string{"system__time": time.Now().Format("15:04:05"), "system__date": time.Now().Format("2006-01-02"), "system__network": ""}
	a.mu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			a.refreshProviders(ctx, a.neededVariables())
			cancel()
			<-ticker.C
		}
	}()
}

func (a *App) pluginUI(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("module")
	page := r.URL.Query().Get("page")
	a.mu.RLock()
	var m Module
	for _, v := range a.moduleList {
		if v.ID == id {
			m = v
		}
	}
	a.mu.RUnlock()
	if m.ID == "" {
		http.Error(w, "Module unavailable", 404)
		return
	}
	allowed := page == m.SettingsUI && page != ""
	for _, c := range m.PuzzleCategories {
		for _, b := range c.Blocks {
			if b.UI == page && page != "" {
				allowed = true
			}
		}
	}
	if !allowed || filepath.IsAbs(page) {
		http.Error(w, "Page not declared", 403)
		return
	}
	base := moduleDirectory(m)
	path := filepath.Join(base, page)
	rel, e := filepath.Rel(base, path)
	if e != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "Path", 403)
		return
	}
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	rel, e = filepath.Rel(base, resolved)
	if e != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "Path", 403)
		return
	}
	data, e := os.ReadFile(resolved)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data:; connect-src 'none'; form-action 'none'")
	w.Write(data)
}
func readonlyVariable(k string) bool {
	return strings.HasPrefix(k, "system__") || strings.Contains(k, "__")
}

func (a *App) acceptPluginVariables(m Module, reply map[string]any) {
	values, ok := reply["variables"].(map[string]any)
	if !ok {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.moduleVars == nil {
		a.moduleVars = map[string]string{}
	}
	for _, def := range m.Variables {
		if v, ok := values[def.ID]; ok {
			a.moduleVars[m.ID+"__"+def.ID] = fmt.Sprint(v)
		}
	}
}
