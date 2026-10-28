//go:build windows

package main

import (
	"errors"
	"math"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

type widgetPending struct {
	Panel   Panel
	Control Control
	Value   float64
	Due     time.Time
}

func widgetHit(o *overlay) (int, point) {
	var cursor point
	call("GetCursorPos", uintptr(unsafe.Pointer(&cursor)))
	call("ScreenToClient", o.h, uintptr(unsafe.Pointer(&cursor)))
	for i, r := range o.items {
		if cursor.X >= r.L && cursor.X <= r.R && cursor.Y >= r.T && cursor.Y <= r.B {
			return i, cursor
		}
	}
	return -1, cursor
}
func widgetState(o *overlay, b Control) ControlState {
	nativeApp.mu.RLock()
	state, ok := nativeApp.controlStates[controlStateKey("widgets", o.p.ID, b.ID)]
	nativeApp.mu.RUnlock()
	if !ok {
		return initialControlState(nativeApp.runtimeConfig(), b)
	}
	return state
}

func widgetValue(o *overlay, index int) float64 {
	if o.sliderIndex == index && (o.sliderDragging || o.sliderQueued) {
		return o.sliderValue
	}
	return widgetState(o, widgetControls(o)[index]).Value
}
func widgetDrag(o *overlay) {
	if o.sliderIndex < 0 || o.sliderIndex >= len(widgetControls(o)) || o.sliderIndex >= len(o.items) {
		return
	}
	_, pt := widgetHit(o)
	b := widgetControls(o)[o.sliderIndex]
	r := o.items[o.sliderIndex]
	maximum := o.sliderMaximum
	if maximum <= 0 {
		maximum, _ = sliderSpec(nativeApp.runtimeConfig(), b)
	}
	o.sliderValue = maximum * widgetSliderFraction(WidgetBox{int(r.L), int(r.T), int(r.R - r.L), int(r.B - r.T)}, sliderVertical(b.Appearance), b.Icon != "" || b.Label != "", int(pt.X), int(pt.Y))
	o.sliderQueued = true
	if o.sliderPending == nil {
		o.sliderPending = map[string]*widgetPending{}
	}
	key := o.p.ID + "/" + b.ID
	pending := o.sliderPending[key]
	if pending == nil && widgetState(o, b).Value == o.sliderValue {
		o.sliderQueued = false
		call("InvalidateRect", o.h, 0, 1)
		return
	}
	if pending == nil {
		pending = &widgetPending{Panel: o.p, Control: b}
		o.sliderPending[key] = pending
	}
	pending.Value = o.sliderValue
	call("InvalidateRect", o.h, 0, 1)
}
func widgetDown(o *overlay) {
	index, _ := widgetHit(o)
	if index < 0 {
		return
	}
	b := widgetControls(o)[index]
	o.pressedID = b.ID
	duration := .22
	if b.Appearance.Activation != nil {
		duration = *b.Appearance.Activation
	}
	o.pressedUntil = time.Now().Add(time.Duration(duration * float64(time.Second)))
	if b.Kind == "slider" {
		o.sliderIndex = index
		o.sliderMaximum, _ = sliderSpec(nativeApp.runtimeConfig(), b)
		o.sliderDragging = true
		o.sliderQueued = false
		call("SetCapture", o.h)
		widgetDrag(o)
	}
	call("InvalidateRect", o.h, 0, 1)
}
func widgetUp(o *overlay) {
	if o.sliderDragging {
		widgetDrag(o)
		o.sliderDragging = false
		o.sliderQueued = true
		call("ReleaseCapture")
		tickWidget(o)
		return
	}
	index, _ := widgetHit(o)
	if index < 0 || widgetControls(o)[index].ID != o.pressedID {
		return
	}
	b := widgetControls(o)[index]
	value := "click"
	if b.Kind == "toggle" {
		value = strconv.FormatBool(!widgetState(o, b).Active)
	}
	err := nativeApp.activatePanelControl(o.p, b, value, RunScope{Session: "desktop", Mode: "widgets", Host: o.hostID, Panel: o.p.ID})
	if err != nil {
		nativeApp.log(err.Error())
	}
	duration := .22
	if b.Appearance.Activation != nil {
		duration = *b.Appearance.Activation
	}
	o.pressedUntil = time.Now().Add(time.Duration(duration * float64(time.Second)))
	call("InvalidateRect", o.h, 0, 1)
}
func tickWidget(o *overlay) {
	index, _ := widgetHit(o)
	if len(o.hoverProgress) != len(widgetControls(o)) {
		o.hoverProgress = make([]float64, len(widgetControls(o)))
	}
	repaint := false
	for i := range o.hoverProgress {
		target := 0.0
		if i == index {
			target = 1
		}
		value := o.hoverProgress[i] + (target-o.hoverProgress[i])*.6
		if math.Abs(value-target) < .01 {
			value = target
		}
		if value != o.hoverProgress[i] {
			repaint = true
		}
		o.hoverProgress[i] = value
		if widgetControls(o)[i].Appearance.Animation != "" && widgetControls(o)[i].Appearance.Animation != "none" {
			repaint = true
		}
	}
	o.hover = index
	pressed := time.Now().Before(o.pressedUntil)
	if pressed || o.pressedActive {
		repaint = true
	}
	o.pressedActive = pressed
	for key, pending := range o.sliderPending {
		b := pending.Control
		dragging := o.sliderDragging && o.sliderIndex >= 0 && o.sliderIndex < len(widgetControls(o)) && o.p.ID == pending.Panel.ID && widgetControls(o)[o.sliderIndex].ID == b.ID
		if dragging && b.Appearance.SliderMode != "online" {
			continue
		}
		if time.Now().Before(pending.Due) {
			continue
		}
		err := nativeApp.activatePanelControl(pending.Panel, b, strconv.FormatFloat(pending.Value, 'f', -1, 64), RunScope{Session: "desktop", Mode: "widgets", Host: o.hostID, Panel: pending.Panel.ID})
		interval := .5
		if b.Appearance.SliderInterval != nil {
			interval = *b.Appearance.SliderInterval
		}
		pending.Due = time.Now().Add(time.Duration(interval * float64(time.Second)))
		if err == nil {
			delete(o.sliderPending, key)
			if dragging || o.sliderIndex >= 0 && o.sliderIndex < len(widgetControls(o)) && widgetControls(o)[o.sliderIndex].ID == b.ID {
				o.sliderQueued = false
			}
		} else if !errors.Is(err, ErrMacroBusy) {
			delete(o.sliderPending, key)
			nativeApp.log(err.Error())
		}
		repaint = true
	}

	if repaint {
		call("InvalidateRect", o.h, 0, 1)
	}
}
func mixWidgetColor(a, b uintptr, t float64) uintptr {
	result := uintptr(0)
	for shift := uint(0); shift < 24; shift += 8 {
		x, y := float64((a>>shift)&255), float64((b>>shift)&255)
		result |= uintptr(max(0, min(255, x+(y-x)*t))) << shift
	}
	return result
}
func widgetText(dc uintptr, value string, r rect, ink uintptr, size int, center bool) {
	if value == "" {
		return
	}
	font := gc("CreateFontW", uintptr(size), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(ws("Segoe UI"))))
	old := gc("SelectObject", dc, font)
	defer gc("DeleteObject", font)
	defer gc("SelectObject", dc, old)
	gc("SetBkMode", dc, 1)
	gc("SetTextColor", dc, ink)
	flags := uintptr(0x8824)
	if center {
		flags |= 1
	}
	call("DrawTextW", dc, uintptr(unsafe.Pointer(ws(value))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
}
func widgetLine(dc uintptr, x1, y1, x2, y2 int32, width int, ink uintptr) {
	pen := gc("CreatePen", 0, uintptr(width), ink)
	old := gc("SelectObject", dc, pen)
	gc("MoveToEx", dc, uintptr(x1), uintptr(y1), 0)
	gc("LineTo", dc, uintptr(x2), uintptr(y2))
	gc("SelectObject", dc, old)
	gc("DeleteObject", pen)
}
func drawWidgetControls(dc uintptr, o *overlay, r rect, c Config, fg, card, accent uintptr) {
	metrics := widgetMetrics
	controls, boxes := widgetSlots(o.p, int(r.B))
	o.renderControls = controls
	o.items = nil
	o.paintedBoxes = make([]WidgetBox, len(boxes))
	for _, box := range boxes {
		o.items = append(o.items, rect{int32(box.X), int32(box.Y), int32(box.X + box.W), int32(box.Y + box.H)})
	}
	if o.p.Material != "transparent" {
		strokeRound(dc, rect{0, 0, r.R - 1, r.B - 1}, color("#2a322e"))
	}
	widgetText(dc, o.p.Name, rect{12, 8, r.R - 12, 36}, accent, metrics.Title, true)
	for i, b := range widgetControls(o) {
		box := o.items[i]
		base := box
		bg, ink, symbol := card, fg, accent
		if b.Appearance.Background != "" {
			bg = color(b.Appearance.Background)
		}
		if b.Appearance.Text != "" {
			ink = color(b.Appearance.Text)
		}
		if b.Color != "" {
			symbol = color(b.Color)
		}
		hover := 0.0
		if i < len(o.hoverProgress) {
			hover = o.hoverProgress[i]
		}
		active := b.Kind == "toggle" && widgetState(o, b).Active
		strength := hover * .16
		if active {
			strength = .3
		}
		bg = mixWidgetColor(bg, symbol, strength)
		if active {
			inset := int32(float64(box.R-box.L) * .02)
			box.L += inset
			box.R -= inset
			box.T++
			box.B--
		}
		pressed := o.pressedID == b.ID && time.Now().Before(o.pressedUntil)
		if pressed {
			switch b.Appearance.Press {
			case "scale", "":
				box.L += 2
				box.R -= 2
				box.T += 1
				box.B -= 1
			case "flash":
				bg = mixWidgetColor(bg, 0xffffff, .3)
			}
		}
		o.paintedBoxes[i] = WidgetBox{int(box.L), int(box.T), int(box.R - box.L), int(box.B - box.T)}
		if b.Appearance.Effect != "transparent" {
			if b.Appearance.Effect == "shadow" {
				shadow := box
				shadow.T += 3
				shadow.B += 3
				roundedFill(dc, shadow, 0x080808)
			}
			roundedFill(dc, box, bg)
			if image := o.buttonImages[b.ID]; image != nil {
				drawButtonImage(dc, image, box, b.Appearance.ImageFit)
			}
			border := color("#2a322e")
			if b.Appearance.Border != "" {
				border = color(b.Appearance.Border)
			}
			border = mixWidgetColor(border, symbol, hover*.7)
			if active || b.Appearance.Effect == "glow" {
				border = symbol
			}
			strokeRound(dc, box, border)
			if b.Appearance.Effect == "glass" {
				widgetLine(dc, box.L+10, box.T+1, box.R-10, box.T+1, 1, mixWidgetColor(bg, 0xffffff, .35))
			}
		}
		if pressed && b.Appearance.Press == "ripple" {
			elapsed := .22
			if b.Appearance.Activation != nil {
				elapsed = max(.01, *b.Appearance.Activation)
			}
			fraction := 1 - float64(time.Until(o.pressedUntil))/float64(time.Duration(elapsed*float64(time.Second)))
			radius := int32(5 + fraction*18)
			circle(dc, (box.L+box.R)/2, (box.T+box.B)/2, radius, bg, symbol)
		}
		labelX, labelY, labelBottom := box.L+12, box.T, box.B
		if b.Kind == "slider" {
			labelBottom = min(box.B, box.T+32)
		}
		if b.Icon != "" {
			glyph := rect{box.L + 10, labelY, box.L + 34, labelBottom}
			phase := float64(time.Now().UnixMilli()%6000) / 1000
			size := 20
			if b.Appearance.Animation == "float" {
				delta := int32(math.Sin(phase*3.5) * 3)
				glyph.T += delta
				glyph.B += delta
			}
			if b.Appearance.Animation == "pulse" {
				size += int(math.Sin(phase*4) * 3)
			}
			if image := o.iconImages[b.ID]; image != nil {
				drawWidgetIcon(dc, image, glyph)
			} else {
				if b.Appearance.Animation == "spin" {
					drawRotatingWidgetGlyph(dc, b.Icon, glyph, symbol, size, phase)
				} else {
					widgetText(dc, b.Icon, glyph, symbol, size, true)
				}
			}
			labelX = box.L + 44
		}
		if active {
			ink = mixWidgetColor(ink, symbol, .8)
		}
		labelRight := box.R - 12
		if b.Kind == "toggle" {
			labelRight = box.R - 52
		}
		widgetText(dc, resolveLabel(b.Label, c.Variables), rect{labelX, labelY, labelRight, labelBottom}, ink, metrics.Font, false)
		if b.Kind == "toggle" {
			track := rect{box.R - 43, box.T + 15, box.R - 11, box.T + 31}
			trackColor := color("#56615b")
			if active {
				trackColor = symbol
			}
			roundedFill(dc, track, trackColor)
			cx := track.L + 8
			if active {
				cx = track.R - 8
			}
			circle(dc, cx, (track.T+track.B)/2, 6, 0xf3f3f3, 0xf3f3f3)
		}
		if b.Kind == "slider" {
			maximum, _ := sliderSpec(c, b)
			fraction := max(0, min(1, widgetValue(o, i)/maximum))
			track, thumb := color("#56615b"), symbol
			if b.Appearance.SliderTrack != "" {
				track = color(b.Appearance.SliderTrack)
			}
			if b.Appearance.SliderThumb != "" {
				thumb = color(b.Appearance.SliderThumb)
			}
			width := 6
			if b.Appearance.SliderStyle == "thin" {
				width = 2
			}
			if b.Appearance.SliderStyle == "filled" {
				width = 10
			}
			radius := 9
			if b.Appearance.SliderThumbSize > 0 {
				radius = b.Appearance.SliderThumbSize / 2
			}
			if sliderVertical(b.Appearance) {
				top := int32(14)
				if b.Icon != "" || b.Label != "" {
					top = 40
				}
				x, y1, y2 := (base.L+base.R)/2, base.T+top, base.B-14
				widgetLine(dc, x, y1, x, y2, width, track)
				cy := y2 - int32(fraction*float64(y2-y1))
				if b.Appearance.SliderStyle == "filled" {
					widgetLine(dc, x, cy, x, y2, width, thumb)
				}
				circle(dc, x, cy, int32(radius), thumb, thumb)
			} else {
				x1, x2, y := base.L+14, base.R-14, base.B-18
				if b.Icon == "" && b.Label == "" {
					y = (base.T + base.B) / 2
				}
				widgetLine(dc, x1, y, x2, y, width, track)
				cx := x1 + int32(fraction*float64(x2-x1))
				if b.Appearance.SliderStyle == "filled" {
					widgetLine(dc, x1, y, cx, y, width, thumb)
				}
				circle(dc, cx, y, int32(radius), thumb, thumb)
			}
		}
	}
}
func drawRotatingWidgetGlyph(dc uintptr, value string, r rect, ink uintptr, size int, phase float64) {
	saved := gc("SaveDC", dc)
	defer gc("RestoreDC", dc, saved)
	gc("SetGraphicsMode", dc, 2)
	angle := phase * math.Pi / 1.5
	cx, cy := float64(r.L+r.R)/2, float64(r.T+r.B)/2
	co, si := math.Cos(angle), math.Sin(angle)
	transform := [6]float32{float32(co), float32(si), float32(-si), float32(co), float32(cx - co*cx + si*cy), float32(cy - si*cx - co*cy)}
	gc("SetWorldTransform", dc, uintptr(unsafe.Pointer(&transform)))
	widgetText(dc, value, r, ink, size, true)
}

func drawWidgetIcon(dc uintptr, icon *overlay, r rect) {
	type bitmapInfo struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		Used, Important        uint32
	}
	info := bitmapInfo{Size: 40, Width: icon.imageW, Height: -icon.imageH, Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	bitmap := gc("CreateDIBSection", dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		return
	}
	defer gc("DeleteObject", bitmap)
	copy(unsafe.Slice((*byte)(bits), len(icon.background)), icon.background)
	memory := gc("CreateCompatibleDC", dc)
	if memory == 0 {
		return
	}
	defer gc("DeleteDC", memory)
	old := gc("SelectObject", memory, bitmap)
	defer gc("SelectObject", memory, old)
	size := min(r.R-r.L, r.B-r.T, int32(24))
	x, y := r.L+(r.R-r.L-size)/2, r.T+(r.B-r.T-size)/2
	alphaBlend := syscall.NewLazyDLL("msimg32.dll").NewProc("AlphaBlend")
	alphaBlend.Call(dc, uintptr(x), uintptr(y), uintptr(size), uintptr(size), memory, 0, 0, uintptr(icon.imageW), uintptr(icon.imageH), 0x01ff0000)
}

func beginWidgetBuffer(target uintptr, o *overlay, r rect) (uintptr, func()) {
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
	clear(data)
	return memory, func() {
		gc("GdiFlush")
		_, boxes := widgetSlots(o.p, int(r.B))
		if len(o.paintedBoxes) == len(boxes) {
			boxes = o.paintedBoxes
		}
		glass := oneOf(o.p.Material, "mica", "acrylic", "glass") && !o.materialFallback
		panel := o.p
		panel.Controls = widgetControls(o)
		composeWidgetAlpha(data, int(r.R), int(r.B), panel, glass, boxes)
		gc("BitBlt", target, 0, 0, uintptr(r.R), uintptr(r.B), memory, 0, 0, 0xcc0020)
		gc("SelectObject", memory, old)
		gc("DeleteDC", memory)
		gc("DeleteObject", bitmap)
	}
}

func widgetControls(o *overlay) []Control {
	if o.renderControls != nil {
		return o.renderControls
	}
	return controlLeaves(o.p.Controls)
}
