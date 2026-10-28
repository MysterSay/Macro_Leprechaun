package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFloatingThemesInstallExtendAndContainment(t *testing.T) {
	a := testApp(t)
	if err := installFloatingEffects(a.root); err != nil {
		t.Fatal(err)
	}
	engine := httptest.NewRecorder()
	a.floatingAsset(engine, httptest.NewRequest("GET", "/floating-effects/_engine/menu-v1.css", nil))
	if engine.Code != 200 || !strings.Contains(engine.Header().Get("Content-Type"), "text/css") {
		t.Fatal("engine CSS missing", engine.Code)
	}
	themes := a.floatingThemes()
	if len(themes) != 6 {
		t.Fatal("expected 5 visual + 1 sound", len(themes))
	}
	base := filepath.Join(a.root, "effects", "floating")
	custom := filepath.Join(base, "my-theme")
	os.MkdirAll(custom, 0700)
	os.WriteFile(filepath.Join(custom, "theme.json5"), []byte("{name:'My Theme',engineVersion:1,layers:[],colors:{}}"), 0600)
	os.WriteFile(filepath.Join(custom, "theme.css"), []byte(".menu-node{color:red}"), 0600)
	if len(a.floatingThemes()) != 7 {
		t.Fatal("custom folder was not discovered")
	}
	// Installing again must preserve edited styles.
	defaultCSS := filepath.Join(base, "default", "theme.css")
	os.WriteFile(defaultCSS, []byte("custom"), 0600)
	if err := installFloatingEffects(a.root); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(defaultCSS)
	if string(raw) != "custom" {
		t.Fatal("user style overwritten")
	}
	for _, path := range []string{"../default/theme.css", "../../setting/settings.sqlite"} {
		if _, err := readEffectFile(base, "my-theme", path); err == nil {
			t.Fatal("path traversal", path)
		}
	}
	external := filepath.Join(t.TempDir(), "secret.css")
	os.WriteFile(external, []byte("secret"), 0600)
	if err := os.Symlink(external, filepath.Join(custom, "link.css")); err == nil {
		if _, err := readEffectFile(base, "my-theme", "link.css"); err == nil {
			t.Fatal("symlink escaped theme")
		}
	}
	w := httptest.NewRecorder()
	a.floatingAsset(w, httptest.NewRequest("GET", "/floating-effects/my-theme/theme.css", nil))
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "text/css") {
		t.Fatal(w.Code, w.Body)
	}
}
func TestFloatingBridgeAuthenticationActivationAndNavigation(t *testing.T) {
	a := testApp(t)
	c := a.runtimeConfig()
	m := Macro{ID: "kando", Name: "Kando", Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "root", Type: "input", Mode: "button", Next: "write"}, {ID: "write", Type: "actions.set", Value: actionPuzzle("actions.set", AppAction{Variable: "result", Source: "static", Value: "selected"}).Value}}}
	c.Macros = append(c.Macros, m)
	p := &c.Floating[0]
	p.Controls = []Control{{ID: "target", Kind: "button", Macro: m.ID}}
	p.FloatingStyle = &FloatingConfig{Theme: "default", Options: map[string]any{"enableMenuAnimations": true, "zoomFactor": 1.25, "keepInputFocus": true}}
	c.Variables["result"] = ""
	if err := a.save(c); err != nil {
		t.Fatal(err)
	}
	if w := request(a, "GET", "/api/floating/themes", "", ""); w.Code != 401 {
		t.Fatal("themes available without admin", w.Code)
	}
	q, _ := json.Marshal(map[string]string{"host": p.ID, "panel": p.ID, "control": "target"})
	if w := request(a, "POST", "/api/floating/activate", string(q), a.admin); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	deadline := time.Now().Add(time.Second)
	for a.variablesSnapshot()["result"] != "selected" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if a.variablesSnapshot()["result"] != "selected" {
		t.Fatal("bridge did not execute macro")
	}
	c = a.runtimeConfig()
	child := c.Floating[0]
	child.ID = "child"
	child.Parent = p.ID
	c.Floating = append(c.Floating, child)
	if err := a.save(c); err != nil {
		t.Fatal(err)
	}
	if err := a.navigate(RunScope{Session: "desktop", Mode: "floating", Host: p.ID, Panel: p.ID}, "change", "child"); err != nil {
		t.Fatal(err)
	}
	if w := request(a, "POST", "/api/floating/activate", string(q), a.admin); w.Code != 409 {
		t.Fatal("stale panel accepted", w.Code)
	}
	w := request(a, "GET", "/api/floating/state?host="+p.ID, "", a.admin)
	var data struct {
		Panel Panel `json:"panel"`
	}
	json.Unmarshal(w.Body.Bytes(), &data)
	if w.Code != 200 || data.Panel.ID != "child" {
		t.Fatal(w.Code, w.Body)
	}
	c.Floating[0].FloatingStyle.Options["zoomFactor"] = "invalid"
	if err := a.save(c); err == nil {
		t.Fatal("wrong option type accepted")
	}
}
