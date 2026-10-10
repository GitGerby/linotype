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

	// Find dominant color (heuristically ignoring background, assuming idx 0 or lowest frequency is bg if transparent)
	// Actually, just find the highest frequency non-transparent color. If multiple, pick highest count.
	// Often text is the most frequent non-background color. Let's find the most frequent color among idx 1, 2, 3 
	// that has alpha > 0.
	
	var dominantIdx uint8 = 0
	maxCount := -1
	for i := uint8(1); i < 4; i++ {
		if alphas[i] > 0 && counts[i] > maxCount {
			maxCount = counts[i]
			dominantIdx = i
		}
	}
	
	if maxCount == -1 {
		// fallback to any non-transparent
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
