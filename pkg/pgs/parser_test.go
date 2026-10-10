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

func TestParserCompositionObjectFlags(t *testing.T) {
	// Construct PCS with 2 composition objects:
	// Object 1: forced-on (0x40), not cropped -> 8 bytes (no crop data)
	// Object 2: cropped (0x80) -> 16 bytes (has crop data)
	pcsData := make([]byte, 11)
	binary.BigEndian.PutUint16(pcsData[0:2], 1920)
	binary.BigEndian.PutUint16(pcsData[2:4], 1080)
	pcsData[4] = 24
	binary.BigEndian.PutUint16(pcsData[5:7], 1)
	pcsData[7] = 0x80 // Epoch start
	pcsData[8] = 0
	pcsData[9] = 1
	pcsData[10] = 2 // 2 objects

	// Object 1: ID=1, Win=0, Flags=0x40 (forced on, NOT cropped), X=100, Y=200
	obj1 := make([]byte, 8)
	binary.BigEndian.PutUint16(obj1[0:2], 1)
	obj1[2] = 0
	obj1[3] = 0x40 // forced-on flag
	binary.BigEndian.PutUint16(obj1[4:6], 100)
	binary.BigEndian.PutUint16(obj1[6:8], 200)

	// Object 2: ID=2, Win=0, Flags=0x80 (cropped), X=300, Y=400, Crop: (10, 20, 30, 40)
	obj2 := make([]byte, 16)
	binary.BigEndian.PutUint16(obj2[0:2], 2)
	obj2[2] = 0
	obj2[3] = 0x80 // cropped flag
	binary.BigEndian.PutUint16(obj2[4:6], 300)
	binary.BigEndian.PutUint16(obj2[6:8], 400)
	binary.BigEndian.PutUint16(obj2[8:10], 10)
	binary.BigEndian.PutUint16(obj2[10:12], 20)
	binary.BigEndian.PutUint16(obj2[12:14], 30)
	binary.BigEndian.PutUint16(obj2[14:16], 40)

	pcsData = append(pcsData, obj1...)
	pcsData = append(pcsData, obj2...)

	var stream []byte
	stream = append(stream, buildSegment(SegmentTypePCS, pcsData)...)
	stream = append(stream, buildSegment(SegmentTypeEND, nil)...)

	sets, err := Parse(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}
	pcs := sets[0].PCS
	if len(pcs.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(pcs.Objects))
	}

	o1 := pcs.Objects[0]
	if o1.ObjectID != 1 {
		t.Errorf("obj 1 ID: expected 1, got %d", o1.ObjectID)
	}
	if o1.CroppedFlag {
		t.Errorf("obj 1 CroppedFlag: expected false for flags=0x40, got true")
	}
	if !o1.ForcedOnFlag {
		t.Errorf("obj 1 ForcedOnFlag: expected true for flags=0x40, got false")
	}
	if o1.X != 100 || o1.Y != 200 {
		t.Errorf("obj 1 position: expected (100, 200), got (%d, %d)", o1.X, o1.Y)
	}

	o2 := pcs.Objects[1]
	if o2.ObjectID != 2 {
		t.Errorf("obj 2 ID: expected 2, got %d", o2.ObjectID)
	}
	if !o2.CroppedFlag {
		t.Errorf("obj 2 CroppedFlag: expected true for flags=0x80, got false")
	}
	if o2.ForcedOnFlag {
		t.Errorf("obj 2 ForcedOnFlag: expected false for flags=0x80, got true")
	}
	if o2.X != 300 || o2.Y != 400 {
		t.Errorf("obj 2 position: expected (300, 400), got (%d, %d)", o2.X, o2.Y)
	}
	if o2.CropX != 10 || o2.CropY != 20 || o2.CropWidth != 30 || o2.CropHeight != 40 {
		t.Errorf("obj 2 crop: expected (10, 20, 30, 40), got (%d, %d, %d, %d)", o2.CropX, o2.CropY, o2.CropWidth, o2.CropHeight)
	}
}


