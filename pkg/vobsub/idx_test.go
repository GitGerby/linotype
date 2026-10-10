package vobsub

import (
	"fmt"
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
