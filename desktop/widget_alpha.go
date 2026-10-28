package main

// GDI leaves alpha at zero. Restore it explicitly so opaque control cards do
// not become additive/washed out when the island uses a DWM glass backdrop.
func composeWidgetAlpha(pixels []byte, width, height int, panel Panel, glass bool, boxes []WidgetBox) {
	for i := 0; i+3 < len(pixels); i += 4 {
		if !glass || pixels[i] != 0 || pixels[i+1] != 0 || pixels[i+2] != 0 {
			if pixels[i+3] == 0 {
				pixels[i+3] = 255
			}
		}
	}
	if !glass {
		return
	}
	for index, box := range boxes {
		if index >= len(panel.Controls) {
			break
		}
		effect := panel.Controls[index].Appearance.Effect
		if effect == "transparent" {
			continue
		}
		alpha := byte(255)
		if effect == "glass" {
			alpha = 140
		}
		radius := min(9, box.H/2, box.W/2)
		for y := max(0, box.Y); y < min(height, box.Y+box.H); y++ {
			for x := max(0, box.X); x < min(width, box.X+box.W); x++ {
				cx := max(box.X+radius, min(box.X+box.W-radius-1, x))
				cy := max(box.Y+radius, min(box.Y+box.H-radius-1, y))
				dx, dy := x-cx, y-cy
				if dx*dx+dy*dy > radius*radius {
					continue
				}
				i := (y*width + x) * 4
				pixels[i+3] = alpha
				if alpha != 255 {
					for c := 0; c < 3; c++ {
						pixels[i+c] = byte(uint32(pixels[i+c]) * uint32(alpha) / 255)
					}
				}
			}
		}
	}
}
