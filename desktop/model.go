package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Step struct {
	Type  string   `json:"type"`
	Value string   `json:"value"`
	Args  []string `json:"args,omitempty"`
	MS    int      `json:"ms,omitempty"`
}
type Puzzle struct {
	Body      string  `json:"body,omitempty"`
	Next      string  `json:"next,omitempty"`
	TrueNext  string  `json:"trueNext,omitempty"`
	FalseNext string  `json:"falseNext,omitempty"`
	Mode      string  `json:"mode,omitempty"`
	Max       float64 `json:"max,omitempty"`
	Variable  string  `json:"variable,omitempty"`
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Category  string  `json:"category"`
	Value     string  `json:"value"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
}
type Appearance struct {
	SliderOrientation string   `json:"sliderOrientation,omitempty"`
	SliderMax         float64  `json:"sliderMax,omitempty"`
	ImageFit          string   `json:"imageFit,omitempty"`
	SliderStyle       string   `json:"sliderStyle,omitempty"`
	SliderMode        string   `json:"sliderMode,omitempty"`
	SliderInterval    *float64 `json:"sliderInterval,omitempty"`
	SliderTrack       string   `json:"sliderTrack,omitempty"`
	SliderThumb       string   `json:"sliderThumb,omitempty"`
	SliderThumbSize   int      `json:"sliderThumbSize,omitempty"`
	Activation        *float64 `json:"activation,omitempty"`
	Image             string   `json:"image,omitempty"`
	Text              string   `json:"text,omitempty"`
	Border            string   `json:"border,omitempty"`
	Background        string   `json:"background,omitempty"`
	Animation         string   `json:"animation,omitempty"`
	Effect            string   `json:"effect,omitempty"`
	Press             string   `json:"press,omitempty"`
}
type Macro struct {
	Lazy         bool          `json:"lazy,omitempty"`
	File         string        `json:"file,omitempty"`
	Summary      *MacroSummary `json:"summary,omitempty"`
	MissingFile  string        `json:"missingFile,omitempty"`
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Icon         string        `json:"icon"`
	Steps        []Step        `json:"steps"`
	Puzzles      []Puzzle      `json:"puzzles,omitempty"`
	GraphVersion int           `json:"graphVersion,omitempty"`
}
type Control struct {
	Input      string     `json:"input,omitempty"`
	ColSpan    int        `json:"colSpan,omitempty"`
	RowSpan    int        `json:"rowSpan,omitempty"`
	SplitAxis  string     `json:"splitAxis,omitempty"`
	Segments   []Control  `json:"segments,omitempty"`
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Icon       string     `json:"icon"`
	Kind       string     `json:"kind"`
	Macro      string     `json:"macro"`
	OffMacro   string     `json:"offMacro,omitempty"`
	Variable   string     `json:"variable,omitempty"`
	Color      string     `json:"color"`
	Cell       *int       `json:"cell,omitempty"`
	Appearance Appearance `json:"appearance,omitempty"`
}
type Panel struct {
	FloatingStyle *FloatingConfig `json:"floatingStyle,omitempty"`
	Radial        *RadialStyle    `json:"radial,omitempty"`
	Material      string          `json:"material,omitempty"`
	Parent        string          `json:"parent,omitempty"`
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Enabled       bool            `json:"enabled"`
	Columns       int             `json:"columns"`
	Rows          int             `json:"rows,omitempty"`
	Background    string          `json:"background,omitempty"`
	Edge          string          `json:"edge"`
	Monitor       int             `json:"monitor"`
	Position      int             `json:"position"`
	Thickness     int             `json:"thickness"`
	Hotkey        string          `json:"hotkey"`
	Radius        int             `json:"radius"`
	Controls      []Control       `json:"controls"`
}
type Settings struct {
	CacheMacros         bool   `json:"cacheMacros"`
	PollUnusedVariables bool   `json:"pollUnusedVariables"`
	DisabledSmart       bool   `json:"disabledSmart"`
	DisabledWidgets     bool   `json:"disabledWidgets"`
	DisabledFloating    bool   `json:"disabledFloating"`
	Autostart           bool   `json:"autostart"`
	AllowLoops          bool   `json:"allowLoops"`
	Language            string `json:"language"`
	Theme               string `json:"theme"`
	Accent              string `json:"accent"`
	Port                int    `json:"port"`
}
type Config struct {
	ModuleSettings map[string]map[string]any `json:"moduleSettings,omitempty"`
	Settings       Settings                  `json:"settings"`
	Smart          []Panel                   `json:"smart"`
	Widgets        []Panel                   `json:"widgets"`
	Floating       []Panel                   `json:"floating"`
	Macros         []Macro                   `json:"macros"`
	Variables      map[string]string         `json:"variables"`
	Modules        map[string]bool           `json:"modules"`
}
type App struct {
	providerMu         sync.Mutex
	providerLast       map[string]time.Time
	networkLast        time.Time
	variableWatchUntil time.Time
	providerErrors     map[string]string
	cancels            map[string]context.CancelFunc
	moduleVars         map[string]string
	systemVars         map[string]string
	navigation         map[string]Navigation
	mu                 sync.RWMutex
	cfg                Config
	root, admin        string
	devices            map[string]string
	pairing            string
	pairUntil          int64
	attempts           map[string]int
	logs               []string
	running            map[string]bool
	moduleList         []Module
	controlStates      map[string]ControlState
}

func token() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func defaults() Config {
	c := Config{Settings: Settings{Language: "uk", Theme: "dark", Accent: "#6dde91", Port: 47640}, Variables: map[string]string{"name": "Олександр", "volume": "50"}, Modules: map[string]bool{}}
	c.Macros = []Macro{{ID: "notepad", Name: "Блокнот", Icon: "▤", Steps: []Step{{Type: "launch", Value: "notepad.exe"}}}, {ID: "calc", Name: "Калькулятор", Icon: "⊞", Steps: []Step{{Type: "launch", Value: "calc.exe"}}}, {ID: "play", Name: "Медіа: пауза / відтворення", Icon: "▶", Steps: []Step{{Type: "keys", Value: "MEDIA_PLAY_PAUSE"}}}, {ID: "mute", Name: "Медіа: вимкнути звук", Icon: "◖", Steps: []Step{{Type: "keys", Value: "VOLUME_MUTE"}}}, {ID: "hello", Name: "Вставити привітання", Icon: "✎", Steps: []Step{{Type: "text", Value: "Привіт, ${name}!"}}}}
	controls := []Control{{ID: "a", Label: "Блокнот", Icon: "▤", Kind: "button", Macro: "notepad", Color: "#6dde91"}, {ID: "b", Label: "Калькулятор", Icon: "⊞", Kind: "button", Macro: "calc", Color: "#8db6ff"}, {ID: "c", Label: "Відтворення", Icon: "▶", Kind: "button", Macro: "play", Color: "#d9a4ff"}, {ID: "d", Label: "Без звуку", Icon: "◖", Kind: "button", Macro: "mute", Color: "#ffbd7b"}, {ID: "e", Label: "Привітання", Icon: "✎", Kind: "button", Macro: "hello", Color: "#6dde91"}}
	c.Smart = []Panel{{ID: "phone", Name: "Головна", Enabled: true, Columns: 3, Controls: append([]Control{}, controls...)}}
	c.Widgets = []Panel{{ID: "edge", Name: "Швидкий доступ", Enabled: true, Columns: 3, Edge: "right", Position: 50, Thickness: 4, Radius: 180, Controls: append([]Control{}, controls[:4]...)}}
	c.Floating = []Panel{{ID: "radial", Name: "Основне меню", Enabled: true, Columns: 3, Hotkey: "CTRL+ALT+SPACE", Radius: 160, Controls: append([]Control{}, controls...)}}
	return c
}

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

func validate(c Config) error {
	if c.Smart == nil || c.Widgets == nil || c.Floating == nil || c.Macros == nil {
		return fmt.Errorf("smart, widgets, floating and macros must be arrays")
	}
	if c.Variables == nil || c.Modules == nil {
		return fmt.Errorf("variables and modules must be JSON objects")
	}
	if c.Settings.Port < 1024 || c.Settings.Port > 65535 {
		return fmt.Errorf("port: 1024..65535")
	}
	if c.Settings.Theme != "dark" && c.Settings.Theme != "light" {
		return fmt.Errorf("theme")
	}
	if !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(c.Settings.Accent) {
		return fmt.Errorf("accent: #RRGGBB")
	}
	if !idPattern.MatchString(c.Settings.Language) {
		return fmt.Errorf("language")
	}
	if len(c.Macros) > 500 || len(c.Widgets) > 20 || len(c.Floating) > 20 || len(c.Smart) > 30 {
		return fmt.Errorf("too many items")
	}
	ids := map[string]bool{}
	for _, m := range c.Macros {
		if !idPattern.MatchString(m.ID) || ids[m.ID] || len(m.Steps) > 100 || len(m.Puzzles) > 100 {
			return fmt.Errorf("invalid/duplicate macro ID or >100 steps")
		}
		ids[m.ID] = true
		if m.MissingFile != "" {
			if filepath.Base(m.MissingFile) != m.MissingFile || len(m.Puzzles) > 0 || len(m.Steps) > 0 || m.GraphVersion != 0 {
				return fmt.Errorf("invalid missing macro")
			}
			continue
		}
		if m.Lazy {
			if m.File == "" || filepath.Base(m.File) != m.File || m.Summary == nil || len(m.Puzzles) != 0 || len(m.Steps) != 0 {
				return fmt.Errorf("invalid macro catalog")
			}
			continue
		}
		pi := map[string]bool{}
		for _, p := range m.Puzzles {
			if !idPattern.MatchString(p.ID) || pi[p.ID] || len(p.Type) > 160 || p.Type == "" || p.X < 0 || p.Y < 0 || p.X > 10000 || p.Y > 10000 || len(p.Value) > 16384 {
				return fmt.Errorf("invalid puzzle")
			}
			pi[p.ID] = true
		}
		if m.GraphVersion != 0 && m.GraphVersion != 1 {
			return fmt.Errorf("unsupported graph version")
		}
		if m.GraphVersion == 1 {
			if e := validateGraph(m); e != nil {
				return e
			}
		}
		if m.Steps == nil {
			return fmt.Errorf("steps must be an array")
		}
		for _, s := range m.Steps {
			switch s.Type {
			case "launch", "shell", "text", "keys", "delay", "variable", "module":
			default:
				return fmt.Errorf("unknown step: %s", s.Type)
			}
			if s.MS < 0 || s.MS > 60000 {
				return fmt.Errorf("delay: 0..60000")
			}
		}
	}
	hot := map[string]bool{}
	for mode, panels := range [][]Panel{c.Smart, c.Widgets, c.Floating} {
		pids := map[string]bool{}
		for _, p := range panels {
			if !idPattern.MatchString(p.ID) || pids[p.ID] {
				return fmt.Errorf("invalid/duplicate panel ID")
			}
			pids[p.ID] = true
			if p.Rows < 0 || p.Rows > 30 {
				return fmt.Errorf("rows: 1..30")
			}
			if p.Controls == nil {
				return fmt.Errorf("controls must be an array")
			}
			if p.Columns < 1 || p.Columns > 8 || len(p.Controls) > 48 {
				return fmt.Errorf("columns: 1..8; controls: max 48")
			}
			if mode == 1 && (p.Thickness < 2 || p.Thickness > 12 || p.Position < 0 || p.Position > 100 || p.Monitor < 0 || p.Monitor > 16 || (p.Edge != "left" && p.Edge != "right" && p.Edge != "top" && p.Edge != "bottom")) {
				return fmt.Errorf("widget geometry")
			}
			if mode == 2 {
				if err := validateFloating(p.FloatingStyle); err != nil {
					return err
				}
				if p.Radial != nil {
					if err := validateRadialStyle(*p.Radial); err != nil {
						return err
					}
				}
				if p.Radius < 90 || p.Radius > 350 || len(p.Controls) > 12 {
					return fmt.Errorf("radial radius 90..350, max 12 controls")
				}
				if p.Parent == "" {
					h := strings.ToUpper(p.Hotkey)
					if _, _, e := parseHotkey(h); e != nil {
						return e
					}
					if p.Enabled && hot[h] {
						return fmt.Errorf("duplicate hotkey")
					}
					if p.Enabled {
						hot[h] = true
					}
				}
			}
			if err := validateControlGeometry(p, mode); err != nil {
				return err
			}
			ci := map[string]bool{}
			for _, b := range controlLeaves(p.Controls) {
				if !idPattern.MatchString(b.ID) || ci[b.ID] {
					return fmt.Errorf("invalid/duplicate control ID")
				}
				ci[b.ID] = true
				if b.Appearance.Activation != nil && (*b.Appearance.Activation < 0 || *b.Appearance.Activation > 60) {
					return fmt.Errorf("activation duration: 0..60 seconds")
				}

				for _, v := range []string{b.Appearance.Text, b.Appearance.Border, b.Appearance.Background, b.Appearance.SliderTrack, b.Appearance.SliderThumb} {
					if v != "" && !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(v) {
						return fmt.Errorf("appearance color must be #RRGGBB")
					}
				}
				if !oneOf(b.Appearance.Animation, "", "none", "pulse", "spin", "float") || !oneOf(b.Appearance.Effect, "", "none", "glow", "shadow", "glass", "transparent") || !oneOf(b.Appearance.Press, "", "scale", "none", "flash", "ripple") {
					return fmt.Errorf("unknown appearance effect")
				}
				if !oneOf(b.Appearance.ImageFit, "", "stretch", "contain", "cover") || !oneOf(b.Appearance.SliderStyle, "", "standard", "thin", "filled", "vertical") || !oneOf(b.Appearance.SliderMode, "", "after", "online") {
					return fmt.Errorf("unknown slider/image style")
				}
				if v := b.Appearance.SliderInterval; v != nil && !(*v >= 0.05 && *v <= 60) {
					return fmt.Errorf("slider interval: 0.05..60 seconds")
				}
				if v := b.Appearance.SliderThumbSize; v != 0 && (v < 8 || v > 40) {
					return fmt.Errorf("slider thumb: 8..40")
				}
				if !oneOf(b.Appearance.SliderOrientation, "", "horizontal", "vertical") || b.Appearance.SliderMax < 0 || b.Appearance.SliderMax > 1e9 {
					return fmt.Errorf("invalid slider orientation/maximum")
				}
				if b.Kind == "slider" && b.Variable != "" && (readonlyVariable(b.Variable) || !idPattern.MatchString(b.Variable)) {
					return fmt.Errorf("slider variable is read-only or invalid")
				}
				if mode == 2 && b.Kind != "button" {
					return fmt.Errorf("native panels support buttons; forms belong to smart panels")
				}
				if b.Macro != "" && !ids[b.Macro] {
					return fmt.Errorf("unknown macro: %s", b.Macro)
				}
				if b.Input != "" {
					matched := false
					for _, macro := range c.Macros {
						if macro.ID == b.Macro {
							if root := rootInputFor(macro, b.Input); macro.MissingFile != "" || root != nil && root.Mode == b.Kind {
								matched = true
							}
						}
					}
					if !matched {
						return fmt.Errorf("control input does not match macro")
					}
				}
				if b.OffMacro != "" && !ids[b.OffMacro] {
					return fmt.Errorf("unknown offMacro")
				}
				if b.Kind != "button" && b.Kind != "toggle" && b.Kind != "slider" && b.Kind != "input" {
					return fmt.Errorf("control kind")
				}

			}
		}
	}
	for _, panels := range [][]Panel{c.Smart, c.Widgets, c.Floating} {
		roots := 0
		for _, p := range panels {
			if p.Parent == "" {
				roots++
			}
		}
		if roots > 1 {
			return fmt.Errorf("кожен режим може мати лише одну кореневу панель")
		}
		parents := map[string]string{}
		for _, p := range panels {
			parents[p.ID] = p.Parent
		}
		for _, p := range panels {
			seen := map[string]bool{p.ID: true}
			for parent := p.Parent; parent != ""; parent = parents[parent] {
				if _, ok := parents[parent]; !ok || seen[parent] {
					return fmt.Errorf("invalid panel parent or cycle")
				}
				seen[parent] = true
			}
		}
	}

	return nil
}
func (a *App) save(c Config) error {
	c.Macros = append([]Macro{}, c.Macros...)
	for k := range c.Variables {
		if readonlyVariable(k) {
			delete(c.Variables, k)
		}
	}
	for i := range c.Widgets {
		c.Widgets[i].Enabled = true
	}
	for i := range c.Floating {
		c.Floating[i].Enabled = true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.canonicalizeCatalog(&c); err != nil {
		return err
	}
	if err := validate(c); err != nil {
		return err
	}
	if e := writeStorage(a.root, c); e != nil {
		return e
	}
	if err := applyMacroPolicy(a.root, &c); err != nil {
		return err
	}
	a.cfg = c
	if err := platformAutostart(c.Settings.Autostart); err != nil {
		log.Print("Autostart: ", err)
	}
	return nil
}
func (a *App) config() Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	b, _ := json.Marshal(a.cfg)
	var c Config
	_ = json.Unmarshal(b, &c)
	a.addVariableValuesLocked(&c)
	return c
}
func (a *App) log(s string) {
	log.Print(s)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logs = append(a.logs, s)
	if len(a.logs) > 100 {
		a.logs = a.logs[len(a.logs)-100:]
	}
}
func parseHotkey(s string) (uint32, uint32, error) {
	var mod uint32 = 0x4000
	var key uint32
	for _, p := range strings.Split(strings.ToUpper(s), "+") {
		switch p {
		case "CTRL":
			mod |= 2
		case "ALT":
			mod |= 1
		case "SHIFT":
			mod |= 4
		case "WIN":
			mod |= 8
		default:
			if key != 0 {
				return 0, 0, fmt.Errorf("one hotkey key required")
			}
			if p == "SPACE" {
				key = 32
			} else if len(p) == 1 && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= '0' && p[0] <= '9')) {
				key = uint32(p[0])
			} else if strings.HasPrefix(p, "F") {
				var n int
				fmt.Sscanf(p, "F%d", &n)
				if n >= 1 && n <= 24 {
					key = uint32(111 + n)
				}
			}
			if key == 0 {
				return 0, 0, fmt.Errorf("unsupported hotkey: %s", s)
			}
		}
	}
	if key == 0 {
		return 0, 0, fmt.Errorf("hotkey needs a key")
	}
	return mod, key, nil
}

func oneOf(v string, values ...string) bool {
	for _, s := range values {
		if v == s {
			return true
		}
	}
	return false
}

// Migrate old independent roots into a single explicit tree without dropping panels.
func normalizePanelRoots(c *Config) {
	for _, panels := range [][]Panel{c.Smart, c.Widgets, c.Floating} {
		root := ""
		for i := range panels {
			if panels[i].Parent == "" {
				if root == "" {
					root = panels[i].ID
				} else {
					panels[i].Parent = root
				}
			}
		}
	}
}

func (a *App) addVariableValuesLocked(target *Config) {
	c := *target
	for _, module := range a.moduleList {
		for _, v := range module.Variables {
			if variableEnabled(c, module, v) {
				key := module.ID + "__" + v.ID
				if _, ok := c.Variables[key]; !ok {
					c.Variables[key] = "—"
				}
			}
		}
	}
	for k, v := range a.systemVars {
		c.Variables[k] = v
	}
	for _, module := range a.moduleList {
		for _, variable := range module.Variables {
			if variableEnabled(c, module, variable) {
				key := module.ID + "__" + variable.ID
				if value, ok := a.moduleVars[key]; ok {
					c.Variables[key] = value
				}
			}
		}
	}
	if a.systemVars != nil {
		now := time.Now()
		c.Variables["system__time"] = now.Format("15:04:05")
		c.Variables["system__date"] = now.Format("2006-01-02")
	}
	*target = c
}
