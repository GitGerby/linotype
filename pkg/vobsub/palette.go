package vobsub

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

// Palette represents a 16-color DVD subtitle palette.
type Palette [16]color.RGBA

// DefaultPalette returns a standard high-contrast fallback palette.
func DefaultPalette() *Palette {
	var p Palette
	p[0] = color.RGBA{0, 0, 0, 0}
	p[1] = color.RGBA{0, 0, 0, 255}
	p[2] = color.RGBA{255, 255, 255, 255}
	p[3] = color.RGBA{128, 128, 128, 255}
	for i := 4; i < 16; i++ {
		p[i] = color.RGBA{0, 0, 0, 255}
	}
	return &p
}

// ParsePalette parses an .idx palette line.
func ParsePalette(line string) (*Palette, error) {
	line = strings.TrimPrefix(line, "palette: ")
	parts := strings.Split(line, ",")
	if len(parts) != 16 {
		return nil, fmt.Errorf("invalid palette length")
	}
	var p Palette
	for i, part := range parts {
		val, err := strconv.ParseUint(strings.TrimSpace(part), 16, 32)
		if err != nil {
			return nil, err
		}
		p[i] = color.RGBA{uint8(val >> 16), uint8(val >> 8), uint8(val), 255}
	}
	return &p, nil
}
