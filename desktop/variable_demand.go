package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func variableEnabled(c Config, m Module, v PluginVariable) bool {
	if !c.Modules[m.ID] {
		return false
	}
	if v.EnableSetting != "" {
		flag, _ := c.ModuleSettings[m.ID][v.EnableSetting].(bool)
		return flag
	}
	return true
}
func (a *App) neededVariables() map[string]bool {
	a.mu.RLock()
	c := a.cfg
	modules := append([]Module{}, a.moduleList...)
	watch := time.Now().Before(a.variableWatchUntil)
	active := []string{}
	for key, running := range a.running {
		if running {
			active = append(active, strings.SplitN(key, "/", 2)[0])
		}
	}
	a.mu.RUnlock()
	needed := map[string]bool{}
	macros := map[string]Macro{}
	for _, m := range c.Macros {
		macros[m.ID] = m
	}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		m, ok := macros[id]
		if !ok || m.MissingFile != "" {
			return
		}
		summary := m.Summary
		if summary == nil {
			summary = summarizeMacro(m)
		}
		for _, name := range summary.References {
			needed[name] = true
		}
		for _, other := range summary.Calls {
			visit(other)
		}
		for _, plugin := range summary.Plugins {
			for _, module := range modules {
				if module.ID == plugin {
					for _, name := range module.UsesVariables {
						needed[name] = true
					}
				}
			}
		}
		for _, p := range m.Puzzles {
			if strings.HasPrefix(p.Type, "plugin:") {
				parts := strings.Split(strings.TrimPrefix(p.Type, "plugin:"), "/")
				for _, module := range modules {
					if module.ID == parts[0] {
						for _, name := range module.UsesVariables {
							needed[name] = true
						}
					}
				}
			}
		}
	}
	for _, key := range active {
		visit(key)
	}
	for mode, panels := range [][]Panel{c.Smart, c.Widgets, c.Floating} {
		disabled := mode == 0 && c.Settings.DisabledSmart || mode == 1 && c.Settings.DisabledWidgets || mode == 2 && c.Settings.DisabledFloating
		if disabled {
			continue
		}
		for _, panel := range panels {
			if !panel.Enabled {
				continue
			}
			for _, b := range controlLeaves(panel.Controls) {
				visit(b.Macro)
				visit(b.OffMacro)
				for _, token := range labelReference.FindAllString(b.Label, -1) {
					needed[token[2:len(token)-1]] = true
				}
				if b.Kind == "slider" && b.Variable != "" {
					needed[b.Variable] = true
				}
			}
		}
	}
	if c.Settings.PollUnusedVariables || watch {
		needed["system__network"] = true
		for _, m := range modules {
			for _, v := range m.Variables {
				if variableEnabled(c, m, v) {
					needed[m.ID+"__"+v.ID] = true
				}
			}
		}
	}
	return needed
}
func (a *App) variableStates() map[string]string {
	needed := a.neededVariables()
	a.mu.RLock()
	defer a.mu.RUnlock()
	states := map[string]string{}
	for _, m := range a.moduleList {
		for _, v := range m.Variables {
			if variableEnabled(a.cfg, m, v) {
				name := m.ID + "__" + v.ID
				states[name] = "paused"
				if needed[name] {
					states[name] = "active"
				}
			}
		}
	}
	states["system__network"] = "paused"
	if needed["system__network"] {
		states["system__network"] = "active"
	}
	return states
}
func (a *App) refreshProviders(ctx context.Context, needed map[string]bool) error {
	a.providerMu.Lock()
	defer a.providerMu.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	c := a.runtimeConfig()
	a.mu.RLock()
	modules := append([]Module{}, a.moduleList...)
	a.mu.RUnlock()
	var firstError error
	for _, m := range modules {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if m.API != 2 || m.Error != "" || !c.Modules[m.ID] {
			continue
		}
		names := []string{}
		for _, v := range m.Variables {
			if needed[m.ID+"__"+v.ID] && variableEnabled(c, m, v) {
				names = append(names, v.ID)
			}
		}
		if len(names) == 0 {
			continue
		}
		a.mu.RLock()
		last := a.providerLast[m.ID]
		missing := false
		for _, name := range names {
			value, exists := a.moduleVars[m.ID+"__"+name]
			if !exists || value == "—" {
				missing = true
			}
		}
		a.mu.RUnlock()
		if (!missing || a.providerError(m.ID)) && time.Since(last) < 2*time.Second {
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		reply, err := a.pluginCallRequested(callCtx, m, "variables", "", nil, names)
		cancel()
		a.mu.Lock()
		if a.providerErrors == nil {
			a.providerErrors = map[string]string{}
		}
		previous := a.providerErrors[m.ID]
		if a.providerLast == nil {
			a.providerLast = map[string]time.Time{}
		}
		a.providerLast[m.ID] = time.Now()
		if err != nil {
			a.providerErrors[m.ID] = err.Error()
		} else {
			delete(a.providerErrors, m.ID)
			if a.moduleVars == nil {
				a.moduleVars = map[string]string{}
			}
			values, _ := reply["variables"].(map[string]any)
			for _, name := range names {
				if value, ok := values[name]; ok {
					a.moduleVars[m.ID+"__"+name] = fmt.Sprint(value)
				}
			}
			if a.providerLast == nil {
				a.providerLast = map[string]time.Time{}
			}
			a.providerLast[m.ID] = time.Now()
		}
		a.mu.Unlock()
		if err != nil {
			if previous != err.Error() {
				a.log("Addon variables / " + m.ID + ": " + err.Error())
			}
			if firstError == nil {
				firstError = err
			}
		}
	}
	if needed["system__network"] {
		a.mu.RLock()
		last := a.networkLast
		a.mu.RUnlock()
		if time.Since(last) > 10*time.Second {
			network := platformNetworkName()
			a.mu.Lock()
			if a.systemVars == nil {
				a.systemVars = map[string]string{}
			}
			a.systemVars["system__network"] = network
			a.networkLast = time.Now()
			a.mu.Unlock()
		}
	}
	return firstError
}
func (a *App) warmMacroVariables(ctx context.Context, m Macro) error {
	summary := m.Summary
	if summary == nil {
		summary = summarizeMacro(m)
	}
	needed := map[string]bool{}
	for _, name := range summary.References {
		needed[name] = true
	}
	a.mu.RLock()
	for _, id := range summary.Plugins {
		for _, module := range a.moduleList {
			if module.ID == id {
				for _, name := range module.UsesVariables {
					needed[name] = true
				}
			}
		}
	}
	a.mu.RUnlock()
	return a.refreshProviders(ctx, needed)
}

func (a *App) providerError(id string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.providerErrors[id] != ""
}
