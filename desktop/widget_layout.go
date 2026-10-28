package main

// One set of dimensions drives the real Win32 island and its HTML preview.
type WidgetMetrics struct {
	Width    int `json:"width"`
	Top      int `json:"top"`
	Pad      int `json:"pad"`
	Gap      int `json:"gap"`
	Button   int `json:"button"`
	Slider   int `json:"slider"`
	Vertical int `json:"vertical"`
	Radius   int `json:"radius"`
	Font     int `json:"font"`
	Title    int `json:"title"`
}

var widgetMetrics = WidgetMetrics{320, 44, 12, 6, 44, 76, 144, 18, 14, 12}

type WidgetBox struct{ X, Y, W, H int }

func sliderVertical(a Appearance) bool {
	return a.SliderOrientation == "vertical" || a.SliderOrientation == "" && a.SliderStyle == "vertical"
}
func widgetLayout(panel Panel, maxHeight int) (int, int, []WidgetBox) {
	m := widgetMetrics
	sum := 0
	heights := []int{}
	for _, b := range panel.Controls {
		h := widgetControlHeight(b)
		heights = append(heights, h)
		sum += h + m.Gap
	}
	height := m.Top + sum + m.Gap
	factor := 1.0
	if maxHeight > m.Top+m.Gap && height > maxHeight {
		factor = float64(maxHeight-m.Top-m.Gap) / float64(max(1, sum))
		height = maxHeight
	}
	y := m.Top
	rows := []WidgetBox{}
	for _, h := range heights {
		scaled := max(1, int(float64(h+m.Gap)*factor))
		rows = append(rows, WidgetBox{m.Pad, y, m.Width - 2*m.Pad, max(1, scaled-m.Gap)})
		y += scaled
	}
	return m.Width, height, rows
}
func widgetSliderFraction(box WidgetBox, vertical bool, header bool, x, y int) float64 {
	var fraction float64
	if vertical {
		top := 14
		if header {
			top = 40
		}
		fraction = float64(box.Y+box.H-14-y) / float64(max(1, box.H-top-14))
	} else {
		fraction = float64(x-box.X-14) / float64(max(1, box.W-28))
	}
	return max(0, min(1, fraction))
}

func widgetControlHeight(b Control) int {
	m := widgetMetrics
	if len(b.Segments) > 0 {
		height := m.Button
		for _, part := range b.Segments {
			height = max(height, widgetControlHeight(part))
		}
		if b.SplitAxis == "vertical" {
			height = height*len(b.Segments) + 2*(len(b.Segments)-1)
		}
		return height
	}
	if b.Kind != "slider" {
		return m.Button
	}
	if sliderVertical(b.Appearance) {
		return m.Vertical
	}
	if b.Icon == "" && b.Label == "" {
		return m.Button
	}
	return m.Slider
}
func widgetSlots(panel Panel, maxHeight int) ([]Control, []WidgetBox) {
	_, _, boxes := widgetLayout(panel, maxHeight)
	controls := []Control{}
	slots := []WidgetBox{}
	for i, b := range panel.Controls {
		box := boxes[i]
		if len(b.Segments) == 0 {
			controls = append(controls, b)
			slots = append(slots, box)
			continue
		}
		count := len(b.Segments)
		for j, part := range b.Segments {
			slot := box
			if b.SplitAxis == "vertical" {
				start := j * (box.H + 2) / count
				end := (j + 1) * (box.H + 2) / count
				slot.Y += start
				slot.H = max(1, end-start-2)
			} else {
				start := j * (box.W + 2) / count
				end := (j + 1) * (box.W + 2) / count
				slot.X += start
				slot.W = max(1, end-start-2)
			}
			controls = append(controls, part)
			slots = append(slots, slot)
		}
	}
	return controls, slots
}
