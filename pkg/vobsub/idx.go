package vobsub

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"image/color"
	"io"
	"strconv"
	"strings"
)

// IdxInfo represents parsed information from an .idx file.
type IdxInfo struct {
	Width        int
	Height       int
	Palette      *Palette
	CustomColors *CustomColors
	Tracks       []TrackInfo
}

// CustomColors represents custom colors and transparency settings from an .idx file.
type CustomColors struct {
	Enabled bool
	Colors  [4]color.RGBA
	Tridx   [4]bool // true if transparent
}

// TrackInfo represents a subtitle track found in an .idx file.
type TrackInfo struct {
	Index    int
	Language string
	Cues     []Cue
}

// Cue represents a timestamp and file position.
type Cue struct {
	Timestamp string
	FilePos   int64
}

// ParseIdx parses an .idx text file.
func ParseIdx(r io.Reader) (*IdxInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	trimmed := strings.TrimSpace(string(data))
	trimmed = strings.TrimPrefix(trimmed, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) > 0 {
		lowerDecoded := bytes.ToLower(decoded)
		if bytes.Contains(lowerDecoded, []byte("palette:")) || bytes.Contains(lowerDecoded, []byte("size:")) || bytes.Contains(lowerDecoded, []byte("id:")) {
			data = decoded
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	info := &IdxInfo{
		Palette: DefaultPalette(),
	}

	var currentTrack *TrackInfo

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := stripInlineComment(rawLine)
		if strings.TrimSpace(line) == "" {
			continue
		}
		for strings.HasSuffix(strings.TrimSpace(line), "\\") && scanner.Scan() {
			nextLine := stripInlineComment(scanner.Text())
			trimmedLine := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "\\"))
			if strings.TrimSpace(nextLine) != "" {
				line = trimmedLine + " " + strings.TrimSpace(nextLine)
			} else {
				line = trimmedLine + " \\"
			}
		}
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, "\\")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lowerLine := strings.ToLower(line)
		colonIdx := strings.Index(lowerLine, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.TrimSpace(lowerLine[:colonIdx])
		val := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "size":
			dimStr := strings.ToLower(val)
			parts := strings.Split(dimStr, "x")
			if len(parts) == 2 {
				w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
				h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err1 == nil && err2 == nil && w > 0 && h > 0 {
					info.Width = w
					info.Height = h
				}
			}
		case "palette":
			p, err := ParsePalette(line)
			if err != nil {
				return nil, err
			}
			info.Palette = p
			if info.CustomColors != nil && info.CustomColors.Enabled {
				applyCustomColorsToPalette(info.Palette, info.CustomColors)
			}
		case "custom colors":
			cc := parseCustomColorsLine(line)
			if cc != nil {
				info.CustomColors = cc
				if cc.Enabled && info.Palette != nil {
					applyCustomColorsToPalette(info.Palette, cc)
				}
			}
		case "colors":
			if info.CustomColors == nil {
				info.CustomColors = &CustomColors{
					Tridx: [4]bool{true, false, false, false},
				}
			}
			parseColorsDirective(val, info.CustomColors)
			if info.CustomColors.Enabled && info.Palette != nil {
				applyCustomColorsToPalette(info.Palette, info.CustomColors)
			}
		case "tridx":
			if info.CustomColors == nil {
				info.CustomColors = &CustomColors{
					Tridx: [4]bool{true, false, false, false},
				}
			}
			parseTridxString(val, info.CustomColors)
		case "id":
			// id: en, index: 0
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				cIdx := strings.Index(parts[0], ":")
				var lang string
				if cIdx != -1 {
					lang = strings.TrimSpace(parts[0][cIdx+1:])
				}
				idxSub := strings.Index(strings.ToLower(parts[1]), "index:")
				if idxSub != -1 {
					idxStr := strings.TrimSpace(parts[1][idxSub+len("index:"):])
					idx, err := strconv.Atoi(idxStr)
					if err == nil {
						info.Tracks = append(info.Tracks, TrackInfo{
							Index:    idx,
							Language: lang,
						})
						currentTrack = &info.Tracks[len(info.Tracks)-1]
					}
				}
			}
		case "timestamp":
			// timestamp: 00:01:23:456, filepos: 000001000
			if currentTrack == nil {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				cIdx := strings.Index(parts[0], ":")
				var ts string
				if cIdx != -1 {
					ts = strings.TrimSpace(parts[0][cIdx+1:])
				}
				fpSub := strings.Index(strings.ToLower(parts[1]), "filepos:")
				if fpSub != -1 {
					fpStr := strings.TrimSpace(parts[1][fpSub+len("filepos:"):])
					// filepos is hex
					fp, err := strconv.ParseInt(fpStr, 16, 64)
					if err == nil {
						currentTrack.Cues = append(currentTrack.Cues, Cue{
							Timestamp: ts,
							FilePos:   fp,
						})
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

func parseCustomColorsLine(line string) *CustomColors {
	line = stripInlineComment(line)
	cc := &CustomColors{
		Tridx: [4]bool{true, false, false, false},
	}
	content := line
	if idx := strings.Index(content, ":"); idx != -1 {
		content = strings.TrimSpace(content[idx+1:])
	}

	parts := strings.Split(content, ",")
	if len(parts) == 0 {
		return nil
	}

	state := strings.ToUpper(strings.TrimSpace(parts[0]))
	if state == "ON" || state == "1" || state == "YES" || state == "TRUE" {
		cc.Enabled = true
	} else {
		cc.Enabled = false
	}

	lowerContent := strings.ToLower(content)
	if tridxIdx := strings.Index(lowerContent, "tridx:"); tridxIdx != -1 {
		rest := strings.TrimSpace(content[tridxIdx+len("tridx:"):])
		if commaIdx := strings.Index(rest, ","); commaIdx != -1 {
			rest = strings.TrimSpace(rest[:commaIdx])
		}
		parseTridxString(rest, cc)
	}

	if colorsIdx := strings.Index(lowerContent, "colors:"); colorsIdx != -1 {
		colorsPart := content[colorsIdx+len("colors:"):]
		parseColorsDirective(colorsPart, cc)
	}

	return cc
}

func parseTridxString(s string, cc *CustomColors) {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s) && i < 4; i++ {
		cc.Tridx[i] = s[i] == '1'
	}
}

func parseColorsDirective(line string, cc *CustomColors) {
	line = stripInlineComment(line)
	if idx := strings.Index(line, ":"); idx != -1 {
		line = strings.TrimSpace(line[idx+1:])
	}
	line = strings.ReplaceAll(line, ";", ",")
	parts := strings.Split(line, ",")
	var colors []string
	for _, p := range parts {
		for _, token := range strings.Fields(p) {
			clean := strings.TrimSpace(token)
			if clean != "" {
				colors = append(colors, clean)
			}
		}
	}
	for i := 0; i < len(colors) && i < 4; i++ {
		clean := strings.TrimPrefix(colors[i], "0x")
		clean = strings.TrimPrefix(clean, "0X")
		clean = strings.TrimPrefix(clean, "#")
		if len(clean) == 3 {
			clean = string([]byte{clean[0], clean[0], clean[1], clean[1], clean[2], clean[2]})
		}
		val, err := strconv.ParseUint(clean, 16, 32)
		if err == nil {
			cc.Colors[i] = color.RGBA{R: uint8(val >> 16), G: uint8(val >> 8), B: uint8(val), A: 255}
		}
	}
}

func applyCustomColorsToPalette(pal *Palette, cc *CustomColors) {
	if pal == nil || cc == nil || !cc.Enabled {
		return
	}
	// Check if cc.Colors has contrast. If all dark, make text slot white.
	hasContrast := false
	for i := 0; i < 4; i++ {
		luma := 0.299*float64(cc.Colors[i].R) + 0.587*float64(cc.Colors[i].G) + 0.114*float64(cc.Colors[i].B)
		if luma >= 60 {
			hasContrast = true
			break
		}
	}
	if !hasContrast {
		cc.Colors[0] = color.RGBA{0, 0, 0, 0}
		cc.Colors[1] = color.RGBA{255, 255, 255, 255}
		cc.Colors[2] = color.RGBA{0, 0, 0, 255}
		cc.Colors[3] = color.RGBA{128, 128, 128, 255}
	}

	pal[0] = cc.Colors[0]
	pal[1] = cc.Colors[1]
	pal[2] = cc.Colors[2]
	pal[3] = cc.Colors[3]
	for i := 4; i < 16; i++ {
		pal[i] = cc.Colors[i%4]
	}
}
