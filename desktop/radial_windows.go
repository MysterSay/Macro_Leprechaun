//go:build windows

package main

import (
	"unsafe"
)

func nativeRadialShape(dc uintptr, s radialShape, background, border uintptr) {
	brush := gc("CreateSolidBrush", background)
	pen := gc("CreatePen", 0, 1, border)
	ob, op := gc("SelectObject", dc, brush), gc("SelectObject", dc, pen)
	x, y, r := int32(s.X), int32(s.Y), int32(s.R)
	switch s.Shape {
	case "square":
		gc("Rectangle", dc, uintptr(x-r), uintptr(y-r), uintptr(x+r), uintptr(y+r))
	case "rounded":
		gc("RoundRect", dc, uintptr(x-r), uintptr(y-r), uintptr(x+r), uintptr(y+r), uintptr(r*7/10), uintptr(r*7/10))
	case "diamond":
		points := []point{{x, y - r}, {x + r, y}, {x, y + r}, {x - r, y}}
		gc("Polygon", dc, uintptr(unsafe.Pointer(&points[0])), 4)
	default:
		gc("Ellipse", dc, uintptr(x-r), uintptr(y-r), uintptr(x+r), uintptr(y+r))
	}
	gc("SelectObject", dc, ob)
	gc("SelectObject", dc, op)
	gc("DeleteObject", brush)
	gc("DeleteObject", pen)
}
func drawRadialControls(dc uintptr, o *overlay, r rect, c Config) {
	s := radialStyle(o.p)
	shapes := radialLayout(o.p, int(r.R), int(r.B), o.hover)
	center := shapes[0]
	gc("SetBkMode", dc, 1)
	for _, shape := range shapes[1:] {
		widgetLine(dc, int32(center.X), int32(center.Y), int32(shape.X), int32(shape.Y), 8, color(s.Connector))
	}
	nativeRadialShape(dc, center, color(s.Background), color(s.Border))
	title := o.p.Name
	if o.hover >= 0 && o.hover < len(o.p.Controls) {
		title = resolveLabel(o.p.Controls[o.hover].Label, c.Variables)
	}
	box := rect{int32(center.X - center.R + 10), int32(center.Y - center.R + 10), int32(center.X + center.R - 10), int32(center.Y + center.R - 10)}
	// Multiline text, transparent GDI background: no white rectangles.
	gc("SetTextColor", dc, color(s.Text))
	measured := box
	call("DrawTextW", dc, uintptr(unsafe.Pointer(ws(title))), ^uintptr(0), uintptr(unsafe.Pointer(&measured)), 0x2411)
	height := min(box.B-box.T, measured.B-measured.T)
	box.T = int32(center.Y) - height/2
	call("DrawTextW", dc, uintptr(unsafe.Pointer(ws(title))), ^uintptr(0), uintptr(unsafe.Pointer(&box)), 0x2011)
	for i, b := range o.p.Controls {
		shape := shapes[i+1]
		bg, ink := color(s.Background), color(s.Text)
		if b.Color != "" {
			ink = color(b.Color)
		}
		if i == o.hover {
			bg, ink = color(s.Hover), color(s.HoverText)
		}
		nativeRadialShape(dc, shape, bg, color(s.Border))
		box := rect{int32(shape.X - shape.R), int32(shape.Y - shape.R), int32(shape.X + shape.R), int32(shape.Y + shape.R)}
		o.items = append(o.items, box)
		if icon := o.iconImages[b.ID]; icon != nil {
			drawWidgetIcon(dc, icon, box)
		} else {
			widgetText(dc, b.Icon, box, ink, max(12, min(36, s.ButtonSize*44/100)), true)
		}
	}
}
func beginRadialBuffer(target uintptr, o *overlay, r rect) (uintptr, func()) {
	type bitmapInfo struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		Used, Important        uint32
	}
	info := bitmapInfo{Size: 40, Width: r.R, Height: -r.B, Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bitmap := gc("CreateDIBSection", target, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return target, func() {}
	}
	memory := gc("CreateCompatibleDC", target)
	if memory == 0 {
		gc("DeleteObject", bitmap)
		return target, func() {}
	}
	old := gc("SelectObject", memory, bitmap)
	data := unsafe.Slice((*byte)(bits), int(r.R*r.B)*4)
	return memory, func() {
		gc("GdiFlush")
		composeRadialAlpha(data, int(r.R), int(r.B), o.p, o.hover)
		var window rect
		call("GetWindowRect", o.h, uintptr(unsafe.Pointer(&window)))
		destination := point{window.L, window.T}
		size := point{r.R, r.B}
		source := point{}
		blend := uint32(0x01ff0000)
		call("UpdateLayeredWindow", o.h, 0, uintptr(unsafe.Pointer(&destination)), uintptr(unsafe.Pointer(&size)), memory, uintptr(unsafe.Pointer(&source)), 0, uintptr(unsafe.Pointer(&blend)), 2)
		gc("SelectObject", memory, old)
		gc("DeleteDC", memory)
		gc("DeleteObject", bitmap)
	}
}
