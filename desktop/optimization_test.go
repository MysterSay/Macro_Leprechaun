package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLazyMacroLifecycle(t *testing.T) {
	a := testApp(t)
	c := a.runtimeConfig()
	c.Settings.CacheMacros = false
	if err := a.save(c); err != nil {
		t.Fatal(err)
	}
	m := a.cfg.Macros[0]
	if !m.Lazy || len(m.Steps) != 0 || len(m.Puzzles) != 0 {
		t.Fatal("body retained")
	}
	full, err := a.macroForRun(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if full.Lazy || len(full.Steps)+len(full.Puzzles) == 0 {
		t.Fatal("body unavailable")
	}
	if !a.cfg.Macros[0].Lazy {
		t.Fatal("run populated persistent cache")
	}
	full.Name = "edited on demand"
	c = a.runtimeConfig()
	c.Macros[0] = full
	if err = a.save(c); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Macros[0].File == m.File {
		t.Fatal("edited file hash unchanged")
	}
	restored, err := loadStorage(a.root)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Macros[0].Lazy || restored.Macros[0].Name != full.Name {
		t.Fatal("catalog roundtrip")
	}
	c = a.runtimeConfig()
	c.Settings.CacheMacros = true
	if err = a.save(c); err != nil {
		t.Fatal(err)
	}
	if a.cfg.Macros[0].Lazy || len(a.cfg.Macros[0].Steps)+len(a.cfg.Macros[0].Puzzles) == 0 {
		t.Fatal("cache switch failed")
	}
	c = a.runtimeConfig()
	c.Macros[0].File = "other.json"
	if err = a.save(c); err == nil {
		t.Fatal("forged catalog accepted")
	}
}
func TestLazyBodyNotReadByStatus(t *testing.T) {
	a := testApp(t)
	c := a.runtimeConfig()
	c.Settings.CacheMacros = false
	if err := a.save(c); err != nil {
		t.Fatal(err)
	}
	m := a.cfg.Macros[0]
	os.WriteFile(filepath.Join(a.root, "macros", m.File), []byte("invalid JSON"), 0600)
	if w := request(a, "GET", "/api/status", "", a.admin); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w := request(a, "GET", "/api/config", "", a.admin); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if _, err := a.macroForRun(m.ID); err == nil {
		t.Fatal("activation did not read body")
	}
}
func TestDemandProviderSleepsAndRequestsOnlyUsed(t *testing.T) {
	a := testApp(t)
	m := sourceAddon(t, a, "demand", "python", `import json,sys
q=json.load(sys.stdin)
with open('calls.jsonl','a') as f: f.write(json.dumps(q['requestedVariables'])+'\n')
print(json.dumps({'ok':True,'variables':{'cpu':'42','gpu':'88'}}))
`)
	m.Variables = []PluginVariable{{ID: "cpu"}, {ID: "gpu"}}
	a.moduleList = []Module{m}
	a.cfg.Modules[m.ID] = true
	ctx := context.Background()
	if err := a.refreshProviders(ctx, a.neededVariables()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.root, "plagin", "demand", "calls.jsonl")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unused provider ran")
	}
	a.cfg.Smart[0].Controls[0].Label = "CPU ${demand__cpu}"
	if err := a.refreshProviders(ctx, a.neededVariables()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	if err = json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &names); err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "cpu" {
		t.Fatal(string(raw))
	}
	if a.runtimeConfig().Variables["demand__cpu"] != "42" {
		t.Fatal("value missing")
	}
	if a.variableStates()["demand__gpu"] != "paused" {
		t.Fatal("unused metric not paused")
	}
	a.cfg.Settings.DisabledSmart = true
	if a.neededVariables()["demand__cpu"] {
		t.Fatal("disabled mode keeps provider awake")
	}
	a.variableWatchUntil = time.Now().Add(time.Second)
	if !a.neededVariables()["demand__gpu"] {
		t.Fatal("watcher did not wake variables")
	}
	a.variableWatchUntil = time.Time{}
	if a.neededVariables()["demand__gpu"] {
		t.Fatal("watcher stayed awake")
	}
}

func TestActiveUnboundMacroKeepsVariablesFresh(t *testing.T) {
	a := testApp(t)
	m := Macro{ID: "background", Name: "Background", Steps: []Step{{Type: "text", Value: "${stats__cpu}"}}}
	a.cfg.Macros = append(a.cfg.Macros, catalogMacro(m))
	a.cfg.Settings.DisabledSmart = true
	a.cfg.Settings.DisabledWidgets = true
	a.cfg.Settings.DisabledFloating = true
	if a.neededVariables()["stats__cpu"] {
		t.Fatal("unbound idle macro polls")
	}
	a.running[m.ID+"/input"] = true
	if !a.neededVariables()["stats__cpu"] {
		t.Fatal("active unbound macro paused")
	}
	delete(a.running, m.ID+"/input")
	if a.neededVariables()["stats__cpu"] {
		t.Fatal("finished macro still polls")
	}
}
