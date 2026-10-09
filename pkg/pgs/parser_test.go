package pgs

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestDecodeRLE(t *testing.T) {
	// Simple case: 2x2 image, one color
	// color 5, run of 4
	// 0x00 0x84 0x05 (0x80 | 4 = 0x84, color = 5)
	rle := []byte{0x00, 0x84, 0x05}
	decoded, err := DecodeRLE(rle, 2, 2)
	if err != nil {
		t.Fatalf("DecodeRLE failed: %v", err)
	}
	if len(decoded) != 4 {
		t.Fatalf("expected 4 pixels, got %d", len(decoded))
	}
	for i, b := range decoded {
		if b != 5 {
			t.Errorf("pixel %d: expected 5, got %d", i, b)
		}
	}

	// Test End of Line (0x00 0x00)
	rle = []byte{0x05, 0x00, 0x00} // single pixel color 5, EOL. Pad rest to 0.
	decoded, err = DecodeRLE(rle, 2, 1)
	if err != nil {
		t.Fatalf("DecodeRLE failed: %v", err)
	}
	if decoded[0] != 5 || decoded[1] != 0 {
		t.Errorf("expected [5, 0], got %v", decoded)
	}
}

func TestPaletteConversion(t *testing.T) {
	p := Palette{}
	// Red color in YCbCr roughly Y=81, Cb=90, Cr=240
	p.Entries[1] = PaletteEntry{Y: 81, Cb: 90, Cr: 240, Alpha: 255}

	rgba := p.ToRGBA(1)
	if rgba.A != 255 {
		t.Errorf("expected Alpha 255, got %d", rgba.A)
	}
	if rgba.R < 250 {
		t.Errorf("expected Red ~255, got %d", rgba.R)
	}
}

func buildSegment(segType byte, data []byte) []byte {
	var header [13]byte
	header[0] = 0x50
	header[1] = 0x47
	binary.BigEndian.PutUint32(header[2:6], 90000) // 1 second PTS
	binary.BigEndian.PutUint32(header[6:10], 0)     // 0 DTS
	header[10] = segType
	binary.BigEndian.PutUint16(header[11:13], uint16(len(data)))
	return append(header[:], data...)
}

func TestParserIntegration(t *testing.T) {
	var stream []byte

	// 1. PCS
	pcsData := make([]byte, 11)
	binary.BigEndian.PutUint16(pcsData[0:2], 1920) // Width
	binary.BigEndian.PutUint16(pcsData[2:4], 1080) // Height
	pcsData[4] = 24                             // FrameRate
	binary.BigEndian.PutUint16(pcsData[5:7], 1)    // CompositionNumber
	pcsData[7] = 0x80                           // State: Epoch Start
	pcsData[8] = 0                              // PaletteUpdateFlag
	pcsData[9] = 1                              // PaletteID
	pcsData[10] = 0                             // Num Objects
	stream = append(stream, buildSegment(SegmentTypePCS, pcsData)...)

	// 2. WDS
	wdsData := []byte{1, 0, 0, 0, 0, 0, 1920 / 256, 1920 % 256, 1080 / 256, 1080 % 256}
	stream = append(stream, buildSegment(SegmentTypeWDS, wdsData)...)

	// 3. PDS
	pdsData := []byte{1, 0, 1, 81, 90, 240, 255} // Palette ID 1, Version 0, index 1 = Red
	stream = append(stream, buildSegment(SegmentTypePDS, pdsData)...)

	// 4. ODS
	odsData := make([]byte, 11)
	binary.BigEndian.PutUint16(odsData[0:2], 0) // Object ID
	odsData[2] = 0                              // Version
	odsData[3] = 0xC0                           // First and last sequence
	// Total data len (3 bytes)
	binary.BigEndian.PutUint16(odsData[5:7], 2) // We just pretend it's short
	binary.BigEndian.PutUint16(odsData[7:9], 2) // Width
	binary.BigEndian.PutUint16(odsData[9:11], 2) // Height
	odsData = append(odsData, []byte{0x00, 0x84, 0x01}...) // 4 pixels of color 1
	stream = append(stream, buildSegment(SegmentTypeODS, odsData)...)

	// 5. END
	stream = append(stream, buildSegment(SegmentTypeEND, []byte{})...)

	reader := bytes.NewReader(stream)
	sets, err := Parse(reader)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(sets) != 1 {
		t.Fatalf("Expected 1 display set, got %d", len(sets))
	}

	set := sets[0]
	if set.PTS != time.Second {
		t.Errorf("Expected 1s PTS, got %v", set.PTS)
	}
	if set.PCS.Width != 1920 || set.PCS.Height != 1080 {
		t.Errorf("Expected PCS 1920x1080, got %dx%d", set.PCS.Width, set.PCS.Height)
	}
	if len(set.Objects) != 1 {
		t.Fatalf("Expected 1 object, got %d", len(set.Objects))
	}

	// Test render
	pngData, counts, err := RenderPNG(&set.Objects[0], &set.Palette)
	if err != nil {
		t.Fatalf("RenderPNG failed: %v", err)
	}
	if len(pngData) == 0 {
		t.Errorf("Expected non-empty PNG data")
	}
	if counts[1] != 4 {
		t.Errorf("Expected 4 pixels of color 1, got %d", counts[1])
	}

	// Test dominant color
	dom := DominantColor(&set.Palette, counts)
	if dom.A != 255 {
		t.Errorf("Expected dominant color to have A=255")
	}
}

