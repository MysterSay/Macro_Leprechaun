package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func sourceAddon(t *testing.T, a *App, id, runtime, source string) Module {
	t.Helper()
	dir := filepath.Join(a.root, "plagin", id)
	os.MkdirAll(filepath.Join(dir, "src"), 0700)
	entry := "src/plugin.py"
	if runtime == "javascript" {
		entry = "src/plugin.js"
	}
	if runtime == "powershell" {
		entry = "src/plugin.ps1"
	}
	m := Module{ID: id, Name: id, API: 2, Runtime: runtime, Entry: entry, SettingsUI: "settings.html", Variables: []PluginVariable{{ID: "result"}}}
	raw, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600)
	os.WriteFile(filepath.Join(dir, entry), []byte(source), 0600)
	os.WriteFile(filepath.Join(dir, "settings.html"), []byte("<h1>source addon</h1>"), 0600)
	a.scanModules()
	for _, found := range a.moduleList {
		if found.ID == id {
			if found.Error != "" {
				t.Fatal(found.Error)
			}
			return found
		}
	}
	t.Fatal("addon not scanned")
	return Module{}
}
func TestSourcePythonReloadUnicodeAndUI(t *testing.T) {
	if _, e := exec.LookPath("python3"); e != nil {
		t.Skip("python3 not installed")
	}
	a := testApp(t)
	source := `import sys,json,os
q=json.load(sys.stdin)
assert q['value']=='Привіт ${volume}'
assert os.getcwd()==os.environ['MACROLEPRECHAUN_ADDON_DIR']
assert sys.argv[1:] == ['space value', "a'\";$()"]
print(json.dumps({'ok':True,'variables':{'result':'перша версія'}}))
`
	m := sourceAddon(t, a, "source_test", "python", source)
	m.Args = []string{"space value", "a'\";$()"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, e := a.pluginCall(ctx, m, "execute", "text", "Привіт ${volume}")
	if e != nil {
		t.Fatal(e)
	}
	if reply["variables"].(map[string]any)["result"] != "перша версія" {
		t.Fatal(reply)
	}
	// No rescan/restart: a saved source edit must be used by the next call.
	source = `import json,sys
q=json.load(sys.stdin)
print(json.dumps({'ok':True,'variables':{'result':'друга версія'}}))
`
	os.WriteFile(m.Entry, []byte(source), 0600)
	reply, e = a.pluginCall(ctx, m, "execute", "text", "")
	if e != nil || reply["variables"].(map[string]any)["result"] != "друга версія" {
		t.Fatal(reply, e)
	}
	r := httptest.NewRequest("GET", "/api/plugin/ui?module=source_test&page=settings.html", nil)
	w := httptest.NewRecorder()
	a.pluginUI(w, r)
	if w.Code != 200 || w.Body.String() != "<h1>source addon</h1>" {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestSourceJavaScript(t *testing.T) {
	if _, e := exec.LookPath("node"); e != nil {
		t.Skip("node not installed")
	}
	a := testApp(t)
	m := sourceAddon(t, a, "js_test", "javascript", `let s='';process.stdin.setEncoding('utf8');process.stdin.on('data',x=>s+=x);process.stdin.on('end',()=>{const q=JSON.parse(s);console.log(JSON.stringify({ok:true,variables:{result:q.value}}))});`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, e := a.pluginCall(ctx, m, "execute", "text", "JS працює")
	if e != nil || reply["variables"].(map[string]any)["result"] != "JS працює" {
		t.Fatal(reply, e)
	}
}
func TestSourceAddonCannotEscapeDirectory(t *testing.T) {
	a := testApp(t)
	dir := filepath.Join(a.root, "plagin", "escape")
	os.MkdirAll(dir, 0700)
	m := Module{ID: "escape", API: 2, Runtime: "python", Entry: "../outside.py"}
	b, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600)
	a.scanModules()
	if len(a.moduleList) != 1 || a.moduleList[0].Error == "" {
		t.Fatal("outside entry accepted")
	}
}
func TestPowerShellSource(t *testing.T) {
	if _, e := exec.LookPath("pwsh"); e != nil {
		t.Skip("pwsh not installed")
	}
	a := testApp(t)
	m := sourceAddon(t, a, "ps_test", "powershell", `$q=[Console]::In.ReadToEnd()|ConvertFrom-Json;@{ok=$true;variables=@{result=$q.value}}|ConvertTo-Json -Compress`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reply, e := a.pluginCall(ctx, m, "execute", "text", "PowerShell Привіт")
	if e != nil || reply["variables"].(map[string]any)["result"] != "PowerShell Привіт" {
		t.Fatal(reply, e)
	}
}

func TestBundledSourceAddonsPowerShell(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not installed")
	}
	a := testApp(t)
	for _, id := range []string{"pcstats", "notifications", "example"} {
		directory := filepath.Join("..", "addons", id)
		if id == "example" {
			directory = filepath.Join("..", "modules", id)
		}
		directory, _ = filepath.Abs(directory)
		b, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m Module
		if err = json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		m.directory = directory
		m.Entry = filepath.Join(directory, m.Entry)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		reply, err := a.pluginCall(ctx, m, "variables", "", nil)
		cancel()
		if err != nil {
			t.Fatal(id, err)
		}
		if id == "pcstats" && len(reply["variables"].(map[string]any)) != 0 {
			t.Fatal("statistics not off by default")
		}
		if id == "example" {
			ctx, cancel = context.WithTimeout(context.Background(), 8*time.Second)
			reply, err = a.pluginCall(ctx, m, "execute", "text", "Шаблон працює")
			cancel()
			if err != nil || reply["variables"].(map[string]any)["message"] != "Шаблон працює" {
				t.Fatal(reply, err)
			}
		}
	}
}
