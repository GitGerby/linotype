package vobsub

import (
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
