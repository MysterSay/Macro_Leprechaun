package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestRadialStyleRoundtripAndLayout(t *testing.T) {
	a := testApp(t)
	c := a.runtimeConfig()
	s := radialStyle(c.Floating[0])
	s.ButtonSize = 110
	s.Shape = "diamond"
	s.Opacity = .55
	s.HoverShadow = 45
	c.Floating[0].Radial = &s
	if err := a.save(c); err != nil {
		t.Fatal(err)
	}
	restored, err := loadStorage(a.root)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(restored.Floating[0].Radial)
	if radialStyle(restored.Floating[0]) != s {
		t.Fatal(string(raw))
	}
	p := restored.Floating[0]
	extent := radialExtent(p)
	layout := radialLayout(p, extent, extent, 0)
	for i, b := range layout {
		if b.X-b.R-float64(b.Shadow) < 0 || b.Y-b.R-float64(b.Shadow) < 0 || b.X+b.R+float64(b.Shadow) > float64(extent) || b.Y+b.R+float64(b.Shadow) > float64(extent) {
			t.Fatal("clipped node", i)
		}
		if i > 0 {
			if radialPanelIndex(p, b.X-float64(extent)/2, b.Y-float64(extent)/2) != i-1 {
				t.Fatal("hit test disagrees")
			}
		}
	}
	for i := 1; i < len(layout); i++ {
		next := layout[i%len(p.Controls)+1]
		if math.Hypot(layout[i].X-next.X, layout[i].Y-next.Y) < layout[i].R+next.R {
			t.Fatal("nodes overlap")
		}
	}
	bad := s
	bad.Opacity = 1.1
	c.Floating[0].Radial = &bad
	if err := a.save(c); err == nil {
		t.Fatal("invalid opacity accepted")
	}
}
func TestRadialAlphaAndShadow(t *testing.T) {
	p := Panel{Radius: 100, Controls: []Control{}}
	s := radialStyle(p)
	s.Opacity = .5
	s.Shadow = 12
	p.Radial = &s
	const size = 160
	data := make([]byte, size*size*4)
	for i := 0; i < len(data); i += 4 {
		data[i] = 255
		data[i+2] = 255
	}
	center := (80*size + 80) * 4
	data[center] = 100
	data[center+1] = 80
	data[center+2] = 60
	composeRadialAlpha(data, size, size, p, -1)
	if data[3] != 0 {
		t.Fatal("transparent desktop became opaque")
	}
	if data[center+3] != 127 || data[center] != 49 || data[center+2] != 29 {
		t.Fatal("opacity must premultiply", data[center:center+4])
	}
	shadow := (80*size + 135) * 4
	if data[shadow+3] == 0 || data[shadow+3] >= 127 || data[shadow] != 0 || data[shadow+2] != 0 {
		t.Fatal("shadow must be translucent black", data[shadow:shadow+4])
	}
}
