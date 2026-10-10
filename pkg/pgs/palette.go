package pgs

import (
	"image/color"
	"math"
)

// ToRGBA converts a YCbCr palette entry to an RGBA color using BT.709 color space.
// PGS uses BT.709 YCbCr color space:
//   R = Y + 1.5748 * (Cr - 128)
//   G = Y - 0.1873 * (Cb - 128) - 0.4681 * (Cr - 128)
//   B = Y + 1.8556 * (Cb - 128)
func (p *Palette) ToRGBA(index byte) color.RGBA {
	entry := p.Entries[index]
	y := float64(entry.Y)
	cb := float64(entry.Cb)
	cr := float64(entry.Cr)

	r := y + 1.5748*(cr-128)
	g := y - 0.1873*(cb-128) - 0.4681*(cr-128)
	b := y + 1.8556*(cb-128)

	return color.RGBA{
		R: clamp(r),
		G: clamp(g),
		B: clamp(b),
		A: entry.Alpha,
	}
}

func clamp(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(math.Round(v))
}

// DominantColor returns the most frequently used non-transparent,
// non-background text color in the palette based on the given usage counts.
// It prioritizes actual text pixels by ignoring dark outlines, shadows,
// and translucent pixels in the first pass.
func DominantColor(palette *Palette, usageCounts [256]int) color.RGBA {
	maxCount := 0
	bestIndex := byte(0)

	// First pass: look for non-dark, high-alpha text pixels (excluding outline and shadow)
	for i := 0; i < 256; i++ {
		entry := palette.Entries[i]
		if entry.Alpha < 64 {
			continue
		}

		diffCb := int(entry.Cb) - 128
		if diffCb < 0 {
			diffCb = -diffCb
		}
		diffCr := int(entry.Cr) - 128
		if diffCr < 0 {
			diffCr = -diffCr
		}
		isNeutral := diffCb <= 20 && diffCr <= 20
		if isNeutral && entry.Y < 80 {
			// Dark outline or shadow
			continue
		}

		if usageCounts[i] > maxCount {
			maxCount = usageCounts[i]
			bestIndex = byte(i)
		}
	}

	if maxCount > 0 {
		return palette.ToRGBA(bestIndex)
	}

	// Second pass fallback: any non-transparent pixel if all visible pixels are dark
	for i := 0; i < 256; i++ {
		if palette.Entries[i].Alpha == 0 {
			continue
		}
		if usageCounts[i] > maxCount {
			maxCount = usageCounts[i]
			bestIndex = byte(i)
		}
	}

	if maxCount == 0 {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}

	return palette.ToRGBA(bestIndex)
}

