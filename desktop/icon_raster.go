package main

import (
	"bytes"
	"fmt"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"image"
	imagedraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

func rasterIcon(path string, size int) (*image.RGBA, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 2<<20 {
		return nil, fmt.Errorf("icon too large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if icon.ViewBox.W <= 0 || icon.ViewBox.H <= 0 {
			return nil, fmt.Errorf("invalid SVG viewBox")
		}
		scale := min(float64(size)/icon.ViewBox.W, float64(size)/icon.ViewBox.H)
		w, h := icon.ViewBox.W*scale, icon.ViewBox.H*scale
		icon.SetTarget((float64(size)-w)/2, (float64(size)-h)/2, w, h)
		scanner := rasterx.NewScannerGV(size, size, out, out.Bounds())
		dasher := rasterx.NewDasher(size, size, scanner)
		icon.Draw(dasher, 1)
		return out, nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width*config.Height > 16000000 {
		return nil, fmt.Errorf("invalid image")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := src.Bounds()
	scale := min(float64(size)/float64(bounds.Dx()), float64(size)/float64(bounds.Dy()))
	w, h := max(1, int(float64(bounds.Dx())*scale)), max(1, int(float64(bounds.Dy())*scale))
	target := image.Rect((size-w)/2, (size-h)/2, (size-w)/2+w, (size-h)/2+h)
	xdraw.CatmullRom.Scale(out, target, src, bounds, imagedraw.Over, nil)
	return out, nil
}
