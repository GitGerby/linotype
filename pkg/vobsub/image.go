package vobsub

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// Render renders a 2-bit pixel array with palette and alpha to an image.RGBA.
// It also returns the dominant text color.
func Render(pixels []byte, w, h int, colors [4]uint8, alphas [4]uint8, palette *Palette) (*image.RGBA, color.RGBA, error) {
	if len(pixels) < w*h {
		return nil, color.RGBA{}, fmt.Errorf("pixel data too short: got %d, need %d", len(pixels), w*h)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	
	// Map 2-bit local colors to 32-bit RGBA
	var mapped [4]color.RGBA
	for i := 0; i < 4; i++ {
		c := palette[colors[i]]
		mapped[i] = color.RGBA{R: c.R, G: c.G, B: c.B, A: alphas[i]}
	}

	counts := make(map[uint8]int)
	
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := pixels[y*w+x]
			img.SetRGBA(x, y, mapped[idx])
			if alphas[idx] > 0 {
				counts[idx]++
			}
		}
	}

	isDark := func(c color.RGBA) bool {
		luma := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
		maxChan := c.R
		if c.G > maxChan {
			maxChan = c.G
		}
		if c.B > maxChan {
			maxChan = c.B
		}
		diffRG := int(c.R) - int(c.G)
		if diffRG < 0 {
			diffRG = -diffRG
		}
		diffGB := int(c.G) - int(c.B)
		if diffGB < 0 {
			diffGB = -diffGB
		}
		diffBR := int(c.B) - int(c.R)
		if diffBR < 0 {
			diffBR = -diffBR
		}
		maxDiff := diffRG
		if diffGB > maxDiff {
			maxDiff = diffGB
		}
		if diffBR > maxDiff {
			maxDiff = diffBR
		}
		isNeutral := maxDiff <= 25
		return (isNeutral && luma < 80) || maxChan < 60
	}

	// Find dominant color, prioritizing bright text pixels over dark outline/shadow pixels.
	// In DVD SPU, idx 1 is the pattern (text body), while idx 2 and 3 are emphasis (outline/shadow).
	var dominantIdx uint8 = 0
	maxCount := -1

	// Pass 1: non-dark, high-alpha pixels (actual text body)
	for i := uint8(1); i < 4; i++ {
		if alphas[i] >= 64 && !isDark(mapped[i]) && counts[i] > maxCount {
			maxCount = counts[i]
			dominantIdx = i
		}
	}

	// Pass 2 fallback: any visible pixel
	if maxCount == -1 {
		for i := uint8(1); i < 4; i++ {
			if alphas[i] > 0 && counts[i] > maxCount {
				maxCount = counts[i]
				dominantIdx = i
			}
		}
	}

	if maxCount == -1 {
		for i := uint8(0); i < 4; i++ {
			if alphas[i] > 0 && counts[i] > maxCount {
				maxCount = counts[i]
				dominantIdx = i
			}
		}
	}

	var dominantColor color.RGBA
	if maxCount != -1 {
		dominantColor = mapped[dominantIdx]
	} else {
		dominantColor = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}

	return img, dominantColor, nil
}

// EncodePNG encodes an image to PNG bytes.
func EncodePNG(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
