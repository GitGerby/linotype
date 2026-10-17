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
// It guarantees that rendered subtitle images sent for OCR are high-contrast and easily parsable,
// eliminating bugs where palettes produce transparent backgrounds with black text.
func Render(pixels []byte, w, h int, colors [4]uint8, alphas [4]uint8, palette *Palette) (*image.RGBA, color.RGBA, error) {
	if len(pixels) < w*h {
		return nil, color.RGBA{}, fmt.Errorf("pixel data too short: got %d, need %d", len(pixels), w*h)
	}
	if palette == nil || !palette.HasContrast() {
		palette = DefaultPalette()
	}

	// Map 2-bit local colors to 32-bit RGBA
	var mapped [4]color.RGBA
	for i := 0; i < 4; i++ {
		c := palette[colors[i]]
		mapped[i] = color.RGBA{R: c.R, G: c.G, B: c.B, A: alphas[i]}
	}

	counts := make(map[uint8]int)
	for _, idx := range pixels {
		counts[idx]++
	}

	ensureHighContrast(&mapped, counts)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := pixels[y*w+x]
			img.SetRGBA(x, y, mapped[idx])
		}
	}

	dominantColor := getDominantColor(mapped, counts)
	return img, dominantColor, nil
}

// ensureHighContrast validates the mapped RGBA colors against the background and pixel counts.
// If the background is transparent or dark, it guarantees that text body pixels are rendered
// with high-contrast bright colors (e.g. white with dark outline) so vision LLMs can easily parse them.
func ensureHighContrast(mapped *[4]color.RGBA, counts map[uint8]int) {
	bgIsTransparent := mapped[0].A < 64
	bgIsDark := bgIsTransparent || isColorDark(mapped[0])

	// Identify primary text slot among foreground slots {1, 2, 3}.
	// In DVD SPU, slot 1 is pattern (text body).
	// If slot 1 has no pixels or is not used, pick the foreground slot with the highest count.
	textSlot := uint8(1)
	if counts[1] == 0 {
		maxCount := 0
		for i := uint8(2); i < 4; i++ {
			if counts[i] > maxCount {
				maxCount = counts[i]
				textSlot = i
			}
		}
	}

	if bgIsDark {
		hasBrightForeground := false
		brightSlot := uint8(0)
		for i := uint8(1); i < 4; i++ {
			if counts[i] > 0 && mapped[i].A >= 64 && !isColorDark(mapped[i]) {
				hasBrightForeground = true
				if brightSlot == 0 {
					brightSlot = i
				}
			}
		}

		if !hasBrightForeground {
			// All visible foreground pixels are dark/black on a transparent/dark background.
			// Elevate text slot to crisp white, outline slot to black, shadow slot to grey.
			textAlpha := uint8(255)
			if mapped[textSlot].A >= 128 {
				textAlpha = mapped[textSlot].A
			}
			mapped[textSlot] = color.RGBA{R: 255, G: 255, B: 255, A: textAlpha}
			for i := uint8(1); i < 4; i++ {
				if i != textSlot {
					if i == 2 || (textSlot == 2 && i == 1) {
						mapped[i] = color.RGBA{R: 0, G: 0, B: 0, A: 255}
					} else {
						mapped[i] = color.RGBA{R: 128, G: 128, B: 128, A: 255}
					}
				}
			}
		} else if counts[textSlot] > 0 && isColorDark(mapped[textSlot]) && brightSlot != 0 {
			// Text slot is dark, but an outline/emphasis slot is bright.
			// Swap RGB colors so text body is bright and outline is dark.
			origText := mapped[textSlot]
			mapped[textSlot].R = mapped[brightSlot].R
			mapped[textSlot].G = mapped[brightSlot].G
			mapped[textSlot].B = mapped[brightSlot].B
			if mapped[textSlot].A < 128 {
				mapped[textSlot].A = 255
			}
			mapped[brightSlot].R = origText.R
			mapped[brightSlot].G = origText.G
			mapped[brightSlot].B = origText.B
		} else if counts[textSlot] > 0 && mapped[textSlot].A < 128 {
			mapped[textSlot].A = 255
		}
	} else {
		// Background is bright and opaque. Ensure text slot is dark.
		hasDarkForeground := false
		for i := uint8(1); i < 4; i++ {
			if counts[i] > 0 && mapped[i].A > 0 && isColorDark(mapped[i]) {
				hasDarkForeground = true
				break
			}
		}
		if !hasDarkForeground {
			mapped[textSlot] = color.RGBA{R: 0, G: 0, B: 0, A: 255}
		}
	}
}

func getDominantColor(mapped [4]color.RGBA, counts map[uint8]int) color.RGBA {
	textSlot := uint8(1)
	if counts[1] == 0 {
		maxCount := 0
		for i := uint8(2); i < 4; i++ {
			if counts[i] > maxCount {
				maxCount = counts[i]
				textSlot = i
			}
		}
	}

	if counts[textSlot] > 0 && mapped[textSlot].A > 0 {
		return mapped[textSlot]
	}

	// Pass 1: non-dark, high-alpha pixels
	maxCount := -1
	var dominantIdx uint8 = 0
	for i := uint8(1); i < 4; i++ {
		if mapped[i].A >= 64 && !isColorDark(mapped[i]) && counts[i] > maxCount {
			maxCount = counts[i]
			dominantIdx = i
		}
	}

	// Pass 2: any visible pixel
	if maxCount == -1 {
		for i := uint8(1); i < 4; i++ {
			if mapped[i].A > 0 && counts[i] > maxCount {
				maxCount = counts[i]
				dominantIdx = i
			}
		}
	}

	if maxCount != -1 {
		return mapped[dominantIdx]
	}

	return color.RGBA{R: 255, G: 255, B: 255, A: 255}
}

// isColorDark returns true if the color is dark (low luminance or low channel max).
func isColorDark(c color.RGBA) bool {
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

// EncodePNG encodes an image to PNG bytes.
func EncodePNG(img *image.RGBA) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
