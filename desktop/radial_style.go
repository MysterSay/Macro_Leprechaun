package main

import (
	"fmt"
	"math"
	"regexp"
)

type RadialStyle struct {
	ButtonSize  int     `json:"buttonSize"`
	CenterSize  int     `json:"centerSize"`
	Shape       string  `json:"shape"`
	Background  string  `json:"background"`
	Text        string  `json:"text"`
	Border      string  `json:"border"`
	Connector   string  `json:"connector"`
	Hover       string  `json:"hover"`
	HoverText   string  `json:"hoverText"`
	Opacity     float64 `json:"opacity"`
	Shadow      int     `json:"shadow"`
	HoverShadow int     `json:"hoverShadow"`
}

func radialStyle(p Panel) RadialStyle {
	if p.Radial != nil {
		return *p.Radial
	}
	return RadialStyle{ButtonSize: 50, CenterSize: 100, Shape: "circle", Background: "#191e22", Text: "#edf3ee", Border: "#2a322e", Connector: "#22282e", Hover: "#6cda81", HoverText: "#101412", Opacity: 1, Shadow: 10, HoverShadow: 18}
}
func validateRadialStyle(s RadialStyle) error {
	if s.ButtonSize < 24 || s.ButtonSize > 120 || s.CenterSize < 48 || s.CenterSize > 180 || !oneOf(s.Shape, "circle", "rounded", "square", "diamond") || math.IsNaN(s.Opacity) || s.Opacity < 0.05 || s.Opacity > 1 || s.Shadow < 0 || s.Shadow > 40 || s.HoverShadow < 0 || s.HoverShadow > 60 {
		return fmt.Errorf("invalid radial style geometry/opacity")
	}
	hex := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	for _, v := range []string{s.Background, s.Text, s.Border, s.Connector, s.Hover, s.HoverText} {
		if !hex.MatchString(v) {
			return fmt.Errorf("invalid radial style color")
		}
	}
	return nil
}
func radialDistance(p Panel) float64 {
	s := radialStyle(p)
	distance := math.Max(100, float64(p.Radius)*.72)
	if len(p.Controls) > 1 {
		distance = math.Max(distance, (float64(s.ButtonSize)*1.15+12)/(2*math.Sin(math.Pi/float64(len(p.Controls)))))
	}
	return math.Max(distance, float64(s.CenterSize)/2+float64(s.ButtonSize)*.575+16)
}
func radialExtent(p Panel) int {
	s := radialStyle(p)
	return int(math.Ceil(radialDistance(p)+float64(s.ButtonSize)*.575+float64(max(s.Shadow, s.HoverShadow))+8)) * 2
}
func radialPanelIndex(p Panel, dx, dy float64) int {
	if math.Hypot(dx, dy) < float64(radialStyle(p).CenterSize)/2+2 {
		return -1
	}
	return radialIndex(dx*100, dy*100, len(p.Controls))
}

type radialShape struct {
	X, Y, R float64
	Shape   string
	Shadow  int
}

func radialLayout(p Panel, width, height int, hover int) []radialShape {
	s := radialStyle(p)
	cx, cy := float64(width)/2, float64(height)/2
	shapes := []radialShape{{cx, cy, float64(s.CenterSize) / 2, "circle", s.Shadow}}
	for i := range p.Controls {
		a := float64(i)*2*math.Pi/float64(len(p.Controls)) - math.Pi/2
		size := float64(s.ButtonSize) / 2
		shadow := s.Shadow
		if i == hover {
			size *= 1.15
			shadow = s.HoverShadow
		}
		shapes = append(shapes, radialShape{cx + math.Cos(a)*radialDistance(p), cy + math.Sin(a)*radialDistance(p), size, s.Shape, shadow})
	}
	return shapes
}
func radialShapeDistance(s radialShape, x, y float64) float64 {
	dx, dy := math.Abs(x-s.X), math.Abs(y-s.Y)
	switch s.Shape {
	case "square":
		return math.Hypot(math.Max(dx-s.R, 0), math.Max(dy-s.R, 0)) + math.Min(math.Max(dx-s.R, dy-s.R), 0)
	case "rounded":
		rr := s.R * .35
		qx, qy := dx-s.R+rr, dy-s.R+rr
		return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - rr
	case "diamond":
		return (dx + dy - s.R) / math.Sqrt2
	}
	return math.Hypot(dx, dy) - s.R
}

func composeRadialAlpha(data []byte, width, height int, p Panel, hover int) {
	style := radialStyle(p)

	// GDI paints on a chroma key, then a premultiplied alpha DIB is sent to the
	// layered window. Opacity and shadows never flatten onto the desktop.
	for i := 0; i < len(data); i += 4 {
		if data[i] == 255 && data[i+1] == 0 && data[i+2] == 255 {
			data[i] = 0
			data[i+1] = 0
			data[i+2] = 0
			data[i+3] = 0
		} else {
			alpha := byte(style.Opacity * 255)
			for ch := 0; ch < 3; ch++ {
				data[i+ch] = byte(uint32(data[i+ch]) * uint32(alpha) / 255)
			}
			data[i+3] = alpha
		}
	}
	for _, shape := range radialLayout(p, width, height, hover) {
		if shape.Shadow == 0 {
			continue
		}
		extent := shape.R + float64(shape.Shadow)
		for y := max(0, int(shape.Y-extent)); y < min(height, int(shape.Y+extent)+1); y++ {
			for x := max(0, int(shape.X-extent)); x < min(width, int(shape.X+extent)+1); x++ {
				i := (y*width + x) * 4
				if data[i+3] >= byte(style.Opacity*255) {
					continue
				}
				d := radialShapeDistance(shape, float64(x), float64(y))
				if d <= 0 || d >= float64(shape.Shadow) {
					continue
				}
				fall := 1 - d/float64(shape.Shadow)
				alpha := byte(110 * fall * fall * style.Opacity)
				if alpha > data[i+3] {
					data[i] = 0
					data[i+1] = 0
					data[i+2] = 0
					data[i+3] = alpha
				}
			}
		}
	}
}
