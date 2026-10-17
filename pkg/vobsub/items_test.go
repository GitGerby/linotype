package vobsub

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func buildSyntheticStream() []byte {
	spuData := make([]byte, 34)
	binary.BigEndian.PutUint16(spuData[0:2], 34) // total size
	binary.BigEndian.PutUint16(spuData[2:4], 10) // DCSQT offset

	// RLE at 4 and 7
	spuData[4] = 0x00
	spuData[5] = 0x00
	spuData[7] = 0x00
	spuData[8] = 0x00

	// DCSQT at 10
	binary.BigEndian.PutUint16(spuData[10:12], 0)
	binary.BigEndian.PutUint16(spuData[12:14], 34)

	idx := 14
	spuData[idx] = 0x01
	idx++ // STA_DSP
	spuData[idx] = 0x03
	spuData[idx+1] = 0x12
	spuData[idx+2] = 0x34
	idx += 3 // SET_COLOR
	spuData[idx] = 0x04
	spuData[idx+1] = 0x56
	spuData[idx+2] = 0x78
	idx += 3 // SET_CONTR
	// SET_DAREA: X=50, colEnd=249, Y=400, rowEnd=439
	// colStart = 50 (0x032), colEnd = 249 (0x0F9) -> 0x03, 0x20, 0xF9
	// rowStart = 400 (0x190), rowEnd = 439 (0x1B7) -> 0x19, 0x01, 0xB7
	spuData[idx] = 0x05
	spuData[idx+1] = 0x03
	spuData[idx+2] = 0x20
	spuData[idx+3] = 0xF9
	spuData[idx+4] = 0x19
	spuData[idx+5] = 0x01
	spuData[idx+6] = 0xB7
	idx += 7
	// SET_DSPXA
	spuData[idx] = 0x06
	binary.BigEndian.PutUint16(spuData[idx+1:], 4)
	binary.BigEndian.PutUint16(spuData[idx+3:], 7)
	idx += 5
	spuData[idx] = 0xFF // CMD_END

	// Wrap in MPEG-PS
	var stream []byte
	// Pack header
	stream = append(stream, []byte{
		0x00, 0x00, 0x01, 0xBA,
		0x44, 0x00, 0x04, 0x00, 0x04, 0x01, 0x01, 0x89, 0xC3, 0xF8,
	}...)

	// PES packet
	pesLen := uint16(9 + len(spuData))
	pesHeader := []byte{
		0x00, 0x00, 0x01, 0xBD,
		byte(pesLen >> 8), byte(pesLen & 0xFF),
		0x80, 0x80, 0x05, 0x21, 0x00, 0x01, 0x00, 0x01, // PES header + PTS
		0x20, // Substream ID
	}
	stream = append(stream, pesHeader...)
	stream = append(stream, spuData...)
	return stream
}

func TestParseStream_Dimensions(t *testing.T) {
	stream := buildSyntheticStream()

	// 1. Explicit 640x480
	items, err := ParseStream(bytes.NewReader(stream), DefaultPalette(), 640, 480)
	if err != nil {
		t.Fatalf("ParseStream failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].VideoWidth != 640 || items[0].VideoHeight != 480 {
		t.Errorf("expected 640x480, got %dx%d", items[0].VideoWidth, items[0].VideoHeight)
	}
	if items[0].X != 50 || items[0].Y != 400 {
		t.Errorf("expected X=50, Y=400, got X=%d, Y=%d", items[0].X, items[0].Y)
	}

	// 2. Default 0, 0 -> should default to 720x480
	itemsDef, err := ParseStream(bytes.NewReader(stream), DefaultPalette(), 0, 0)
	if err != nil {
		t.Fatalf("ParseStream failed: %v", err)
	}
	if len(itemsDef) != 1 {
		t.Fatalf("expected 1 item, got %d", len(itemsDef))
	}
	if itemsDef[0].VideoWidth != 720 || itemsDef[0].VideoHeight != 480 {
		t.Errorf("expected default 720x480, got %dx%d", itemsDef[0].VideoWidth, itemsDef[0].VideoHeight)
	}
}

func TestParseIndexedWithInfo_Dimensions(t *testing.T) {
	stream := buildSyntheticStream()
	// Cue points to the PES packet (offset 14)
	idxInfo := &IdxInfo{
		Width:   640,
		Height:  480,
		Palette: DefaultPalette(),
		Tracks: []TrackInfo{
			{
				Index:    0,
				Language: "en",
				Cues: []Cue{
					{
						Timestamp: "00:00:01:000",
						FilePos:   14,
					},
				},
			},
		},
	}

	items, err := ParseIndexedWithInfo(idxInfo, bytes.NewReader(stream), 0)
	if err != nil {
		t.Fatalf("ParseIndexedWithInfo failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].VideoWidth != 640 || items[0].VideoHeight != 480 {
		t.Errorf("expected 640x480 from idxInfo, got %dx%d", items[0].VideoWidth, items[0].VideoHeight)
	}
	if items[0].X != 50 || items[0].Y != 400 {
		t.Errorf("expected X=50, Y=400, got X=%d, Y=%d", items[0].X, items[0].Y)
	}
}

func TestParseStream_PALDimensions(t *testing.T) {
	stream := buildSyntheticStream()
	items, err := ParseStream(bytes.NewReader(stream), DefaultPalette(), 720, 576)
	if err != nil {
		t.Fatalf("ParseStream failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].VideoWidth != 720 || items[0].VideoHeight != 576 {
		t.Errorf("expected 720x576, got %dx%d", items[0].VideoWidth, items[0].VideoHeight)
	}
}
