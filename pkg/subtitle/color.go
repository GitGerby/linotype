package subtitle

import (
	"image/color"
)

// IsCustomTextColor reports whether c represents a distinct, non-white, non-dark
// custom text color that should override default subtitle styling.
//
// White, off-white, neutral grayscale, and dark outline/shadow colors return false,
// ensuring subtitle text renders using the default style (white text with outline)
// rather than being overridden to black, muddy gray, or off-white.
func IsCustomTextColor(c color.RGBA) bool {
	if c.A == 0 {
		return false
	}

	// Subtitle text fill must not be dark (dark colors in subtitle streams are outlines or shadows).
	luma := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
	maxChan := c.R
	if c.G > maxChan {
		maxChan = c.G
	}
	if c.B > maxChan {
		maxChan = c.B
	}
	if (luma < 60 && maxChan < 180) || maxChan < 80 {
		return false
	}

	// Check if the color is neutral / grayscale / white.
	// If all color channels are close to each other, it's white or light grey.
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

	// If saturation is low (channels are within 25 of each other), it's grayscale/white.
	if maxDiff <= 25 {
		return false
	}

	return true
}

