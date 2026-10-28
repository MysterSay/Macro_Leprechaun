package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWidgetControlLayoutAndOrientation(t *testing.T) {
	panel := Panel{Controls: []Control{{Kind: "button"}, {Kind: "toggle"}, {Kind: "slider", Label: "Volume"}, {Kind: "slider", Appearance: Appearance{SliderOrientation: "vertical"}}}}
	w, h, rows := widgetLayout(panel, 1000)
	if w != 320 || h != 382 || len(rows) != 4 {
		t.Fatal(w, h, rows)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].Y < rows[i-1].Y+rows[i-1].H {
			t.Fatal("overlapping controls")
		}
	}
	if rows[2].H != 76 || rows[3].H != 144 {
		t.Fatal(rows)
	}
	_, h, rows = widgetLayout(panel, 220)
	if h > 220 || rows[len(rows)-1].Y+rows[len(rows)-1].H > 220 {
		t.Fatal("overflow")
	}
	box := WidgetBox{12, 44, 296, 144}
	for _, test := range []struct {
		v, header bool
		x, y      int
		want      float64
	}{{false, true, 26, 100, 0}, {false, true, 294, 100, 1}, {true, true, 160, 84, 1}, {true, true, 160, 174, 0}, {true, false, 160, 58, 1}, {true, false, 160, 174, 0}} {
		if got := widgetSliderFraction(box, test.v, test.header, test.x, test.y); math.Abs(got-test.want) > .001 {
			t.Fatal(test, got)
		}
	}
}
func TestSliderOptionalVariableAndWidgetToggle(t *testing.T) {
	a := testApp(t)
	m := Macro{ID: "optional", Name: "Optional slider", Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "i", Type: "input", Mode: "slider", Max: 100}}}
	if err := validateGraph(m); err != nil {
		t.Fatal(err)
	}
	_, vars, err := graphPath(m, InputEvent{Kind: "slider", Value: "75"})
	if err != nil || len(vars) != 0 {
		t.Fatal(vars, err)
	}
	a.mu.Lock()
	a.cfg.Macros = append(a.cfg.Macros, m)
	a.mu.Unlock()
	p := a.cfg.Widgets[0]
	b := p.Controls[0]
	b.Kind = "slider"
	b.Macro = m.ID
	if err := a.activatePanelControl(p, b, "75", RunScope{Mode: "widgets", Session: "desktop", Panel: p.ID}); err != nil {
		t.Fatal(err)
	}
	if a.controlStates["widgets/"+p.ID+"/"+b.ID].Value != 75 {
		t.Fatal("native slider state not updated")
	}
	if _, found := a.config().Variables[""]; found {
		t.Fatal("empty variable written")
	}
	m = Macro{ID: "togglewidget", Name: "Toggle", Steps: []Step{}, GraphVersion: 1, Puzzles: []Puzzle{{ID: "i", Type: "input", Mode: "toggle", TrueNext: "yes", FalseNext: "no"}, {ID: "yes", Type: "actions.set", Value: actionPuzzle("actions.set", AppAction{Variable: "volume", Source: "static", Value: "99"}).Value}, {ID: "no", Type: "actions.set", Value: actionPuzzle("actions.set", AppAction{Variable: "volume", Source: "static", Value: "1"}).Value}}}
	a.mu.Lock()
	a.cfg.Macros = append(a.cfg.Macros, m)
	a.mu.Unlock()
	b.Kind = "toggle"
	b.Macro = m.ID
	for _, value := range []string{"true", "false"} {
		if err := a.activatePanelControl(p, b, value, RunScope{Mode: "widgets", Session: "desktop", Panel: p.ID}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		for {
			a.mu.RLock()
			busy := a.running[m.ID]
			a.mu.RUnlock()
			if !busy {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("toggle did not finish")
			default:
				time.Sleep(time.Millisecond)
			}
		}
		cancel()
	}
	if a.config().Variables["volume"] != "1" {
		t.Fatal("false branch not executed")
	}
}
func TestSVGIconRasterForNativeWidget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "icon.svg")
	os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="8" fill="#00ff00"/></svg>`), 0600)
	icon, err := rasterIcon(path, 32)
	if err != nil {
		t.Fatal(err)
	}
	if icon.RGBAAt(16, 16).G < 240 || icon.RGBAAt(16, 16).A < 240 || icon.RGBAAt(0, 0).A != 0 {
		t.Fatal("SVG colors/alpha incorrect")
	}
}

func TestWidgetOpaqueCardsKeepAlphaOnGlass(t *testing.T) {
	panel := Panel{Controls: []Control{{Appearance: Appearance{Effect: "none"}}, {Appearance: Appearance{Effect: "transparent"}}}}
	pixels := make([]byte, 40*40*4)
	boxes := []WidgetBox{{2, 2, 16, 16}, {22, 2, 16, 16}}
	center := (10*40 + 10) * 4
	pixels[center] = 40
	composeWidgetAlpha(pixels, 40, 40, panel, true, boxes)
	if pixels[center+3] != 255 {
		t.Fatal("card remains transparent")
	}
	if pixels[3] != 0 {
		t.Fatal("glass background became opaque")
	}
	if pixels[(10*40+30)*4+3] != 0 {
		t.Fatal("transparent control got an opaque card")
	}
}

func TestSplitWidgetSlots(t *testing.T) {
	p := Panel{Controls: []Control{{ID: "outer", SplitAxis: "horizontal", Segments: []Control{{ID: "left", Kind: "button"}, {ID: "right", Kind: "slider", Label: "Volume"}}}}}
	controls, boxes := widgetSlots(p, 1000)
	if len(controls) != 2 || len(boxes) != 2 || controls[1].ID != "right" || boxes[0].X+boxes[0].W >= boxes[1].X {
		t.Fatal(controls, boxes)
	}
	p.Controls[0].SplitAxis = "vertical"
	_, boxes = widgetSlots(p, 1000)
	if boxes[0].Y+boxes[0].H >= boxes[1].Y {
		t.Fatal(boxes)
	}
}
