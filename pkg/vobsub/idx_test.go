package vobsub

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
)

func TestParsePalette(t *testing.T) {
	line := "palette: 000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000"
	p, err := ParsePalette(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p[1].R != 0x7f {
		t.Errorf("expected 0x7f for R, got %x", p[1].R)
	}
}

func TestParseIdx(t *testing.T) {
	fixture := `size: 720x480
palette: 000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000
id: en, index: 0
timestamp: 00:01:23:456, filepos: 000001000
timestamp: 00:01:25:000, filepos: 000002000
id: fr, index: 1
timestamp: 00:02:00:000, filepos: 000003000
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette R=0x7f")
	}
	if info.Width != 720 || info.Height != 480 {
		t.Errorf("expected 720x480, got %dx%d", info.Width, info.Height)
	}
	if len(info.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(info.Tracks))
	}
	if info.Tracks[0].Language != "en" || info.Tracks[0].Index != 0 {
		t.Errorf("track 0 info mismatch")
	}
	if len(info.Tracks[0].Cues) != 2 {
		t.Fatalf("expected 2 cues for track 0")
	}
	if info.Tracks[0].Cues[0].Timestamp != "00:01:23:456" || info.Tracks[0].Cues[0].FilePos != 0x1000 {
		t.Errorf("cue 0 mismatch: %+v", info.Tracks[0].Cues[0])
	}
}

func TestParseIdx_640x480(t *testing.T) {
	fixture := `size: 640x480
palette: 000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Width != 640 || info.Height != 480 {
		t.Errorf("expected 640x480, got %dx%d", info.Width, info.Height)
	}
}

func TestParseIdx_HexEncoded(t *testing.T) {
	fixture := `size: 640x480
id: en, index: 0
`
	hexStr := fmt.Sprintf("%x", fixture)
	info, err := ParseIdx(strings.NewReader(hexStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Width != 640 || info.Height != 480 {
		t.Errorf("expected 640x480, got %dx%d", info.Width, info.Height)
	}
	if len(info.Tracks) != 1 || info.Tracks[0].Language != "en" {
		t.Errorf("track mismatch from hex idx")
	}
}

func TestParseIdx_CaseInsensitive(t *testing.T) {
	fixture := `Size: 640x480
Palette: 000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000
ID: en, Index: 0
Timestamp: 00:01:23:456, Filepos: 000001000
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Width != 640 || info.Height != 480 {
		t.Errorf("expected 640x480, got %dx%d", info.Width, info.Height)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette R=0x7f, got %x", info.Palette[1].R)
	}
	if len(info.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(info.Tracks))
	}
	if info.Tracks[0].Language != "en" || info.Tracks[0].Index != 0 {
		t.Errorf("track 0 mismatch: %+v", info.Tracks[0])
	}
	if len(info.Tracks[0].Cues) != 1 || info.Tracks[0].Cues[0].Timestamp != "00:01:23:456" || info.Tracks[0].Cues[0].FilePos != 0x1000 {
		t.Errorf("cue mismatch: %+v", info.Tracks[0].Cues)
	}
}

func TestParsePalette_Variations(t *testing.T) {
	// 1. Without space after colon
	line1 := "palette:000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000"
	p1, err := ParsePalette(line1)
	if err != nil {
		t.Fatalf("unexpected error on line1: %v", err)
	}
	if p1[1].R != 0x7f {
		t.Errorf("expected 0x7f for R")
	}

	// 2. Capitalized Palette:
	line2 := "Palette: 000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000"
	p2, err := ParsePalette(line2)
	if err != nil {
		t.Fatalf("unexpected error on line2: %v", err)
	}
	if p2[1].R != 0x7f {
		t.Errorf("expected 0x7f for R")
	}
}

func TestParseIdx_CustomColors(t *testing.T) {
	fixture := `# VobSub index file
size: 720x480
palette: 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000
custom colors: ON, tridx: 1000, colors: 000000, ffffff, 000000, 808080
id: en, index: 0
timestamp: 00:01:23:456, filepos: 000001000
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.CustomColors == nil || !info.CustomColors.Enabled {
		t.Fatalf("expected custom colors to be enabled")
	}
	if !info.CustomColors.Tridx[0] {
		t.Errorf("expected tridx[0] to be true (transparent)")
	}
	if info.CustomColors.Tridx[1] {
		t.Errorf("expected tridx[1] to be false (opaque)")
	}
	// Palette index 1 should now be white from custom colors
	if info.Palette[1].R != 255 || info.Palette[1].G != 255 || info.Palette[1].B != 255 {
		t.Errorf("expected white text from custom colors, got %+v", info.Palette[1])
	}
}

func TestParseIdx_MultiLinePalette(t *testing.T) {
	fixture := `size: 720x480
palette: 000000, 7f7f7f, e0e0e0, 000000, 000000, 000000, 000000, 000000, \
         111111, 222222, 333333, 444444, 555555, 666666, 777777, 888888
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing multiline palette: %v", err)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette[1].R=0x7f, got %x", info.Palette[1].R)
	}
	if info.Palette[8].R != 0x11 {
		t.Errorf("expected palette[8].R=0x11, got %x", info.Palette[8].R)
	}
}

func TestParseIdx_AllBlackCustomColors(t *testing.T) {
	fixture := `size: 720x480
palette: 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000
custom colors: ON, tridx: 1000, colors: 000000, 000000, 000000, 000000
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Text slot must be elevated to white despite all-black custom colors
	if info.Palette[1].R != 255 || info.Palette[1].G != 255 || info.Palette[1].B != 255 {
		t.Errorf("expected white fallback for all-black custom colors, got %+v", info.Palette[1])
	}
}

func TestParseIdx_SeparateDirectives(t *testing.T) {
	fixture := `size: 720x480
palette: 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000
custom colors: ON
tridx: 1000
colors: 000000, ffffff, 000000, 808080
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.CustomColors == nil || !info.CustomColors.Enabled {
		t.Fatalf("expected custom colors enabled")
	}
	if !info.CustomColors.Tridx[0] || info.CustomColors.Tridx[1] {
		t.Errorf("expected tridx[0]=true, tridx[1]=false, got %+v", info.CustomColors.Tridx)
	}
	if info.Palette[1].R != 255 {
		t.Errorf("expected white for palette[1]")
	}
}

func TestParseIdx_SpaceDelimitedPalette(t *testing.T) {
	fixture := `size: 720x480
palette: 000000 7f7f7f e0e0e0 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000 000000
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette[1].R=0x7f, got %x", info.Palette[1].R)
	}
}

func TestParseIdx_HashPrefixPalette(t *testing.T) {
	fixture := `# VobSub index file
size: 720x480 # frame size
palette: #000000, #7f7f7f, #e0e0e0, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000, #000000 # palette comment
id: en, index: 0 # english
timestamp: 00:01:23:456, filepos: 000001000 # cue 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing idx with hash prefixes: %v", err)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette[1].R=0x7f, got %x", info.Palette[1].R)
	}
	if info.Width != 720 || info.Height != 480 {
		t.Errorf("expected 720x480, got %dx%d", info.Width, info.Height)
	}
	if len(info.Tracks) != 1 || len(info.Tracks[0].Cues) != 1 {
		t.Fatalf("expected 1 track with 1 cue")
	}
}

func TestParseIdx_HashPrefixCustomColors(t *testing.T) {
	fixture := `size: 720x480
palette: 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000
custom colors: ON, tridx: 1000, colors: #000000, #ffffff, #000000, #808080 # custom colors comment
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Palette[1].R != 255 || info.Palette[1].G != 255 || info.Palette[1].B != 255 {
		t.Errorf("expected white text from # custom colors, got %+v", info.Palette[1])
	}
}

func TestParseIdx_MultilineHashPrefixPalette(t *testing.T) {
	fixture := `size: 720x480
palette: #000000, #7f7f7f, #e0e0e0, #000000, #000000, #000000, #000000, #000000, \
         #111111, #222222, #333333, #444444, #555555, #666666, #777777, #888888
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing multiline palette with hash: %v", err)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette[1].R=0x7f, got %x", info.Palette[1].R)
	}
	if info.Palette[8].R != 0x11 {
		t.Errorf("expected palette[8].R=0x11, got %x", info.Palette[8].R)
	}
}

func TestParseIdx_MultilinePaletteWithCommentOnContinuationLine(t *testing.T) {
	fixture := `size: 720x480
palette: #000000, #7f7f7f, #e0e0e0, #000000, #000000, #000000, #000000, #000000, \ # first 8 colors
         #111111, #222222, #333333, #444444, #555555, #666666, #777777, #888888   # second 8 colors
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing multiline palette with comments on continuation: %v", err)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette[1].R=0x7f, got %x", info.Palette[1].R)
	}
	if info.Palette[8].R != 0x11 {
		t.Errorf("expected palette[8].R=0x11, got %x", info.Palette[8].R)
	}
}

func TestParseIdx_MultilinePaletteMultipleContinuations(t *testing.T) {
	fixture := `size: 720x480
palette: #000000, #7f7f7f, \ # chunk 1
         #e0e0e0, #000000, \ # chunk 2
         #111111, #222222, #333333, #444444
id: en, index: 0
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing multiple continuations: %v", err)
	}
	if info.Palette[1].R != 0x7f {
		t.Errorf("expected palette[1].R=0x7f, got %x", info.Palette[1].R)
	}
	if info.Palette[4].R != 0x11 {
		t.Errorf("expected palette[4].R=0x11, got %x", info.Palette[4].R)
	}
}

func TestParseIdx_CommentWithHexWords(t *testing.T) {
	fixture := `size: 720x480 #bad timing
palette: #000, #fff, #000, #888 #abc, comment
custom colors: ON, tridx: 1000, colors: #000, #fff, #000, #888 #fff; note
id: en, index: 0 #fee for license
`
	info, err := ParseIdx(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("unexpected error parsing idx with hex-mimicking comments: %v", err)
	}
	if info.Width != 720 || info.Height != 480 {
		t.Errorf("expected 720x480, got %dx%d", info.Width, info.Height)
	}
	if info.Palette[1] != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("expected white for palette[1], got %+v", info.Palette[1])
	}
	if info.CustomColors == nil || !info.CustomColors.Enabled {
		t.Errorf("expected custom colors enabled")
	}
}
