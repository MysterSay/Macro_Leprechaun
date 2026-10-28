package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPluginOwnershipAndCategory(t *testing.T) {
	a := testApp(t)
	m := Module{ID: "one", Variables: []PluginVariable{{ID: "metric", Name: "Metric"}}}
	a.acceptPluginVariables(m, map[string]any{"variables": map[string]any{"metric": "12", "name": "hijack"}})
	if a.variablesSnapshot()["one__metric"] != "12" || a.variablesSnapshot()["name"] == "hijack" {
		t.Fatal("plugin namespace escaped")
	}
	base := filepath.Join(a.root, "plagin", "bad")
	os.MkdirAll(base, 0700)
	m = Module{ID: "bad", API: 2, Executable: "test.exe", PuzzleCategories: []PuzzleCategory{{ID: "a"}, {ID: "b"}}}
	b, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(base, "manifest.json"), b, 0600)
	os.WriteFile(filepath.Join(base, "test.exe"), []byte{}, 0600)
	a.scanModules()
	if len(a.moduleList) != 1 || a.moduleList[0].Error == "" {
		t.Fatal("multiple categories accepted")
	}
}
func TestPluginUIIsDeclared(t *testing.T) {
	a := testApp(t)
	base := filepath.Join(a.root, "plagin", "p")
	os.MkdirAll(base, 0700)
	os.WriteFile(filepath.Join(base, "settings.html"), []byte("<h1>settings</h1>"), 0600)
	a.moduleList = []Module{{ID: "p", Executable: filepath.Join(base, "p.exe"), SettingsUI: "settings.html"}}
	for _, tc := range []struct {
		page string
		code int
	}{{"settings.html", 200}, {"../config.json", 403}} {
		r := httptest.NewRequest("GET", "/api/plugin/ui?module=p&page="+tc.page, nil)
		w := httptest.NewRecorder()
		a.pluginUI(w, r)
		if w.Code != tc.code {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.code == 200 && w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("missing sandbox policy")
		}
	}
}

func TestPluginSDKHelper(t *testing.T) {
	if len(os.Args) < 2 || os.Args[1] != "-test.run=TestPluginSDKHelper" {
		return
	}
	var q map[string]any
	if json.NewDecoder(os.Stdin).Decode(&q) != nil || q["value"] != "123" {
		os.Exit(2)
	}
	os.Stderr.WriteString("diagnostic on stderr\n")
	json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "variables": map[string]any{"metric": "44", "name": "bad"}})
	os.Exit(0)
}
func TestPluginExecuteProtocol(t *testing.T) {
	a := testApp(t)
	m := Module{ID: "sdk_test", API: 2, Executable: os.Args[0], Args: []string{"-test.run=TestPluginSDKHelper"}, Variables: []PluginVariable{{ID: "metric"}}, PuzzleCategories: []PuzzleCategory{{ID: "tools", Blocks: []PuzzleDefinition{{ID: "literal", Editor: "text"}}}}}
	a.moduleList = []Module{m}
	a.cfg.Modules[m.ID] = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := a.executePlugin(ctx, Puzzle{Type: "plugin:sdk_test/tools/literal", Value: "123"}); e != nil {
		t.Fatal(e)
	}
	if a.variablesSnapshot()["sdk_test__metric"] != "44" || a.variablesSnapshot()["name"] == "bad" {
		t.Fatal("plugin variable boundary failed")
	}
}
