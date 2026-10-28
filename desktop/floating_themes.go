package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:effects
var floatingDefaults embed.FS

type FloatingConfig struct {
	Theme   string            `json:"theme"`
	Colors  map[string]string `json:"colors,omitempty"`
	Options map[string]any    `json:"options,omitempty"`
}
type FloatingTheme struct {
	ID          string                     `json:"id"`
	Description string                     `json:"description"`
	Presets     map[string]json.RawMessage `json:"presets"`
	Error       string                     `json:"error,omitempty"`
}

func installFloatingEffects(root string) error {
	base := filepath.Join(root, "effects", "floating")
	return fs.WalkDir(floatingDefaults, "effects", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(base, strings.TrimPrefix(path, "effects"))
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		raw, err := floatingDefaults.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0600)
	})
}
func (a *App) floatingThemes() []FloatingTheme {
	base := filepath.Join(a.root, "effects", "floating")
	entries, _ := os.ReadDir(base)
	themes := []FloatingTheme{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "_") || !idPattern.MatchString(entry.Name()) {
			continue
		}
		name := entry.Name()
		theme := FloatingTheme{ID: name, Presets: map[string]json.RawMessage{}}
		raw, err := readEffectFile(base, name, "theme.json5")
		if os.IsNotExist(err) {
			raw, err = readEffectFile(base, name, "theme.json")
		}
		if err != nil {
			theme.Error = err.Error()
		} else {
			theme.Description = string(raw)
		}
		presets, _ := os.ReadDir(filepath.Join(base, name, "presets"))
		for _, p := range presets {
			if p.IsDir() || !strings.HasSuffix(p.Name(), ".json") {
				continue
			}
			raw, err := readEffectFile(base, name, "presets/"+p.Name())
			if err == nil && json.Valid(raw) {
				theme.Presets[strings.TrimSuffix(p.Name(), ".json")] = raw
			}
		}
		themes = append(themes, theme)
	}
	return themes
}
func readEffectFile(base, theme, path string) ([]byte, error) {
	if !idPattern.MatchString(theme) || filepath.IsAbs(path) {
		return nil, fmt.Errorf("invalid theme path")
	}
	root := filepath.Join(base, theme)
	target := filepath.Join(root, filepath.FromSlash(path))
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("theme path escapes directory")
	}
	stat, err := os.Stat(realTarget)
	if err != nil {
		return nil, err
	}
	if stat.Size() > 2<<20 {
		return nil, fmt.Errorf("theme asset exceeds 2 MiB")
	}
	return os.ReadFile(realTarget)
}
func (a *App) floatingAsset(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/floating-effects/"), "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(filepath.Ext(parts[1]))
	if !oneOf(ext, ".css", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".woff", ".woff2", ".ttf", ".json", ".json5", ".wav", ".mp3", ".ogg") {
		http.NotFound(w, r)
		return
	}
	raw, err := readEffectFile(filepath.Join(a.root, "effects", "floating"), parts[0], parts[1])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	if ext == ".css" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	if ext == ".svg" {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	}
	w.Write(raw)
}
func validateFloating(c *FloatingConfig) error {
	if c == nil {
		return nil
	}
	if !idPattern.MatchString(c.Theme) {
		return fmt.Errorf("floating theme ID")
	}
	if len(c.Colors) > 64 || len(c.Options) > 80 {
		return fmt.Errorf("too many floating theme options")
	}
	for k, v := range c.Colors {
		if len(k) > 80 || len(v) > 200 {
			return fmt.Errorf("floating color value too long")
		}
	}
	if err := validateFloatingOptions(c.Options); err != nil {
		return err
	}
	raw, _ := json.Marshal(c.Options)
	if len(raw) > 16384 {
		return fmt.Errorf("floating options too large")
	}
	return nil
}

func validateFloatingOptions(options map[string]any) error {
	booleans := " enableSelectionWedges drawQuickSelectKey underlineQuickSelectKey enableMenuAnimations enablePointerReactiveEffects enableMarkingMode enableTurboMode warpMouse returnPointerToMenuOpeningPosition hoverMode hoverModeNeedsConfirmation triggerCenterClickOnKeyRelease rmbSelectsParent keepInputFocus hideOnFocusOut windowsInkWorkaround anchoredMode useFixedPosition enableGamepad hideSettingsButton "
	limits := map[string][2]float64{"zoomFactor": {.5, 3}, "fadeInDuration": {0, 5000}, "fadeOutDuration": {0, 2000}, "centerDeadZone": {0, 10000}, "maxSelectionRadius": {0, 10000}, "minParentDistance": {0, 10000}, "dragThreshold": {0, 10000}, "fixedPositionX": {0, 1}, "fixedPositionY": {0, 1}, "gestureMinStrokeLength": {0, 10000}, "gestureMinStrokeAngle": {0, 360}, "gestureJitterThreshold": {0, 10000}, "gesturePauseTimeout": {0, 10000}, "fixedStrokeLength": {0, 10000}, "gamepadBackButton": {-1, 31}, "gamepadCloseButton": {-1, 31}, "soundVolume": {0, 1}}
	for key, value := range options {
		if strings.Contains(booleans, " "+key+" ") {
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("floating boolean: %s", key)
			}
			continue
		}
		if bounds, ok := limits[key]; ok {
			v, ok := value.(float64)
			if !ok {
				switch n := value.(type) {
				case int:
					v = float64(n)
					ok = true
				}
			}
			if !ok || v < bounds[0] || v > bounds[1] {
				return fmt.Errorf("floating numeric option: %s", key)
			}
			continue
		}
		switch key {
		case "settingsButtonPosition":
			v, _ := value.(string)
			if !oneOf(v, "top-left", "top-right", "bottom-left", "bottom-right") {
				return fmt.Errorf("floating settings position")
			}
		case "soundTheme":
			v, _ := value.(string)
			if !idPattern.MatchString(v) {
				return fmt.Errorf("floating sound theme")
			}
		default:
			return fmt.Errorf("unknown floating option: %s", key)
		}
	}
	return nil
}
