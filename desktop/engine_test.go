package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a := &App{root: t.TempDir(), admin: token(), devices: map[string]string{}, attempts: map[string]int{}, running: map[string]bool{}, cfg: defaults()}
	a.cfg.Settings.CacheMacros = true
	if e := a.save(a.cfg); e != nil {
		t.Fatal(e)
	}
	return a
}
func request(a *App, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.168.0.7:1234"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	return w
}
func TestPairingPermissionsAndRevoke(t *testing.T) {
	a := testApp(t)
	a.pairing = "123456"
	a.pairUntil = time.Now().Add(time.Minute).Unix()
	w := request(a, "POST", "/api/pair", `{"code":"123456","name":"Phone"}`, "")
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var q map[string]string
	json.Unmarshal(w.Body.Bytes(), &q)
	tok := q["token"]
	if tok == "" {
		t.Fatal("no token")
	}
	for _, path := range []string{"/api/config", "/api/status"} {
		if request(a, "GET", path, "", tok).Code != 401 {
			t.Fatal("phone accessed admin endpoint")
		}
	}
	if request(a, "GET", "/api/panels", "", tok).Code != 200 {
		t.Fatal("phone cannot load panels")
	}
	if request(a, "POST", "/api/pair", `{"code":"123456","name":"Second"}`, "").Code != 403 {
		t.Fatal("reused code")
	}
	if request(a, "POST", "/api/revoke", `{"id":"`+tok+`"}`, a.admin).Code != 200 {
		t.Fatal("revoke")
	}
	if request(a, "GET", "/api/panels", "", tok).Code != 401 {
		t.Fatal("revoked token accepted")
	}
}
func TestCannotInjectCommandsFromPhone(t *testing.T) {
	a := testApp(t)
	a.devices["phone"] = "Phone"
	w := request(a, "POST", "/api/control", `{"panel":"phone","control":"a","value":"x","command":"evil"}`, "phone")
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = request(a, "POST", "/api/run", `{"id":"calc"}`, "phone")
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestExpiredAndRateLimitedCode(t *testing.T) {
	a := testApp(t)
	a.pairing = "123456"
	a.pairUntil = time.Now().Add(-time.Minute).Unix()
	if request(a, "POST", "/api/pair", `{"code":"123456","name":"P"}`, "").Code != 403 {
		t.Fatal("expired accepted")
	}
	a.pairUntil = time.Now().Add(time.Minute).Unix()
	for i := 0; i < 5; i++ {
		request(a, "POST", "/api/pair", `{"code":"000000","name":"P"}`, "")
	}
	if request(a, "POST", "/api/pair", `{"code":"123456","name":"P"}`, "").Code != 403 {
		t.Fatal("rate limit failed")
	}
}
func TestConfigAndOrigin(t *testing.T) {
	a := testApp(t)
	c := defaults()
	c.Macros = append(c.Macros, c.Macros[0])
	if a.save(c) == nil {
		t.Fatal("duplicate ID accepted")
	}
	c = defaults()
	c.Floating[0].Hotkey = "CTRL+???"
	if a.save(c) == nil {
		t.Fatal("invalid hotkey accepted")
	}
	r := httptest.NewRequest("POST", "/api/run", strings.NewReader(`{"id":"calc"}`))
	r.Header.Set("Authorization", "Bearer "+a.admin)
	r.Header.Set("Origin", "http://evil.test")
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatal(w.Code)
	}
}
func TestMacroVariableAndBusy(t *testing.T) {
	a := testApp(t)
	a.cfg.Macros = append(a.cfg.Macros, Macro{ID: "var", Name: "Variable", Steps: []Step{{Type: "variable", Value: "test", Args: []string{"${input}"}}}})
	if e := a.run("missing", nil); e == nil {
		t.Fatal("missing macro ran")
	}
	a.running["var"] = true
	if e := a.run("var", nil); e == nil {
		t.Fatal("busy macro ran")
	}
	delete(a.running, "var")
	if e := a.run("var", map[string]string{"input": "correct"}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 100; i++ {
		a.mu.RLock()
		v := a.cfg.Variables["test"]
		a.mu.RUnlock()
		if v == "correct" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("variable expansion failed")
}

func TestValidateImportDoesNotApply(t *testing.T) {
	a := testApp(t)
	before := a.config()
	next := a.config()
	next.Settings.Accent = "#ff0000"
	next.Variables["imported"] = "yes"
	body, _ := json.Marshal(next)
	if w := request(a, "POST", "/api/config/validate", string(body), a.admin); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if a.config().Settings.Accent != before.Settings.Accent || a.config().Variables["imported"] != "" {
		t.Fatal("dry validation changed configuration")
	}
	a.devices["phone"] = "Phone"
	if w := request(a, "POST", "/api/config/validate", string(body), "phone"); w.Code != 401 {
		t.Fatal("phone validated admin config", w.Code)
	}
	if w := request(a, "POST", "/api/config/validate", string(body)+`{}`, a.admin); w.Code != 400 {
		t.Fatal("trailing JSON accepted")
	}
	next.Smart[0].Controls = nil
	body, _ = json.Marshal(next)
	if w := request(a, "POST", "/api/config/validate", string(body), a.admin); w.Code != 400 {
		t.Fatal("null controls accepted")
	}
}

func TestRadialSectors(t *testing.T) {
	for _, q := range []struct {
		x, y    float64
		n, want int
	}{{0, 0, 4, -1}, {30, 30, 4, -1}, {0, -100, 4, 0}, {100, 0, 4, 1}, {0, 100, 4, 2}, {-100, 0, 4, 3}, {2000, 0, 4, 1}, {0, -100, 0, -1}, {0, -100, 1, 0}} {
		if got := radialIndex(q.x, q.y, q.n); got != q.want {
			t.Fatalf("radialIndex(%v,%v,%v)=%v, want %v", q.x, q.y, q.n, got, q.want)
		}
	}
}

func TestStorageRoundTripAndLayout(t *testing.T) {
	a := testApp(t)
	c := a.config()
	cell := 10
	c.Smart[0].Rows = 5
	c.Smart[0].Controls[0].Cell = &cell
	c.Smart[0].Controls[0].Appearance = Appearance{Text: "#ffffff", Border: "#123456", Background: "#202020", Animation: "pulse", Effect: "glow", Press: "ripple"}
	c.Macros[0].Puzzles = []Puzzle{{ID: "piece", Type: "actions.condition", Category: "actions", Value: "draft", X: 96, Y: 48}}
	if e := a.save(c); e != nil {
		t.Fatal(e)
	}
	got, e := loadStorage(a.root)
	if e != nil {
		t.Fatal(e)
	}
	if *got.Smart[0].Controls[0].Cell != 10 || got.Smart[0].Controls[0].Appearance.Press != "ripple" || got.Macros[0].Puzzles[0].Type != "actions.condition" {
		t.Fatal("storage did not preserve puzzles/layout/style")
	}
	for _, dir := range []string{"macros", "setting", "icon", "image", "plagin"} {
		if s, e := os.Stat(filepath.Join(a.root, dir)); e != nil || !s.IsDir() {
			t.Fatal("missing data directory", dir)
		}
	}
	bytes, e := os.ReadFile(filepath.Join(a.root, "setting", "settings.sqlite"))
	if e != nil || !strings.HasPrefix(string(bytes), "SQLite format 3") {
		t.Fatal("not a real SQLite database")
	}
	if e = a.run(c.Macros[0].ID, nil); e == nil {
		t.Fatal("puzzle draft was executed")
	}
	c.Smart[0].Controls[1].Cell = &cell
	if e = a.save(c); e == nil {
		t.Fatal("overlapping cells accepted")
	}
	unchanged, e := loadStorage(a.root)
	if e != nil || len(unchanged.Macros[0].Puzzles) != 1 {
		t.Fatal("invalid save changed stored profile")
	}
}

func TestLocalSymbolImportAndNoButtonVariables(t *testing.T) {
	a := testApp(t)
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M1 1h10v10H1z" fill="red"/></svg>`
	body, _ := json.Marshal(map[string]string{"kind": "icon", "data": base64.StdEncoding.EncodeToString([]byte(svg))})
	w := request(a, "POST", "/api/media/import", string(body), a.admin)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	if !strings.HasSuffix(result["path"], ".svg") {
		t.Fatal(result)
	}
	if _, e := os.Stat(filepath.Join(a.root, "icon", result["name"])); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{`<svg><script>alert(1)</script></svg>`, `<svg onload="x()"></svg>`, `<svg><image href="https://example.org/x"/></svg>`} {
		if validateSVG([]byte(bad)) == nil {
			t.Fatal("unsafe SVG accepted")
		}
	}
	if iconURLAllowed("https://127.0.0.1/x.svg") || iconURLAllowed("http://svgl.app/x.svg") || iconURLAllowed("https://api.iconify.design@evil.test/x") {
		t.Fatal("unsupported URL accepted")
	}
	if !iconURLAllowed("https://api.iconify.design/mdi/eye.svg") {
		t.Fatal("valid icon URL rejected")
	}
	a.cfg.Smart[0].Controls[0].Variable = "test"
	a.cfg.Smart[0].Controls[0].Kind = "input"
	a.cfg.Macros[0].Steps = []Step{{Type: "delay", MS: 1}}
	a.devices["phone"] = "Phone"
	w = request(a, "POST", "/api/control", `{"panel":"phone","control":"a","value":"must not be a variable"}`, "phone")
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	if a.config().Variables["test"] != "" {
		t.Fatal("button wrote a variable")
	}
}

func TestInputGraphBranchesAndDetached(t *testing.T) {
	m := Macro{ID: "g", Name: "Graph", Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "input", Type: "input", Mode: "toggle", TrueNext: "yes", FalseNext: "no"}, {ID: "yes", Type: "text.insert", Next: "after"}, {ID: "after", Type: "actions.delay"}, {ID: "no", Type: "win.program"}, {ID: "detached", Type: "win.command"}}}
	for _, q := range []struct {
		v    string
		want string
	}{{"true", "yes,after"}, {"false", "no"}} {
		path, _, e := graphPath(m, InputEvent{Kind: "toggle", Value: q.v})
		if e != nil {
			t.Fatal(e)
		}
		ids := []string{}
		for _, p := range path {
			ids = append(ids, p.ID)
		}
		if strings.Join(ids, ",") != q.want {
			t.Fatal(ids)
		}
	}
	m.Puzzles[2].Next = "yes"
	if validateGraph(m) == nil {
		t.Fatal("cycle accepted")
	}
	m.Puzzles[2].Next = ""
	m.Puzzles[3].Next = "yes"
	if validateGraph(m) == nil {
		t.Fatal("multiple parents accepted")
	}
	m.Puzzles[3].Next = ""
	m.Puzzles[0].TrueNext = "missing"
	if validateGraph(m) == nil {
		t.Fatal("broken edge accepted")
	}
}
func TestSliderInputWritesBeforeBranch(t *testing.T) {
	a := testApp(t)
	m := Macro{ID: "slider", Name: "Slider", Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "start", Type: "input", Mode: "slider", Max: 250, Variable: "volume", Next: "action"}, {ID: "action", Type: "actions.get"}, {ID: "loose", Type: "text.insert"}}}
	a.cfg.Macros = append(a.cfg.Macros, m)
	a.cfg.Smart[0].Controls[0].Kind = "slider"
	a.cfg.Smart[0].Controls[0].Macro = "slider"
	a.devices["phone"] = "Phone"
	if w := request(a, "POST", "/api/control", `{"panel":"phone","control":"a","value":"125"}`, "phone"); w.Code != 200 {
		t.Fatal(w.Body)
	}
	for i := 0; i < 100; i++ {
		if a.config().Variables["volume"] == "125" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if a.config().Variables["volume"] != "125" {
		t.Fatal("slider variable not written")
	}
	a.mu.RLock()
	state := a.controlStates["phone/a"]
	logs := strings.Join(a.logs, "\n")
	a.mu.RUnlock()
	if state.Value != 125 {
		t.Fatal(state)
	}
	if strings.Contains(logs, "loose") {
		t.Fatal("detached puzzle visited")
	}
	if w := request(a, "POST", "/api/control", `{"panel":"phone","control":"a","value":"251"}`, "phone"); w.Code != 400 {
		t.Fatal("out-of-range accepted", w.Code)
	}
}
