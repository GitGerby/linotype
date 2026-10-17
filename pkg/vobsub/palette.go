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
// In DVD SPU, index 0 is background (transparent), index 1 is pattern (text body),
// index 2 is emphasis 1 (outline), and index 3 is emphasis 2 (shadow).
func DefaultPalette() *Palette {
	var p Palette
	p[0] = color.RGBA{0, 0, 0, 0}         // Transparent background
	p[1] = color.RGBA{255, 255, 255, 255} // White text body
	p[2] = color.RGBA{0, 0, 0, 255}       // Black outline
	p[3] = color.RGBA{128, 128, 128, 255} // Grey shadow
	for i := 4; i < 16; i++ {
		switch i % 4 {
		case 1:
			p[i] = color.RGBA{255, 255, 255, 255} // White
		case 2:
			p[i] = color.RGBA{0, 0, 0, 255} // Black
		case 3:
			p[i] = color.RGBA{255, 255, 0, 255} // Yellow
		default:
			p[i] = color.RGBA{128, 128, 128, 255} // Grey
		}
	}
	return &p
}

// HasContrast reports whether the palette contains at least one bright,
// visible color suitable for subtitle text recognition, and is not all-identical.
func (p *Palette) HasContrast() bool {
	if p == nil {
		return false
	}
	allSame := true
	hasBright := false
	for i := 0; i < len(p); i++ {
		if p[i] != p[0] {
			allSame = false
		}
		// Rec. 601 luma
		luma := 0.299*float64(p[i].R) + 0.587*float64(p[i].G) + 0.114*float64(p[i].B)
		if luma >= 60 {
			hasBright = true
		}
	}
	return hasBright && !allSame
}

// ParsePalette parses an .idx palette line.
// It handles formatting variations (0x prefixes, # prefixes, comments, trailing commas,
// 4-color or 16-color palettes) and automatically falls back to DefaultPalette
// if the parsed palette is entirely black, dark, or lacks contrast.
func ParsePalette(line string) (*Palette, error) {
	// Strip inline comments
	line = stripInlineComment(line)
	// Strip line continuations
	line = strings.ReplaceAll(line, "\\", " ")
	if idx := strings.Index(line, ":"); idx != -1 {
		line = strings.TrimSpace(line[idx+1:])
	}

	// Normalize delimiters: replace semicolons and split across commas and spaces
	line = strings.ReplaceAll(line, ";", ",")
	rawParts := strings.Split(line, ",")
	var parts []string
	for _, part := range rawParts {
		for _, token := range strings.Fields(part) {
			clean := strings.TrimSpace(token)
			if clean != "" {
				parts = append(parts, clean)
			}
		}
	}

	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid palette length: got %d parts, need at least 4", len(parts))
	}

	p := *DefaultPalette()
	count := len(parts)
	if count > 16 {
		count = 16
	}

	for i := 0; i < count; i++ {
		clean := strings.TrimSpace(parts[i])
		clean = strings.TrimPrefix(clean, "0x")
		clean = strings.TrimPrefix(clean, "0X")
		clean = strings.TrimPrefix(clean, "#")

		if len(clean) == 3 {
			clean = string([]byte{clean[0], clean[0], clean[1], clean[1], clean[2], clean[2]})
		}

		val, err := strconv.ParseUint(clean, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid palette color %q at index %d: %w", parts[i], i, err)
		}
		p[i] = color.RGBA{R: uint8(val >> 16), G: uint8(val >> 8), B: uint8(val), A: 255}
	}

	// If palette is all-black / all-dark / uniform, fall back to high-contrast DefaultPalette
	if !p.HasContrast() {
		return DefaultPalette(), nil
	}

	return &p, nil
}

// stripInlineComment removes an inline comment starting with '#' from line,
// while preserving '#' prefixes on hex color values (such as #000000 or #fff).
func stripInlineComment(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	// If the line has a key:value structure where the key is known NOT to contain colors
	// (e.g. size, id, timestamp, tridx), any '#' is an inline comment.
	allowColors := true
	maxColors := 16
	if colonIdx := strings.Index(line, ":"); colonIdx != -1 {
		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		switch key {
		case "palette":
			maxColors = 16
		case "colors":
			maxColors = 4
		case "custom colors":
			maxColors = 4
		default:
			allowColors = false
		}
	}

	for i := 0; i < len(line); i++ {
		if line[i] != '#' {
			continue
		}
		if allowColors && isHexColorPrefix(line, i, maxColors) {
			continue
		}
		return strings.TrimSpace(line[:i])
	}
	return strings.TrimSpace(line)
}

func isHexColorPrefix(line string, i int, maxColors int) bool {
	if i >= len(line) || line[i] != '#' {
		return false
	}

	// Must be preceded by start of string, whitespace, or delimiter
	if i > 0 {
		prev := line[i-1]
		if prev != ' ' && prev != '\t' && prev != ',' && prev != ';' && prev != ':' && prev != '=' && prev != '\\' {
			return false
		}
	}

	// Lone '#' or followed by whitespace is always a comment
	if i+1 >= len(line) || line[i+1] == ' ' || line[i+1] == '\t' {
		return false
	}

	// If maxColors have already appeared on this line before this token, this must be a comment
	if countColorsInLine(line[:i]) >= maxColors {
		return false
	}

	// Extract this token up to next delimiter
	tokenEnd := i + 1
	for tokenEnd < len(line) && !isDelimiter(line[tokenEnd]) {
		tokenEnd++
	}
	token := line[i:tokenEnd]
	if !isValidColorToken(token) {
		return false
	}

	// Examine all subsequent tokens up to the next '#' or end of line.
	// In a valid palette/colors directive, any subsequent token before a comment must be a valid color.
	remainder := line[tokenEnd:]
	nextHash := strings.Index(remainder, "#")
	chunk := remainder
	if nextHash != -1 {
		chunk = remainder[:nextHash]
	}

	chunk = strings.ReplaceAll(chunk, ";", " ")
	chunk = strings.ReplaceAll(chunk, ",", " ")
	chunk = strings.ReplaceAll(chunk, "\\", " ")
	for _, tok := range strings.Fields(chunk) {
		if !isValidColorToken(tok) {
			return false
		}
	}

	return true
}

func isDelimiter(b byte) bool {
	return b == ' ' || b == '\t' || b == ',' || b == ';' || b == '\\' || b == '\r' || b == '\n' || b == '#' || b == ':'
}

func isHexDigit(b byte) bool {
	return ('0' <= b && b <= '9') || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F')
}

func isValidColorToken(s string) bool {
	token := strings.TrimSpace(s)
	token = strings.TrimPrefix(token, "0x")
	token = strings.TrimPrefix(token, "0X")
	token = strings.TrimPrefix(token, "#")
	if len(token) != 3 && len(token) != 6 && len(token) != 8 {
		return false
	}
	for i := 0; i < len(token); i++ {
		if !isHexDigit(token[i]) {
			return false
		}
	}
	return true
}

func countColorsInLine(s string) int {
	if idx := strings.Index(s, ":"); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.ReplaceAll(s, ";", " ")
	s = strings.ReplaceAll(s, ",", " ")
	s = strings.ReplaceAll(s, "\\", " ")
	count := 0
	for _, tok := range strings.Fields(s) {
		if isValidColorToken(tok) {
			count++
		}
	}
	return count
}
